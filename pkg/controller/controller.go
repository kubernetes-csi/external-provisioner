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
	jsonv2 "encoding/json"
	"fmt"
	"k8s.io/apimachinery/pkg/util/json"
	"net"
	"strings"
	"time"

	"github.com/golang/glog"
	"github.com/pborman/uuid"

	"github.com/kubernetes-incubator/external-storage/lib/controller"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/container-storage-interface/spec/lib/go/csi"
)

const (
	volumeAttributesAnnotation = "csi.volume.kubernetes.io/volume-attributes"
)

type csiProvisioner struct {
	client     kubernetes.Interface
	csiClient  csi.ControllerClient
	driverName string
	timeout    time.Duration
	identity   string
	config     *rest.Config
}

var _ controller.Provisioner = &csiProvisioner{}

// Version of CSI this client implements
var (
	csiVersion = csi.Version{
		Major: 0,
		Minor: 1,
		Patch: 0,
	}
	accessMode = &csi.VolumeCapability_AccessMode{
		Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
	}
	accessType = &csi.VolumeCapability_Mount{
		Mount: &csi.VolumeCapability_MountVolume{},
	}
	// Each provisioner have a identify string to distinguish with others. This
	// identify string will be added in PV annoations under this key.
	provisionerIDAnn = "csiProvisionerIdentity"
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

func connect(address string, timeout time.Duration) (*grpc.ClientConn, error) {
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
			return conn, nil // return nil, subsequent GetPluginInfo will show the real connection error
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

	req := csi.GetPluginInfoRequest{
		Version: &csiVersion,
	}

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

func supportsControllerCreateVolume(conn *grpc.ClientConn, timeout time.Duration) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	client := csi.NewControllerClient(conn)
	req := csi.ControllerGetCapabilitiesRequest{
		Version: &csiVersion,
	}

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

func NewCSIProvisioner(client kubernetes.Interface, csiEndpoint string, connectionTimeout time.Duration, identity string) controller.Provisioner {
	grpcClient, err := connect(csiEndpoint, connectionTimeout)
	if err != nil || grpcClient == nil {
		glog.Fatalf("failed to connect to csi endpoint :%v", err)
	}
	ok, err := supportsControllerCreateVolume(grpcClient, connectionTimeout)
	if err != nil {
		glog.Fatalf("failed to get support info :%v", err)
	}
	if !ok {
		glog.Fatalf("no create/delete volume support detected")
	}
	driver, err := getDriverName(grpcClient, connectionTimeout)
	if err != nil {
		glog.Fatalf("failed to get driver info :%v", err)
	}

	csiClient := csi.NewControllerClient(grpcClient)
	provisioner := &csiProvisioner{
		client:     client,
		driverName: driver,
		csiClient:  csiClient,
		timeout:    connectionTimeout,
		identity:   identity,
	}
	return provisioner
}

func printIndentedJson(data interface{}) string {
	var indentedJSON []byte

	indentedJSON, err := jsonv2.MarshalIndent(data, "", "\t")
	if err != nil {
		return fmt.Sprintf("JSON parse error: %v", err)
	}
	return string(indentedJSON)
}

// getKeyFromSecret returns a key for a specific action passed as a parameter,
// if specified action is not found, then error is returned.
func getKeyFromSecret(secretName string, secretNamespace string, action string, client kubernetes.Interface) (string, error) {
	if client == nil {
		return "", fmt.Errorf("Cannot get kube client")
	}
	secrets, err := client.CoreV1().Secrets(secretNamespace).Get(secretName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	for k, v := range secrets.Data {
		// key must match value in action variable
		if k == action {
			return string(v), nil
		}
	}
	// For backward compatibility need to check for a default key
	for k, v := range secrets.Data {
		if k == "default" {
			return string(v), nil
		}
	}

	return "", fmt.Errorf("failed to find action: %s in secret: %s/%s ", action, secretName, secretNamespace)
}

func (p *csiProvisioner) Provision(options controller.VolumeOptions) (*v1.PersistentVolume, error) {
	if options.PVC.Spec.Selector != nil {
		return nil, fmt.Errorf("claim Selector is not supported")
	}
	// create random share name
	share := fmt.Sprintf("kubernetes-dynamic-pvc-%s", uuid.NewUUID())
	capacity := options.PVC.Spec.Resources.Requests[v1.ResourceName(v1.ResourceStorage)]
	volSizeBytes := capacity.Value()

	userCredentials := map[string]string{}
	if storageClassName := options.PVC.Spec.StorageClassName; *storageClassName != "" {
		fmt.Printf("><SB> provisioner was called for storage class: %s and for user: %s\n", *storageClassName, options.PVC.Spec.UserID)
		storageClass, err := p.client.StorageV1().StorageClasses().Get(*storageClassName, metav1.GetOptions{})
		if err != nil {
			glog.Warningf("failed to retrieve information about the storage class: %s with error: %v", storageClassName, err)
		} else {
			if storageClass.SecretRefs != nil {
				// Now we can extract user's key for "create" action
				if secret, found := storageClass.SecretRefs[options.PVC.Spec.UserID]; found {
					key, err := getKeyFromSecret(secret.Name, secret.Namespace, "create", p.client)
					if err == nil {
						userCredentials[options.PVC.Spec.UserID] = key
						fmt.Printf("><SB> Extracted user name: %s secret name: %s secret namespace: %s key: %s \n", options.PVC.Spec.UserID, secret.Name, secret.Namespace, key)
					} else {
						glog.Warningf("failed to retrieve secret: %s/%s with error: %v", secret.Name, secret.Namespace, err)
					}
				} else {
					glog.Warningf("user: %s is not found in storageclass: %s/%s", options.PVC.Spec.UserID, secret.Name, secret.Namespace)
				}
			}
		}
	}
	// Create a CSI CreateVolumeRequest
	req := csi.CreateVolumeRequest{
		Name:       share,
		Version:    &csiVersion,
		Parameters: options.Parameters,
		VolumeCapabilities: []*csi.VolumeCapability{
			{
				AccessType: accessType,
				AccessMode: accessMode,
			},
		},
		CapacityRange: &csi.CapacityRange{
			RequiredBytes: uint64(volSizeBytes),
			LimitBytes:    uint64(volSizeBytes),
		},
		UserCredentials: userCredentials,
	}
	ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()

	rep, err := p.csiClient.CreateVolume(ctx, &req)
	if err != nil {
		return nil, err
	}
	if rep.VolumeInfo != nil {
		glog.V(3).Infof("create volume rep: %+v", *rep.VolumeInfo)
	}

	annotations := map[string]string{provisionerIDAnn: p.identity}
	attributesString, err := json.Marshal(rep.VolumeInfo.Attributes)
	if err != nil {
		glog.V(2).Infof("fail parsing volume attributes: %+v", rep.VolumeInfo.Attributes)
	} else {
		annotations[volumeAttributesAnnotation] = string(attributesString)
	}
	pv := &v1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name:        share,
			Annotations: annotations,
		},
		Spec: v1.PersistentVolumeSpec{
			UserID: options.PVC.Spec.UserID,
			PersistentVolumeReclaimPolicy: options.PersistentVolumeReclaimPolicy,
			AccessModes:                   options.PVC.Spec.AccessModes,
			Capacity: v1.ResourceList{
				v1.ResourceName(v1.ResourceStorage): options.PVC.Spec.Resources.Requests[v1.ResourceName(v1.ResourceStorage)],
			},
			// TODO wait for CSI VolumeSource API
			PersistentVolumeSource: v1.PersistentVolumeSource{
				CSI: &v1.CSIPersistentVolumeSource{
					Driver:       p.driverName,
					VolumeHandle: p.volumeIdToHandle(rep.VolumeInfo.Id),
				},
			},
		},
	}

	glog.Infof("successfully created PV %+v for user: %s", pv.Spec.PersistentVolumeSource, pv.Spec.UserID)

	return pv, nil
}

func (p *csiProvisioner) Delete(volume *v1.PersistentVolume) error {
	if volume == nil || volume.Spec.CSI == nil {
		return fmt.Errorf("invalid CSI PV")
	}
	ann, ok := volume.Annotations[provisionerIDAnn]
	if !ok {
		return fmt.Errorf("identity annotation not found on PV")
	}
	if ann != p.identity {
		return &controller.IgnoredError{Reason: "identity annotation on PV does not match ours"}
	}
	volumeId := p.volumeHandleToId(volume.Spec.CSI.VolumeHandle)
	req := csi.DeleteVolumeRequest{
		Version:  &csiVersion,
		VolumeId: volumeId,
	}
	ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()

	_, err := p.csiClient.DeleteVolume(ctx, &req)
	return err
}

//TODO use a unique volume handle from and to Id
func (p *csiProvisioner) volumeIdToHandle(id string) string {
	return id
}

func (p *csiProvisioner) volumeHandleToId(handle string) string {
	return handle
}
