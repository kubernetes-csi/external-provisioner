/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/go-logr/logr"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/kubernetes-csi/csi-lib-utils/accessmodes"
	"github.com/kubernetes-csi/external-provisioner/v5/pkg/features"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	v1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	_ "k8s.io/apimachinery/pkg/util/json"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	storagelistersv1 "k8s.io/client-go/listers/storage/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	"sigs.k8s.io/sig-storage-lib-external-provisioner/v13/controller"
	"sigs.k8s.io/sig-storage-lib-external-provisioner/v13/util"

	"github.com/kubernetes-csi/csi-lib-utils/connection"
	"github.com/kubernetes-csi/csi-lib-utils/metrics"
	"github.com/kubernetes-csi/csi-lib-utils/rpc"
	snapapi "github.com/kubernetes-csi/external-snapshotter/client/v8/apis/volumesnapshot/v1"
	snapclientset "github.com/kubernetes-csi/external-snapshotter/client/v8/clientset/versioned"
	referenceGrantv1beta1 "sigs.k8s.io/gateway-api/pkg/client/listers/apis/v1beta1"
)

// secretParamsMap provides a mapping of current as well as deprecated secret keys
type secretParamsMap struct {
	name                         string
	deprecatedSecretNameKey      string
	deprecatedSecretNamespaceKey string
	secretNameKey                string
	secretNamespaceKey           string
}

const (
	// CSI Parameters prefixed with csiParameterPrefix are not passed through
	// to the driver on CreateVolumeRequest calls. Instead they are intended
	// to used by the CSI external-provisioner and maybe used to populate
	// fields in subsequent CSI calls or Kubernetes API objects.
	csiParameterPrefix = "csi.storage.k8s.io/"

	prefixedFsTypeKey = csiParameterPrefix + "fstype"

	prefixedDefaultSecretNameKey      = csiParameterPrefix + "secret-name"
	prefixedDefaultSecretNamespaceKey = csiParameterPrefix + "secret-namespace"

	prefixedProvisionerSecretNameKey      = csiParameterPrefix + "provisioner-secret-name"
	prefixedProvisionerSecretNamespaceKey = csiParameterPrefix + "provisioner-secret-namespace"

	prefixedControllerPublishSecretNameKey      = csiParameterPrefix + "controller-publish-secret-name"
	prefixedControllerPublishSecretNamespaceKey = csiParameterPrefix + "controller-publish-secret-namespace"

	prefixedNodeStageSecretNameKey      = csiParameterPrefix + "node-stage-secret-name"
	prefixedNodeStageSecretNamespaceKey = csiParameterPrefix + "node-stage-secret-namespace"

	prefixedNodePublishSecretNameKey      = csiParameterPrefix + "node-publish-secret-name"
	prefixedNodePublishSecretNamespaceKey = csiParameterPrefix + "node-publish-secret-namespace"

	prefixedControllerExpandSecretNameKey      = csiParameterPrefix + "controller-expand-secret-name"
	prefixedControllerExpandSecretNamespaceKey = csiParameterPrefix + "controller-expand-secret-namespace"

	prefixedNodeExpandSecretNameKey      = csiParameterPrefix + "node-expand-secret-name"
	prefixedNodeExpandSecretNamespaceKey = csiParameterPrefix + "node-expand-secret-namespace"

	// [Deprecated] CSI Parameters that are put into fields but
	// NOT stripped from the parameters passed to CreateVolume
	provisionerSecretNameKey      = "csiProvisionerSecretName"
	provisionerSecretNamespaceKey = "csiProvisionerSecretNamespace"

	controllerPublishSecretNameKey      = "csiControllerPublishSecretName"
	controllerPublishSecretNamespaceKey = "csiControllerPublishSecretNamespace"

	nodeStageSecretNameKey      = "csiNodeStageSecretName"
	nodeStageSecretNamespaceKey = "csiNodeStageSecretNamespace"

	nodePublishSecretNameKey      = "csiNodePublishSecretName"
	nodePublishSecretNamespaceKey = "csiNodePublishSecretNamespace"

	// PV and PVC metadata, used for sending to drivers in the  create requests, added as parameters, optional.
	pvcNameKey      = "csi.storage.k8s.io/pvc/name"
	pvcNamespaceKey = "csi.storage.k8s.io/pvc/namespace"
	pvNameKey       = "csi.storage.k8s.io/pv/name"

	snapshotKind     = "VolumeSnapshot"
	snapshotAPIGroup = snapapi.GroupName       // "snapshot.storage.k8s.io"
	pvcKind          = "PersistentVolumeClaim" // Native types don't require an API group

	tokenPVNameKey       = "pv.name"
	tokenPVCNameKey      = "pvc.name"
	tokenPVCNameSpaceKey = "pvc.namespace"

	ResyncPeriodOfCsiNodeInformer        = 1 * time.Hour
	ResyncPeriodOfReferenceGrantInformer = 1 * time.Hour

	deleteVolumeRetryCount = 5

	annMigratedTo = "pv.kubernetes.io/migrated-to"
	// TODO: Beta will be deprecated and removed in a later release
	annBetaStorageProvisioner = "volume.beta.kubernetes.io/storage-provisioner"
	annStorageProvisioner     = "volume.kubernetes.io/storage-provisioner"
	annSelectedNode           = "volume.kubernetes.io/selected-node"

	// Annotation for secret name and namespace will be added to the pv object
	// and used at pvc deletion time.
	annDeletionProvisionerSecretRefName      = "volume.kubernetes.io/provisioner-deletion-secret-name"
	annDeletionProvisionerSecretRefNamespace = "volume.kubernetes.io/provisioner-deletion-secret-namespace"

	snapshotNotBound = "snapshot %s not bound"

	pvcCloneFinalizer = "provisioner.storage.kubernetes.io/cloning-protection"
	// snapshotSourceProtectionFinalizer is managed by the external-provisioner to track
	// in-progress provisioning operations. It's distinct from the external-snapshotter's own
	// "volumesnapshot-as-source-protection" finalizer, which will be deprecated and removed in a future release.
	snapshotSourceProtectionFinalizer = "provisioner.storage.kubernetes.io/volumesnapshot-as-source-protection"

	annAllowVolumeModeChange = "snapshot.storage.kubernetes.io/allow-volume-mode-change"
)

var (
	defaultSecretParams = secretParamsMap{
		name:               "Default",
		secretNameKey:      prefixedDefaultSecretNameKey,
		secretNamespaceKey: prefixedDefaultSecretNamespaceKey,
	}

	provisionerSecretParams = secretParamsMap{
		name:                         "Provisioner",
		deprecatedSecretNameKey:      provisionerSecretNameKey,
		deprecatedSecretNamespaceKey: provisionerSecretNamespaceKey,
		secretNameKey:                prefixedProvisionerSecretNameKey,
		secretNamespaceKey:           prefixedProvisionerSecretNamespaceKey,
	}

	nodePublishSecretParams = secretParamsMap{
		name:                         "NodePublish",
		deprecatedSecretNameKey:      nodePublishSecretNameKey,
		deprecatedSecretNamespaceKey: nodePublishSecretNamespaceKey,
		secretNameKey:                prefixedNodePublishSecretNameKey,
		secretNamespaceKey:           prefixedNodePublishSecretNamespaceKey,
	}

	controllerPublishSecretParams = secretParamsMap{
		name:                         "ControllerPublish",
		deprecatedSecretNameKey:      controllerPublishSecretNameKey,
		deprecatedSecretNamespaceKey: controllerPublishSecretNamespaceKey,
		secretNameKey:                prefixedControllerPublishSecretNameKey,
		secretNamespaceKey:           prefixedControllerPublishSecretNamespaceKey,
	}

	nodeStageSecretParams = secretParamsMap{
		name:                         "NodeStage",
		deprecatedSecretNameKey:      nodeStageSecretNameKey,
		deprecatedSecretNamespaceKey: nodeStageSecretNamespaceKey,
		secretNameKey:                prefixedNodeStageSecretNameKey,
		secretNamespaceKey:           prefixedNodeStageSecretNamespaceKey,
	}

	controllerExpandSecretParams = secretParamsMap{
		name:               "ControllerExpand",
		secretNameKey:      prefixedControllerExpandSecretNameKey,
		secretNamespaceKey: prefixedControllerExpandSecretNamespaceKey,
	}

	nodeExpandSecretParams = secretParamsMap{
		name:               "NodeExpand",
		secretNameKey:      prefixedNodeExpandSecretNameKey,
		secretNamespaceKey: prefixedNodeExpandSecretNamespaceKey,
	}
)

// ProvisionerCSITranslator contains the set of CSI Translation functionality
// required by the provisioner
type ProvisionerCSITranslator interface {
	TranslateInTreeStorageClassToCSI(logger logr.Logger, inTreePluginName string, sc *storagev1.StorageClass) (*storagev1.StorageClass, error)
	TranslateCSIPVToInTree(pv *v1.PersistentVolume) (*v1.PersistentVolume, error)
	IsPVMigratable(pv *v1.PersistentVolume) bool
	TranslateInTreePVToCSI(logger logr.Logger, pv *v1.PersistentVolume) (*v1.PersistentVolume, error)

	IsMigratedCSIDriverByName(csiPluginName string) bool
	GetInTreeNameFromCSIName(pluginName string) (string, error)
}

// requiredCapabilities provides a set of extra capabilities required for special/optional features provided by a plugin
type requiredCapabilities struct {
	snapshot     bool
	clone        bool
	modifyVolume bool
}

// NodeDeployment contains additional parameters for running external-provisioner alongside a
// CSI driver on one or more nodes in the cluster.
type NodeDeployment struct {
	// NodeName is the name of the node in Kubernetes on which the external-provisioner runs.
	NodeName string
	// ClaimInformer is needed to detect when some other external-provisioner
	// became the owner of a PVC while the local one is still waiting before
	// trying to become the owner itself.
	ClaimInformer coreinformers.PersistentVolumeClaimInformer
	// NodeInfo is the result of NodeGetInfo. It is need to determine which
	// PVs were created for the node.
	NodeInfo *csi.NodeGetInfoResponse
	// ImmediateBinding enables support for PVCs with immediate binding.
	ImmediateBinding bool
	// BaseDelay is the initial time that the external-provisioner waits
	// before trying to become the owner of a PVC with immediate binding.
	BaseDelay time.Duration
	// MaxDelay is the maximum for the initial wait time.
	MaxDelay time.Duration
}

type internalNodeDeployment struct {
	NodeDeployment
	rateLimiter workqueue.TypedRateLimiter[any]
}

type csiProvisioner struct {
	client                                kubernetes.Interface
	csiClient                             csi.ControllerClient
	grpcClient                            *grpc.ClientConn
	snapshotClient                        snapclientset.Interface
	timeout                               time.Duration
	identity                              string
	volumeNamePrefix                      string
	defaultFSType                         string
	volumeNameUUIDLength                  int
	driverName                            string
	pluginCapabilities                    rpc.PluginCapabilitySet
	controllerCapabilities                rpc.ControllerCapabilitySet
	supportsMigrationFromInTreePluginName string
	strictTopology                        bool
	immediateTopology                     bool
	translator                            ProvisionerCSITranslator
	scLister                              storagelistersv1.StorageClassLister
	csiNodeLister                         storagelistersv1.CSINodeLister
	nodeLister                            corelisters.NodeLister
	claimLister                           corelisters.PersistentVolumeClaimLister
	vaLister                              storagelistersv1.VolumeAttachmentLister
	referenceGrantLister                  referenceGrantv1beta1.ReferenceGrantLister
	extraCreateMetadata                   bool
	eventRecorder                         record.EventRecorder
	nodeDeployment                        *internalNodeDeployment
	controllerPublishReadOnly             bool
	preventVolumeModeConversion           bool
	pvcNodeStore                          TopologyProvider
}

var (
	_ controller.Provisioner      = &csiProvisioner{}
	_ controller.BlockProvisioner = &csiProvisioner{}
	_ controller.Qualifier        = &csiProvisioner{}
)

// Each provisioner have a identify string to distinguish with others. This
// identify string will be added in PV annotations under this key.
var provisionerIDKey = "storage.kubernetes.io/csiProvisionerIdentity"

func Connect(ctx context.Context, address string, metricsManager metrics.CSIMetricsManager) (*grpc.ClientConn, error) {
	return connection.Connect(ctx, address, metricsManager, connection.OnConnectionLoss(connection.ExitOnConnectionLoss()))
}

func Probe(ctx context.Context, conn *grpc.ClientConn, singleCallTimeout time.Duration) error {
	return rpc.ProbeForever(ctx, conn, singleCallTimeout)
}

func GetDriverName(conn *grpc.ClientConn, timeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return rpc.GetDriverName(ctx, conn)
}

func GetDriverCapabilities(conn *grpc.ClientConn, timeout time.Duration) (rpc.PluginCapabilitySet, rpc.ControllerCapabilitySet, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	pluginCapabilities, err := rpc.GetPluginCapabilities(ctx, conn)
	if err != nil {
		return nil, nil, err
	}

	/* Each CSI operation gets its own timeout / context */
	ctx, cancel = context.WithTimeout(context.Background(), timeout)
	defer cancel()
	controllerCapabilities, err := rpc.GetControllerCapabilities(ctx, conn)
	if err != nil {
		return nil, nil, err
	}

	return pluginCapabilities, controllerCapabilities, nil
}

func GetNodeInfo(conn *grpc.ClientConn, timeout time.Duration) (*csi.NodeGetInfoResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	client := csi.NewNodeClient(conn)
	return client.NodeGetInfo(ctx, &csi.NodeGetInfoRequest{})
}

// NewCSIProvisioner creates new CSI provisioner.
//
// vaLister is optional and only needed when VolumeAttachments are
// meant to be checked before deleting a volume.
func NewCSIProvisioner(client kubernetes.Interface,
	connectionTimeout time.Duration,
	identity string,
	volumeNamePrefix string,
	volumeNameUUIDLength int,
	grpcClient *grpc.ClientConn,
	snapshotClient snapclientset.Interface,
	driverName string,
	pluginCapabilities rpc.PluginCapabilitySet,
	controllerCapabilities rpc.ControllerCapabilitySet,
	supportsMigrationFromInTreePluginName string,
	strictTopology bool,
	immediateTopology bool,
	translator ProvisionerCSITranslator,
	scLister storagelistersv1.StorageClassLister,
	csiNodeLister storagelistersv1.CSINodeLister,
	nodeLister corelisters.NodeLister,
	claimLister corelisters.PersistentVolumeClaimLister,
	vaLister storagelistersv1.VolumeAttachmentLister,
	referenceGrantLister referenceGrantv1beta1.ReferenceGrantLister,
	extraCreateMetadata bool,
	defaultFSType string,
	nodeDeployment *NodeDeployment,
	controllerPublishReadOnly bool,
	preventVolumeModeConversion bool,
	pvcNodeStore TopologyProvider,
) controller.Provisioner {
	broadcaster := record.NewBroadcaster()
	broadcaster.StartLogging(klog.Infof)
	broadcaster.StartRecordingToSink(&corev1.EventSinkImpl{Interface: client.CoreV1().Events(v1.NamespaceAll)})
	eventRecorder := broadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: "external-provisioner"})

	csiClient := csi.NewControllerClient(grpcClient)

	provisioner := &csiProvisioner{
		client:                                client,
		grpcClient:                            grpcClient,
		csiClient:                             csiClient,
		snapshotClient:                        snapshotClient,
		timeout:                               connectionTimeout,
		identity:                              identity,
		volumeNamePrefix:                      volumeNamePrefix,
		defaultFSType:                         defaultFSType,
		volumeNameUUIDLength:                  volumeNameUUIDLength,
		driverName:                            driverName,
		pluginCapabilities:                    pluginCapabilities,
		controllerCapabilities:                controllerCapabilities,
		supportsMigrationFromInTreePluginName: supportsMigrationFromInTreePluginName,
		strictTopology:                        strictTopology,
		immediateTopology:                     immediateTopology,
		translator:                            translator,
		scLister:                              scLister,
		csiNodeLister:                         csiNodeLister,
		nodeLister:                            nodeLister,
		claimLister:                           claimLister,
		vaLister:                              vaLister,
		referenceGrantLister:                  referenceGrantLister,
		extraCreateMetadata:                   extraCreateMetadata,
		eventRecorder:                         eventRecorder,
		controllerPublishReadOnly:             controllerPublishReadOnly,
		preventVolumeModeConversion:           preventVolumeModeConversion,
		pvcNodeStore:                          pvcNodeStore,
	}
	if nodeDeployment != nil {
		provisioner.nodeDeployment = &internalNodeDeployment{
			NodeDeployment: *nodeDeployment,
			rateLimiter:    newItemExponentialFailureRateLimiterWithJitter(nodeDeployment.BaseDelay, nodeDeployment.MaxDelay),
		}
		// Remove deleted PVCs from rate limiter.
		claimHandler := cache.ResourceEventHandlerFuncs{
			DeleteFunc: func(obj any) {
				if unknown, ok := obj.(cache.DeletedFinalStateUnknown); ok && unknown.Obj != nil {
					obj = unknown.Obj
				}
				if claim, ok := obj.(*v1.PersistentVolumeClaim); ok {
					provisioner.nodeDeployment.rateLimiter.Forget(claim.UID)
				}
			},
		}
		provisioner.nodeDeployment.ClaimInformer.Informer().AddEventHandler(claimHandler)
	}

	return provisioner
}

// This function get called before any attempt to communicate with the driver.
// Before initiating Create/Delete API calls provisioner checks if Capabilities:
// PluginControllerService,  ControllerCreateVolume sre supported and gets the  driver name.
func (p *csiProvisioner) checkDriverCapabilities(rc *requiredCapabilities) error {
	if !p.pluginCapabilities[csi.PluginCapability_Service_CONTROLLER_SERVICE] {
		return fmt.Errorf("CSI driver does not support dynamic provisioning: plugin CONTROLLER_SERVICE capability is not reported")
	}

	if !p.controllerCapabilities[csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME] {
		return fmt.Errorf("CSI driver does not support dynamic provisioning: controller CREATE_DELETE_VOLUME capability is not reported")
	}

	if rc.snapshot {
		// Check whether plugin supports create snapshot
		// If not, create volume from snapshot cannot proceed
		if !p.controllerCapabilities[csi.ControllerServiceCapability_RPC_CREATE_DELETE_SNAPSHOT] {
			return fmt.Errorf("CSI driver does not support snapshot restore: controller CREATE_DELETE_SNAPSHOT capability is not reported")
		}
	}
	if rc.clone {
		// Check whether plugin supports clone operations
		// If not, create volume from pvc cannot proceed
		if !p.controllerCapabilities[csi.ControllerServiceCapability_RPC_CLONE_VOLUME] {
			return fmt.Errorf("CSI driver does not support clone operations: controller CLONE_VOLUME capability is not reported")
		}
	}
	if rc.modifyVolume {
		// Check whether plugin supports modifying volumes
		// If not, PVCs with an associated VolumeAttributesClass cannot be created
		if !p.controllerCapabilities[csi.ControllerServiceCapability_RPC_MODIFY_VOLUME] {
			return fmt.Errorf("CSI driver does not support VolumeAttributesClass: controller MODIFY_VOLUME capability is not reported")
		}
	}

	return nil
}

func makeVolumeName(prefix, pvcUID string, volumeNameUUIDLength int) (string, error) {
	// create persistent name based on a volumeNamePrefix and volumeNameUUIDLength
	// of PVC's UID
	if len(prefix) == 0 {
		return "", fmt.Errorf("volume name prefix cannot be of length 0")
	}
	if len(pvcUID) == 0 {
		return "", fmt.Errorf("corrupted PVC object, it is missing UID")
	}
	if volumeNameUUIDLength == -1 {
		// Default behavior is to not truncate or remove dashes
		return fmt.Sprintf("%s-%s", prefix, pvcUID), nil
	}
	// Else we remove all dashes from UUID and truncate to volumeNameUUIDLength
	return fmt.Sprintf("%s-%s", prefix, strings.Replace(string(pvcUID), "-", "", -1)[0:volumeNameUUIDLength]), nil
}

func getAccessTypeBlock() *csi.VolumeCapability_Block {
	return &csi.VolumeCapability_Block{
		Block: &csi.VolumeCapability_BlockVolume{},
	}
}

func getAccessTypeMount(fsType string, mountFlags []string) *csi.VolumeCapability_Mount {
	return &csi.VolumeCapability_Mount{
		Mount: &csi.VolumeCapability_MountVolume{
			FsType:     fsType,
			MountFlags: mountFlags,
		},
	}
}

func getVolumeCapability(
	claim *v1.PersistentVolumeClaim,
	sc *storagev1.StorageClass,
	pvcAccessMode v1.PersistentVolumeAccessMode,
	fsType string,
	supportsSingleNodeMultiWriter bool,
) (*csi.VolumeCapability, error) {
	accessMode, err := accessmodes.ToCSIAccessMode([]v1.PersistentVolumeAccessMode{pvcAccessMode}, supportsSingleNodeMultiWriter)
	if err != nil {
		return nil, err
	}

	if util.CheckPersistentVolumeClaimModeBlock(claim) {
		return &csi.VolumeCapability{
			AccessType: getAccessTypeBlock(),
			AccessMode: &csi.VolumeCapability_AccessMode{
				Mode: accessMode,
			},
		}, nil
	}
	return &csi.VolumeCapability{
		AccessType: getAccessTypeMount(fsType, sc.MountOptions),
		AccessMode: &csi.VolumeCapability_AccessMode{
			Mode: accessMode,
		},
	}, nil
}

func (p *csiProvisioner) getVolumeCapabilities(
	claim *v1.PersistentVolumeClaim,
	sc *storagev1.StorageClass,
	fsType string,
) ([]*csi.VolumeCapability, error) {
	supportsSingleNodeMultiWriter := false
	if p.controllerCapabilities[csi.ControllerServiceCapability_RPC_SINGLE_NODE_MULTI_WRITER] {
		supportsSingleNodeMultiWriter = true
	}

	volumeCaps := make([]*csi.VolumeCapability, 0)
	for _, pvcAccessMode := range claim.Spec.AccessModes {
		volumeCap, err := getVolumeCapability(claim, sc, pvcAccessMode, fsType, supportsSingleNodeMultiWriter)
		if err != nil {
			return []*csi.VolumeCapability{}, err
		}
		volumeCaps = append(volumeCaps, volumeCap)
	}
	return volumeCaps, nil
}

type deletionSecretParams struct {
	name      string
	namespace string
}

type prepareProvisionResult struct {
	fsType              string
	migratedVolume      bool
	req                 *csi.CreateVolumeRequest
	csiPVSource         *v1.CSIPersistentVolumeSource
	provDeletionSecrets *deletionSecretParams
}

// prepareProvision does non-destructive parameter checking and preparations for provisioning a volume.
func (p *csiProvisioner) prepareProvision(ctx context.Context, claim *v1.PersistentVolumeClaim, sc *storagev1.StorageClass, selectedNodeName string) (*prepareProvisionResult, controller.ProvisioningState, error) {
	if sc == nil {
		return nil, controller.ProvisioningFinished, errors.New("storage class was nil")
	}

	// normalize dataSource and dataSourceRef.
	dataSource, err := p.dataSource(ctx, claim)
	if err != nil {
		return nil, controller.ProvisioningFinished, err
	}

	migratedVolume := false
	if p.supportsMigrationFromInTreePluginName != "" {
		// NOTE: we cannot depend on PVC.Annotations[volume.beta.kubernetes.io/storage-provisioner] to get
		// the in-tree provisioner name in case of CSI migration scenarios. The annotation will be
		// set to the CSI provisioner name by PV controller for migration scenarios
		// so that external provisioner can correctly pick up the PVC pointing to an in-tree plugin
		if sc.Provisioner == p.supportsMigrationFromInTreePluginName {
			klog.V(2).Infof("translating storage class for in-tree plugin %s to CSI", sc.Provisioner)

			// TODO replace klog.TODO() once contextual logging is implemented for provisioner
			storageClass, err := p.translator.TranslateInTreeStorageClassToCSI(klog.TODO(), p.supportsMigrationFromInTreePluginName, sc)
			if err != nil {
				return nil, controller.ProvisioningFinished, fmt.Errorf("failed to translate storage class: %v", err)
			}
			sc = storageClass
			migratedVolume = true
		} else {
			klog.V(4).Infof("skip translation of storage class for plugin: %s", sc.Provisioner)
		}
	}

	// Make sure the plugin is capable of fulfilling the requested options
	rc := &requiredCapabilities{}
	if dataSource != nil {
		// PVC.Spec.DataSource.Name is the name of the VolumeSnapshot API object
		if dataSource.Name == "" {
			return nil, controller.ProvisioningFinished, fmt.Errorf("the PVC source not found for PVC %s", claim.Name)
		}

		switch dataSource.Kind {
		case snapshotKind:
			if dataSource.APIVersion != snapshotAPIGroup {
				return nil, controller.ProvisioningFinished, fmt.Errorf("the PVC source does not belong to the right APIGroup. Expected %s, Got %s", snapshotAPIGroup, dataSource.APIVersion)
			}
			rc.snapshot = true
		case pvcKind:
			rc.clone = true
		default:
			// DataSource is not VolumeSnapshot and PVC
			// Assume external data populator to create the volume, and there is no more work for us to do
			p.eventRecorder.Event(claim, v1.EventTypeNormal, "Provisioning", "Assuming an external populator will provision the volume")
			return nil, controller.ProvisioningFinished, &controller.IgnoredError{
				Reason: fmt.Sprintf("data source (%s) is not handled by the provisioner, assuming an external populator will provision it",
					dataSource.Kind),
			}
		}
	}

	var vacName string
	if utilfeature.DefaultFeatureGate.Enabled(features.VolumeAttributesClass) {
		if claim.Spec.VolumeAttributesClassName != nil {
			vacName = *claim.Spec.VolumeAttributesClassName
		}
	}

	if vacName != "" {
		rc.modifyVolume = true
	}

	if err := p.checkDriverCapabilities(rc); err != nil {
		return nil, controller.ProvisioningFinished, err
	}

	if claim.Spec.Selector != nil {
		return nil, controller.ProvisioningFinished, fmt.Errorf("claim Selector is not supported")
	}

	pvName, err := makeVolumeName(p.volumeNamePrefix, string(claim.ObjectMeta.UID), p.volumeNameUUIDLength)
	if err != nil {
		return nil, controller.ProvisioningFinished, err
	}

	fsTypesFound := 0
	fsType := ""
	for k, v := range sc.Parameters {
		if strings.ToLower(k) == "fstype" || k == prefixedFsTypeKey {
			fsType = v
			fsTypesFound++
		}
		if strings.ToLower(k) == "fstype" {
			klog.Warning(deprecationWarning("fstype", prefixedFsTypeKey, ""))
		}
	}
	if fsTypesFound > 1 {
		return nil, controller.ProvisioningFinished, fmt.Errorf("fstype specified in parameters with both \"fstype\" and \"%s\" keys", prefixedFsTypeKey)
	}
	if fsType == "" && p.defaultFSType != "" {
		fsType = p.defaultFSType
	}

	capacity := claim.Spec.Resources.Requests[v1.ResourceName(v1.ResourceStorage)]
	volSizeBytes := capacity.Value()

	volumeCaps, err := p.getVolumeCapabilities(claim, sc, fsType)
	if err != nil {
		return nil, controller.ProvisioningFinished, err
	}

	// Create a CSI CreateVolumeRequest and Response
	req := csi.CreateVolumeRequest{
		Name:               pvName,
		Parameters:         sc.Parameters,
		VolumeCapabilities: volumeCaps,
		CapacityRange: &csi.CapacityRange{
			RequiredBytes: int64(volSizeBytes),
		},
	}

	if dataSource != nil && (rc.clone || rc.snapshot) {
		volumeContentSource, err := p.getVolumeContentSource(ctx, claim, sc, dataSource)
		if err != nil {
			return nil, controller.ProvisioningNoChange, fmt.Errorf("error getting handle for DataSource Type %s by Name %s: %v", dataSource.Kind, dataSource.Name, err)
		}
		req.VolumeContentSource = volumeContentSource
	}

	if dataSource != nil && rc.clone {
		err = p.setCloneFinalizer(ctx, dataSource)
		if err != nil {
			return nil, controller.ProvisioningNoChange, err
		}
	}

	if dataSource != nil && rc.snapshot {
		err = p.setSnapshotFinalizer(ctx, dataSource)
		if err != nil {
			return nil, controller.ProvisioningNoChange, err
		}
	}

	if p.supportsTopology() {
		requirements, err := GenerateAccessibilityRequirements(
			p.client,
			p.driverName,
			claim.UID,
			claim.Name,
			sc.AllowedTopologies,
			selectedNodeName,
			p.strictTopology,
			p.immediateTopology,
			p.csiNodeLister,
			p.nodeLister,
			p.pvcNodeStore)
		if apierrors.IsNotFound(err) {
			// The node or CSINode object can't be found, ask the scheduler for a reschedule
			return nil, controller.ProvisioningReschedule, err
		} else if err != nil {
			return nil, controller.ProvisioningNoChange, fmt.Errorf("error generating accessibility requirements: %v", err)
		}
		req.AccessibilityRequirements = requirements
	}

	// Resolve provision secret credentials.
	provisionerSecretRef, err := getSecretReference(provisionerSecretParams, sc.Parameters, pvName, claim)
	if err != nil {
		return nil, controller.ProvisioningNoChange, err
	}
	provisionerCredentials, err := getCredentials(ctx, p.client, provisionerSecretRef)
	if err != nil {
		return nil, controller.ProvisioningNoChange, err
	}
	req.Secrets = provisionerCredentials

	// Resolve controller publish, node stage, node publish secret references
	controllerPublishSecretRef, err := getSecretReference(controllerPublishSecretParams, sc.Parameters, pvName, claim)
	if err != nil {
		return nil, controller.ProvisioningNoChange, err
	}
	nodeStageSecretRef, err := getSecretReference(nodeStageSecretParams, sc.Parameters, pvName, claim)
	if err != nil {
		return nil, controller.ProvisioningNoChange, err
	}
	nodePublishSecretRef, err := getSecretReference(nodePublishSecretParams, sc.Parameters, pvName, claim)
	if err != nil {
		return nil, controller.ProvisioningNoChange, err
	}
	controllerExpandSecretRef, err := getSecretReference(controllerExpandSecretParams, sc.Parameters, pvName, claim)
	if err != nil {
		return nil, controller.ProvisioningNoChange, err
	}
	nodeExpandSecretRef, err := getSecretReference(nodeExpandSecretParams, sc.Parameters, pvName, claim)
	if err != nil {
		return nil, controller.ProvisioningNoChange, err
	}
	csiPVSource := &v1.CSIPersistentVolumeSource{
		Driver: p.driverName,
		// VolumeHandle and VolumeAttributes will be added after provisioning.
		ControllerPublishSecretRef: controllerPublishSecretRef,
		NodeStageSecretRef:         nodeStageSecretRef,
		NodePublishSecretRef:       nodePublishSecretRef,
		ControllerExpandSecretRef:  controllerExpandSecretRef,
		NodeExpandSecretRef:        nodeExpandSecretRef,
	}

	req.Parameters, err = removePrefixedParameters(sc.Parameters)
	if err != nil {
		return nil, controller.ProvisioningFinished, fmt.Errorf("failed to strip CSI Parameters of prefixed keys: %v", err)
	}

	if p.extraCreateMetadata {
		// add pvc and pv metadata to request for use by the plugin
		req.Parameters[pvcNameKey] = claim.GetName()
		req.Parameters[pvcNamespaceKey] = claim.GetNamespace()
		req.Parameters[pvNameKey] = pvName
	}
	deletionAnnSecrets := new(deletionSecretParams)

	if provisionerSecretRef != nil {
		deletionAnnSecrets.name = provisionerSecretRef.Name
		deletionAnnSecrets.namespace = provisionerSecretRef.Namespace
	}

	if vacName != "" {
		vac, err := p.client.StorageV1().VolumeAttributesClasses().Get(ctx, vacName, metav1.GetOptions{})
		if err != nil {
			return nil, controller.ProvisioningNoChange, err
		}

		if vac.DriverName != p.driverName {
			return nil, controller.ProvisioningFinished, fmt.Errorf("VAC %s referenced in PVC is for driver %s which does not match driver name %s", vacName, vac.DriverName, p.driverName)
		}

		req.MutableParameters = vac.Parameters
	}

	return &prepareProvisionResult{
		fsType:              fsType,
		migratedVolume:      migratedVolume,
		req:                 &req,
		csiPVSource:         csiPVSource,
		provDeletionSecrets: deletionAnnSecrets,
	}, controller.ProvisioningNoChange, nil

}

func (p *csiProvisioner) Provision(ctx context.Context, options controller.ProvisionOptions) (*v1.PersistentVolume, controller.ProvisioningState, error) {
	claim := options.PVC
	provisioner, ok := claim.Annotations[annStorageProvisioner]
	if !ok {
		provisioner = claim.Annotations[annBetaStorageProvisioner]
	}
	if provisioner != p.driverName && claim.Annotations[annMigratedTo] != p.driverName {
		// The storage provisioner annotation may not equal driver name but the
		// PVC could have annotation "migrated-to" which is the new way to
		// signal a PVC is migrated (k8s v1.17+)
		return nil, controller.ProvisioningFinished, &controller.IgnoredError{
			Reason: fmt.Sprintf("PVC annotated with external-provisioner name %s does not match provisioner driver name %s. This could mean the PVC is not migrated",
				provisioner,
				p.driverName),
		}
	}

	// The same check already ran in ShouldProvision, but perhaps
	// it couldn't complete due to some unexpected error.
	owned, err := p.checkNode(ctx, claim, options.StorageClass, "provision")
	if err != nil {
		return nil, controller.ProvisioningNoChange,
			fmt.Errorf("node check failed: %v", err)
	}
	if !owned {
		return nil, controller.ProvisioningNoChange, &controller.IgnoredError{
			Reason: fmt.Sprintf("not responsible for provisioning of PVC %s/%s because it is not assigned to node %q", claim.Namespace, claim.Name, p.nodeDeployment.NodeName),
		}
	}

	result, state, err := p.prepareProvision(ctx, claim, options.StorageClass, options.SelectedNodeName)
	if result == nil {
		return nil, state, err
	}
	req := result.req
	volSizeBytes := req.CapacityRange.RequiredBytes
	pvName := req.Name
	provisionerCredentials := req.Secrets

	createCtx := markAsMigrated(ctx, result.migratedVolume)
	createCtx, cancel := context.WithTimeout(createCtx, p.timeout)
	defer cancel()
	rep, err := p.csiClient.CreateVolume(createCtx, req)
	if err != nil {
		// Giving up after an error and telling the pod scheduler to retry with a different node
		// only makes sense if:
		// - The CSI driver supports topology: without that, the next CreateVolume call after
		//   rescheduling will be exactly the same.
		// - We are working on a volume with late binding: only in that case will
		//   provisioning be retried if we give up for now.
		// - The error is one where rescheduling is
		//   a) allowed (i.e. we don't have to keep calling CreateVolume because the operation might be running) and
		//   b) it makes sense (typically local resource exhausted).
		//   isFinalError is going to check this.
		//
		// We do this regardless whether the driver has asked for strict topology because
		// even drivers which did not ask for it explicitly might still only look at the first
		// topology entry and thus succeed after rescheduling.
		mayReschedule := p.supportsTopology() &&
			len(options.SelectedNodeName) > 0
		state := checkError(err, mayReschedule)
		klog.V(5).Infof("CreateVolume failed, supports topology = %v, node selected %v => may reschedule = %v => state = %v: %v",
			p.supportsTopology(),
			options.SelectedNodeName,
			mayReschedule,
			state,
			err)
		// Delete the entry in in memory cache if the error is final
		if p.supportsTopology() && (state == controller.ProvisioningFinished || state == controller.ProvisioningReschedule) {
			p.pvcNodeStore.Delete(claim.UID)
		}
		return nil, state, err
	}

	if rep.Volume != nil {
		klog.V(3).Infof("create volume rep: %+v", rep.Volume)
	}
	volumeAttributes := map[string]string{provisionerIDKey: p.identity}
	maps.Copy(volumeAttributes, rep.Volume.VolumeContext)
	respCap := rep.GetVolume().GetCapacityBytes()

	// According to CSI spec CreateVolume should be able to return capacity = 0, which means it is unknown. for example NFS/FTP
	if respCap == 0 {
		respCap = volSizeBytes
		klog.V(3).Infof("csiClient response volume with size 0, which is not supported by apiServer, will use claim size:%d", respCap)
	} else if respCap < volSizeBytes {
		capErr := fmt.Errorf("created volume capacity %v less than requested capacity %v", respCap, volSizeBytes)
		delReq := &csi.DeleteVolumeRequest{
			VolumeId: rep.GetVolume().GetVolumeId(),
		}
		err = cleanupVolume(ctx, p, delReq, provisionerCredentials)
		if err != nil {
			capErr = fmt.Errorf("%v. Cleanup of volume %s failed, volume is orphaned: %v", capErr, pvName, err)
		}
		// use InBackground to retry the call, hoping the volume is deleted correctly next time.
		return nil, controller.ProvisioningInBackground, capErr
	}

	if options.PVC.Spec.DataSource != nil ||
		(utilfeature.DefaultFeatureGate.Enabled(features.CrossNamespaceVolumeDataSource) &&
			options.PVC.Spec.DataSourceRef != nil && options.PVC.Spec.DataSourceRef.Namespace != nil &&
			len(*options.PVC.Spec.DataSourceRef.Namespace) > 0) {
		contentSource := rep.GetVolume().ContentSource
		if contentSource == nil {
			sourceErr := fmt.Errorf("volume content source missing")
			delReq := &csi.DeleteVolumeRequest{
				VolumeId: rep.GetVolume().GetVolumeId(),
			}
			err = cleanupVolume(ctx, p, delReq, provisionerCredentials)
			if err != nil {
				sourceErr = fmt.Errorf("%v. cleanup of volume %s failed, volume is orphaned: %v", sourceErr, pvName, err)
			}
			return nil, controller.ProvisioningInBackground, sourceErr
		}
	}
	pvReadOnly := false
	volCaps := req.GetVolumeCapabilities()
	// if the request only has one accessmode and if its ROX, set readonly to true
	// TODO: check for the driver capability of MULTI_NODE_READER_ONLY capability from the CSI driver
	if len(volCaps) == 1 && volCaps[0].GetAccessMode().GetMode() == csi.VolumeCapability_AccessMode_MULTI_NODE_READER_ONLY && p.controllerPublishReadOnly {
		pvReadOnly = true
	}

	result.csiPVSource.VolumeHandle = p.volumeIdToHandle(rep.Volume.VolumeId)
	result.csiPVSource.VolumeAttributes = volumeAttributes
	result.csiPVSource.ReadOnly = pvReadOnly
	pv := &v1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: pvName,
		},
		Spec: v1.PersistentVolumeSpec{
			AccessModes:  options.PVC.Spec.AccessModes,
			MountOptions: options.StorageClass.MountOptions,
			Capacity: v1.ResourceList{
				v1.ResourceName(v1.ResourceStorage): bytesToQuantity(respCap),
			},
			// TODO wait for CSI VolumeSource API
			PersistentVolumeSource: v1.PersistentVolumeSource{
				CSI: result.csiPVSource,
			},
		},
	}

	// Set annDeletionSecretRefName and namespace in PV object.
	if result.provDeletionSecrets != nil {
		klog.V(5).Infof("createVolumeOperation: set annotation [%s/%s] on pv [%s].", annDeletionProvisionerSecretRefNamespace, annDeletionProvisionerSecretRefName, pv.Name)
		metav1.SetMetaDataAnnotation(&pv.ObjectMeta, annDeletionProvisionerSecretRefName, result.provDeletionSecrets.name)
		metav1.SetMetaDataAnnotation(&pv.ObjectMeta, annDeletionProvisionerSecretRefNamespace, result.provDeletionSecrets.namespace)
	} else {
		metav1.SetMetaDataAnnotation(&pv.ObjectMeta, annDeletionProvisionerSecretRefName, "")
		metav1.SetMetaDataAnnotation(&pv.ObjectMeta, annDeletionProvisionerSecretRefNamespace, "")
	}

	if options.StorageClass.ReclaimPolicy != nil {
		pv.Spec.PersistentVolumeReclaimPolicy = *options.StorageClass.ReclaimPolicy
	}

	if p.supportsTopology() {
		pv.Spec.NodeAffinity = GenerateVolumeNodeAffinity(rep.Volume.AccessibleTopology)
	}

	// Set VolumeMode to PV if it is passed via PVC spec when Block feature is enabled
	if options.PVC.Spec.VolumeMode != nil {
		pv.Spec.VolumeMode = options.PVC.Spec.VolumeMode
	}
	// Set FSType if PV is not Block Volume
	if !util.CheckPersistentVolumeClaimModeBlock(options.PVC) {
		pv.Spec.PersistentVolumeSource.CSI.FSType = result.fsType
	}

	vacName := claim.Spec.VolumeAttributesClassName
	if utilfeature.DefaultFeatureGate.Enabled(features.VolumeAttributesClass) && vacName != nil && *vacName != "" {
		pv.Spec.VolumeAttributesClassName = vacName
	}

	klog.V(2).Infof("successfully created PV %v for PVC %v and csi volume name %v", pv.Name, options.PVC.Name, pv.Spec.CSI.VolumeHandle)

	if result.migratedVolume {
		pv, err = p.translator.TranslateCSIPVToInTree(pv)
		if err != nil {
			klog.Warningf("failed to translate CSI PV to in-tree due to: %v. Deleting provisioned PV", err)
			deleteErr := p.Delete(ctx, pv)
			if deleteErr != nil {
				klog.Warningf("failed to delete partly provisioned PV: %v", deleteErr)
				// Retry the call again to clean up the orphan
				return nil, controller.ProvisioningInBackground, err
			}
			return nil, controller.ProvisioningFinished, err
		}
	}

	klog.V(5).Infof("successfully created PV %+v", pv.Spec.PersistentVolumeSource)
	// Remove entry from the in memory cache
	if p.supportsTopology() {
		p.pvcNodeStore.Delete(claim.UID)
	}

	// Remove snapshot finalizer if this PVC was provisioned from a snapshot
	if claim.Spec.DataSource != nil && claim.Spec.DataSource.Kind == snapshotKind {
		if err := p.removeSnapshotFinalizer(ctx, claim.Namespace, claim.Spec.DataSource.Name); err != nil {
			klog.Warningf("Failed to remove snapshot finalizer from %s/%s: %v", claim.Namespace, claim.Spec.DataSource.Name, err)
			// Don't fail provisioning if we can't remove the finalizer - it will be cleaned up later
		}
	}

	return pv, controller.ProvisioningFinished, nil
}

func (p *csiProvisioner) setCloneFinalizer(ctx context.Context, dataSource *v1.ObjectReference) error {
	claim, err := p.claimLister.PersistentVolumeClaims(dataSource.Namespace).Get(dataSource.Name)
	if err != nil {
		return err
	}

	clone := claim.DeepCopy()
	if !checkFinalizer(clone, pvcCloneFinalizer) {
		clone.Finalizers = append(clone.Finalizers, pvcCloneFinalizer)
		_, err := p.client.CoreV1().PersistentVolumeClaims(clone.Namespace).Update(ctx, clone, metav1.UpdateOptions{})
		return err
	}

	return nil
}

func (p *csiProvisioner) setSnapshotFinalizer(ctx context.Context, dataSource *v1.ObjectReference) error {
	snapshot, err := p.snapshotClient.SnapshotV1().VolumeSnapshots(dataSource.Namespace).Get(ctx, dataSource.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	snapshotClone := snapshot.DeepCopy()
	if !checkFinalizer(snapshotClone, snapshotSourceProtectionFinalizer) {
		snapshotClone.Finalizers = append(snapshotClone.Finalizers, snapshotSourceProtectionFinalizer)
		_, err := p.snapshotClient.SnapshotV1().VolumeSnapshots(snapshotClone.Namespace).Update(ctx, snapshotClone, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
		klog.V(3).Infof("Added finalizer %s to snapshot %s/%s", snapshotSourceProtectionFinalizer, snapshotClone.Namespace, snapshotClone.Name)
	}

	return nil
}

func (p *csiProvisioner) removeSnapshotFinalizer(ctx context.Context, namespace, name string) error {
	snapshot, err := p.snapshotClient.SnapshotV1().VolumeSnapshots(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Snapshot already deleted, nothing to do
			return nil
		}
		return err
	}

	if !checkFinalizer(snapshot, snapshotSourceProtectionFinalizer) {
		// Finalizer not present, nothing to do
		return nil
	}

	// Remove the finalizer
	finalizers := make([]string, 0)
	for _, finalizer := range snapshot.ObjectMeta.Finalizers {
		if finalizer != snapshotSourceProtectionFinalizer {
			finalizers = append(finalizers, finalizer)
		}
	}

	snapshotClone := snapshot.DeepCopy()
	snapshotClone.Finalizers = finalizers
	_, err = p.snapshotClient.SnapshotV1().VolumeSnapshots(snapshotClone.Namespace).Update(ctx, snapshotClone, metav1.UpdateOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Snapshot was deleted while we were trying to update it, that's fine
			return nil
		}
		return err
	}

	klog.V(3).Infof("Removed finalizer %s from snapshot %s/%s", snapshotSourceProtectionFinalizer, namespace, name)
	return nil
}

func (p *csiProvisioner) supportsTopology() bool {
	return SupportsTopology(p.pluginCapabilities)
}

func removePrefixedParameters(param map[string]string) (map[string]string, error) {
	newParam := map[string]string{}
	for k, v := range param {
		if strings.HasPrefix(k, csiParameterPrefix) {
			// Check if its well known
			switch k {
			case prefixedFsTypeKey:
			case prefixedProvisionerSecretNameKey:
			case prefixedProvisionerSecretNamespaceKey:
			case prefixedControllerPublishSecretNameKey:
			case prefixedControllerPublishSecretNamespaceKey:
			case prefixedNodeStageSecretNameKey:
			case prefixedNodeStageSecretNamespaceKey:
			case prefixedNodePublishSecretNameKey:
			case prefixedNodePublishSecretNamespaceKey:
			case prefixedControllerExpandSecretNameKey:
			case prefixedControllerExpandSecretNamespaceKey:
			case prefixedDefaultSecretNameKey:
			case prefixedDefaultSecretNamespaceKey:
			case prefixedNodeExpandSecretNameKey:
			case prefixedNodeExpandSecretNamespaceKey:
			default:
				return map[string]string{}, fmt.Errorf("found unknown parameter key \"%s\" with reserved namespace %s", k, csiParameterPrefix)
			}
		} else {
			// Don't strip, add this key-value to new map
			// Deprecated parameters prefixed with "csi" are not stripped to preserve backwards compatibility
			newParam[k] = v
		}
	}
	return newParam, nil
}

// getVolumeContentSource is a helper function to process provisioning requests that include a DataSource
// currently we provide Snapshot and PVC, the default case allows the provisioner to still create a volume
// so that an external controller can act upon it.   Additional DataSource types can be added here with
// an appropriate implementation function
func (p *csiProvisioner) getVolumeContentSource(ctx context.Context, claim *v1.PersistentVolumeClaim, sc *storagev1.StorageClass, dataSource *v1.ObjectReference) (*csi.VolumeContentSource, error) {
	switch dataSource.Kind {
	case snapshotKind:
		return p.getSnapshotSource(ctx, claim, sc, dataSource)
	case pvcKind:
		return p.getPVCSource(ctx, claim, sc, dataSource)
	default:
		// For now we shouldn't pass other things to this function, but treat it as a noop and extend as needed
		return nil, nil
	}
}

// getPVCSource verifies DataSource.Kind of type PersistentVolumeClaim, making sure that the requested PVC is available/ready
// returns the VolumeContentSource for the requested PVC
func (p *csiProvisioner) getPVCSource(ctx context.Context, claim *v1.PersistentVolumeClaim, sc *storagev1.StorageClass, dataSource *v1.ObjectReference) (*csi.VolumeContentSource, error) {

	sourcePVC, err := p.claimLister.PersistentVolumeClaims(dataSource.Namespace).Get(dataSource.Name)
	if err != nil {
		return nil, fmt.Errorf("error getting PVC %s (namespace %q) from api server: %v", dataSource.Name, claim.Namespace, err)
	}
	if string(sourcePVC.Status.Phase) != "Bound" {
		return nil, fmt.Errorf("the PVC DataSource %s must have a status of Bound.  Got %v", dataSource.Name, sourcePVC.Status)
	}
	if sourcePVC.ObjectMeta.DeletionTimestamp != nil {
		return nil, fmt.Errorf("the PVC DataSource %s is currently being deleted", dataSource.Name)
	}

	if sourcePVC.Spec.StorageClassName == nil {
		return nil, fmt.Errorf("the source PVC (%s) storageclass cannot be empty", sourcePVC.Name)
	}

	if claim.Spec.StorageClassName == nil {
		return nil, fmt.Errorf("the requested PVC (%s) storageclass cannot be empty", claim.Name)
	}

	capacity := claim.Spec.Resources.Requests[v1.ResourceName(v1.ResourceStorage)]
	requestedSize := capacity.Value()
	srcCapacity := sourcePVC.Spec.Resources.Requests[v1.ResourceName(v1.ResourceStorage)]
	srcPVCSize := srcCapacity.Value()
	if requestedSize < srcPVCSize {
		return nil, fmt.Errorf("error, new PVC request must be greater than or equal in size to the specified PVC data source, requested %v but source is %v", requestedSize, srcPVCSize)
	}

	if sourcePVC.Spec.VolumeName == "" {
		return nil, fmt.Errorf("volume name is empty in source PVC %s", sourcePVC.Name)
	}

	sourcePV, err := p.client.CoreV1().PersistentVolumes().Get(ctx, sourcePVC.Spec.VolumeName, metav1.GetOptions{})
	if err != nil {
		klog.Warningf("error getting volume %s for PVC %s/%s: %s", sourcePVC.Spec.VolumeName, sourcePVC.Namespace, sourcePVC.Name, err)
		return nil, fmt.Errorf("claim in dataSource not bound or invalid")
	}

	if sourcePV.Spec.CSI == nil {
		klog.Warningf("error getting volume source from %s for PVC %s/%s", sourcePVC.Spec.VolumeName, sourcePVC.Namespace, sourcePVC.Name)
		return nil, fmt.Errorf("claim in dataSource not bound or invalid")
	}

	if sourcePV.Spec.CSI.Driver != sc.Provisioner {
		klog.Warningf("the source volume %s for PVC %s/%s is handled by a different CSI driver than requested by StorageClass %s", sourcePVC.Spec.VolumeName, sourcePVC.Namespace, sourcePVC.Name, *claim.Spec.StorageClassName)
		return nil, fmt.Errorf("claim in dataSource not bound or invalid")
	}

	if sourcePV.Spec.ClaimRef == nil {
		klog.Warningf("the source volume %s for PVC %s/%s is not bound", sourcePVC.Spec.VolumeName, sourcePVC.Namespace, sourcePVC.Name)
		return nil, fmt.Errorf("claim in dataSource not bound or invalid")
	}

	if sourcePV.Spec.ClaimRef.UID != sourcePVC.UID || sourcePV.Spec.ClaimRef.Namespace != sourcePVC.Namespace || sourcePV.Spec.ClaimRef.Name != sourcePVC.Name {
		klog.Warningf("the source volume %s for PVC %s/%s is bound to a different PVC than requested", sourcePVC.Spec.VolumeName, sourcePVC.Namespace, sourcePVC.Name)
		return nil, fmt.Errorf("claim in dataSource not bound or invalid")
	}

	if sourcePV.Status.Phase != v1.VolumeBound {
		klog.Warningf("the source volume %s for PVC %s/%s status is \"%s\", should instead be \"%s\"", sourcePVC.Spec.VolumeName, sourcePVC.Namespace, sourcePVC.Name, sourcePV.Status.Phase, v1.VolumeBound)
		return nil, fmt.Errorf("claim in dataSource not bound or invalid")
	}

	if claim.Spec.VolumeMode == nil || *claim.Spec.VolumeMode == v1.PersistentVolumeFilesystem {
		if sourcePV.Spec.VolumeMode != nil && *sourcePV.Spec.VolumeMode != v1.PersistentVolumeFilesystem {
			return nil, fmt.Errorf("the source PVC and destination PVCs must have the same volume mode for cloning.  Source is Block, but new PVC requested Filesystem")
		}
	}

	if claim.Spec.VolumeMode != nil && *claim.Spec.VolumeMode == v1.PersistentVolumeBlock {
		if sourcePV.Spec.VolumeMode == nil || *sourcePV.Spec.VolumeMode != v1.PersistentVolumeBlock {
			return nil, fmt.Errorf("the source PVC and destination PVCs must have the same volume mode for cloning.  Source is Filesystem, but new PVC requested Block")
		}
	}

	volumeSource := csi.VolumeContentSource_Volume{
		Volume: &csi.VolumeContentSource_VolumeSource{
			VolumeId: sourcePV.Spec.CSI.VolumeHandle,
		},
	}
	klog.V(5).Infof("VolumeContentSource_Volume %+v", volumeSource)

	volumeContentSource := &csi.VolumeContentSource{
		Type: &volumeSource,
	}
	return volumeContentSource, nil
}

// getSnapshotSource verifies DataSource.Kind of type VolumeSnapshot, making sure that the requested Snapshot is available/ready
// returns the VolumeContentSource for the requested snapshot
func (p *csiProvisioner) getSnapshotSource(ctx context.Context, claim *v1.PersistentVolumeClaim, sc *storagev1.StorageClass, dataSource *v1.ObjectReference) (*csi.VolumeContentSource, error) {
	snapshotObj, err := p.snapshotClient.SnapshotV1().VolumeSnapshots(dataSource.Namespace).Get(ctx, dataSource.Name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("error getting snapshot %s from api server: %v", dataSource.Name, err)
	}

	if snapshotObj.ObjectMeta.DeletionTimestamp != nil {
		// VolumeSnapshot is being deleted. Check if provisioning already started by looking for
		// the specific finalizer added by external-snapshotter when a snapshot is used as a data source.
		// If the finalizer exists, it means provisioning was started before deletion began,
		// so we should continue to prevent resource leaks.
		// If the finalizer doesn't exist, this is a new provisioning attempt and should be rejected.
		if !checkFinalizer(snapshotObj, snapshotSourceProtectionFinalizer) {
			return nil, fmt.Errorf("snapshot %s is being deleted", dataSource.Name)
		}
		klog.V(3).Infof("Snapshot %s/%s is being deleted but has volumesnapshot-as-source-protection finalizer, allowing provisioning to continue", dataSource.Namespace, dataSource.Name)
	}
	klog.V(5).Infof("VolumeSnapshot %+v", snapshotObj)

	if snapshotObj.Status == nil || snapshotObj.Status.BoundVolumeSnapshotContentName == nil {
		return nil, fmt.Errorf(snapshotNotBound, dataSource.Name)
	}

	snapContentObj, err := p.snapshotClient.SnapshotV1().VolumeSnapshotContents().Get(ctx, *snapshotObj.Status.BoundVolumeSnapshotContentName, metav1.GetOptions{})
	if err != nil {
		klog.Warningf("error getting snapshotcontent %s for snapshot %s/%s from api server: %s", *snapshotObj.Status.BoundVolumeSnapshotContentName, snapshotObj.Namespace, snapshotObj.Name, err)
		return nil, fmt.Errorf(snapshotNotBound, dataSource.Name)
	}

	if snapContentObj.Spec.VolumeSnapshotRef.UID != snapshotObj.UID || snapContentObj.Spec.VolumeSnapshotRef.Namespace != snapshotObj.Namespace || snapContentObj.Spec.VolumeSnapshotRef.Name != snapshotObj.Name {
		klog.Warningf("snapshotcontent %s for snapshot %s/%s is bound to a different snapshot", *snapshotObj.Status.BoundVolumeSnapshotContentName, snapshotObj.Namespace, snapshotObj.Name)
		return nil, fmt.Errorf(snapshotNotBound, dataSource.Name)
	}

	if snapContentObj.Spec.Driver != sc.Provisioner {
		klog.Warningf("snapshotcontent %s for snapshot %s/%s is handled by a different CSI driver than requested by StorageClass %s", *snapshotObj.Status.BoundVolumeSnapshotContentName, snapshotObj.Namespace, snapshotObj.Name, sc.Name)
		return nil, fmt.Errorf(snapshotNotBound, dataSource.Name)
	}

	if snapshotObj.Status.ReadyToUse == nil || !*snapshotObj.Status.ReadyToUse {
		return nil, fmt.Errorf("snapshot %s is not Ready", dataSource.Name)
	}

	klog.V(5).Infof("VolumeSnapshotContent %+v", snapContentObj)

	if snapContentObj.Status == nil || snapContentObj.Status.SnapshotHandle == nil {
		return nil, fmt.Errorf("snapshot handle %s is not available", dataSource.Name)
	}

	snapshotSource := csi.VolumeContentSource_Snapshot{
		Snapshot: &csi.VolumeContentSource_SnapshotSource{
			SnapshotId: *snapContentObj.Status.SnapshotHandle,
		},
	}
	klog.V(5).Infof("VolumeContentSource_Snapshot %+v", snapshotSource)

	if snapshotObj.Status.RestoreSize != nil {
		capacity, exists := claim.Spec.Resources.Requests[v1.ResourceName(v1.ResourceStorage)]
		if !exists {
			return nil, fmt.Errorf("error getting capacity for PVC %s when creating snapshot %s", claim.Name, snapshotObj.Name)
		}
		volSizeBytes := capacity.Value()
		klog.V(5).Infof("Requested volume size is %d and snapshot size is %d for the source snapshot %s", int64(volSizeBytes), int64(snapshotObj.Status.RestoreSize.Value()), snapshotObj.Name)
		// When restoring volume from a snapshot, the volume size should
		// be equal to or larger than its snapshot size.
		if int64(volSizeBytes) < int64(snapshotObj.Status.RestoreSize.Value()) {
			return nil, fmt.Errorf("requested volume size %d is less than the size %d for the source snapshot %s", int64(volSizeBytes), int64(snapshotObj.Status.RestoreSize.Value()), snapshotObj.Name)
		}
		if int64(volSizeBytes) > int64(snapshotObj.Status.RestoreSize.Value()) {
			klog.Warningf("requested volume size %d is greater than the size %d for the source snapshot %s. Volume plugin needs to handle volume expansion.", int64(volSizeBytes), int64(snapshotObj.Status.RestoreSize.Value()), snapshotObj.Name)
		}
	}

	if p.preventVolumeModeConversion {
		if snapContentObj.Spec.SourceVolumeMode != nil && claim.Spec.VolumeMode != nil && *snapContentObj.Spec.SourceVolumeMode != *claim.Spec.VolumeMode {
			// Attempt to modify volume mode during volume creation.
			// Verify if this volume is allowed to alter its mode.
			allowVolumeModeChange, ok := snapContentObj.Annotations[annAllowVolumeModeChange]
			if !ok {
				return nil, fmt.Errorf("requested volume %s/%s modifies the mode of the source volume but does not have permission to do so. "+
					"%s annotation is not present on snapshotcontent %s", claim.Namespace, claim.Name, annAllowVolumeModeChange, snapContentObj.Name)
			}
			allowVolumeModeChangeBool, err := strconv.ParseBool(allowVolumeModeChange)
			if err != nil {
				return nil, fmt.Errorf("requested volume %s/%s modifies the mode of the source volume but does not have permission to do so. "+
					"failed to convert %s annotation value to boolean with error: %v", claim.Namespace, claim.Name, annAllowVolumeModeChange, err)
			}
			if !allowVolumeModeChangeBool {
				return nil, fmt.Errorf("requested volume %s/%s modifies the mode of the source volume but does not have permission to do so. "+
					"%s is set to false on snapshotcontent %s", claim.Namespace, claim.Name, annAllowVolumeModeChange, snapContentObj.Name)
			}
		}
	}

	volumeContentSource := &csi.VolumeContentSource{
		Type: &snapshotSource,
	}

	return volumeContentSource, nil
}

func (p *csiProvisioner) Delete(ctx context.Context, volume *v1.PersistentVolume) error {
	if volume == nil {
		return fmt.Errorf("invalid CSI PV")
	}

	var err error
	var migratedVolume bool
	if p.translator.IsPVMigratable(volume) {
		// we end up here only if CSI migration is enabled in-tree (both overall
		// and for the specific plugin that is migratable) causing in-tree PV
		// controller to yield deletion of PVs with in-tree source to external provisioner
		// based on AnnDynamicallyProvisioned annotation.
		migratedVolume = true
		// TODO replace klog.TODO() once contextual logging is implemented for provisioner
		volume, err = p.translator.TranslateInTreePVToCSI(klog.TODO(), volume)
		if err != nil {
			return err
		}
	}

	if volume.Spec.CSI == nil {
		return fmt.Errorf("invalid CSI PV")
	}

	// If we run on a single node, then we shouldn't delete volumes
	// that we didn't create. In practice, that means that the volume
	// is accessible (only!) on this node.
	if p.nodeDeployment != nil {
		accessible, err := VolumeIsAccessible(volume.Spec.NodeAffinity, p.nodeDeployment.NodeInfo.AccessibleTopology)
		if err != nil {
			return fmt.Errorf("checking volume affinity failed: %v", err)
		}
		if !accessible {
			return &controller.IgnoredError{
				Reason: "PV was not provisioned on this node",
			}
		}
	}

	volumeId := p.volumeHandleToId(volume.Spec.CSI.VolumeHandle)

	rc := &requiredCapabilities{}
	if err := p.checkDriverCapabilities(rc); err != nil {
		return err
	}

	req := csi.DeleteVolumeRequest{
		VolumeId: volumeId,
	}

	err = p.handleSecretsForDeletion(ctx, volume, &req, migratedVolume)
	if err != nil {
		return err
	}
	deleteCtx := markAsMigrated(ctx, migratedVolume)
	deleteCtx, cancel := context.WithTimeout(deleteCtx, p.timeout)
	defer cancel()

	if err := p.canDeleteVolume(volume); err != nil {
		return err
	}

	_, err = p.csiClient.DeleteVolume(deleteCtx, &req)

	return err
}

func (p *csiProvisioner) handleSecretsForDeletion(ctx context.Context, volume *v1.PersistentVolume, req *csi.DeleteVolumeRequest, migratedVolume bool) error {
	var err error
	if metav1.HasAnnotation(volume.ObjectMeta, annDeletionProvisionerSecretRefName) && metav1.HasAnnotation(volume.ObjectMeta, annDeletionProvisionerSecretRefNamespace) {
		annDeletionSecretName := volume.Annotations[annDeletionProvisionerSecretRefName]
		annDeletionSecretNamespace := volume.Annotations[annDeletionProvisionerSecretRefNamespace]
		if annDeletionSecretName != "" && annDeletionSecretNamespace != "" {
			provisionerSecretRef := &v1.SecretReference{}
			provisionerSecretRef.Name = annDeletionSecretName
			provisionerSecretRef.Namespace = annDeletionSecretNamespace
			credentials, err := getCredentials(ctx, p.client, provisionerSecretRef)
			if err != nil {
				// Continue with deletion, as the secret may have already been deleted.
				klog.Errorf("failed to get credentials for volume %s: %s", volume.Name, err.Error())
			}
			req.Secrets = credentials
		} else if annDeletionSecretName == "" && annDeletionSecretNamespace == "" {
			klog.V(2).Infof("volume %s does not need any deletion secrets", volume.Name)
		}
	} else {
		err := p.getSecretsFromSC(ctx, volume, migratedVolume, req)
		if err != nil {
			return err
		}
	}
	return err
}

func (p *csiProvisioner) getSecretsFromSC(ctx context.Context, volume *v1.PersistentVolume, migratedVolume bool, req *csi.DeleteVolumeRequest) error {
	// get secrets if StorageClass specifies it
	storageClassName := util.GetPersistentVolumeClass(volume)
	if len(storageClassName) != 0 {
		if storageClass, err := p.scLister.Get(storageClassName); err == nil {
			if migratedVolume && storageClass.Provisioner == p.supportsMigrationFromInTreePluginName {
				klog.V(2).Infof("translating storage class for in-tree plugin %s to CSI", storageClass.Provisioner)
				// TODO replace klog.TODO() once contextual logging is implemented for provisioner
				storageClass, err = p.translator.TranslateInTreeStorageClassToCSI(klog.TODO(), p.supportsMigrationFromInTreePluginName, storageClass)
				if err != nil {
					return err
				}
			}

			// Resolve provision secret credentials.
			if volume.Spec.ClaimRef == nil {
				klog.Warningf("claim reference does not exists in volume: %s, proceeding to delete without secrets.", volume.Name)
				return nil
			}
			provisionerSecretRef, err := getSecretReference(provisionerSecretParams, storageClass.Parameters, volume.Name, &v1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      volume.Spec.ClaimRef.Name,
					Namespace: volume.Spec.ClaimRef.Namespace,
				},
			})
			if err != nil {
				return fmt.Errorf("failed to get secretreference for volume %s: %v", volume.Name, err)
			}

			credentials, err := getCredentials(ctx, p.client, provisionerSecretRef)
			if err != nil {
				// Continue with deletion, as the secret may have already been deleted.
				klog.Errorf("Failed to get credentials for volume %s: %s", volume.Name, err.Error())
			}
			req.Secrets = credentials
		} else {
			klog.Warningf("failed to get storageclass: %s, proceeding to delete without secrets. %v", storageClassName, err)
		}
	}
	return nil
}

func (p *csiProvisioner) canDeleteVolume(volume *v1.PersistentVolume) error {
	if p.vaLister == nil {
		// Nothing to check.
		return nil
	}

	// Verify if volume is attached to a node before proceeding with deletion
	vaList, err := p.vaLister.List(labels.Everything())
	if err != nil {
		return fmt.Errorf("failed to list volumeattachments: %v", err)
	}

	for _, va := range vaList {
		if va.Spec.Source.PersistentVolumeName != nil && *va.Spec.Source.PersistentVolumeName == volume.Name {
			return fmt.Errorf("persistentvolume %s is still attached to node %s", volume.Name, va.Spec.NodeName)
		}
	}

	return nil
}

func (p *csiProvisioner) SupportsBlock(ctx context.Context) bool {
	// SupportsBlock always return true, because current CSI spec doesn't allow checking
	// drivers' capability of block volume before creating volume.
	// Drivers that don't support block volume should return error for CreateVolume called
	// by Provision if block AccessType is specified.
	return true
}

func (p *csiProvisioner) ShouldProvision(ctx context.Context, claim *v1.PersistentVolumeClaim) bool {
	provisioner, ok := claim.Annotations[annStorageProvisioner]
	if !ok {
		provisioner = claim.Annotations[annBetaStorageProvisioner]
	}
	migratedTo := claim.Annotations[annMigratedTo]
	if provisioner != p.driverName && migratedTo != p.driverName {
		// Non-migrated in-tree volume is requested.
		return false
	}
	// Either CSI volume is requested or in-tree volume is migrated to CSI in PV controller
	// and therefore PVC has CSI annotation.
	//
	// But before we start provisioning, check that we are (or can
	// become) the owner if there are multiple provisioner instances.
	// That we do this here is crucial because if we return false here,
	// the claim will be ignored without logging an event for it.
	// We don't want each provisioner instance to log events for the same
	// claim unless they really need to do some work for it.
	owned, err := p.checkNode(ctx, claim, nil, "should provision")
	if err == nil {
		if !owned {
			return false
		}
	} else {
		// This is unexpected. Here we can only log it and let
		// a provisioning attempt start. If that still fails,
		// a proper event will be created.
		klog.V(2).Infof("trying to become an owner of PVC %s/%s in advance failed, will try again during provisioning: %s",
			claim.Namespace, claim.Name, err)
	}

	// Start provisioning.
	return true
}

// TODO use a unique volume handle from and to Id
func (p *csiProvisioner) volumeIdToHandle(id string) string {
	return id
}

func (p *csiProvisioner) volumeHandleToId(handle string) string {
	return handle
}

// checkNode optionally checks whether the PVC is assigned to the current node.
// If the PVC uses immediate binding, it will try to take the PVC for provisioning
// on the current node. Returns true if provisioning can proceed, an error
// in case of a failure that prevented checking.
func (p *csiProvisioner) checkNode(ctx context.Context, claim *v1.PersistentVolumeClaim, sc *storagev1.StorageClass, caller string) (provision bool, err error) {
	if p.nodeDeployment == nil {
		return true, nil
	}

	var selectedNode string
	if claim.Annotations != nil {
		selectedNode = claim.Annotations[annSelectedNode]
	}
	switch selectedNode {
	case "":
		logger := klog.V(5)
		if logger.Enabled() {
			logger.Infof("%s: checking node for PVC %s/%s with resource version %s", caller, claim.Namespace, claim.Name, claim.ResourceVersion)
			defer func() {
				logger.Infof("%s: done checking node for PVC %s/%s with resource version %s: provision %v, err %v", caller, claim.Namespace, claim.Name, claim.ResourceVersion, provision, err)
			}()
		}

		if sc == nil {
			var err error
			sc, err = p.scLister.Get(*claim.Spec.StorageClassName)
			if err != nil {
				return false, err
			}
		}
		if sc.VolumeBindingMode == nil ||
			*sc.VolumeBindingMode != storagev1.VolumeBindingImmediate ||
			!p.nodeDeployment.ImmediateBinding {
			return false, nil
		}

		// If the storage class has AllowedTopologies set, then
		// it must match our own. We can find out by trying to
		// create accessibility requirements.  If that fails,
		// we should not become the owner.
		if len(sc.AllowedTopologies) > 0 && p.supportsTopology() {
			node, err := p.nodeLister.Get(p.nodeDeployment.NodeName)
			if err != nil {
				return false, err
			}
			if _, err := GenerateAccessibilityRequirements(
				p.client,
				p.driverName,
				claim.UID,
				claim.Name,
				sc.AllowedTopologies,
				node.Name,
				p.strictTopology,
				p.immediateTopology,
				p.csiNodeLister,
				p.nodeLister,
				p.pvcNodeStore); err != nil {
				if logger.Enabled() {
					logger.Infof("%s: ignoring PVC %s/%s, allowed topologies is not compatible: %v", caller, claim.Namespace, claim.Name, err)
				}
				return false, nil
			}
		}

		// Try to select the current node if there is a chance of it
		// being created there, i.e. there is currently enough free space (checked in becomeOwner).
		//
		// If later volume provisioning fails on this node, the annotation will be unset and node
		// selection will happen again. If no other node picks up the volume, then the PVC remains
		// in the queue and this check will be repeated from time to time.
		//
		// A lot of different external-provisioner instances will try to do this at the same time.
		// To avoid the thundering herd problem, we sleep in becomeOwner for a short random amount of time
		// (for new PVCs) or exponentially increasing time (for PVCs were we already had a conflict).
		if err := p.nodeDeployment.becomeOwner(ctx, p, claim); err != nil {
			return false, fmt.Errorf("PVC %s/%s: %v", claim.Namespace, claim.Name, err)
		}

		// We are now either the owner or someone else is. We'll check when the updated PVC
		// enters the workqueue and gets processed by sig-storage-lib-external-provisioner.
		return false, nil
	case p.nodeDeployment.NodeName:
		// Our node is selected.
		return true, nil
	default:
		// Some other node is selected, ignore it.
		return false, nil
	}
}

func (p *csiProvisioner) checkCapacity(ctx context.Context, claim *v1.PersistentVolumeClaim, selectedNodeName string) (bool, error) {
	capacity := claim.Spec.Resources.Requests[v1.ResourceName(v1.ResourceStorage)]
	volSizeBytes := capacity.Value()
	if volSizeBytes == 0 {
		// Nothing to check.
		return true, nil
	}

	if claim.Spec.StorageClassName == nil {
		return false, errors.New("empty storage class name")
	}
	sc, err := p.scLister.Get(*claim.Spec.StorageClassName)
	if err != nil {
		return false, err
	}

	node, err := p.nodeLister.Get(selectedNodeName)
	if err != nil {
		return false, err
	}

	result, _, err := p.prepareProvision(ctx, claim, sc, node.Name)
	if err != nil {
		return false, err
	}

	// In practice, we expect exactly one entry here once a node
	// has been selected. But we have to be prepared for more than
	// one (=> check all, success if there is at least one) and
	// none (no node selected => check once without topology).
	topologies := []*csi.Topology{nil}
	if result.req.AccessibilityRequirements != nil && len(result.req.AccessibilityRequirements.Requisite) > 0 {
		topologies = result.req.AccessibilityRequirements.Requisite
	}
	for _, topology := range topologies {
		req := &csi.GetCapacityRequest{
			VolumeCapabilities: result.req.VolumeCapabilities,
			Parameters:         result.req.Parameters,
			AccessibleTopology: topology,
		}
		resp, err := p.csiClient.GetCapacity(ctx, req)
		if err != nil {
			return false, fmt.Errorf("GetCapacity: %v", err)
		}
		if volSizeBytes <= resp.AvailableCapacity {
			// Enough capacity at the moment.
			return true, nil
		}
	}

	// Currently not enough capacity anywhere.
	return false, nil
}

// becomeOwner updates the PVC with the current node as selected node.
// Returns an error if something unexpectedly failed, otherwise an updated PVC with
// the current node selected or nil if not the owner.
func (nc *internalNodeDeployment) becomeOwner(ctx context.Context, p *csiProvisioner, claim *v1.PersistentVolumeClaim) error {
	requeues := nc.rateLimiter.NumRequeues(claim.UID)
	delay := nc.rateLimiter.When(claim.UID)
	klog.V(5).Infof("will try to become owner of PVC %s/%s with resource version %s in %s (attempt #%d)", claim.Namespace, claim.Name, claim.ResourceVersion, delay, requeues)
	sleep, cancel := context.WithTimeout(ctx, delay)
	defer cancel()
	// When the base delay is high we also should check less often.
	// With multiple provisioners running in parallel, it becomes more
	// likely that one of them became the owner quickly, so we don't
	// want to check too slowly either.
	pollInterval := max(nc.BaseDelay/100, 10*time.Millisecond)
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	check := func() (bool, *v1.PersistentVolumeClaim, error) {
		current, err := nc.ClaimInformer.Lister().PersistentVolumeClaims(claim.Namespace).Get(claim.Name)
		if err != nil {
			return false, nil, fmt.Errorf("PVC not found: %v", err)
		}
		if claim.UID != current.UID {
			return false, nil, errors.New("PVC was replaced")
		}
		if current.Annotations != nil && current.Annotations[annSelectedNode] != "" && current.Annotations[annSelectedNode] != nc.NodeName {
			return true, current, nil
		}
		return false, current, nil
	}
	var stop bool
	var current *v1.PersistentVolumeClaim
	var err error
loop:
	for {
		select {
		case <-ctx.Done():
			return errors.New("timed out waiting to become PVC owner")
		case <-sleep.Done():
			stop, current, err = check()
			break loop
		case <-ticker.C:
			// Abort the waiting early if we know that someone else is the owner.
			stop, current, err = check()
			if err != nil || stop {
				break loop
			}
		}
	}
	if err != nil {
		return err
	}
	if stop {
		// Some other instance was faster and we don't need to provision for
		// this PVC. If the PVC needs to be rescheduled, we start the delay from scratch.
		nc.rateLimiter.Forget(claim.UID)
		klog.V(5).Infof("did not become owner of PVC %s/%s with resource revision %s, now owned by %s with resource revision %s",
			claim.Namespace, claim.Name, claim.ResourceVersion,
			current.Annotations[annSelectedNode], current.ResourceVersion)
		return nil
	}

	// Check capacity as late as possible before trying to become the owner, because that is a
	// relatively expensive operation.
	//
	// The exact same parameters are computed here as if we were provisioning. If a precondition
	// is violated, like "storage class does not exist", then we have two options:
	// - silently ignore the problem, but if all instances do that, the problem is not surfaced
	//   to the user
	// - try to become the owner and let provisioning start, which then will probably
	//   fail the same way, but then has a chance to inform the user via events
	//
	// We do the latter.
	hasCapacity, err := p.checkCapacity(ctx, claim, p.nodeDeployment.NodeName)
	if err != nil {
		klog.V(3).Infof("proceeding with becoming owner although the capacity check failed: %v", err)
	} else if !hasCapacity {
		// Don't try to provision.
		klog.V(5).Infof("not enough capacity for PVC %s/%s with resource revision %s", claim.Namespace, claim.Name, claim.ResourceVersion)
		return nil
	}

	// Update PVC with our node as selected node if necessary.
	current = current.DeepCopy()
	if current.Annotations == nil {
		current.Annotations = map[string]string{}
	}
	if current.Annotations[annSelectedNode] == nc.NodeName {
		// A mere sanity check. Should not happen.
		klog.V(5).Infof("already owner of PVC %s/%s with updated resource version %s", current.Namespace, current.Name, current.ResourceVersion)
		return nil
	}
	current.Annotations[annSelectedNode] = nc.NodeName
	klog.V(5).Infof("trying to become owner of PVC %s/%s with resource version %s now", current.Namespace, current.Name, current.ResourceVersion)
	current, err = p.client.CoreV1().PersistentVolumeClaims(current.Namespace).Update(ctx, current, metav1.UpdateOptions{})
	if err != nil {
		// Next attempt will use a longer delay and most likely
		// stop quickly once we see who owns the PVC now.
		if apierrors.IsConflict(err) {
			// Lost the race or some other concurrent modification. Repeat the attempt.
			klog.V(3).Infof("conflict during PVC %s/%s update, will try again", claim.Namespace, claim.Name)
			return nc.becomeOwner(ctx, p, claim)
		}
		// Some unexpected error. Report it.
		return fmt.Errorf("selecting node %q for PVC failed: %v", nc.NodeName, err)
	}

	// Successfully became owner. Future delays will be smaller again.
	nc.rateLimiter.Forget(claim.UID)
	klog.V(5).Infof("became owner of PVC %s/%s with updated resource version %s", current.Namespace, current.Name, current.ResourceVersion)
	return nil
}

// verifyAndGetSecretNameAndNamespaceTemplate gets the values (templates) associated
// with the parameters specified in "secret" and verifies that they are specified correctly.
func verifyAndGetSecretNameAndNamespaceTemplate(secret secretParamsMap, storageClassParams map[string]string) (nameTemplate, namespaceTemplate string, err error) {
	numName := 0
	numNamespace := 0

	if t, ok := storageClassParams[secret.deprecatedSecretNameKey]; ok {
		nameTemplate = t
		numName++
		klog.Warning(deprecationWarning(secret.deprecatedSecretNameKey, secret.secretNameKey, ""))
	}
	if t, ok := storageClassParams[secret.deprecatedSecretNamespaceKey]; ok {
		namespaceTemplate = t
		numNamespace++
		klog.Warning(deprecationWarning(secret.deprecatedSecretNamespaceKey, secret.secretNamespaceKey, ""))
	}
	if t, ok := storageClassParams[secret.secretNameKey]; ok {
		nameTemplate = t
		numName++
	}
	if t, ok := storageClassParams[secret.secretNamespaceKey]; ok {
		namespaceTemplate = t
		numNamespace++
	}

	if numName > 1 || numNamespace > 1 {
		// Double specified error
		return "", "", fmt.Errorf("%s secrets specified in parameters with both \"csi\" and \"%s\" keys", secret.name, csiParameterPrefix)
	} else if numName != numNamespace {
		// Not both 0 or both 1
		return "", "", fmt.Errorf("either name and namespace for %s secrets specified, Both must be specified", secret.name)
	} else if numName == 1 {
		// Case where we've found a name and a namespace template
		if nameTemplate == "" || namespaceTemplate == "" {
			return "", "", fmt.Errorf("%s secrets specified in parameters but value of either namespace or name is empty", secret.name)
		}
		return nameTemplate, namespaceTemplate, nil
	} else if numName == 0 {
		// No secrets specified
		return "", "", nil
	} else {
		// THIS IS NOT A VALID CASE
		return "", "", fmt.Errorf("unknown error with getting secret name and namespace templates")
	}
}

// getSecretReference returns a reference to the secret specified in the given nameTemplate
// and namespaceTemplate, or an error if the templates are not specified correctly.
// no lookup of the referenced secret is performed, and the secret may or may not exist.
//
// supported tokens for name resolution:
// - ${pv.name}
// - ${pvc.namespace}
// - ${pvc.name}
// - ${pvc.annotations['ANNOTATION_KEY']} (e.g. ${pvc.annotations['example.com/node-publish-secret-name']})
//
// supported tokens for namespace resolution:
// - ${pv.name}
// - ${pvc.namespace}
//
// an error is returned in the following situations:
// - the nameTemplate or namespaceTemplate contains a token that cannot be resolved
// - the resolved name is not a valid secret name
// - the resolved namespace is not a valid namespace name
func getSecretReference(secretParams secretParamsMap, storageClassParams map[string]string, pvName string, pvc *v1.PersistentVolumeClaim) (*v1.SecretReference, error) {
	nameTemplate, namespaceTemplate, err := verifyAndGetSecretNameAndNamespaceTemplate(secretParams, storageClassParams)
	if err != nil {
		return nil, fmt.Errorf("failed to get name and namespace template from params: %v", err)
	}

	// if didn't find secrets for specific call, try to check default values
	if nameTemplate == "" && namespaceTemplate == "" {
		nameTemplate, namespaceTemplate, err = verifyAndGetSecretNameAndNamespaceTemplate(defaultSecretParams, storageClassParams)
		if err != nil {
			return nil, fmt.Errorf("failed to get default name and namespace template from params: %v", err)
		}
	}

	if nameTemplate == "" && namespaceTemplate == "" {
		return nil, nil
	}

	ref := &v1.SecretReference{}
	{
		// Secret namespace template can make use of the PV name or the PVC namespace.
		// Note that neither of those things are under the control of the PVC user.
		namespaceParams := map[string]string{tokenPVNameKey: pvName}
		if pvc != nil {
			namespaceParams[tokenPVCNameSpaceKey] = pvc.Namespace
		}

		resolvedNamespace, err := resolveTemplate(namespaceTemplate, namespaceParams)
		if err != nil {
			return nil, fmt.Errorf("error resolving value %q: %v", namespaceTemplate, err)
		}
		if len(validation.IsDNS1123Label(resolvedNamespace)) > 0 {
			if namespaceTemplate != resolvedNamespace {
				return nil, fmt.Errorf("%q resolved to %q which is not a valid namespace name", namespaceTemplate, resolvedNamespace)
			}
			return nil, fmt.Errorf("%q is not a valid namespace name", namespaceTemplate)
		}
		ref.Namespace = resolvedNamespace
	}

	{
		// Secret name template can make use of the PV name, PVC name or namespace, or a PVC annotation.
		// Note that PVC name and annotations are under the PVC user's control.
		nameParams := map[string]string{tokenPVNameKey: pvName}
		if pvc != nil {
			nameParams[tokenPVCNameKey] = pvc.Name
			nameParams[tokenPVCNameSpaceKey] = pvc.Namespace
			for k, v := range pvc.Annotations {
				nameParams["pvc.annotations['"+k+"']"] = v
			}
		}
		resolvedName, err := resolveTemplate(nameTemplate, nameParams)
		if err != nil {
			return nil, fmt.Errorf("error resolving value %q: %v", nameTemplate, err)
		}
		if len(validation.IsDNS1123Subdomain(resolvedName)) > 0 {
			if nameTemplate != resolvedName {
				return nil, fmt.Errorf("%q resolved to %q which is not a valid secret name", nameTemplate, resolvedName)
			}
			return nil, fmt.Errorf("%q is not a valid secret name", nameTemplate)
		}
		ref.Name = resolvedName
	}

	return ref, nil
}

func resolveTemplate(template string, params map[string]string) (string, error) {
	missingParams := sets.NewString()
	resolved := os.Expand(template, func(k string) string {
		v, ok := params[k]
		if !ok {
			missingParams.Insert(k)
		}
		return v
	})
	if missingParams.Len() > 0 {
		return "", fmt.Errorf("invalid tokens: %q", missingParams.List())
	}
	return resolved, nil
}

func getCredentials(ctx context.Context, k8s kubernetes.Interface, ref *v1.SecretReference) (map[string]string, error) {
	if ref == nil {
		return nil, nil
	}

	secret, err := k8s.CoreV1().Secrets(ref.Namespace).Get(ctx, ref.Name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("error getting secret %s in namespace %s: %v", ref.Name, ref.Namespace, err)
	}

	credentials := map[string]string{}
	for key, value := range secret.Data {
		credentials[key] = string(value)
	}
	return credentials, nil
}

func bytesToQuantity(bytes int64) resource.Quantity {
	quantity := resource.NewQuantity(bytes, resource.BinarySI)
	return *quantity
}

func deprecationWarning(deprecatedParam, newParam, removalVersion string) string {
	if removalVersion == "" {
		removalVersion = "a future release"
	}
	newParamPhrase := ""
	if len(newParam) != 0 {
		newParamPhrase = fmt.Sprintf(", please use \"%s\" instead", newParam)
	}
	return fmt.Sprintf("\"%s\" is deprecated and will be removed in %s%s", deprecatedParam, removalVersion, newParamPhrase)
}

func checkError(err error, mayReschedule bool) controller.ProvisioningState {
	// Sources:
	// https://github.com/grpc/grpc/blob/master/doc/statuscodes.md
	// https://github.com/container-storage-interface/spec/blob/master/spec.md
	st, ok := status.FromError(err)
	if !ok {
		// This is not gRPC error. The operation must have failed before gRPC
		// method was called, otherwise we would get gRPC error.
		// We don't know if any previous CreateVolume is in progress, be on the safe side.
		return controller.ProvisioningInBackground
	}
	switch st.Code() {
	case codes.ResourceExhausted:
		// CSI: operation not pending, "Unable to provision in `accessible_topology`"
		// However, it also could be from the transport layer for "message size exceeded".
		// Cannot be decided properly here and needs to be resolved in the spec
		// https://github.com/container-storage-interface/spec/issues/419.
		// What we assume here for now is that message size limits are large enough that
		// the error really comes from the CSI driver.
		if mayReschedule {
			// may succeed elsewhere -> give up for now
			return controller.ProvisioningReschedule
		}
		// We still assume that gRPRC message size limits are large enough, see above.
		// The CSI driver has run out of space, provisioning is not in progress.
		return controller.ProvisioningFinished
	case codes.Canceled, // gRPC: Client Application cancelled the request
		codes.DeadlineExceeded, // gRPC: Timeout
		codes.Unavailable,      // gRPC: Server shutting down, TCP connection broken - previous CreateVolume() may be still in progress.
		codes.Aborted:          // CSI: Operation pending for volume
		return controller.ProvisioningInBackground
	}
	// All other errors mean that provisioning either did not
	// even start or failed. It is for sure not in progress.
	return controller.ProvisioningFinished
}

func cleanupVolume(ctx context.Context, p *csiProvisioner, delReq *csi.DeleteVolumeRequest, provisionerCredentials map[string]string) error {
	var err error
	delReq.Secrets = provisionerCredentials
	deleteCtx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()
	for range deleteVolumeRetryCount {
		_, err = p.csiClient.DeleteVolume(deleteCtx, delReq)
		if err == nil {
			break
		}
	}
	return err
}

func checkFinalizer(obj metav1.Object, finalizer string) bool {
	return slices.Contains(obj.GetFinalizers(), finalizer)
}

func markAsMigrated(parent context.Context, hasMigrated bool) context.Context {
	return context.WithValue(parent, connection.AdditionalInfoKey, connection.AdditionalInfo{Migrated: strconv.FormatBool(hasMigrated)})
}

func (p *csiProvisioner) dataSource(ctx context.Context, claim *v1.PersistentVolumeClaim) (*v1.ObjectReference, error) {
	var dataSource v1.ObjectReference

	if claim.Spec.DataSource != nil {
		dataSource.Kind = claim.Spec.DataSource.Kind
		dataSource.Name = claim.Spec.DataSource.Name

		if claim.Spec.DataSource.APIGroup != nil {
			dataSource.APIVersion = *claim.Spec.DataSource.APIGroup
		}

		dataSource.Namespace = claim.Namespace

	} else if claim.Spec.DataSourceRef != nil {
		if !utilfeature.DefaultFeatureGate.Enabled(features.CrossNamespaceVolumeDataSource) &&
			claim.Spec.DataSourceRef.Namespace != nil && len(*claim.Spec.DataSourceRef.Namespace) > 0 {
			return nil, fmt.Errorf("dataSourceRef namespace specified but the CrossNamespaceVolumeDataSource feature is disabled")
		} else {
			dataSource.Kind = claim.Spec.DataSourceRef.Kind
			dataSource.Name = claim.Spec.DataSourceRef.Name
			if claim.Spec.DataSourceRef.APIGroup != nil {
				dataSource.APIVersion = *claim.Spec.DataSourceRef.APIGroup
			}
			if claim.Spec.DataSourceRef.Namespace != nil {
				dataSource.Namespace = *claim.Spec.DataSourceRef.Namespace
			} else {
				dataSource.Namespace = claim.Namespace
			}
		}
	} else {
		return nil, nil
	}

	// In case of cross namespace data source, check if it accepts references.
	if claim.Namespace != dataSource.Namespace {
		// Get all ReferenceGrants in data source's namespace
		referenceGrants, err := p.referenceGrantLister.ReferenceGrants(dataSource.Namespace).List(labels.Everything())
		if err != nil {
			return nil, fmt.Errorf("error getting ReferenceGrants in %s namespace from api server: %v", dataSource.Namespace, err)
		}
		if allowed, err := IsGranted(ctx, claim, referenceGrants); err != nil || !allowed {
			return nil, err
		}
	}

	return &dataSource, nil
}
