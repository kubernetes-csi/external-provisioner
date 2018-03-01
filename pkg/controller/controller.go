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
	_ "k8s.io/apimachinery/pkg/util/json"
	"net"
	"strings"
	"time"

	"github.com/golang/glog"

	"github.com/kubernetes-incubator/external-storage/lib/controller"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"

	"github.com/container-storage-interface/spec/lib/go/csi/v0"
)

const (
	secretNameKey      = "csiProvisionerSecretName"
	secretNamespaceKey = "csiProvisionerSecretNamespace"
)

type csiProvisioner struct {
	client               kubernetes.Interface
	csiClient            csi.ControllerClient
	driverName           string
	timeout              time.Duration
	identity             string
	volumeNamePrefix     string
	volumeNameUUIDLength int
	config               *rest.Config
}

var _ controller.Provisioner = &csiProvisioner{}

var (
	accessMode = &csi.VolumeCapability_AccessMode{
		Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
	}
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

func NewCSIProvisioner(client kubernetes.Interface, csiEndpoint string, connectionTimeout time.Duration, identity string, volumeNamePrefix string, volumeNameUUIDLength int) controller.Provisioner {
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
		client:               client,
		driverName:           driver,
		csiClient:            csiClient,
		timeout:              connectionTimeout,
		identity:             identity,
		volumeNamePrefix:     volumeNamePrefix,
		volumeNameUUIDLength: volumeNameUUIDLength,
	}
	return provisioner
}

func (p *csiProvisioner) Provision(options controller.VolumeOptions) (*v1.PersistentVolume, error) {
	if options.PVC.Spec.Selector != nil {
		return nil, fmt.Errorf("claim Selector is not supported")
	}
	// create random share name
	share := fmt.Sprintf("%s-%s", p.volumeNamePrefix, strings.Replace(string(uuid.NewUUID()), "-", "", -1)[0:p.volumeNameUUIDLength])
	capacity := options.PVC.Spec.Resources.Requests[v1.ResourceName(v1.ResourceStorage)]
	volSizeBytes := capacity.Value()

	controllerCreateSecrets := map[string]string{}
	secret := v1.SecretReference{}
	// get secrets if StorageClass specifies it
	if sn, ok := options.Parameters[secretNameKey]; ok {
		ns := "default"
		if len(options.Parameters[secretNamespaceKey]) != 0 {
			ns = options.Parameters[secretNamespaceKey]
		}
		controllerCreateSecrets = getCredentialsFromSecret(p.client, sn, ns)
		secret.Name = sn
		secret.Namespace = ns
	}

	// Create a CSI CreateVolumeRequest
	req := csi.CreateVolumeRequest{

		Name:                    share,
		Parameters:              options.Parameters,
		ControllerCreateSecrets: controllerCreateSecrets,
		VolumeCapabilities: []*csi.VolumeCapability{
			{
				AccessType: accessType,
				AccessMode: accessMode,
			},
		},
		CapacityRange: &csi.CapacityRange{
			RequiredBytes: int64(volSizeBytes),
			LimitBytes:    int64(volSizeBytes),
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()

	rep, err := p.csiClient.CreateVolume(ctx, &req)
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
	pv := &v1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: share,
		},
		Spec: v1.PersistentVolumeSpec{
			PersistentVolumeReclaimPolicy: options.PersistentVolumeReclaimPolicy,
			AccessModes:                   options.PVC.Spec.AccessModes,
			Capacity: v1.ResourceList{
				v1.ResourceName(v1.ResourceStorage): options.PVC.Spec.Resources.Requests[v1.ResourceName(v1.ResourceStorage)],
			},
			// TODO wait for CSI VolumeSource API
			PersistentVolumeSource: v1.PersistentVolumeSource{
				CSI: &v1.CSIPersistentVolumeSource{
					Driver:               p.driverName,
					VolumeHandle:         p.volumeIdToHandle(rep.Volume.Id),
					VolumeAttributes:     volumeAttributes,
					NodePublishSecretRef: &secret,
				},
			},
		},
	}

	glog.Infof("successfully created PV %+v", pv.Spec.PersistentVolumeSource)

	return pv, nil
}

func (p *csiProvisioner) Delete(volume *v1.PersistentVolume) error {
	if volume == nil || volume.Spec.CSI == nil {
		return fmt.Errorf("invalid CSI PV")
	}
	volumeId := p.volumeHandleToId(volume.Spec.CSI.VolumeHandle)
	req := csi.DeleteVolumeRequest{
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

func getCredentialsFromSecret(k8s kubernetes.Interface, secretName, nameSpace string) map[string]string {
	credentials := map[string]string{}
	if len(secretName) == 0 {
		return credentials
	}
	secret, err := k8s.CoreV1().Secrets(nameSpace).Get(secretName, metav1.GetOptions{})
	if err != nil {
		glog.Warningf("failed to find the secret %s in the namespace %s with error: %v\n", secretName, nameSpace, err)
		return credentials
	}
	for key, value := range secret.Data {
		credentials[key] = string(value)
	}

	return credentials
}
