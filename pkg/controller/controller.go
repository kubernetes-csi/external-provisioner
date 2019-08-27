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
	"crypto/md5"
	"errors"
	"fmt"
	"math"
	"os"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/kubernetes-csi/csi-lib-utils/connection"
	"github.com/kubernetes-csi/external-provisioner/pkg/features"
	snapapi "github.com/kubernetes-csi/external-snapshotter/pkg/apis/volumesnapshot/v1alpha1"
	snapclientset "github.com/kubernetes-csi/external-snapshotter/pkg/client/clientset/versioned"
	"sigs.k8s.io/sig-storage-lib-external-provisioner/controller"
	"sigs.k8s.io/sig-storage-lib-external-provisioner/util"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	_ "k8s.io/apimachinery/pkg/util/json"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	csitranslationlib "k8s.io/csi-translation-lib"
	"k8s.io/klog"

	"google.golang.org/grpc"
)

//secretParamsMap provides a mapping of current as well as deprecated secret keys
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

	// Defines parameters for ExponentialBackoff used for executing
	// CSI CreateVolume API call, it gives approx 4 minutes for the CSI
	// driver to complete a volume creation.
	backoffDuration = time.Second * 5
	backoffFactor   = 1.2
	backoffSteps    = 10

	defaultFSType = "ext4"

	snapshotKind     = "VolumeSnapshot"
	snapshotAPIGroup = snapapi.GroupName       // "snapshot.storage.k8s.io"
	pvcKind          = "PersistentVolumeClaim" // Native types don't require an API group

	tokenPVNameKey       = "pv.name"
	tokenPVCNameKey      = "pvc.name"
	tokenPVCNameSpaceKey = "pvc.namespace"
)

var (
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
)

// requiredCapabilities provides a set of extra capabilities required for special/optional features provided by a plugin
type requiredCapabilities struct {
	snapshot bool
	clone    bool
}

// CSIProvisioner struct
type csiProvisioner struct {
	client                                kubernetes.Interface
	csiClient                             csi.ControllerClient
	grpcClient                            *grpc.ClientConn
	snapshotClient                        snapclientset.Interface
	timeout                               time.Duration
	identity                              string
	volumeNamePrefix                      string
	volumeNameUUIDLength                  int
	volumeNamesReadable                   bool
	config                                *rest.Config
	driverName                            string
	pluginCapabilities                    connection.PluginCapabilitySet
	controllerCapabilities                connection.ControllerCapabilitySet
	supportsMigrationFromInTreePluginName string
	strictTopology                        bool
}

var _ controller.Provisioner = &csiProvisioner{}
var _ controller.BlockProvisioner = &csiProvisioner{}
var _ controller.ProvisionerExt = &csiProvisioner{}

var (
	// Each provisioner have a identify string to distinguish with others. This
	// identify string will be added in PV annoations under this key.
	provisionerIDKey = "storage.kubernetes.io/csiProvisionerIdentity"
)

func Connect(address string) (*grpc.ClientConn, error) {
	return connection.Connect(address, connection.OnConnectionLoss(connection.ExitOnConnectionLoss()))
}

func Probe(conn *grpc.ClientConn, singleCallTimeout time.Duration) error {
	return connection.ProbeForever(conn, singleCallTimeout)
}

func GetDriverName(conn *grpc.ClientConn, timeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return connection.GetDriverName(ctx, conn)
}

func GetDriverCapabilities(conn *grpc.ClientConn, timeout time.Duration) (connection.PluginCapabilitySet, connection.ControllerCapabilitySet, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	pluginCapabilities, err := connection.GetPluginCapabilities(ctx, conn)
	if err != nil {
		return nil, nil, err
	}

	/* Each CSI operation gets its own timeout / context */
	ctx, cancel = context.WithTimeout(context.Background(), timeout)
	defer cancel()
	controllerCapabilities, err := connection.GetControllerCapabilities(ctx, conn)
	if err != nil {
		return nil, nil, err
	}

	return pluginCapabilities, controllerCapabilities, nil
}

// NewCSIProvisioner creates new CSI provisioner
func NewCSIProvisioner(client kubernetes.Interface,
	connectionTimeout time.Duration,
	identity string,
	volumeNamePrefix string,
	volumeNameUUIDLength int,
	volumeNamesReadable bool,
	grpcClient *grpc.ClientConn,
	snapshotClient snapclientset.Interface,
	driverName string,
	pluginCapabilities connection.PluginCapabilitySet,
	controllerCapabilities connection.ControllerCapabilitySet,
	supportsMigrationFromInTreePluginName string,
	strictTopology bool) controller.Provisioner {

	csiClient := csi.NewControllerClient(grpcClient)
	provisioner := &csiProvisioner{
		client:                                client,
		grpcClient:                            grpcClient,
		csiClient:                             csiClient,
		snapshotClient:                        snapshotClient,
		timeout:                               connectionTimeout,
		identity:                              identity,
		volumeNamePrefix:                      volumeNamePrefix,
		volumeNameUUIDLength:                  volumeNameUUIDLength,
		volumeNamesReadable:                   volumeNamesReadable,
		driverName:                            driverName,
		pluginCapabilities:                    pluginCapabilities,
		controllerCapabilities:                controllerCapabilities,
		supportsMigrationFromInTreePluginName: supportsMigrationFromInTreePluginName,
		strictTopology:                        strictTopology,
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

	return nil
}

func (p *csiProvisioner) makeVolumeName(pvcNamespace, pvcName, pvcUID string) (string, error) {
	// create persistent name based on a volumeNamePrefix and volumeNameUUIDLength
	// of PVC's UID; if enabled, utilize PVC namespace and name as well
	if(len(p.volumeNamePrefix) == 0) {
		return "", fmt.Errorf("Volume name prefix cannot be of length 0")
	}
	if len(pvcUID) == 0 {
		return "", fmt.Errorf("corrupted PVC object, it is missing UID")
	}
	pvcUID = strings.Replace(pvcUID, "-", "", -1)

	var fullName string
	if(p.volumeNamesReadable) {
		if(len(pvcNamespace) == 0) {
			return "", fmt.Errorf("corrupted PVC object, it is missing namespace")
		}
		if(len(pvcName) == 0) {
			return "", fmt.Errorf("corrupted PVC object, it is missing name")
		}
		pvcHash := md5.Sum([]byte(pvcNamespace + "/" + pvcName))
		pvcNamespace = strings.Replace(pvcNamespace, "-", "", -1)
		pvcName = strings.Replace(pvcName, "-", "", -1)
		fullName = fmt.Sprintf("%s-%s-%x", pvcNamespace, pvcName, pvcHash)
	} else {
		fullName = pvcUID
	}
	if(p.volumeNameUUIDLength == -1) {
		return fmt.Sprintf("%s-%s", p.volumeNamePrefix, fullName), nil
	}
	return fmt.Sprintf("%s-%s", p.volumeNamePrefix, fullName[0:p.volumeNameUUIDLength]), nil
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

func getAccessMode(pvcAccessMode v1.PersistentVolumeAccessMode) *csi.VolumeCapability_AccessMode {
	switch pvcAccessMode {
	case v1.ReadWriteOnce:
		return &csi.VolumeCapability_AccessMode{
			Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
		}
	case v1.ReadWriteMany:
		return &csi.VolumeCapability_AccessMode{
			Mode: csi.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER,
		}
	case v1.ReadOnlyMany:
		return &csi.VolumeCapability_AccessMode{
			Mode: csi.VolumeCapability_AccessMode_MULTI_NODE_READER_ONLY,
		}
	default:
		return nil
	}
}

func getVolumeCapability(
	options controller.ProvisionOptions,
	pvcAccessMode v1.PersistentVolumeAccessMode,
	fsType string,
) *csi.VolumeCapability {
	if util.CheckPersistentVolumeClaimModeBlock(options.PVC) {
		return &csi.VolumeCapability{
			AccessType: getAccessTypeBlock(),
			AccessMode: getAccessMode(pvcAccessMode),
		}
	}
	return &csi.VolumeCapability{
		AccessType: getAccessTypeMount(fsType, options.StorageClass.MountOptions),
		AccessMode: getAccessMode(pvcAccessMode),
	}

}

func (p *csiProvisioner) Provision(options controller.ProvisionOptions) (*v1.PersistentVolume, error) {
	// The controller should call ProvisionExt() instead, but just in case...
	pv, _, err := p.ProvisionExt(options)
	return pv, err
}

func (p *csiProvisioner) ProvisionExt(options controller.ProvisionOptions) (*v1.PersistentVolume, controller.ProvisioningState, error) {
	if options.StorageClass == nil {
		return nil, controller.ProvisioningFinished, errors.New("storage class was nil")
	}

	migratedVolume := false
	if p.supportsMigrationFromInTreePluginName != "" {
		// NOTE: we cannot depend on PVC.Annotations[volume.beta.kubernetes.io/storage-provisioner] to get
		// the in-tree provisioner name in case of CSI migration scenarios. The annotation will be
		// set to the CSI provisioner name by PV controller for migration scenarios
		// so that external provisioner can correctly pick up the PVC pointing to an in-tree plugin
		if options.StorageClass.Provisioner == p.supportsMigrationFromInTreePluginName {
			klog.V(2).Infof("translating storage class for in-tree plugin %s to CSI", options.StorageClass.Provisioner)
			storageClass, err := csitranslationlib.TranslateInTreeStorageClassToCSI(p.supportsMigrationFromInTreePluginName, options.StorageClass)
			if err != nil {
				return nil, controller.ProvisioningFinished, fmt.Errorf("failed to translate storage class: %v", err)
			}
			options.StorageClass = storageClass
			migratedVolume = true
		} else {
			klog.V(4).Infof("skip translation of storage class for plugin: %s", options.StorageClass.Provisioner)
		}
	}

	// Make sure the plugin is capable of fulfilling the requested options
	rc := &requiredCapabilities{}
	if options.PVC.Spec.DataSource != nil {
		// PVC.Spec.DataSource.Name is the name of the VolumeSnapshot API object
		if options.PVC.Spec.DataSource.Name == "" {
			return nil, controller.ProvisioningFinished, fmt.Errorf("the PVC source not found for PVC %s", options.PVC.Name)
		}

		switch options.PVC.Spec.DataSource.Kind {
		case snapshotKind:
			if *(options.PVC.Spec.DataSource.APIGroup) != snapshotAPIGroup {
				return nil, controller.ProvisioningFinished, fmt.Errorf("the PVC source does not belong to the right APIGroup. Expected %s, Got %s", snapshotAPIGroup, *(options.PVC.Spec.DataSource.APIGroup))
			}
			rc.snapshot = true
		case pvcKind:
			rc.clone = true
		default:
			klog.Infof("Unsupported DataSource specified (%s), the provisioner won't act on this request", options.PVC.Spec.DataSource.Kind)
		}
	}
	if err := p.checkDriverCapabilities(rc); err != nil {
		return nil, controller.ProvisioningFinished, err
	}

	if options.PVC.Spec.Selector != nil {
		return nil, controller.ProvisioningFinished, fmt.Errorf("claim Selector is not supported")
	}

	pvName, err := p.makeVolumeName(options.PVC.Namespace, options.PVC.Name, string(options.PVC.ObjectMeta.UID))
	if err != nil {
		return nil, controller.ProvisioningFinished, err
	}

	fsTypesFound := 0
	fsType := ""
	for k, v := range options.StorageClass.Parameters {
		if strings.ToLower(k) == "fstype" || k == prefixedFsTypeKey {
			fsType = v
			fsTypesFound++
		}
		if strings.ToLower(k) == "fstype" {
			klog.Warningf(deprecationWarning("fstype", prefixedFsTypeKey, ""))
		}
	}
	if fsTypesFound > 1 {
		return nil, controller.ProvisioningFinished, fmt.Errorf("fstype specified in parameters with both \"fstype\" and \"%s\" keys", prefixedFsTypeKey)
	}
	if len(fsType) == 0 {
		fsType = defaultFSType
	}

	capacity := options.PVC.Spec.Resources.Requests[v1.ResourceName(v1.ResourceStorage)]
	volSizeBytes := capacity.Value()

	// Get access mode
	volumeCaps := make([]*csi.VolumeCapability, 0)
	for _, pvcAccessMode := range options.PVC.Spec.AccessModes {
		volumeCaps = append(volumeCaps, getVolumeCapability(options, pvcAccessMode, fsType))
	}

	// Create a CSI CreateVolumeRequest and Response
	req := csi.CreateVolumeRequest{
		Name:               pvName,
		Parameters:         options.StorageClass.Parameters,
		VolumeCapabilities: volumeCaps,
		CapacityRange: &csi.CapacityRange{
			RequiredBytes: int64(volSizeBytes),
		},
	}

	if options.PVC.Spec.DataSource != nil && (rc.clone || rc.snapshot) {
		volumeContentSource, err := p.getVolumeContentSource(options)
		if err != nil {
			return nil, controller.ProvisioningNoChange, fmt.Errorf("error getting handle for DataSource Type %s by Name %s: %v", options.PVC.Spec.DataSource.Kind, options.PVC.Spec.DataSource.Name, err)
		}
		req.VolumeContentSource = volumeContentSource
	}

	if p.supportsTopology() {
		requirements, err := GenerateAccessibilityRequirements(
			p.client,
			p.driverName,
			options.PVC.Name,
			options.StorageClass.AllowedTopologies,
			options.SelectedNode,
			p.strictTopology)
		if err != nil {
			return nil, controller.ProvisioningNoChange, fmt.Errorf("error generating accessibility requirements: %v", err)
		}
		req.AccessibilityRequirements = requirements
	}

	klog.V(5).Infof("CreateVolumeRequest %+v", req)

	rep := &csi.CreateVolumeResponse{}

	// Resolve provision secret credentials.
	provisionerSecretRef, err := getSecretReference(provisionerSecretParams, options.StorageClass.Parameters, pvName, &v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      options.PVC.Name,
			Namespace: options.PVC.Namespace,
		},
	})
	if err != nil {
		return nil, controller.ProvisioningNoChange, err
	}
	provisionerCredentials, err := getCredentials(p.client, provisionerSecretRef)
	if err != nil {
		return nil, controller.ProvisioningNoChange, err
	}
	req.Secrets = provisionerCredentials

	// Resolve controller publish, node stage, node publish secret references
	controllerPublishSecretRef, err := getSecretReference(controllerPublishSecretParams, options.StorageClass.Parameters, pvName, options.PVC)
	if err != nil {
		return nil, controller.ProvisioningNoChange, err
	}
	nodeStageSecretRef, err := getSecretReference(nodeStageSecretParams, options.StorageClass.Parameters, pvName, options.PVC)
	if err != nil {
		return nil, controller.ProvisioningNoChange, err
	}
	nodePublishSecretRef, err := getSecretReference(nodePublishSecretParams, options.StorageClass.Parameters, pvName, options.PVC)
	if err != nil {
		return nil, controller.ProvisioningNoChange, err
	}
	controllerExpandSecretRef, err := getSecretReference(controllerExpandSecretParams, options.StorageClass.Parameters, pvName, options.PVC)
	if err != nil {
		return nil, controller.ProvisioningNoChange, err
	}

	req.Parameters, err = removePrefixedParameters(options.StorageClass.Parameters)
	if err != nil {
		return nil, controller.ProvisioningFinished, fmt.Errorf("failed to strip CSI Parameters of prefixed keys: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()
	rep, err = p.csiClient.CreateVolume(ctx, &req)

	if err != nil {
		if isFinalError(err) {
			return nil, controller.ProvisioningFinished, err
		}
		return nil, controller.ProvisioningInBackground, err
	}

	if rep.Volume != nil {
		klog.V(3).Infof("create volume rep: %+v", *rep.Volume)
	}
	volumeAttributes := map[string]string{provisionerIDKey: p.identity}
	for k, v := range rep.Volume.VolumeContext {
		volumeAttributes[k] = v
	}
	respCap := rep.GetVolume().GetCapacityBytes()

	//According to CSI spec CreateVolume should be able to return capacity = 0, which means it is unknown. for example NFS/FTP
	if respCap == 0 {
		respCap = volSizeBytes
		klog.V(3).Infof("csiClient response volume with size 0, which is not supported by apiServer, will use claim size:%d", respCap)
	} else if respCap < volSizeBytes {
		capErr := fmt.Errorf("created volume capacity %v less than requested capacity %v", respCap, volSizeBytes)
		delReq := &csi.DeleteVolumeRequest{
			VolumeId: rep.GetVolume().GetVolumeId(),
		}
		delReq.Secrets = provisionerCredentials
		ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
		defer cancel()
		_, err := p.csiClient.DeleteVolume(ctx, delReq)
		if err != nil {
			capErr = fmt.Errorf("%v. Cleanup of volume %s failed, volume is orphaned: %v", capErr, pvName, err)
		}
		// use InBackground to retry the call, hoping the volume is deleted correctly next time.
		return nil, controller.ProvisioningInBackground, capErr
	}

	pv := &v1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: pvName,
		},
		Spec: v1.PersistentVolumeSpec{
			AccessModes:  options.PVC.Spec.AccessModes,
			MountOptions: options.StorageClass.MountOptions,
			Capacity: v1.ResourceList{
				v1.ResourceName(v1.ResourceStorage): bytesToGiQuantity(respCap),
			},
			// TODO wait for CSI VolumeSource API
			PersistentVolumeSource: v1.PersistentVolumeSource{
				CSI: &v1.CSIPersistentVolumeSource{
					Driver:                     p.driverName,
					VolumeHandle:               p.volumeIdToHandle(rep.Volume.VolumeId),
					VolumeAttributes:           volumeAttributes,
					ControllerPublishSecretRef: controllerPublishSecretRef,
					NodeStageSecretRef:         nodeStageSecretRef,
					NodePublishSecretRef:       nodePublishSecretRef,
					ControllerExpandSecretRef:  controllerExpandSecretRef,
				},
			},
		},
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
		pv.Spec.PersistentVolumeSource.CSI.FSType = fsType
	}

	if migratedVolume {
		pv, err = csitranslationlib.TranslateCSIPVToInTree(pv)
		if err != nil {
			klog.Warningf("failed to translate CSI PV to in-tree due to: %v. Deleting provisioned PV", err)
			deleteErr := p.Delete(pv)
			if deleteErr != nil {
				klog.Warningf("failed to delete partly provisioned PV: %v", deleteErr)
				// Retry the call again to clean up the orphan
				return nil, controller.ProvisioningInBackground, err
			}
			return nil, controller.ProvisioningFinished, err
		}
	}

	klog.V(2).Infof("successfully created PV %v for PVC %v and csi volume name %v", pv.Name, options.PVC.Name, pv.Spec.CSI.VolumeHandle)
	klog.V(5).Infof("successfully created PV %+v", pv.Spec.PersistentVolumeSource)
	return pv, controller.ProvisioningFinished, nil
}

func (p *csiProvisioner) supportsTopology() bool {
	return p.pluginCapabilities[csi.PluginCapability_Service_VOLUME_ACCESSIBILITY_CONSTRAINTS] &&
		utilfeature.DefaultFeatureGate.Enabled(features.Topology)
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
func (p *csiProvisioner) getVolumeContentSource(options controller.ProvisionOptions) (*csi.VolumeContentSource, error) {
	switch options.PVC.Spec.DataSource.Kind {
	case snapshotKind:
		return p.getSnapshotSource(options)
	case pvcKind:
		return p.getPVCSource(options)
	default:
		// For now we shouldn't pass other things to this function, but treat it as a noop and extend as needed
		return nil, nil
	}
}

// getPVCSource verifies DataSource.Kind of type PersistentVolumeClaim, making sure that the requested PVC is available/ready
// returns the VolumeContentSource for the requested PVC
func (p *csiProvisioner) getPVCSource(options controller.ProvisionOptions) (*csi.VolumeContentSource, error) {
	sourcePVC, err := p.client.CoreV1().PersistentVolumeClaims(options.PVC.Namespace).Get(options.PVC.Spec.DataSource.Name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("error getting PVC %s from api server: %v", options.PVC.Spec.DataSource.Name, err)
	}
	if string(sourcePVC.Status.Phase) != "Bound" {
		return nil, fmt.Errorf("the PVC DataSource %s must have a status of Bound.  Got %v", options.PVC.Spec.DataSource.Name, sourcePVC.Status)
	}
	if sourcePVC.ObjectMeta.DeletionTimestamp != nil {
		return nil, fmt.Errorf("the PVC DataSource %s is currently being deleted", options.PVC.Spec.DataSource.Name)
	}

	if sourcePVC.Spec.StorageClassName == nil {
		return nil, fmt.Errorf("the source PVC (%s) storageclass cannot be empty", sourcePVC.Name)
	}

	if options.PVC.Spec.StorageClassName == nil {
		return nil, fmt.Errorf("the requested PVC (%s) storageclass cannot be empty", options.PVC.Name)
	}

	if *sourcePVC.Spec.StorageClassName != *options.PVC.Spec.StorageClassName {
		return nil, fmt.Errorf("the source PVC and destination PVCs must be in the same storage class for cloning.  Source is in %v, but new PVC is in %v",
			*sourcePVC.Spec.StorageClassName, *options.PVC.Spec.StorageClassName)
	}
	capacity := options.PVC.Spec.Resources.Requests[v1.ResourceName(v1.ResourceStorage)]
	requestedSize := capacity.Value()
	if requestedSize < int64(sourcePVC.Spec.Size()) {
		return nil, fmt.Errorf("error, new PVC request must be greater than or equal in size to the specified PVC data source, requested %v but source is %v", requestedSize, sourcePVC.Spec.Size())
	}

	if sourcePVC.Spec.VolumeName == "" {
		return nil, fmt.Errorf("volume name is empty in source PVC %s", sourcePVC.Name)
	}

	sourcePV, err := p.client.CoreV1().PersistentVolumes().Get(sourcePVC.Spec.VolumeName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("error getting PV %s from api server: %v", sourcePVC.Spec.VolumeName, err)
	}

	if sourcePV.Spec.CSI == nil {
		return nil, fmt.Errorf("error getting volume source from persistantVolumeClaim:persistanceVolume %s:%s", sourcePVC.Name, sourcePVC.Spec.VolumeName)
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
func (p *csiProvisioner) getSnapshotSource(options controller.ProvisionOptions) (*csi.VolumeContentSource, error) {
	snapshotObj, err := p.snapshotClient.VolumesnapshotV1alpha1().VolumeSnapshots(options.PVC.Namespace).Get(options.PVC.Spec.DataSource.Name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("error getting snapshot %s from api server: %v", options.PVC.Spec.DataSource.Name, err)
	}
	if snapshotObj.Status.ReadyToUse == false {
		return nil, fmt.Errorf("snapshot %s is not Ready", options.PVC.Spec.DataSource.Name)
	}

	if snapshotObj.ObjectMeta.DeletionTimestamp != nil {
		return nil, fmt.Errorf("snapshot %s is currently being deleted", options.PVC.Spec.DataSource.Name)
	}
	klog.V(5).Infof("VolumeSnapshot %+v", snapshotObj)

	snapContentObj, err := p.snapshotClient.VolumesnapshotV1alpha1().VolumeSnapshotContents().Get(snapshotObj.Spec.SnapshotContentName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("error getting snapshot:snapshotcontent %s:%s from api server: %v", snapshotObj.Name, snapshotObj.Spec.SnapshotContentName, err)
	}
	klog.V(5).Infof("VolumeSnapshotContent %+v", snapContentObj)

	if snapContentObj.Spec.VolumeSnapshotSource.CSI == nil {
		return nil, fmt.Errorf("error getting snapshot source from snapshot:snapshotcontent %s:%s", snapshotObj.Name, snapshotObj.Spec.SnapshotContentName)
	}

	snapshotSource := csi.VolumeContentSource_Snapshot{
		Snapshot: &csi.VolumeContentSource_SnapshotSource{
			SnapshotId: snapContentObj.Spec.VolumeSnapshotSource.CSI.SnapshotHandle,
		},
	}
	klog.V(5).Infof("VolumeContentSource_Snapshot %+v", snapshotSource)

	if snapshotObj.Status.RestoreSize != nil {
		capacity, exists := options.PVC.Spec.Resources.Requests[v1.ResourceName(v1.ResourceStorage)]
		if !exists {
			return nil, fmt.Errorf("error getting capacity for PVC %s when creating snapshot %s", options.PVC.Name, snapshotObj.Name)
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

	volumeContentSource := &csi.VolumeContentSource{
		Type: &snapshotSource,
	}

	return volumeContentSource, nil
}

func (p *csiProvisioner) Delete(volume *v1.PersistentVolume) error {
	if volume == nil {
		return fmt.Errorf("invalid CSI PV")
	}

	var err error
	if csitranslationlib.IsPVMigratable(volume) {
		// we end up here only if CSI migration is enabled in-tree (both overall
		// and for the specific plugin that is migratable) causing in-tree PV
		// controller to yield deletion of PVs with in-tree source to external provisioner
		// based on AnnDynamicallyProvisioned annotation.
		volume, err = csitranslationlib.TranslateInTreePVToCSI(volume)
		if err != nil {
			return err
		}
	}

	if volume.Spec.CSI == nil {
		return fmt.Errorf("invalid CSI PV")
	}

	volumeId := p.volumeHandleToId(volume.Spec.CSI.VolumeHandle)

	rc := &requiredCapabilities{}
	if err := p.checkDriverCapabilities(rc); err != nil {
		return err
	}

	req := csi.DeleteVolumeRequest{
		VolumeId: volumeId,
	}
	// get secrets if StorageClass specifies it
	storageClassName := volume.Spec.StorageClassName
	if len(storageClassName) != 0 {
		if storageClass, err := p.client.StorageV1().StorageClasses().Get(storageClassName, metav1.GetOptions{}); err == nil {
			// Resolve provision secret credentials.
			provisionerSecretRef, err := getSecretReference(provisionerSecretParams, storageClass.Parameters, volume.Name, &v1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      volume.Spec.ClaimRef.Name,
					Namespace: volume.Spec.ClaimRef.Namespace,
				},
			})
			if err != nil {
				return err
			}

			credentials, err := getCredentials(p.client, provisionerSecretRef)
			if err != nil {
				// Continue with deletion, as the secret may have already been deleted.
				klog.Errorf("Failed to get credentials for volume %s: %s", volume.Name, err.Error())
			}
			req.Secrets = credentials
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()

	_, err = p.csiClient.DeleteVolume(ctx, &req)

	return err
}

func (p *csiProvisioner) SupportsBlock() bool {
	// SupportsBlock always return true, because current CSI spec doesn't allow checking
	// drivers' capability of block volume before creating volume.
	// Drivers that don't support block volume should return error for CreateVolume called
	// by Provision if block AccessType is specified.
	return true
}

//TODO use a unique volume handle from and to Id
func (p *csiProvisioner) volumeIdToHandle(id string) string {
	return id
}

func (p *csiProvisioner) volumeHandleToId(handle string) string {
	return handle
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
//  and namespaceTemplate, or an error if the templates are not specified correctly.
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

func getCredentials(k8s kubernetes.Interface, ref *v1.SecretReference) (map[string]string, error) {
	if ref == nil {
		return nil, nil
	}

	secret, err := k8s.CoreV1().Secrets(ref.Namespace).Get(ref.Name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("error getting secret %s in namespace %s: %v", ref.Name, ref.Namespace, err)
	}

	credentials := map[string]string{}
	for key, value := range secret.Data {
		credentials[key] = string(value)
	}
	return credentials, nil
}

func bytesToGiQuantity(bytes int64) resource.Quantity {
	var num int64
	var floatBytes, MiB, GiB float64
	var suffix string
	floatBytes = float64(bytes)
	MiB = 1024 * 1024
	GiB = MiB * 1024
	// Need to give Quantity nice whole numbers or else it
	// sometimes spits out the value in milibytes. We round up.
	if floatBytes < GiB {
		num = int64(math.Ceil(floatBytes / MiB))
		suffix = "Mi"
	} else {
		num = int64(math.Ceil(floatBytes / GiB))
		suffix = "Gi"
	}
	stringQuantity := fmt.Sprintf("%v%s", num, suffix)
	return resource.MustParse(stringQuantity)
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

func isFinalError(err error) bool {
	// Sources:
	// https://github.com/grpc/grpc/blob/master/doc/statuscodes.md
	// https://github.com/container-storage-interface/spec/blob/master/spec.md
	st, ok := status.FromError(err)
	if !ok {
		// This is not gRPC error. The operation must have failed before gRPC
		// method was called, otherwise we would get gRPC error.
		// We don't know if any previous CreateVolume is in progress, be on the safe side.
		return false
	}
	switch st.Code() {
	case codes.Canceled, // gRPC: Client Application cancelled the request
		codes.DeadlineExceeded,  // gRPC: Timeout
		codes.Unavailable,       // gRPC: Server shutting down, TCP connection broken - previous CreateVolume() may be still in progress.
		codes.ResourceExhausted, // gRPC: Server temporarily out of resources - previous CreateVolume() may be still in progress.
		codes.Aborted:           // CSI: Operation pending for volume
		return false
	}
	// All other errors mean that provisioning either did not
	// even start or failed. It is for sure not in progress.
	return true
}
