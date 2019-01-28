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
	"fmt"
	"math"
	"net"
	"os"
	"strings"
	"time"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	_ "k8s.io/apimachinery/pkg/util/json"

	"k8s.io/klog"

	"github.com/kubernetes-sigs/sig-storage-lib-external-provisioner/controller"
	"github.com/kubernetes-sigs/sig-storage-lib-external-provisioner/util"

	"github.com/kubernetes-csi/csi-lib-utils/protosanitizer"
	snapapi "github.com/kubernetes-csi/external-snapshotter/pkg/apis/volumesnapshot/v1alpha1"
	snapclientset "github.com/kubernetes-csi/external-snapshotter/pkg/client/clientset/versioned"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/status"

	"github.com/container-storage-interface/spec/lib/go/csi"
	csiclientset "k8s.io/csi-api/pkg/client/clientset/versioned"

	"github.com/kubernetes-csi/external-provisioner/pkg/features"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
)

type deprecatedSecretParamsMap struct {
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
	snapshotAPIGroup = snapapi.GroupName // "snapshot.storage.k8s.io"
)

var (
	provisionerSecretParams = deprecatedSecretParamsMap{
		name:                         "Provisioner",
		deprecatedSecretNameKey:      provisionerSecretNameKey,
		deprecatedSecretNamespaceKey: provisionerSecretNamespaceKey,
		secretNameKey:                prefixedProvisionerSecretNameKey,
		secretNamespaceKey:           prefixedProvisionerSecretNamespaceKey,
	}

	nodePublishSecretParams = deprecatedSecretParamsMap{
		name:                         "NodePublish",
		deprecatedSecretNameKey:      nodePublishSecretNameKey,
		deprecatedSecretNamespaceKey: nodePublishSecretNamespaceKey,
		secretNameKey:                prefixedNodePublishSecretNameKey,
		secretNamespaceKey:           prefixedNodePublishSecretNamespaceKey,
	}

	controllerPublishSecretParams = deprecatedSecretParamsMap{
		name:                         "ControllerPublish",
		deprecatedSecretNameKey:      controllerPublishSecretNameKey,
		deprecatedSecretNamespaceKey: controllerPublishSecretNamespaceKey,
		secretNameKey:                prefixedControllerPublishSecretNameKey,
		secretNamespaceKey:           prefixedControllerPublishSecretNamespaceKey,
	}

	nodeStageSecretParams = deprecatedSecretParamsMap{
		name:                         "NodeStage",
		deprecatedSecretNameKey:      nodeStageSecretNameKey,
		deprecatedSecretNamespaceKey: nodeStageSecretNamespaceKey,
		secretNameKey:                prefixedNodeStageSecretNameKey,
		secretNamespaceKey:           prefixedNodeStageSecretNamespaceKey,
	}
)

// CSIProvisioner struct
type csiProvisioner struct {
	client               kubernetes.Interface
	csiClient            csi.ControllerClient
	csiAPIClient         csiclientset.Interface
	grpcClient           *grpc.ClientConn
	snapshotClient       snapclientset.Interface
	timeout              time.Duration
	identity             string
	volumeNamePrefix     string
	volumeNameUUIDLength int
	config               *rest.Config
	driverName           string
}

const (
	PluginCapability_CONTROLLER_SERVICE = iota
	PluginCapability_ACCESSIBILITY_CONSTRAINTS
	ControllerCapability_CREATE_DELETE_VOLUME
	ControllerCapability_CREATE_DELETE_SNAPSHOT
)

var _ controller.Provisioner = &csiProvisioner{}
var _ controller.BlockProvisioner = &csiProvisioner{}

var (
	// Each provisioner have a identify string to distinguish with others. This
	// identify string will be added in PV annoations under this key.
	provisionerIDKey = "storage.kubernetes.io/csiProvisionerIdentity"
)

// from external-attacher/pkg/connection
//TODO consolidate ane librarize
func logGRPC(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	klog.V(5).Infof("GRPC call: %s", method)
	klog.V(5).Infof("GRPC request: %s", protosanitizer.StripSecrets(req))
	err := invoker(ctx, method, req, reply, cc, opts...)
	klog.V(5).Infof("GRPC response: %s", protosanitizer.StripSecrets(reply))
	klog.V(5).Infof("GRPC error: %v", err)
	return err
}

func Connect(address string, timeout time.Duration) (*grpc.ClientConn, error) {
	klog.V(2).Infof("Connecting to %s", address)
	dialOptions := []grpc.DialOption{
		grpc.WithInsecure(),
		grpc.WithBackoffMaxDelay(time.Second),
		grpc.WithUnaryInterceptor(logGRPC),
	}
	if strings.HasPrefix(address, "/") {
		dialOptions = append(dialOptions, grpc.WithDialer(func(addr string, timeout time.Duration) (net.Conn, error) {
			return net.DialTimeout("unix", addr, timeout)
		}))
	}
	conn, err := grpc.Dial(address, dialOptions...)

	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	for {
		if !conn.WaitForStateChange(ctx, conn.GetState()) {
			klog.V(4).Infof("Connection timed out")
			return conn, fmt.Errorf("Connection timed out")
		}
		if conn.GetState() == connectivity.Ready {
			klog.V(3).Infof("Connected")
			return conn, nil
		}
		klog.V(4).Infof("Still trying, connection is %s", conn.GetState())
	}
}

func GetDriverName(conn *grpc.ClientConn, timeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	client := csi.NewIdentityClient(conn)

	req := csi.GetPluginInfoRequest{}

	rsp, err := client.GetPluginInfo(ctx, &req)
	if err != nil {
		return "", err
	}
	name := rsp.GetName()
	if name == "" {
		return "", fmt.Errorf("name is empty")
	}
	return name, nil
}

func getDriverCapabilities(conn *grpc.ClientConn, timeout time.Duration) (sets.Int, error) {
	pluginCaps, err := getPluginCapabilities(conn, timeout)
	if err != nil {
		return nil, err
	}

	controllerCaps, err := getControllerCapabilities(conn, timeout)
	if err != nil {
		return nil, err
	}

	capabilities := make(sets.Int)
	for _, cap := range pluginCaps {
		if cap == nil {
			continue
		}
		service := cap.GetService()
		if service == nil {
			continue
		}
		switch service.GetType() {
		case csi.PluginCapability_Service_CONTROLLER_SERVICE:
			capabilities.Insert(PluginCapability_CONTROLLER_SERVICE)
		case csi.PluginCapability_Service_VOLUME_ACCESSIBILITY_CONSTRAINTS:
			capabilities.Insert(PluginCapability_ACCESSIBILITY_CONSTRAINTS)
		}
	}
	for _, cap := range controllerCaps {
		if cap == nil {
			continue
		}
		rpc := cap.GetRpc()
		if rpc == nil {
			continue
		}
		switch rpc.GetType() {
		case csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME:
			capabilities.Insert(ControllerCapability_CREATE_DELETE_VOLUME)
		case csi.ControllerServiceCapability_RPC_CREATE_DELETE_SNAPSHOT:
			capabilities.Insert(ControllerCapability_CREATE_DELETE_SNAPSHOT)
		}
	}
	return capabilities, nil
}

func getPluginCapabilities(conn *grpc.ClientConn, timeout time.Duration) ([]*csi.PluginCapability, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	client := csi.NewIdentityClient(conn)
	req := csi.GetPluginCapabilitiesRequest{}

	rsp, err := client.GetPluginCapabilities(ctx, &req)
	if err != nil {
		return nil, err
	}
	return rsp.GetCapabilities(), nil
}

func getControllerCapabilities(conn *grpc.ClientConn, timeout time.Duration) ([]*csi.ControllerServiceCapability, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	client := csi.NewControllerClient(conn)
	req := csi.ControllerGetCapabilitiesRequest{}

	rsp, err := client.ControllerGetCapabilities(ctx, &req)
	if err != nil {
		return nil, err
	}
	return rsp.GetCapabilities(), nil
}

// NewCSIProvisioner creates new CSI provisioner
func NewCSIProvisioner(client kubernetes.Interface,
	csiAPIClient csiclientset.Interface,
	csiEndpoint string,
	connectionTimeout time.Duration,
	identity string,
	volumeNamePrefix string,
	volumeNameUUIDLength int,
	grpcClient *grpc.ClientConn,
	snapshotClient snapclientset.Interface,
	driverName string) controller.Provisioner {

	csiClient := csi.NewControllerClient(grpcClient)
	provisioner := &csiProvisioner{
		client:               client,
		grpcClient:           grpcClient,
		csiClient:            csiClient,
		csiAPIClient:         csiAPIClient,
		snapshotClient:       snapshotClient,
		timeout:              connectionTimeout,
		identity:             identity,
		volumeNamePrefix:     volumeNamePrefix,
		volumeNameUUIDLength: volumeNameUUIDLength,
		driverName:           driverName,
	}
	return provisioner
}

// This function get called before any attempt to communicate with the driver.
// Before initiating Create/Delete API calls provisioner checks if Capabilities:
// PluginControllerService,  ControllerCreateVolume sre supported and gets the  driver name.
func checkDriverCapabilities(grpcClient *grpc.ClientConn, timeout time.Duration, needSnapshotSupport bool) (sets.Int, error) {
	capabilities, err := getDriverCapabilities(grpcClient, timeout)
	if err != nil {
		return nil, fmt.Errorf("failed to get capabilities: %v", err)
	}

	if !capabilities.Has(PluginCapability_CONTROLLER_SERVICE) {
		return nil, fmt.Errorf("no plugin controller service support detected")
	}

	if !capabilities.Has(ControllerCapability_CREATE_DELETE_VOLUME) {
		return nil, fmt.Errorf("no create/delete volume support detected")
	}

	// If PVC.Spec.DataSource is not nil, it indicates the request is to create volume
	// from snapshot and therefore we should check for snapshot support;
	// otherwise we don't need to check for snapshot support.
	if needSnapshotSupport {
		// Check whether plugin supports create snapshot
		// If not, create volume from snapshot cannot proceed
		if !capabilities.Has(ControllerCapability_CREATE_DELETE_SNAPSHOT) {
			return nil, fmt.Errorf("no create/delete snapshot support detected. Cannot create volume from snapshot")
		}
	}

	return capabilities, nil
}

func makeVolumeName(prefix, pvcUID string, volumeNameUUIDLength int) (string, error) {
	// create persistent name based on a volumeNamePrefix and volumeNameUUIDLength
	// of PVC's UID
	if len(prefix) == 0 {
		return "", fmt.Errorf("Volume name prefix cannot be of length 0")
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
	pvcOptions controller.VolumeOptions,
	pvcAccessMode v1.PersistentVolumeAccessMode,
	fsType string,
) *csi.VolumeCapability {
	if util.CheckPersistentVolumeClaimModeBlock(pvcOptions.PVC) {
		return &csi.VolumeCapability{
			AccessType: getAccessTypeBlock(),
			AccessMode: getAccessMode(pvcAccessMode),
		}
	}
	return &csi.VolumeCapability{
		AccessType: getAccessTypeMount(fsType, pvcOptions.MountOptions),
		AccessMode: getAccessMode(pvcAccessMode),
	}

}

func (p *csiProvisioner) Provision(options controller.VolumeOptions) (*v1.PersistentVolume, error) {
	if options.PVC.Spec.Selector != nil {
		return nil, fmt.Errorf("claim Selector is not supported")
	}

	var needSnapshotSupport bool
	if options.PVC.Spec.DataSource != nil {
		// PVC.Spec.DataSource.Name is the name of the VolumeSnapshot API object
		if options.PVC.Spec.DataSource.Name == "" {
			return nil, fmt.Errorf("the PVC source not found for PVC %s", options.PVC.Name)
		}
		if options.PVC.Spec.DataSource.Kind != snapshotKind {
			return nil, fmt.Errorf("the PVC source is not the right type. Expected %s, Got %s", snapshotKind, options.PVC.Spec.DataSource.Kind)
		}
		if *(options.PVC.Spec.DataSource.APIGroup) != snapshotAPIGroup {
			return nil, fmt.Errorf("the PVC source does not belong to the right APIGroup. Expected %s, Got %s", snapshotAPIGroup, *(options.PVC.Spec.DataSource.APIGroup))
		}
		needSnapshotSupport = true
	}
	capabilities, err := checkDriverCapabilities(p.grpcClient, p.timeout, needSnapshotSupport)
	if err != nil {
		return nil, err
	}

	pvName, err := makeVolumeName(p.volumeNamePrefix, fmt.Sprintf("%s", options.PVC.ObjectMeta.UID), p.volumeNameUUIDLength)
	if err != nil {
		return nil, err
	}

	fsTypesFound := 0
	fsType := ""
	for k, v := range options.Parameters {
		if strings.ToLower(k) == "fstype" {
			fsType = v
			fsTypesFound++
			klog.Warningf(deprecationWarning("fstype", prefixedFsTypeKey, ""))
		} else if k == prefixedFsTypeKey {
			fsType = v
			fsTypesFound++
		}
	}
	if fsTypesFound > 1 {
		return nil, fmt.Errorf("fstype specified in parameters with both \"fstype\" and \"%s\" keys", prefixedFsTypeKey)
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
		Parameters:         options.Parameters,
		VolumeCapabilities: volumeCaps,
		CapacityRange: &csi.CapacityRange{
			RequiredBytes: int64(volSizeBytes),
		},
	}

	if needSnapshotSupport {
		volumeContentSource, err := p.getVolumeContentSource(options)
		if err != nil {
			return nil, fmt.Errorf("error getting snapshot handle for snapshot %s: %v", options.PVC.Spec.DataSource.Name, err)
		}
		req.VolumeContentSource = volumeContentSource
	}

	if capabilities.Has(PluginCapability_ACCESSIBILITY_CONSTRAINTS) &&
		utilfeature.DefaultFeatureGate.Enabled(features.Topology) {
		requirements, err := GenerateAccessibilityRequirements(
			p.client,
			p.csiAPIClient,
			p.driverName,
			options.PVC.Name,
			options.AllowedTopologies,
			options.SelectedNode)
		if err != nil {
			return nil, fmt.Errorf("error generating accessibility requirements: %v", err)
		}
		req.AccessibilityRequirements = requirements
	}

	klog.V(5).Infof("CreateVolumeRequest %+v", req)

	rep := &csi.CreateVolumeResponse{}

	// Resolve provision secret credentials.
	// No PVC is provided when resolving provision/delete secret names, since the PVC may or may not exist at delete time.
	provisionerSecretRef, err := getSecretReference(provisionerSecretParams, options.Parameters, pvName, nil)
	if err != nil {
		return nil, err
	}
	provisionerCredentials, err := getCredentials(p.client, provisionerSecretRef)
	if err != nil {
		return nil, err
	}
	req.Secrets = provisionerCredentials

	// Resolve controller publish, node stage, node publish secret references
	controllerPublishSecretRef, err := getSecretReference(controllerPublishSecretParams, options.Parameters, pvName, options.PVC)
	if err != nil {
		return nil, err
	}
	nodeStageSecretRef, err := getSecretReference(nodeStageSecretParams, options.Parameters, pvName, options.PVC)
	if err != nil {
		return nil, err
	}
	nodePublishSecretRef, err := getSecretReference(nodePublishSecretParams, options.Parameters, pvName, options.PVC)
	if err != nil {
		return nil, err
	}

	req.Parameters, err = removePrefixedParameters(options.Parameters)
	if err != nil {
		return nil, fmt.Errorf("failed to strip CSI Parameters of prefixed keys: %v", err)
	}

	opts := wait.Backoff{Duration: backoffDuration, Factor: backoffFactor, Steps: backoffSteps}
	err = wait.ExponentialBackoff(opts, func() (bool, error) {
		ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
		defer cancel()
		rep, err = p.csiClient.CreateVolume(ctx, &req)
		if err == nil {
			// CreateVolume has finished successfully
			return true, nil
		}

		if status, ok := status.FromError(err); ok {
			if status.Code() == codes.DeadlineExceeded {
				// CreateVolume timed out, give it another chance to complete
				klog.Warningf("CreateVolume timeout: %s has expired, operation will be retried", p.timeout.String())
				return false, nil
			}
		}
		// CreateVolume failed , no reason to retry, bailing from ExponentialBackoff
		return false, err
	})

	if err != nil {
		return nil, err
	}

	if rep.Volume != nil {
		klog.V(3).Infof("create volume rep: %+v", *rep.Volume)
	}
	volumeAttributes := map[string]string{provisionerIDKey: p.identity}
	for k, v := range rep.Volume.VolumeContext {
		volumeAttributes[k] = v
	}
	respCap := rep.GetVolume().GetCapacityBytes()
	if respCap < volSizeBytes {
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
		return nil, capErr
	}

	pv := &v1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: pvName,
		},
		Spec: v1.PersistentVolumeSpec{
			PersistentVolumeReclaimPolicy: options.PersistentVolumeReclaimPolicy,
			AccessModes:                   options.PVC.Spec.AccessModes,
			MountOptions:                  options.MountOptions,
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
				},
			},
		},
	}

	if capabilities.Has(PluginCapability_ACCESSIBILITY_CONSTRAINTS) &&
		utilfeature.DefaultFeatureGate.Enabled(features.Topology) {
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

	klog.Infof("successfully created PV %+v", pv.Spec.PersistentVolumeSource)

	return pv, nil
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

func (p *csiProvisioner) getVolumeContentSource(options controller.VolumeOptions) (*csi.VolumeContentSource, error) {
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
	if volume == nil || volume.Spec.CSI == nil {
		return fmt.Errorf("invalid CSI PV")
	}
	volumeId := p.volumeHandleToId(volume.Spec.CSI.VolumeHandle)

	_, err := checkDriverCapabilities(p.grpcClient, p.timeout, false)
	if err != nil {
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
			// No PVC is provided when resolving provision/delete secret names, since the PVC may or may not exist at delete time.
			provisionerSecretRef, err := getSecretReference(provisionerSecretParams, storageClass.Parameters, volume.Name, nil)
			if err != nil {
				return err
			}
			credentials, err := getCredentials(p.client, provisionerSecretRef)
			if err != nil {
				return err
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
func verifyAndGetSecretNameAndNamespaceTemplate(secret deprecatedSecretParamsMap, storageClassParams map[string]string) (nameTemplate, namespaceTemplate string, err error) {
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
func getSecretReference(secretParams deprecatedSecretParamsMap, storageClassParams map[string]string, pvName string, pvc *v1.PersistentVolumeClaim) (*v1.SecretReference, error) {
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
		namespaceParams := map[string]string{"pv.name": pvName}
		if pvc != nil {
			namespaceParams["pvc.namespace"] = pvc.Namespace
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
		nameParams := map[string]string{"pv.name": pvName}
		if pvc != nil {
			nameParams["pvc.name"] = pvc.Name
			nameParams["pvc.namespace"] = pvc.Namespace
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
