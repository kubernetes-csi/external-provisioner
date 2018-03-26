/*
Copyright 2017 Luis Pabón luis@portworx.com

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
package test

import (
	"testing"

	csi "github.com/container-storage-interface/spec/lib/go/csi/v0"
	gomock "github.com/golang/mock/gomock"
	mock_driver "github.com/kubernetes-csi/csi-test/driver"
	mock_utils "github.com/kubernetes-csi/csi-test/utils"
	"golang.org/x/net/context"
)

func TestPluginInfoResponse(t *testing.T) {

	// Setup mock
	m := gomock.NewController(t)
	defer m.Finish()
	driver := mock_driver.NewMockIdentityServer(m)

	// Setup input
	in := &csi.GetPluginInfoRequest{}

	// Setup mock outout
	out := &csi.GetPluginInfoResponse{
		Name:          "mock",
		VendorVersion: "0.1.1",
		Manifest: map[string]string{
			"hello": "world",
		},
	}

	// Setup expectation
	driver.EXPECT().GetPluginInfo(nil, in).Return(out, nil).Times(1)

	// Actual call
	r, err := driver.GetPluginInfo(nil, in)
	name := r.GetName()
	if err != nil {
		t.Errorf("Error: %s", err.Error())
	}
	if name != "mock" {
		t.Errorf("Unknown name: %s\n", name)
	}
}

func TestGRPCGetPluginInfoReponse(t *testing.T) {

	// Setup mock
	m := gomock.NewController(&mock_utils.SafeGoroutineTester{})
	defer m.Finish()
	driver := mock_driver.NewMockIdentityServer(m)

	// Setup input
	in := &csi.GetPluginInfoRequest{}

	// Setup mock outout
	out := &csi.GetPluginInfoResponse{
		Name:          "mock",
		VendorVersion: "0.1.1",
		Manifest: map[string]string{
			"hello": "world",
		},
	}

	// Setup expectation
	// !IMPORTANT!: Must set context expected value to gomock.Any() to match any value
	driver.EXPECT().GetPluginInfo(gomock.Any(), in).Return(out, nil).Times(1)

	// Create a new RPC
	server := mock_driver.NewMockCSIDriver(&mock_driver.MockCSIDriverServers{
		Identity: driver,
	})
	conn, err := server.Nexus()
	if err != nil {
		t.Errorf("Error: %s", err.Error())
	}
	defer server.Close()

	// Make call
	c := csi.NewIdentityClient(conn)
	r, err := c.GetPluginInfo(context.Background(), in)
	if err != nil {
		t.Errorf("Error: %s", err.Error())
	}

	name := r.GetName()
	if name != "mock" {
		t.Errorf("Unknown name: %s\n", name)
	}
}
