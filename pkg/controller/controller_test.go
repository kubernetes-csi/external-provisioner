/*
Copyright 2018 The Kubernetes Authors.

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
	"fmt"
	"testing"
	"time"

	csi "github.com/container-storage-interface/spec/lib/go/csi/v0"
	"github.com/golang/mock/gomock"
	"github.com/kubernetes-csi/csi-test/driver"
	"google.golang.org/grpc"
)

const (
	timeout = 10 * time.Second
)

type csiConnection struct {
	conn *grpc.ClientConn
}

func New(address string, timeout time.Duration) (csiConnection, error) {
	conn, err := connect(address, timeout)
	if err != nil {
		return csiConnection{}, err
	}
	return csiConnection{
		conn: conn,
	}, nil
}

func createMockServer(t *testing.T) (*gomock.Controller,
	*driver.MockCSIDriver,
	*driver.MockIdentityServer,
	*driver.MockControllerServer,
	csiConnection, error) {
	// Start the mock server
	mockController := gomock.NewController(t)
	controllerServer := driver.NewMockControllerServer(mockController)
	identityServer := driver.NewMockIdentityServer(mockController)
	drv := driver.NewMockCSIDriver(&driver.MockCSIDriverServers{
		Identity:   identityServer,
		Controller: controllerServer,
	})
	drv.Start()

	// Create a client connection to it
	addr := drv.Address()
	csiConn, err := New(addr, timeout)
	if err != nil {
		return nil, nil, nil, nil, csiConnection{}, err
	}

	return mockController, drv, identityServer, controllerServer, csiConn, nil
}

func TestSupportsControllerCreateVolume(t *testing.T) {

	tests := []struct {
		name         string
		output       *csi.ControllerGetCapabilitiesResponse
		injectError  bool
		expectError  bool
		expectResult bool
	}{
		{
			name: "controller create",
			output: &csi.ControllerGetCapabilitiesResponse{
				Capabilities: []*csi.ControllerServiceCapability{
					{
						Type: &csi.ControllerServiceCapability_Rpc{
							Rpc: &csi.ControllerServiceCapability_RPC{
								Type: csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME,
							},
						},
					},
					{
						Type: &csi.ControllerServiceCapability_Rpc{
							Rpc: &csi.ControllerServiceCapability_RPC{
								Type: csi.ControllerServiceCapability_RPC_PUBLISH_UNPUBLISH_VOLUME,
							},
						},
					},
				},
			},
			expectError:  false,
			expectResult: true,
		},
		{
			name: "no controller create",
			output: &csi.ControllerGetCapabilitiesResponse{
				Capabilities: []*csi.ControllerServiceCapability{
					{
						Type: &csi.ControllerServiceCapability_Rpc{
							Rpc: &csi.ControllerServiceCapability_RPC{
								Type: csi.ControllerServiceCapability_RPC_PUBLISH_UNPUBLISH_VOLUME,
							},
						},
					},
				},
			},
			expectError:  false,
			expectResult: false,
		},
		{
			name:         "gRPC error",
			output:       nil,
			injectError:  true,
			expectError:  true,
			expectResult: false,
		},
		{
			name: "empty capability",
			output: &csi.ControllerGetCapabilitiesResponse{
				Capabilities: []*csi.ControllerServiceCapability{
					{
						Type: nil,
					},
				},
			},
			expectError:  false,
			expectResult: false,
		},
		{
			name: "no capabilities",
			output: &csi.ControllerGetCapabilitiesResponse{
				Capabilities: []*csi.ControllerServiceCapability{},
			},
			expectError:  false,
			expectResult: false,
		},
	}
	mockController, driver, _, controllerServer, csiConn, err := createMockServer(t)
	if err != nil {
		t.Fatal(err)
	}
	defer mockController.Finish()
	defer driver.Stop()
	for _, test := range tests {

		in := &csi.ControllerGetCapabilitiesRequest{}

		out := test.output
		var injectedErr error
		if test.injectError {
			injectedErr = fmt.Errorf("mock error")
		}

		controllerServer.EXPECT().ControllerGetCapabilities(gomock.Any(), in).Return(out, injectedErr).Times(1)
		ok, err := supportsControllerCreateVolume(csiConn.conn, timeout)
		if err != nil && !test.expectError {
			t.Errorf("test fail with error: %v\n", err)
		}
		if err == nil && test.expectResult != ok {
			t.Errorf("test fail expected result %t but got %t\n", test.expectResult, ok)
		}
	}
}

func TestSupportsPluginControllerService(t *testing.T) {

	tests := []struct {
		name         string
		output       *csi.GetPluginCapabilitiesResponse
		injectError  bool
		expectError  bool
		expectResult bool
	}{
		{
			name: "controller capability",
			output: &csi.GetPluginCapabilitiesResponse{
				Capabilities: []*csi.PluginCapability{
					{
						Type: &csi.PluginCapability_Service_{
							Service: &csi.PluginCapability_Service{
								Type: csi.PluginCapability_Service_CONTROLLER_SERVICE,
							},
						},
					},
				},
			},
			expectError:  false,
			expectResult: true,
		},
		{
			name: "no controller capability",
			output: &csi.GetPluginCapabilitiesResponse{
				Capabilities: []*csi.PluginCapability{
					{
						Type: &csi.PluginCapability_Service_{
							Service: &csi.PluginCapability_Service{
								Type: csi.PluginCapability_Service_UNKNOWN,
							},
						},
					},
				},
			},
			expectError:  false,
			expectResult: false,
		},
		{
			name:         "gRPC error",
			output:       nil,
			injectError:  true,
			expectError:  true,
			expectResult: false,
		},
		{
			name: "empty capability",
			output: &csi.GetPluginCapabilitiesResponse{
				Capabilities: []*csi.PluginCapability{
					{
						Type: nil,
					},
				},
			},
			expectError:  false,
			expectResult: false,
		},
	}
	mockController, driver, identityServer, _, csiConn, err := createMockServer(t)
	if err != nil {
		t.Fatal(err)
	}
	defer mockController.Finish()
	defer driver.Stop()
	for _, test := range tests {

		in := &csi.GetPluginCapabilitiesRequest{}

		out := test.output
		var injectedErr error
		if test.injectError {
			injectedErr = fmt.Errorf("mock error")
		}

		identityServer.EXPECT().GetPluginCapabilities(gomock.Any(), in).Return(out, injectedErr).Times(1)
		ok, err := supportsPluginControllerService(csiConn.conn, timeout)
		if err != nil && !test.expectError {
			t.Errorf("test fail with error: %v\n", err)
		}
		if err == nil && test.expectResult != ok {
			t.Errorf("test fail expected result %t but got %t\n", test.expectResult, ok)
		}
	}
}

func TestGetDriverName(t *testing.T) {
	tests := []struct {
		name        string
		output      *csi.GetPluginInfoResponse
		injectError bool
		expectError bool
	}{
		{
			name: "success",
			output: &csi.GetPluginInfoResponse{
				Name:          "csi/example",
				VendorVersion: "0.2.0",
				Manifest: map[string]string{
					"hello": "world",
				},
			},
			expectError: false,
		},
		{
			name:        "gRPC error",
			output:      nil,
			injectError: true,
			expectError: true,
		},
		{
			name: "empty name",
			output: &csi.GetPluginInfoResponse{
				Name: "",
			},
			expectError: true,
		},
	}

	mockController, driver, identityServer, _, csiConn, err := createMockServer(t)
	if err != nil {
		t.Fatal(err)
	}
	defer mockController.Finish()
	defer driver.Stop()

	for _, test := range tests {

		in := &csi.GetPluginInfoRequest{}

		out := test.output
		var injectedErr error = nil
		if test.injectError {
			injectedErr = fmt.Errorf("mock error")
		}

		// Setup expectation
		identityServer.EXPECT().GetPluginInfo(gomock.Any(), in).Return(out, injectedErr).Times(1)

		name, err := getDriverName(csiConn.conn, timeout)
		if test.expectError && err == nil {
			t.Errorf("test %q: Expected error, got none", test.name)
		}
		if !test.expectError && err != nil {
			t.Errorf("test %q: got error: %v", test.name, err)
		}
		if err == nil && name != "csi/example" {
			t.Errorf("got unexpected name: %q", name)
		}
	}
}
