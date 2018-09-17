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

	"k8s.io/apimachinery/pkg/api/resource"
	_ "k8s.io/apimachinery/pkg/util/json"

	"github.com/golang/glog"

	"github.com/kubernetes-incubator/external-storage/lib/controller"

	snapapi "github.com/kubernetes-csi/external-snapshotter/pkg/apis/volumesnapshot/v1alpha1"
	snapclientset "github.com/kubernetes-csi/external-snapshotter/pkg/client/clientset/versioned"

	"k8s.io/api/core/v1"
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

	"github.com/container-storage-interface/spec/lib/go/csi/v0"
)

const (
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

// CSIProvisioner struct
type csiProvisioner struct {
	client               kubernetes.Interface
	csiClient            csi.ControllerClient
	grpcClient           *grpc.ClientConn
	snapshotClient       snapclientset.Interface
	timeout              time.Duration
	identity             string
	volumeNamePrefix     string
	volumeNameUUIDLength int
	config               *rest.Config
}

var _ controller.Provisioner = &csiProvisioner{}

var (
	accessType = &csi.VolumeCapability_Mount{
		Mount: &csi.VolumeCapability_MountVolume{},
	}
	// Each provisioner have a identify string to distinguish with others. This
	// identify string will be added in PV annoations under this key.
	provisionerIDKey = "storage.kubernetes.io/csiProvisionerIdentity"
)

// from external-attacher/pkg/connection
//TODO consolidate ane librarize
func logGRPC(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	glog.V(5).Infof("GRPC call: %s", method)
	glog.V(5).Infof("GRPC request: %+v", req)
	err := invoker(ctx, method, req, reply, cc, opts...)
	glog.V(5).Infof("GRPC response: %+v", reply)
	glog.V(5).Infof("GRPC error: %v", err)
	return err
}

func Connect(address string, timeout time.Duration) (*grpc.ClientConn, error) {
	glog.V(2).Infof("Connecting to %s", address)
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
			glog.V(4).Infof("Connection timed out")
			return conn, fmt.Errorf("Connection timed out")
		}
		if conn.GetState() == connectivity.Ready {
			glog.V(3).Infof("Connected")
			return conn, nil
		}
		glog.V(4).Infof("Still trying, connection is %s", conn.GetState())
	}
}

func getDriverName(conn *grpc.ClientConn, timeout time.Duration) (string, error) {
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

func supportsPluginControllerService(conn *grpc.ClientConn, timeout time.Duration) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	client := csi.NewIdentityClient(conn)
	req := csi.GetPluginCapabilitiesRequest{}

	rsp, err := client.GetPluginCapabilities(ctx, &req)
	if err != nil {
		return false, err
	}
	caps := rsp.GetCapabilities()
	for _, cap := range caps {
		if cap == nil {
			continue
		}
		service := cap.GetService()
		if service == nil {
			continue
		}
		if service.GetType() == csi.PluginCapability_Service_CONTROLLER_SERVICE {
			return true, nil
		}
	}
	return false, nil
}

func supportsControllerCreateVolume(conn *grpc.ClientConn, timeout time.Duration) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	client := csi.NewControllerClient(conn)
	req := csi.ControllerGetCapabilitiesRequest{}

	rsp, err := client.ControllerGetCapabilities(ctx, &req)
	if err != nil {
		return false, err
	}
	caps := rsp.GetCapabilities()
	for _, cap := range caps {
		if cap == nil {
			continue
		}
		rpc := cap.GetRpc()
		if rpc == nil {
			continue
		}
		if rpc.GetType() == csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME {
			return true, nil
		}
	}
	return false, nil
}

func supportsControllerCreateSnapshot(conn *grpc.ClientConn, timeout time.Duration) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	client := csi.NewControllerClient(conn)
	req := csi.ControllerGetCapabilitiesRequest{}

	rsp, err := client.ControllerGetCapabilities(ctx, &req)
	if err != nil {
		return false, err
	}
	caps := rsp.GetCapabilities()
	for _, cap := range caps {
		if cap == nil {
			continue
		}
		rpc := cap.GetRpc()
		if rpc == nil {
			continue
		}
		if rpc.GetType() == csi.ControllerServiceCapability_RPC_CREATE_DELETE_SNAPSHOT {
			return true, nil
		}
	}
	return false, nil
}

// NewCSIProvisioner creates new CSI provisioner
func NewCSIProvisioner(client kubernetes.Interface,
	csiEndpoint string,
	connectionTimeout time.Duration,
	identity string,
	volumeNamePrefix string,
	volumeNameUUIDLength int,
	grpcClient *grpc.ClientConn,
	snapshotClient snapclientset.Interface) controller.Provisioner {

	csiClient := csi.NewControllerClient(grpcClient)
	provisioner := &csiProvisioner{
		client:               client,
		grpcClient:           grpcClient,
		csiClient:            csiClient,
		snapshotClient:       snapshotClient,
		timeout:              connectionTimeout,
		identity:             identity,
		volumeNamePrefix:     volumeNamePrefix,
		volumeNameUUIDLength: volumeNameUUIDLength,
	}
	return provisioner
}

// This function get called before any attepmt to communicate with the driver.
// Before initiating Create/Delete API calls provisioner checks if Capabilities:
// PluginControllerService,  ControllerCreateVolume sre supported and gets the  driver name.
func checkDriverState(grpcClient *grpc.ClientConn, timeout time.Duration, needSnapshotSupport bool) (string, error) {
	ok, err := supportsPluginControllerService(grpcClient, timeout)
	if err != nil {
		glog.Errorf("failed to get support info :%v", err)
		return "", err
	}
	if !ok {
		glog.Errorf("no plugin controller service support detected")
		return "", fmt.Errorf("no plugin controller service support detected")
	}
	ok, err = supportsControllerCreateVolume(grpcClient, timeout)
	if err != nil {
		glog.Errorf("failed to get support info :%v", err)
		return "", err
	}
	if !ok {
		glog.Error("no create/delete volume support detected")
		return "", fmt.Errorf("no create/delete volume support detected")
	}

	// If PVC.Spec.DataSource is not nil, it indicates the request is to create volume
	// from snapshot and therefore we should check for snapshot support;
	// otherwise we don't need to check for snapshot support.
	if needSnapshotSupport {
		// Check whether plugin supports create snapshot
		// If not, create volume from snapshot cannot proceed
		ok, err = supportsControllerCreateSnapshot(grpcClient, timeout)
		if err != nil {
			glog.Errorf("failed to get support info :%v", err)
			return "", err
		}
		if !ok {
			glog.Error("no create/delete snapshot support detected. Cannot create volume from shapshot")
			return "", fmt.Errorf("no create/delete snapshot support detected. Cannot create volume from shapshot")
		}
	}

	driverName, err := getDriverName(grpcClient, timeout)
	if err != nil {
		glog.Errorf("failed to get driver info :%v", err)
		return "", err
	}
	return driverName, nil
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
	} else {
		// Else we remove all dashes from UUID and truncate to volumeNameUUIDLength
		return fmt.Sprintf("%s-%s", prefix, strings.Replace(string(pvcUID), "-", "", -1)[0:volumeNameUUIDLength]), nil
	}

}

func (p *csiProvisioner) Provision(options controller.VolumeOptions) (*v1.PersistentVolume, error) {
	if options.PVC.Spec.Selector != nil {
		return nil, fmt.Errorf("claim Selector is not supported")
	}

	var needSnapshotSupport bool = false
	if options.PVC.Spec.DataSource != nil {
		// PVC.Spec.DataSource.Name is the name of the VolumeSnapshot API object
		if options.PVC.Spec.DataSource.Name == "" {
			return nil, fmt.Errorf("the PVC source not found for PVC %s", options.PVC.Name)
		}
		if options.PVC.Spec.DataSource.Kind != snapshotKind {
			return nil, fmt.Errorf("the PVC source is not the right type. Expected %s, Got %s", snapshotKind, options.PVC.Spec.DataSource.Kind)
		}
		if options.PVC.Spec.DataSource.APIGroup != snapshotAPIGroup {
			return nil, fmt.Errorf("the PVC source does not belong to the right APIGroup. Expected %s, Got %s", snapshotAPIGroup, options.PVC.Spec.DataSource.APIGroup)
		}
		needSnapshotSupport = true
	}
	driverName, err := checkDriverState(p.grpcClient, p.timeout, needSnapshotSupport)
	if err != nil {
		return nil, err
	}

	pvName, err := makeVolumeName(p.volumeNamePrefix, fmt.Sprintf("%s", options.PVC.ObjectMeta.UID), p.volumeNameUUIDLength)
	if err != nil {
		return nil, err
	}

	capacity := options.PVC.Spec.Resources.Requests[v1.ResourceName(v1.ResourceStorage)]
	volSizeBytes := capacity.Value()

	// Get access mode
	volumeCaps := make([]*csi.VolumeCapability, 0)
	for _, cap := range options.PVC.Spec.AccessModes {
		switch cap {
		case v1.ReadWriteOnce:
			volumeCaps = append(volumeCaps, &csi.VolumeCapability{
				AccessMode: &csi.VolumeCapability_AccessMode{
					Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
				},
				AccessType: accessType,
			})
		case v1.ReadWriteMany:
			volumeCaps = append(volumeCaps, &csi.VolumeCapability{
				AccessMode: &csi.VolumeCapability_AccessMode{
					Mode: csi.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER,
				},
				AccessType: accessType,
			})
		case v1.ReadOnlyMany:
			volumeCaps = append(volumeCaps, &csi.VolumeCapability{
				AccessMode: &csi.VolumeCapability_AccessMode{
					Mode: csi.VolumeCapability_AccessMode_MULTI_NODE_READER_ONLY,
				},
				AccessType: accessType,
			})
		}
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

	glog.V(5).Infof("CreateVolumeRequest %+v", req)

	rep := &csi.CreateVolumeResponse{}

	// Resolve provision secret credentials.
	// No PVC is provided when resolving provision/delete secret names, since the PVC may or may not exist at delete time.
	provisionerSecretRef, err := getSecretReference(provisionerSecretNameKey, provisionerSecretNamespaceKey, options.Parameters, pvName, nil)
	if err != nil {
		return nil, err
	}
	provisionerCredentials, err := getCredentials(p.client, provisionerSecretRef)
	if err != nil {
		return nil, err
	}
	req.ControllerCreateSecrets = provisionerCredentials

	// Resolve controller publish, node stage, node publish secret references
	controllerPublishSecretRef, err := getSecretReference(controllerPublishSecretNameKey, controllerPublishSecretNamespaceKey, options.Parameters, pvName, options.PVC)
	if err != nil {
		return nil, err
	}
	nodeStageSecretRef, err := getSecretReference(nodeStageSecretNameKey, nodeStageSecretNamespaceKey, options.Parameters, pvName, options.PVC)
	if err != nil {
		return nil, err
	}
	nodePublishSecretRef, err := getSecretReference(nodePublishSecretNameKey, nodePublishSecretNamespaceKey, options.Parameters, pvName, options.PVC)
	if err != nil {
		return nil, err
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
				glog.Warningf("CreateVolume timeout: %s has expired, operation will be retried", p.timeout.String())
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
		glog.V(3).Infof("create volume rep: %+v", *rep.Volume)
	}
	volumeAttributes := map[string]string{provisionerIDKey: p.identity}
	for k, v := range rep.Volume.Attributes {
		volumeAttributes[k] = v
	}
	respCap := rep.GetVolume().GetCapacityBytes()
	if respCap < volSizeBytes {
		capErr := fmt.Errorf("created volume capacity %v less than requested capacity %v", respCap, volSizeBytes)
		delReq := &csi.DeleteVolumeRequest{
			VolumeId: rep.GetVolume().GetId(),
		}
		delReq.ControllerDeleteSecrets = provisionerCredentials
		ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
		defer cancel()
		_, err := p.csiClient.DeleteVolume(ctx, delReq)
		if err != nil {
			capErr = fmt.Errorf("%v. Cleanup of volume %s failed, volume is orphaned: %v", capErr, pvName, err)
		}
		return nil, capErr
	}

	fsType := ""
	for k, v := range options.Parameters {
		switch strings.ToLower(k) {
		case "fstype":
			fsType = v
		}
	}
	if len(fsType) == 0 {
		fsType = defaultFSType
	}
	pv := &v1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: pvName,
		},
		Spec: v1.PersistentVolumeSpec{
			PersistentVolumeReclaimPolicy: options.PersistentVolumeReclaimPolicy,
			AccessModes:                   options.PVC.Spec.AccessModes,
			Capacity: v1.ResourceList{
				v1.ResourceName(v1.ResourceStorage): bytesToGiQuantity(respCap),
			},
			// TODO wait for CSI VolumeSource API
			PersistentVolumeSource: v1.PersistentVolumeSource{
				CSI: &v1.CSIPersistentVolumeSource{
					Driver:                     driverName,
					VolumeHandle:               p.volumeIdToHandle(rep.Volume.Id),
					FSType:                     fsType,
					VolumeAttributes:           volumeAttributes,
					ControllerPublishSecretRef: controllerPublishSecretRef,
					NodeStageSecretRef:         nodeStageSecretRef,
					NodePublishSecretRef:       nodePublishSecretRef,
				},
			},
		},
	}

	glog.Infof("successfully created PV %+v", pv.Spec.PersistentVolumeSource)

	return pv, nil
}

func (p *csiProvisioner) getVolumeContentSource(options controller.VolumeOptions) (*csi.VolumeContentSource, error) {
	snapshotObj, err := p.snapshotClient.VolumesnapshotV1alpha1().VolumeSnapshots(options.PVC.Namespace).Get(options.PVC.Spec.DataSource.Name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("error getting snapshot %s from api server: %v", options.PVC.Spec.DataSource.Name, err)
	}
	if snapshotObj.Status.Ready == false {
		return nil, fmt.Errorf("snapshot %s is not Ready", options.PVC.Spec.DataSource.Name)
	}

	if snapshotObj.ObjectMeta.DeletionTimestamp != nil {
		return nil, fmt.Errorf("snapshot %s is currently being deleted", options.PVC.Spec.DataSource.Name)
	}
	glog.V(5).Infof("VolumeSnapshot %+v", snapshotObj)

	snapContentObj, err := p.snapshotClient.VolumesnapshotV1alpha1().VolumeSnapshotContents().Get(snapshotObj.Spec.SnapshotContentName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("error getting snapshot:snapshotcontent %s:%s from api server: %v", snapshotObj.Name, snapshotObj.Spec.SnapshotContentName, err)
	}
	glog.V(5).Infof("VolumeSnapshotContent %+v", snapContentObj)

	if snapContentObj.Spec.VolumeSnapshotSource.CSI == nil {
		return nil, fmt.Errorf("error getting snapshot source from snapshot:snapshotcontent %s:%s", snapshotObj.Name, snapshotObj.Spec.SnapshotContentName)
	}

	snapshotSource := csi.VolumeContentSource_Snapshot{
		Snapshot: &csi.VolumeContentSource_SnapshotSource{
			Id: snapContentObj.Spec.VolumeSnapshotSource.CSI.SnapshotHandle,
		},
	}
	glog.V(5).Infof("VolumeContentSource_Snapshot %+v", snapshotSource)

	if snapshotObj.Status.RestoreSize != nil {
		capacity, exists := options.PVC.Spec.Resources.Requests[v1.ResourceName(v1.ResourceStorage)]
		if !exists {
			return nil, fmt.Errorf("error getting capacity for PVC %s when creating snapshot %s", options.PVC.Name, snapshotObj.Name)
		}
		volSizeBytes := capacity.Value()
		glog.V(5).Infof("Requested volume size is %d and snapshot size is %d for the source snapshot %s", int64(volSizeBytes), int64(snapshotObj.Status.RestoreSize.Value()), snapshotObj.Name)
		// When restoring volume from a snapshot, the volume size should
		// be equal to or larger than its snapshot size.
		if int64(volSizeBytes) < int64(snapshotObj.Status.RestoreSize.Value()) {
			return nil, fmt.Errorf("requested volume size %d is less than the size %d for the source snapshot %s", int64(volSizeBytes), int64(snapshotObj.Status.RestoreSize.Value()), snapshotObj.Name)
		}
		if int64(volSizeBytes) > int64(snapshotObj.Status.RestoreSize.Value()) {
			glog.Warningf("requested volume size %d is greater than the size %d for the source snapshot %s. Volume plugin needs to handle volume expansion.", int64(volSizeBytes), int64(snapshotObj.Status.RestoreSize.Value()), snapshotObj.Name)
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

	_, err := checkDriverState(p.grpcClient, p.timeout, false)
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
			provisionerSecretRef, err := getSecretReference(provisionerSecretNameKey, provisionerSecretNamespaceKey, storageClass.Parameters, volume.Name, nil)
			if err != nil {
				return err
			}
			credentials, err := getCredentials(p.client, provisionerSecretRef)
			if err != nil {
				return err
			}
			req.ControllerDeleteSecrets = credentials
		}

	}
	ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()

	_, err = p.csiClient.DeleteVolume(ctx, &req)

	return err
}

//TODO use a unique volume handle from and to Id
func (p *csiProvisioner) volumeIdToHandle(id string) string {
	return id
}

func (p *csiProvisioner) volumeHandleToId(handle string) string {
	return handle
}

// getSecretReference returns a reference to the secret specified in the given nameKey and namespaceKey parameters, or an error if the parameters are not specified correctly.
// if neither the name or namespace parameter are set, a nil reference and no error is returned.
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
// - only one of name or namespace is provided
// - the name or namespace parameter contains a token that cannot be resolved
// - the resolved name is not a valid secret name
// - the resolved namespace is not a valid namespace name
func getSecretReference(nameKey, namespaceKey string, storageClassParams map[string]string, pvName string, pvc *v1.PersistentVolumeClaim) (*v1.SecretReference, error) {
	nameTemplate, hasName := storageClassParams[nameKey]
	namespaceTemplate, hasNamespace := storageClassParams[namespaceKey]

	if !hasName && !hasNamespace {
		return nil, nil
	}

	if len(nameTemplate) == 0 || len(namespaceTemplate) == 0 {
		return nil, fmt.Errorf("%s and %s parameters must be specified together", nameKey, namespaceKey)
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
			return nil, fmt.Errorf("error resolving %s value %q: %v", namespaceKey, namespaceTemplate, err)
		}
		if len(validation.IsDNS1123Label(resolvedNamespace)) > 0 {
			if namespaceTemplate != resolvedNamespace {
				return nil, fmt.Errorf("%s parameter %q resolved to %q which is not a valid namespace name", namespaceKey, namespaceTemplate, resolvedNamespace)
			}
			return nil, fmt.Errorf("%s parameter %q is not a valid namespace name", namespaceKey, namespaceTemplate)
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
			return nil, fmt.Errorf("error resolving %s value %q: %v", nameKey, nameTemplate, err)
		}
		if len(validation.IsDNS1123Subdomain(resolvedName)) > 0 {
			if nameTemplate != resolvedName {
				return nil, fmt.Errorf("%s parameter %q resolved to %q which is not a valid secret name", nameKey, nameTemplate, resolvedName)
			}
			return nil, fmt.Errorf("%s parameter %q is not a valid secret name", nameKey, nameTemplate)
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
