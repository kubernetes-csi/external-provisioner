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
	"net"
	"strings"
	"time"

	"github.com/golang/glog"

	"github.com/kubernetes-incubator/external-storage/lib/controller"

	"k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"

	"github.com/container-storage-interface/spec/lib/go/csi"
)

type csiProvisioner struct {
	client    kubernetes.Interface
	csiClient csi.ControllerClient
	timeout   time.Duration
	identity  string
	config    *rest.Config
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
)

// from external-attacher/pkg/connection
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

func NewCSIProvisioner(client kubernetes.Interface, csiEndpoint string, connectionTimeout time.Duration, identity string) controller.Provisioner {
	grpcClient, err := connect(csiEndpoint, connectionTimeout)
	if err != nil || grpcClient == nil {
		glog.Fatalf("failed to connect to csi endpoint :%v", err)
	}

	csiClient := csi.NewControllerClient(grpcClient)
	provisioner := &csiProvisioner{
		client:    client,
		csiClient: csiClient,
		timeout:   connectionTimeout,
		identity:  identity,
	}
	return provisioner
}

func (p *csiProvisioner) Provision(options controller.VolumeOptions) (*v1.PersistentVolume, error) {
	glog.Infof("Provisioner %s Provision(..) called..")
	req := csi.CreateVolumeRequest{
		Name:    "mypv",
		Version: &csiVersion,
		VolumeCapabilities: []*csi.VolumeCapability{
			&csi.VolumeCapability{
				AccessMode: accessMode,
			},
		},
		CapacityRange: &csi.CapacityRange{
			RequiredBytes: 100,
			LimitBytes:    100,
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()

	_, err := p.csiClient.CreateVolume(ctx, &req)
	if err != nil {
		return nil, err
	}

	return nil, nil
}

func (p *csiProvisioner) Delete(volume *v1.PersistentVolume) error {
	glog.Infof("Provisioner %s Delete(..) called..")
	return nil
}
