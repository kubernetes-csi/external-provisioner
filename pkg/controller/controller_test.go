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
	"context"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/golang/mock/gomock"
	"github.com/kubernetes-csi/csi-test/driver"
	"github.com/kubernetes-csi/external-provisioner/pkg/features"
	crdv1 "github.com/kubernetes-csi/external-snapshotter/pkg/apis/volumesnapshot/v1alpha1"
	"github.com/kubernetes-csi/external-snapshotter/pkg/client/clientset/versioned/fake"
	"github.com/kubernetes-sigs/sig-storage-lib-external-provisioner/controller"
	"google.golang.org/grpc"
	"k8s.io/api/core/v1"
	corev1 "k8s.io/api/core/v1"
	storage "k8s.io/api/storage/v1beta1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	utilfeaturetesting "k8s.io/apiserver/pkg/util/feature/testing"
	"k8s.io/client-go/kubernetes"
	fakeclientset "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
	fakecsiclientset "k8s.io/csi-api/pkg/client/clientset/versioned/fake"
)

const (
	timeout = 10 * time.Second
)

type csiConnection struct {
	conn *grpc.ClientConn
}

func New(address string, timeout time.Duration) (csiConnection, error) {
	conn, err := Connect(address, timeout)
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

func TestGetPluginName(t *testing.T) {
	test := struct {
		name   string
		output []*csi.GetPluginInfoResponse
	}{
		name: "success",
		output: []*csi.GetPluginInfoResponse{
			{
				Name:          "csi/example-1",
				VendorVersion: "0.2.0",
				Manifest: map[string]string{
					"hello": "world",
				},
			},
			{
				Name:          "csi/example-2",
				VendorVersion: "0.2.0",
				Manifest: map[string]string{
					"hello": "world",
				},
			},
		},
	}

	mockController, driver, identityServer, _, csiConn, err := createMockServer(t)
	if err != nil {
		t.Fatal(err)
	}
	defer mockController.Finish()
	defer driver.Stop()

	in := &csi.GetPluginInfoRequest{}
	out := test.output[0]

	identityServer.EXPECT().GetPluginInfo(gomock.Any(), in).Return(out, nil).Times(1)
	oldName, err := GetDriverName(csiConn.conn, timeout)
	if err != nil {
		t.Errorf("test %q: Failed to get driver's name", test.name)
	}
	if oldName != test.output[0].Name {
		t.Errorf("test %s: failed, expected %s got %s", test.name, test.output[0].Name, oldName)
	}

	out = test.output[1]
	identityServer.EXPECT().GetPluginInfo(gomock.Any(), in).Return(out, nil).Times(1)
	newName, err := GetDriverName(csiConn.conn, timeout)
	if err != nil {
		t.Errorf("test %s: Failed to get driver's name", test.name)
	}
	if newName != test.output[1].Name {
		t.Errorf("test %q: failed, expected %s got %s", test.name, test.output[1].Name, newName)
	}

	if oldName == newName {
		t.Errorf("test: %s failed, driver's names should not match", test.name)
	}
}

func TestGetDriverCapabilities(t *testing.T) {
	type testcase struct {
		name                   string
		pluginCapabilities     []*csi.PluginCapability_Service_Type
		controllerCapabilities []*csi.ControllerServiceCapability_RPC_Type
		injectPluginError      bool
		injectControllerError  bool
		expectError            bool
	}
	tests := []testcase{{}}

	// Generate test cases by creating all possible combination of capabilities
	for capName, capValue := range csi.PluginCapability_Service_Type_value {
		cap := csi.PluginCapability_Service_Type(capValue)
		var newTests []testcase
		for _, test := range tests {
			newTest := testcase{
				name: fmt.Sprintf("%s,Plugin_%s", test.name, capName),
			}
			copy(newTest.pluginCapabilities, append(test.pluginCapabilities, &cap))
			copy(newTest.controllerCapabilities, test.controllerCapabilities)
			newTests = append(newTests, newTest)
		}
		tests = newTests
	}
	for capName, capValue := range csi.ControllerServiceCapability_RPC_Type_value {
		cap := csi.ControllerServiceCapability_RPC_Type(capValue)
		var newTests []testcase
		for _, test := range tests {
			newTest := testcase{
				name: fmt.Sprintf("%s,Plugin_%s", test.name, capName),
			}
			copy(newTest.pluginCapabilities, test.pluginCapabilities)
			copy(newTest.controllerCapabilities, append(test.controllerCapabilities, &cap))
			newTests = append(newTests, newTest)
		}
		tests = newTests
	}

	// nil capabilities tests
	dummyPluginCap := csi.PluginCapability_Service_CONTROLLER_SERVICE
	dummyControllerCap := csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME
	tests = append(tests, []testcase{
		{
			name:                   "plugin capabilities with nil entries",
			pluginCapabilities:     []*csi.PluginCapability_Service_Type{nil},
			controllerCapabilities: []*csi.ControllerServiceCapability_RPC_Type{&dummyControllerCap},
		},
		{
			name:                   "controller capabilities with nil entries",
			pluginCapabilities:     []*csi.PluginCapability_Service_Type{&dummyPluginCap},
			controllerCapabilities: []*csi.ControllerServiceCapability_RPC_Type{nil},
		},
	}...)

	// gRPC errors
	tests = append(tests, []testcase{
		{
			name:                   "plugin capabilities call with gRPC error",
			pluginCapabilities:     []*csi.PluginCapability_Service_Type{&dummyPluginCap},
			controllerCapabilities: []*csi.ControllerServiceCapability_RPC_Type{&dummyControllerCap},
			injectPluginError:      true,
			expectError:            true,
		},
		{
			name:                   "controller capabilities call with gRPC error",
			pluginCapabilities:     []*csi.PluginCapability_Service_Type{&dummyPluginCap},
			controllerCapabilities: []*csi.ControllerServiceCapability_RPC_Type{&dummyControllerCap},
			injectControllerError:  true,
			expectError:            true,
		},
	}...)

	mockController, driver, identityServer, controllerServer, csiConn, err := createMockServer(t)
	if err != nil {
		t.Fatal(err)
	}
	defer mockController.Finish()
	defer driver.Stop()
	for _, test := range tests {

		var injectedPluginErr, injectedControllerErr error
		if test.injectPluginError {
			injectedPluginErr = fmt.Errorf("mock error")
		}
		if test.injectControllerError {
			injectedControllerErr = fmt.Errorf("mock error")
		}

		var pluginCaps []*csi.PluginCapability
		for _, cap := range test.pluginCapabilities {
			var c *csi.PluginCapability
			if cap == nil {
				c = &csi.PluginCapability{Type: nil}
			} else {
				c = &csi.PluginCapability{
					Type: &csi.PluginCapability_Service_{
						Service: &csi.PluginCapability_Service{
							Type: *cap,
						},
					},
				}
			}
			pluginCaps = append(pluginCaps, c)
		}
		pluginResponse := &csi.GetPluginCapabilitiesResponse{Capabilities: pluginCaps}

		var controllerCaps []*csi.ControllerServiceCapability
		for _, cap := range test.controllerCapabilities {
			var c *csi.ControllerServiceCapability
			if cap == nil {
				c = &csi.ControllerServiceCapability{Type: nil}
			} else {
				c = &csi.ControllerServiceCapability{
					Type: &csi.ControllerServiceCapability_Rpc{
						Rpc: &csi.ControllerServiceCapability_RPC{
							Type: *cap,
						},
					},
				}
			}
			controllerCaps = append(controllerCaps, c)
		}
		controllerResponse := &csi.ControllerGetCapabilitiesResponse{Capabilities: controllerCaps}

		identityServer.EXPECT().GetPluginCapabilities(gomock.Any(), &csi.GetPluginCapabilitiesRequest{}).Return(pluginResponse, injectedPluginErr).Times(1)
		controllerServer.EXPECT().ControllerGetCapabilities(gomock.Any(), &csi.ControllerGetCapabilitiesRequest{}).Return(controllerResponse, injectedControllerErr).MinTimes(0).MaxTimes(1)

		capabilities, err := getDriverCapabilities(csiConn.conn, timeout)
		if err != nil && !test.expectError {
			t.Errorf("test %q failed with error: %v\n", test.name, err)
		}
		if err == nil {
			ok := true
			for _, cap := range test.pluginCapabilities {
				if cap != nil {
					switch *cap {
					case csi.PluginCapability_Service_CONTROLLER_SERVICE:
						ok = ok && capabilities.Has(PluginCapability_CONTROLLER_SERVICE)
					case csi.PluginCapability_Service_VOLUME_ACCESSIBILITY_CONSTRAINTS:
						ok = ok && capabilities.Has(PluginCapability_ACCESSIBILITY_CONSTRAINTS)
					}
				}
			}
			for _, cap := range test.controllerCapabilities {
				if cap != nil {
					switch *cap {
					case csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME:
						ok = ok && capabilities.Has(ControllerCapability_CREATE_DELETE_VOLUME)
					case csi.ControllerServiceCapability_RPC_CREATE_DELETE_SNAPSHOT:
						ok = ok && capabilities.Has(ControllerCapability_CREATE_DELETE_SNAPSHOT)
					}
				}
			}

			if !ok {
				t.Errorf("test %q: missing capabilities", test.name)
			}
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
		var injectedErr error
		if test.injectError {
			injectedErr = fmt.Errorf("mock error")
		}

		// Setup expectation
		identityServer.EXPECT().GetPluginInfo(gomock.Any(), in).Return(out, injectedErr).Times(1)

		name, err := GetDriverName(csiConn.conn, timeout)
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

func TestBytesToQuantity(t *testing.T) {
	tests := []struct {
		testName    string
		bytes       float64
		quantString string
	}{
		{
			"Gibibyte rounding up from above .5",
			5.56 * 1024 * 1024 * 1024,
			"6Gi",
		},
		{
			"Gibibyte rounding up from below .5",
			5.23 * 1024 * 1024 * 1024,
			"6Gi",
		},
		{
			"Gibibyte exact",
			5 * 1024 * 1024 * 1024,
			"5Gi",
		},
		{
			"Mebibyte rounding up from below .5",
			5.23 * 1024 * 1024,
			"6Mi",
		},
		{
			"Mebibyte/Gibibyte barrier (Quantity type rounds this)",
			// (1024 * 1024 * 1024) - 1
			1073741823,
			"1Gi",
		},
	}

	for _, test := range tests {
		q := bytesToGiQuantity(int64(test.bytes))
		if q.String() != test.quantString {
			t.Errorf("test: %s, expected: %v, got: %v", test.testName, test.quantString, q.String())
		}
	}

}

func TestCreateDriverReturnsInvalidCapacityDuringProvision(t *testing.T) {
	// Set up mocks
	var requestedBytes int64 = 100
	mockController, driver, identityServer, controllerServer, csiConn, err := createMockServer(t)
	if err != nil {
		t.Fatal(err)
	}
	defer mockController.Finish()
	defer driver.Stop()

	csiProvisioner := NewCSIProvisioner(nil, nil, driver.Address(), 5*time.Second, "test-provisioner", "test", 5, csiConn.conn, nil)

	// Requested PVC with requestedBytes storage
	opts := controller.VolumeOptions{
		PersistentVolumeReclaimPolicy: v1.PersistentVolumeReclaimDelete,
		PVName:                        "test-name",
		PVC:                           createFakePVC(requestedBytes),
		Parameters:                    map[string]string{},
	}

	// Drivers CreateVolume response with lower capacity bytes than request
	out := &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			CapacityBytes: requestedBytes - 1,
			VolumeId:      "test-volume-id",
		},
	}

	// Set up Mocks
	provisionMockServerSetupExpectations(identityServer, controllerServer)
	controllerServer.EXPECT().CreateVolume(gomock.Any(), gomock.Any()).Return(out, nil).Times(1)
	// Since capacity returned by driver is invalid, we expect the provision call to clean up the volume
	controllerServer.EXPECT().DeleteVolume(gomock.Any(), &csi.DeleteVolumeRequest{
		VolumeId: "test-volume-id",
	}).Return(&csi.DeleteVolumeResponse{}, nil).Times(1)

	// Call provision
	_, err = csiProvisioner.Provision(opts)
	if err == nil {
		t.Errorf("Provision did not cause an error when one was expected")
		return
	}
	t.Logf("Provision encountered an error: %v, expected: create volume capacity less than requested capacity", err)
}

func provisionMockServerSetupExpectations(identityServer *driver.MockIdentityServer, controllerServer *driver.MockControllerServer) {
	identityServer.EXPECT().GetPluginCapabilities(gomock.Any(), gomock.Any()).Return(&csi.GetPluginCapabilitiesResponse{
		Capabilities: []*csi.PluginCapability{
			&csi.PluginCapability{
				Type: &csi.PluginCapability_Service_{
					Service: &csi.PluginCapability_Service{
						Type: csi.PluginCapability_Service_CONTROLLER_SERVICE,
					},
				},
			},
		},
	}, nil).Times(1)
	controllerServer.EXPECT().ControllerGetCapabilities(gomock.Any(), gomock.Any()).Return(&csi.ControllerGetCapabilitiesResponse{
		Capabilities: []*csi.ControllerServiceCapability{
			&csi.ControllerServiceCapability{
				Type: &csi.ControllerServiceCapability_Rpc{
					Rpc: &csi.ControllerServiceCapability_RPC{
						Type: csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME,
					},
				},
			},
		},
	}, nil).Times(1)
	identityServer.EXPECT().GetPluginInfo(gomock.Any(), gomock.Any()).Return(&csi.GetPluginInfoResponse{
		Name:          "test-driver",
		VendorVersion: "test-vendor",
	}, nil).Times(1)
}

// provisionFromSnapshotMockServerSetupExpectations mocks plugin and controller capabilities reported
// by a CSI plugin that supports the snapshot feature
func provisionFromSnapshotMockServerSetupExpectations(identityServer *driver.MockIdentityServer, controllerServer *driver.MockControllerServer) {
	identityServer.EXPECT().GetPluginCapabilities(gomock.Any(), gomock.Any()).Return(&csi.GetPluginCapabilitiesResponse{
		Capabilities: []*csi.PluginCapability{
			{
				Type: &csi.PluginCapability_Service_{
					Service: &csi.PluginCapability_Service{
						Type: csi.PluginCapability_Service_CONTROLLER_SERVICE,
					},
				},
			},
		},
	}, nil).Times(1)
	controllerServer.EXPECT().ControllerGetCapabilities(gomock.Any(), gomock.Any()).Return(&csi.ControllerGetCapabilitiesResponse{
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
						Type: csi.ControllerServiceCapability_RPC_CREATE_DELETE_SNAPSHOT,
					},
				},
			},
		},
	}, nil).Times(1)
	identityServer.EXPECT().GetPluginInfo(gomock.Any(), gomock.Any()).Return(&csi.GetPluginInfoResponse{
		Name:          "test-driver",
		VendorVersion: "test-vendor",
	}, nil).Times(1)
}

func provisionWithTopologyMockServerSetupExpectations(identityServer *driver.MockIdentityServer, controllerServer *driver.MockControllerServer) {
	identityServer.EXPECT().GetPluginCapabilities(gomock.Any(), gomock.Any()).Return(&csi.GetPluginCapabilitiesResponse{
		Capabilities: []*csi.PluginCapability{
			{
				Type: &csi.PluginCapability_Service_{
					Service: &csi.PluginCapability_Service{
						Type: csi.PluginCapability_Service_CONTROLLER_SERVICE,
					},
				},
			},
			{
				Type: &csi.PluginCapability_Service_{
					Service: &csi.PluginCapability_Service{
						Type: csi.PluginCapability_Service_VOLUME_ACCESSIBILITY_CONSTRAINTS,
					},
				},
			},
		},
	}, nil).Times(1)
	controllerServer.EXPECT().ControllerGetCapabilities(gomock.Any(), gomock.Any()).Return(&csi.ControllerGetCapabilitiesResponse{
		Capabilities: []*csi.ControllerServiceCapability{
			{
				Type: &csi.ControllerServiceCapability_Rpc{
					Rpc: &csi.ControllerServiceCapability_RPC{
						Type: csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME,
					},
				},
			},
		},
	}, nil).Times(1)
	identityServer.EXPECT().GetPluginInfo(gomock.Any(), gomock.Any()).Return(&csi.GetPluginInfoResponse{
		Name:          "test-driver",
		VendorVersion: "test-vendor",
	}, nil).Times(1)
}

// Minimal PVC required for tests to function
func createFakePVC(requestBytes int64) *v1.PersistentVolumeClaim {
	return &v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			UID: "testid",
		},
		Spec: v1.PersistentVolumeClaimSpec{
			Selector: nil, // Provisioner doesn't support selector
			Resources: v1.ResourceRequirements{
				Requests: v1.ResourceList{
					v1.ResourceName(v1.ResourceStorage): resource.MustParse(strconv.FormatInt(requestBytes, 10)),
				},
			},
		},
	}
}

// createFakePVCWithVolumeMode returns PVC with VolumeMode
func createFakePVCWithVolumeMode(requestBytes int64, volumeMode v1.PersistentVolumeMode) *v1.PersistentVolumeClaim {
	claim := createFakePVC(requestBytes)
	claim.Spec.VolumeMode = &volumeMode
	return claim
}

func TestGetSecretReference(t *testing.T) {
	testcases := map[string]struct {
		nameKey      string
		namespaceKey string
		params       map[string]string
		pvName       string
		pvc          *v1.PersistentVolumeClaim

		expectRef *v1.SecretReference
		expectErr error
	}{
		"no params": {
			nameKey:      nodePublishSecretNameKey,
			namespaceKey: nodePublishSecretNamespaceKey,
			params:       nil,
			expectRef:    nil,
			expectErr:    nil,
		},
		"empty err": {
			nameKey:      nodePublishSecretNameKey,
			namespaceKey: nodePublishSecretNamespaceKey,
			params:       map[string]string{nodePublishSecretNameKey: "", nodePublishSecretNamespaceKey: ""},
			expectErr:    fmt.Errorf("csiNodePublishSecretName and csiNodePublishSecretNamespace parameters must be specified together"),
		},
		"name, no namespace": {
			nameKey:      nodePublishSecretNameKey,
			namespaceKey: nodePublishSecretNamespaceKey,
			params:       map[string]string{nodePublishSecretNameKey: "foo"},
			expectErr:    fmt.Errorf("csiNodePublishSecretName and csiNodePublishSecretNamespace parameters must be specified together"),
		},
		"namespace, no name": {
			nameKey:      nodePublishSecretNameKey,
			namespaceKey: nodePublishSecretNamespaceKey,
			params:       map[string]string{nodePublishSecretNamespaceKey: "foo"},
			expectErr:    fmt.Errorf("csiNodePublishSecretName and csiNodePublishSecretNamespace parameters must be specified together"),
		},
		"simple - valid": {
			nameKey:      nodePublishSecretNameKey,
			namespaceKey: nodePublishSecretNamespaceKey,
			params:       map[string]string{nodePublishSecretNameKey: "name", nodePublishSecretNamespaceKey: "ns"},
			pvc:          &v1.PersistentVolumeClaim{},
			expectRef:    &v1.SecretReference{Name: "name", Namespace: "ns"},
			expectErr:    nil,
		},
		"simple - valid, no pvc": {
			nameKey:      provisionerSecretNameKey,
			namespaceKey: provisionerSecretNamespaceKey,
			params:       map[string]string{provisionerSecretNameKey: "name", provisionerSecretNamespaceKey: "ns"},
			pvc:          nil,
			expectRef:    &v1.SecretReference{Name: "name", Namespace: "ns"},
			expectErr:    nil,
		},
		"simple - invalid name": {
			nameKey:      nodePublishSecretNameKey,
			namespaceKey: nodePublishSecretNamespaceKey,
			params:       map[string]string{nodePublishSecretNameKey: "bad name", nodePublishSecretNamespaceKey: "ns"},
			pvc:          &v1.PersistentVolumeClaim{},
			expectRef:    nil,
			expectErr:    fmt.Errorf(`csiNodePublishSecretName parameter "bad name" is not a valid secret name`),
		},
		"simple - invalid namespace": {
			nameKey:      nodePublishSecretNameKey,
			namespaceKey: nodePublishSecretNamespaceKey,
			params:       map[string]string{nodePublishSecretNameKey: "name", nodePublishSecretNamespaceKey: "bad ns"},
			pvc:          &v1.PersistentVolumeClaim{},
			expectRef:    nil,
			expectErr:    fmt.Errorf(`csiNodePublishSecretNamespace parameter "bad ns" is not a valid namespace name`),
		},
		"template - valid": {
			nameKey:      nodePublishSecretNameKey,
			namespaceKey: nodePublishSecretNamespaceKey,
			params: map[string]string{
				nodePublishSecretNameKey:      "static-${pv.name}-${pvc.namespace}-${pvc.name}-${pvc.annotations['akey']}",
				nodePublishSecretNamespaceKey: "static-${pv.name}-${pvc.namespace}",
			},
			pvName: "pvname",
			pvc: &v1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "pvcname",
					Namespace:   "pvcnamespace",
					Annotations: map[string]string{"akey": "avalue"},
				},
			},
			expectRef: &v1.SecretReference{Name: "static-pvname-pvcnamespace-pvcname-avalue", Namespace: "static-pvname-pvcnamespace"},
			expectErr: nil,
		},
		"template - invalid namespace tokens": {
			nameKey:      nodePublishSecretNameKey,
			namespaceKey: nodePublishSecretNamespaceKey,
			params: map[string]string{
				nodePublishSecretNameKey:      "myname",
				nodePublishSecretNamespaceKey: "mynamespace${bar}",
			},
			pvc:       &v1.PersistentVolumeClaim{},
			expectRef: nil,
			expectErr: fmt.Errorf(`error resolving csiNodePublishSecretNamespace value "mynamespace${bar}": invalid tokens: ["bar"]`),
		},
		"template - invalid name tokens": {
			nameKey:      nodePublishSecretNameKey,
			namespaceKey: nodePublishSecretNamespaceKey,
			params: map[string]string{
				nodePublishSecretNameKey:      "myname${foo}",
				nodePublishSecretNamespaceKey: "mynamespace",
			},
			pvc:       &v1.PersistentVolumeClaim{},
			expectRef: nil,
			expectErr: fmt.Errorf(`error resolving csiNodePublishSecretName value "myname${foo}": invalid tokens: ["foo"]`),
		},
	}

	for k, tc := range testcases {
		t.Run(k, func(t *testing.T) {
			ref, err := getSecretReference(tc.nameKey, tc.namespaceKey, tc.params, tc.pvName, tc.pvc)
			if !reflect.DeepEqual(err, tc.expectErr) {
				t.Errorf("Expected %v, got %v", tc.expectErr, err)
			}
			if !reflect.DeepEqual(ref, tc.expectRef) {
				t.Errorf("Expected %v, got %v", tc.expectRef, ref)
			}
		})
	}
}

func TestProvision(t *testing.T) {

	var (
		requestedBytes       int64 = 100
		volumeModeFileSystem       = v1.PersistentVolumeFilesystem
		volumeModeBlock            = v1.PersistentVolumeBlock
	)

	type pvSpec struct {
		Name          string
		ReclaimPolicy v1.PersistentVolumeReclaimPolicy
		AccessModes   []v1.PersistentVolumeAccessMode
		VolumeMode    *v1.PersistentVolumeMode
		Capacity      v1.ResourceList
		CSIPVS        *v1.CSIPersistentVolumeSource
	}

	testcases := map[string]struct {
		volOpts           controller.VolumeOptions
		notNilSelector    bool
		driverNotReady    bool
		makeVolumeNameErr bool
		getSecretRefErr   bool
		getCredentialsErr bool
		volWithLessCap    bool
		expectedPVSpec    *pvSpec
		withSecretRefs    bool
		expectErr         bool
		expectCreateVolDo interface{}
	}{
		"normal provision": {
			volOpts: controller.VolumeOptions{
				PersistentVolumeReclaimPolicy: v1.PersistentVolumeReclaimDelete,
				PVName:                        "test-name",
				PVC:                           createFakePVC(requestedBytes),
				Parameters: map[string]string{
					"fstype": "ext3",
				},
			},
			expectedPVSpec: &pvSpec{
				Name:          "test-testi",
				ReclaimPolicy: v1.PersistentVolumeReclaimDelete,
				Capacity: v1.ResourceList{
					v1.ResourceName(v1.ResourceStorage): bytesToGiQuantity(requestedBytes),
				},
				CSIPVS: &v1.CSIPersistentVolumeSource{
					Driver:       "test-driver",
					VolumeHandle: "test-volume-id",
					FSType:       "ext3",
					VolumeAttributes: map[string]string{
						"storage.kubernetes.io/csiProvisionerIdentity": "test-provisioner",
					},
				},
			},
		},
		"provision with access mode multi node multi writer": {
			volOpts: controller.VolumeOptions{
				PersistentVolumeReclaimPolicy: v1.PersistentVolumeReclaimDelete,
				PVName:                        "test-name",
				PVC: &v1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						UID: "testid",
					},
					Spec: v1.PersistentVolumeClaimSpec{
						Selector: nil,
						Resources: v1.ResourceRequirements{
							Requests: v1.ResourceList{
								v1.ResourceName(v1.ResourceStorage): resource.MustParse(strconv.FormatInt(requestedBytes, 10)),
							},
						},
						AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadWriteMany},
					},
				},
				Parameters: map[string]string{},
			},
			expectedPVSpec: &pvSpec{
				Name:          "test-testi",
				ReclaimPolicy: v1.PersistentVolumeReclaimDelete,
				AccessModes:   []v1.PersistentVolumeAccessMode{v1.ReadWriteMany},
				Capacity: v1.ResourceList{
					v1.ResourceName(v1.ResourceStorage): bytesToGiQuantity(requestedBytes),
				},
				CSIPVS: &v1.CSIPersistentVolumeSource{
					Driver:       "test-driver",
					VolumeHandle: "test-volume-id",
					FSType:       "ext4",
					VolumeAttributes: map[string]string{
						"storage.kubernetes.io/csiProvisionerIdentity": "test-provisioner",
					},
				},
			},
			expectCreateVolDo: func(ctx context.Context, req *csi.CreateVolumeRequest) {
				if len(req.GetVolumeCapabilities()) != 1 {
					t.Errorf("Incorrect length in volume capabilities")
				}
				if req.GetVolumeCapabilities()[0].GetAccessMode() == nil {
					t.Errorf("Expected access mode to be set")
				}
				if req.GetVolumeCapabilities()[0].GetAccessMode().GetMode() != csi.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER {
					t.Errorf("Expected multi_node_multi_writer")
				}
			},
		},
		"provision with access mode multi node multi readonly": {
			volOpts: controller.VolumeOptions{
				PersistentVolumeReclaimPolicy: v1.PersistentVolumeReclaimDelete,
				PVName:                        "test-name",
				PVC: &v1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						UID: "testid",
					},
					Spec: v1.PersistentVolumeClaimSpec{
						Selector: nil,
						Resources: v1.ResourceRequirements{
							Requests: v1.ResourceList{
								v1.ResourceName(v1.ResourceStorage): resource.MustParse(strconv.FormatInt(requestedBytes, 10)),
							},
						},
						AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadOnlyMany},
					},
				},
				Parameters: map[string]string{},
			},
			expectedPVSpec: &pvSpec{
				Name:          "test-testi",
				ReclaimPolicy: v1.PersistentVolumeReclaimDelete,
				AccessModes:   []v1.PersistentVolumeAccessMode{v1.ReadOnlyMany},
				Capacity: v1.ResourceList{
					v1.ResourceName(v1.ResourceStorage): bytesToGiQuantity(requestedBytes),
				},
				CSIPVS: &v1.CSIPersistentVolumeSource{
					Driver:       "test-driver",
					VolumeHandle: "test-volume-id",
					FSType:       "ext4",
					VolumeAttributes: map[string]string{
						"storage.kubernetes.io/csiProvisionerIdentity": "test-provisioner",
					},
				},
			},
			expectCreateVolDo: func(ctx context.Context, req *csi.CreateVolumeRequest) {
				if len(req.GetVolumeCapabilities()) != 1 {
					t.Errorf("Incorrect length in volume capabilities")
				}
				if req.GetVolumeCapabilities()[0].GetAccessMode() == nil {
					t.Errorf("Expected access mode to be set")
				}
				if req.GetVolumeCapabilities()[0].GetAccessMode().GetMode() != csi.VolumeCapability_AccessMode_MULTI_NODE_READER_ONLY {
					t.Errorf("Expected multi_node_reader_only")
				}
			},
		},
		"provision with access mode single writer": {
			volOpts: controller.VolumeOptions{
				PersistentVolumeReclaimPolicy: v1.PersistentVolumeReclaimDelete,
				PVName:                        "test-name",
				PVC: &v1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						UID: "testid",
					},
					Spec: v1.PersistentVolumeClaimSpec{
						Selector: nil,
						Resources: v1.ResourceRequirements{
							Requests: v1.ResourceList{
								v1.ResourceName(v1.ResourceStorage): resource.MustParse(strconv.FormatInt(requestedBytes, 10)),
							},
						},
						AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
					},
				},
				Parameters: map[string]string{},
			},
			expectedPVSpec: &pvSpec{
				Name:          "test-testi",
				ReclaimPolicy: v1.PersistentVolumeReclaimDelete,
				AccessModes:   []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
				Capacity: v1.ResourceList{
					v1.ResourceName(v1.ResourceStorage): bytesToGiQuantity(requestedBytes),
				},
				CSIPVS: &v1.CSIPersistentVolumeSource{
					Driver:       "test-driver",
					VolumeHandle: "test-volume-id",
					FSType:       "ext4",
					VolumeAttributes: map[string]string{
						"storage.kubernetes.io/csiProvisionerIdentity": "test-provisioner",
					},
				},
			},
			expectCreateVolDo: func(ctx context.Context, req *csi.CreateVolumeRequest) {
				if len(req.GetVolumeCapabilities()) != 1 {
					t.Errorf("Incorrect length in volume capabilities")
				}
				if req.GetVolumeCapabilities()[0].GetAccessMode() == nil {
					t.Errorf("Expected access mode to be set")
				}
				if req.GetVolumeCapabilities()[0].GetAccessMode().GetMode() != csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER {
					t.Errorf("Expected single_node_writer")
				}
			},
		},
		"provision with multiple access modes": {
			volOpts: controller.VolumeOptions{
				PersistentVolumeReclaimPolicy: v1.PersistentVolumeReclaimDelete,
				PVName:                        "test-name",
				PVC: &v1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						UID: "testid",
					},
					Spec: v1.PersistentVolumeClaimSpec{
						Selector: nil,
						Resources: v1.ResourceRequirements{
							Requests: v1.ResourceList{
								v1.ResourceName(v1.ResourceStorage): resource.MustParse(strconv.FormatInt(requestedBytes, 10)),
							},
						},
						AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadOnlyMany, v1.ReadWriteOnce},
					},
				},
				Parameters: map[string]string{},
			},
			expectedPVSpec: &pvSpec{
				Name:          "test-testi",
				ReclaimPolicy: v1.PersistentVolumeReclaimDelete,
				AccessModes:   []v1.PersistentVolumeAccessMode{v1.ReadOnlyMany, v1.ReadWriteOnce},
				Capacity: v1.ResourceList{
					v1.ResourceName(v1.ResourceStorage): bytesToGiQuantity(requestedBytes),
				},
				CSIPVS: &v1.CSIPersistentVolumeSource{
					Driver:       "test-driver",
					VolumeHandle: "test-volume-id",
					FSType:       "ext4",
					VolumeAttributes: map[string]string{
						"storage.kubernetes.io/csiProvisionerIdentity": "test-provisioner",
					},
				},
			},
			expectCreateVolDo: func(ctx context.Context, req *csi.CreateVolumeRequest) {
				if len(req.GetVolumeCapabilities()) != 2 {
					t.Errorf("Incorrect length in volume capabilities")
				}
				if req.GetVolumeCapabilities()[0].GetAccessMode() == nil {
					t.Errorf("Expected access mode to be set")
				}
				if req.GetVolumeCapabilities()[0].GetAccessMode().GetMode() != csi.VolumeCapability_AccessMode_MULTI_NODE_READER_ONLY {
					t.Errorf("Expected multi reade only")
				}
				if req.GetVolumeCapabilities()[1].GetAccessMode() == nil {
					t.Errorf("Expected access mode to be set")
				}
				if req.GetVolumeCapabilities()[1].GetAccessMode().GetMode() != csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER {
					t.Errorf("Expected single_node_writer")
				}
			},
		},
		"provision with secrets": {
			volOpts: controller.VolumeOptions{
				PVName:     "test-name",
				PVC:        createFakePVC(requestedBytes),
				Parameters: map[string]string{},
			},
			withSecretRefs: true,
			expectedPVSpec: &pvSpec{
				Name: "test-testi",
				Capacity: v1.ResourceList{
					v1.ResourceName(v1.ResourceStorage): bytesToGiQuantity(requestedBytes),
				},
				CSIPVS: &v1.CSIPersistentVolumeSource{
					Driver:       "test-driver",
					VolumeHandle: "test-volume-id",
					FSType:       "ext4",
					VolumeAttributes: map[string]string{
						"storage.kubernetes.io/csiProvisionerIdentity": "test-provisioner",
					},
					ControllerPublishSecretRef: &v1.SecretReference{
						Name:      "ctrlpublishsecret",
						Namespace: "default",
					},
					NodeStageSecretRef: &v1.SecretReference{
						Name:      "nodestagesecret",
						Namespace: "default",
					},
					NodePublishSecretRef: &v1.SecretReference{
						Name:      "nodepublishsecret",
						Namespace: "default",
					},
				},
			},
		},
		"provision with volume mode(Filesystem)": {
			volOpts: controller.VolumeOptions{
				PVName:     "test-name",
				PVC:        createFakePVCWithVolumeMode(requestedBytes, volumeModeFileSystem),
				Parameters: map[string]string{},
			},
			expectedPVSpec: &pvSpec{
				Name: "test-testi",
				Capacity: v1.ResourceList{
					v1.ResourceName(v1.ResourceStorage): bytesToGiQuantity(requestedBytes),
				},
				VolumeMode: &volumeModeFileSystem,
				CSIPVS: &v1.CSIPersistentVolumeSource{
					Driver:       "test-driver",
					VolumeHandle: "test-volume-id",
					FSType:       "ext4",
					VolumeAttributes: map[string]string{
						"storage.kubernetes.io/csiProvisionerIdentity": "test-provisioner",
					},
				},
			},
		},
		"provision with volume mode(Block)": {
			volOpts: controller.VolumeOptions{
				PVName:     "test-name",
				PVC:        createFakePVCWithVolumeMode(requestedBytes, volumeModeBlock),
				Parameters: map[string]string{},
			},
			expectedPVSpec: &pvSpec{
				Name: "test-testi",
				Capacity: v1.ResourceList{
					v1.ResourceName(v1.ResourceStorage): bytesToGiQuantity(requestedBytes),
				},
				VolumeMode: &volumeModeBlock,
				CSIPVS: &v1.CSIPersistentVolumeSource{
					Driver:       "test-driver",
					VolumeHandle: "test-volume-id",
					VolumeAttributes: map[string]string{
						"storage.kubernetes.io/csiProvisionerIdentity": "test-provisioner",
					},
				},
			},
		},
		"fail to get secret reference": {
			volOpts: controller.VolumeOptions{
				PVName:     "test-name",
				PVC:        createFakePVC(requestedBytes),
				Parameters: map[string]string{},
			},
			getSecretRefErr: true,
			expectErr:       true,
		},
		"fail not nil selector": {
			volOpts: controller.VolumeOptions{
				PVName: "test-name",
				PVC:    createFakePVC(requestedBytes),
			},
			notNilSelector: true,
			expectErr:      true,
		},
		"fail driver not ready": {
			volOpts: controller.VolumeOptions{
				PVName: "test-name",
				PVC:    createFakePVC(requestedBytes),
			},
			driverNotReady: true,
			expectErr:      true,
		},
		"fail to make volume name": {
			volOpts: controller.VolumeOptions{
				PVName: "test-name",
				PVC:    createFakePVC(requestedBytes),
			},
			makeVolumeNameErr: true,
			expectErr:         true,
		},
		"fail to get credentials": {
			volOpts: controller.VolumeOptions{
				PVName:     "test-name",
				PVC:        createFakePVC(requestedBytes),
				Parameters: map[string]string{},
			},
			getCredentialsErr: true,
			expectErr:         true,
		},
		"fail vol with less capacity": {
			volOpts: controller.VolumeOptions{
				PVName:     "test-name",
				PVC:        createFakePVC(requestedBytes),
				Parameters: map[string]string{},
			},
			volWithLessCap: true,
			expectErr:      true,
		},
		"provision with mount options": {
			volOpts: controller.VolumeOptions{
				PersistentVolumeReclaimPolicy: v1.PersistentVolumeReclaimDelete,
				PVName:                        "test-name",
				PVC: &v1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						UID: "testid",
					},
					Spec: v1.PersistentVolumeClaimSpec{
						Selector: nil,
						Resources: v1.ResourceRequirements{
							Requests: v1.ResourceList{
								v1.ResourceName(v1.ResourceStorage): resource.MustParse(strconv.FormatInt(requestedBytes, 10)),
							},
						},
						AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
					},
				},
				MountOptions: []string{"foo=bar", "baz=qux"},
				Parameters:   map[string]string{},
			},
			expectedPVSpec: &pvSpec{
				Name:          "test-testi",
				ReclaimPolicy: v1.PersistentVolumeReclaimDelete,
				AccessModes:   []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
				Capacity: v1.ResourceList{
					v1.ResourceName(v1.ResourceStorage): bytesToGiQuantity(requestedBytes),
				},
				CSIPVS: &v1.CSIPersistentVolumeSource{
					Driver:       "test-driver",
					VolumeHandle: "test-volume-id",
					FSType:       "ext4",
					VolumeAttributes: map[string]string{
						"storage.kubernetes.io/csiProvisionerIdentity": "test-provisioner",
					},
				},
			},
			expectCreateVolDo: func(ctx context.Context, req *csi.CreateVolumeRequest) {
				if len(req.GetVolumeCapabilities()) != 1 {
					t.Errorf("Incorrect length in volume capabilities")
				}
				cap := req.GetVolumeCapabilities()[0]
				if cap.GetAccessMode() == nil {
					t.Errorf("Expected access mode to be set")
				}
				if cap.GetAccessMode().GetMode() != csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER {
					t.Errorf("Expected multi reade only")
				}
				if cap.GetMount() == nil {
					t.Errorf("Expected access type to be mount")
				}
				if !reflect.DeepEqual(cap.GetMount().MountFlags, []string{"foo=bar", "baz=qux"}) {
					t.Errorf("Expected 2 mount options")
				}
			},
		},
	}

	mockController, driver, identityServer, controllerServer, csiConn, err := createMockServer(t)
	if err != nil {
		t.Fatal(err)
	}
	defer mockController.Finish()
	defer driver.Stop()

	for k, tc := range testcases {
		var clientSet kubernetes.Interface

		if tc.withSecretRefs {
			clientSet = fakeclientset.NewSimpleClientset(&v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "ctrlpublishsecret",
					Namespace: "default",
				},
			}, &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "nodestagesecret",
					Namespace: "default",
				},
			}, &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "nodepublishsecret",
					Namespace: "default",
				},
			})
		} else {
			clientSet = fakeclientset.NewSimpleClientset()
		}

		csiProvisioner := NewCSIProvisioner(clientSet, nil, driver.Address(), 5*time.Second, "test-provisioner", "test", 5, csiConn.conn, nil)

		out := &csi.CreateVolumeResponse{
			Volume: &csi.Volume{
				CapacityBytes: requestedBytes,
				VolumeId:      "test-volume-id",
			},
		}

		if tc.withSecretRefs {
			tc.volOpts.Parameters[controllerPublishSecretNameKey] = "ctrlpublishsecret"
			tc.volOpts.Parameters[controllerPublishSecretNamespaceKey] = "default"
			tc.volOpts.Parameters[nodeStageSecretNameKey] = "nodestagesecret"
			tc.volOpts.Parameters[nodeStageSecretNamespaceKey] = "default"
			tc.volOpts.Parameters[nodePublishSecretNameKey] = "nodepublishsecret"
			tc.volOpts.Parameters[nodePublishSecretNamespaceKey] = "default"
		}

		if tc.notNilSelector {
			tc.volOpts.PVC.Spec.Selector = &metav1.LabelSelector{}
		} else if tc.driverNotReady {
			identityServer.EXPECT().GetPluginCapabilities(gomock.Any(), gomock.Any()).Return(nil, errors.New("driver not ready")).Times(1)
		} else if tc.makeVolumeNameErr {
			tc.volOpts.PVC.ObjectMeta.UID = ""
			provisionMockServerSetupExpectations(identityServer, controllerServer)
		} else if tc.getSecretRefErr {
			tc.volOpts.Parameters[provisionerSecretNameKey] = ""
			provisionMockServerSetupExpectations(identityServer, controllerServer)
		} else if tc.getCredentialsErr {
			tc.volOpts.Parameters[provisionerSecretNameKey] = "secretx"
			tc.volOpts.Parameters[provisionerSecretNamespaceKey] = "default"
			provisionMockServerSetupExpectations(identityServer, controllerServer)
		} else if tc.volWithLessCap {
			out.Volume.CapacityBytes = int64(80)
			provisionMockServerSetupExpectations(identityServer, controllerServer)
			controllerServer.EXPECT().CreateVolume(gomock.Any(), gomock.Any()).Return(out, nil).Times(1)
			controllerServer.EXPECT().DeleteVolume(gomock.Any(), gomock.Any()).Return(&csi.DeleteVolumeResponse{}, nil).Times(1)
		} else if tc.expectCreateVolDo != nil {
			provisionMockServerSetupExpectations(identityServer, controllerServer)
			controllerServer.EXPECT().CreateVolume(gomock.Any(), gomock.Any()).Do(tc.expectCreateVolDo).Return(out, nil).Times(1)
		} else {
			// Setup regular mock call expectations.
			provisionMockServerSetupExpectations(identityServer, controllerServer)
			controllerServer.EXPECT().CreateVolume(gomock.Any(), gomock.Any()).Return(out, nil).Times(1)
		}

		pv, err := csiProvisioner.Provision(tc.volOpts)
		if tc.expectErr && err == nil {
			t.Errorf("test %q: Expected error, got none", k)
		}
		if !tc.expectErr && err != nil {
			t.Errorf("test %q: got error: %v", k, err)
		}

		if tc.expectedPVSpec != nil {
			if pv.Name != tc.expectedPVSpec.Name {
				t.Errorf("test %q: expected PV name: %q, got: %q", k, tc.expectedPVSpec.Name, pv.Name)
			}

			if pv.Spec.PersistentVolumeReclaimPolicy != tc.expectedPVSpec.ReclaimPolicy {
				t.Errorf("test %q: expected reclaim policy: %v, got: %v", k, tc.expectedPVSpec.ReclaimPolicy, pv.Spec.PersistentVolumeReclaimPolicy)
			}

			if !reflect.DeepEqual(pv.Spec.AccessModes, tc.expectedPVSpec.AccessModes) {
				t.Errorf("test %q: expected access modes: %v, got: %v", k, tc.expectedPVSpec.AccessModes, pv.Spec.AccessModes)
			}

			if !reflect.DeepEqual(pv.Spec.VolumeMode, tc.expectedPVSpec.VolumeMode) {
				t.Errorf("test %q: expected volumeMode: %v, got: %v", k, tc.expectedPVSpec.VolumeMode, pv.Spec.VolumeMode)
			}

			if !reflect.DeepEqual(pv.Spec.Capacity, tc.expectedPVSpec.Capacity) {
				t.Errorf("test %q: expected capacity: %v, got: %v", k, tc.expectedPVSpec.Capacity, pv.Spec.Capacity)
			}

			if tc.expectedPVSpec.CSIPVS != nil {
				if !reflect.DeepEqual(pv.Spec.PersistentVolumeSource.CSI, tc.expectedPVSpec.CSIPVS) {
					t.Errorf("test %q: expected PV: %v, got: %v", k, tc.expectedPVSpec.CSIPVS, pv.Spec.PersistentVolumeSource.CSI)
				}
			}

		}
	}
}

// newSnapshot returns a new snapshot object
func newSnapshot(name, className, boundToContent, snapshotUID, claimName string, ready bool, err *storage.VolumeError, creationTime *metav1.Time, size *resource.Quantity) *crdv1.VolumeSnapshot {
	snapshot := crdv1.VolumeSnapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			Namespace:       "default",
			UID:             types.UID(snapshotUID),
			ResourceVersion: "1",
			SelfLink:        "/apis/snapshot.storage.k8s.io/v1alpha1/namespaces/" + "default" + "/volumesnapshots/" + name,
		},
		Spec: crdv1.VolumeSnapshotSpec{
			Source: &corev1.TypedLocalObjectReference{
				Name: claimName,
				Kind: "PersistentVolumeClaim",
			},
			VolumeSnapshotClassName: &className,
			SnapshotContentName:     boundToContent,
		},
		Status: crdv1.VolumeSnapshotStatus{
			CreationTime: creationTime,
			ReadyToUse:   ready,
			Error:        err,
			RestoreSize:  size,
		},
	}

	return &snapshot
}

// newContent returns a new content with given attributes
func newContent(name, className, snapshotHandle, volumeUID, volumeName, boundToSnapshotUID, boundToSnapshotName string, size *int64, creationTime *int64) *crdv1.VolumeSnapshotContent {
	content := crdv1.VolumeSnapshotContent{
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			ResourceVersion: "1",
		},
		Spec: crdv1.VolumeSnapshotContentSpec{
			VolumeSnapshotSource: crdv1.VolumeSnapshotSource{
				CSI: &crdv1.CSIVolumeSnapshotSource{
					RestoreSize:    size,
					Driver:         "test-driver",
					SnapshotHandle: snapshotHandle,
					CreationTime:   creationTime,
				},
			},
			VolumeSnapshotClassName: &className,
			PersistentVolumeRef: &v1.ObjectReference{
				Kind:       "PersistentVolume",
				APIVersion: "v1",
				UID:        types.UID(volumeUID),
				Name:       volumeName,
			},
		},
	}
	if boundToSnapshotName != "" {
		content.Spec.VolumeSnapshotRef = &v1.ObjectReference{
			Kind:       "VolumeSnapshot",
			APIVersion: "snapshot.storage.k8s.io/v1alpha1",
			UID:        types.UID(boundToSnapshotUID),
			Namespace:  "default",
			Name:       boundToSnapshotName,
		}
	}

	return &content
}

// TestProvisionFromSnapshot tests create volume from snapshot
func TestProvisionFromSnapshot(t *testing.T) {
	var apiGrp string = "snapshot.storage.k8s.io"
	var unsupportedAPIGrp string = "unsupported.group.io"
	var requestedBytes int64 = 1000
	var snapName string = "test-snapshot"
	var snapClassName string = "test-snapclass"
	var timeNow = time.Now().UnixNano()
	var metaTimeNowUnix = &metav1.Time{
		Time: time.Unix(0, timeNow),
	}

	type pvSpec struct {
		Name          string
		ReclaimPolicy v1.PersistentVolumeReclaimPolicy
		AccessModes   []v1.PersistentVolumeAccessMode
		Capacity      v1.ResourceList
		CSIPVS        *v1.CSIPersistentVolumeSource
	}

	testcases := map[string]struct {
		volOpts              controller.VolumeOptions
		restoredVolSizeSmall bool
		wrongDataSource      bool
		snapshotStatusReady  bool
		expectedPVSpec       *pvSpec
		expectErr            bool
	}{
		"provision with volume snapshot data source": {
			volOpts: controller.VolumeOptions{
				PersistentVolumeReclaimPolicy: v1.PersistentVolumeReclaimDelete,
				PVName:                        "test-name",
				PVC: &v1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						UID: "testid",
					},
					Spec: v1.PersistentVolumeClaimSpec{
						Selector: nil,
						Resources: v1.ResourceRequirements{
							Requests: v1.ResourceList{
								v1.ResourceName(v1.ResourceStorage): resource.MustParse(strconv.FormatInt(requestedBytes, 10)),
							},
						},
						AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
						DataSource: &v1.TypedLocalObjectReference{
							Name:     snapName,
							Kind:     "VolumeSnapshot",
							APIGroup: &apiGrp,
						},
					},
				},
				Parameters: map[string]string{},
			},
			snapshotStatusReady: true,
			expectedPVSpec: &pvSpec{
				Name:          "test-testi",
				ReclaimPolicy: v1.PersistentVolumeReclaimDelete,
				AccessModes:   []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
				Capacity: v1.ResourceList{
					v1.ResourceName(v1.ResourceStorage): bytesToGiQuantity(requestedBytes),
				},
				CSIPVS: &v1.CSIPersistentVolumeSource{
					Driver:       "test-driver",
					VolumeHandle: "test-volume-id",
					FSType:       "ext4",
					VolumeAttributes: map[string]string{
						"storage.kubernetes.io/csiProvisionerIdentity": "test-provisioner",
					},
				},
			},
		},
		"fail vol size less than snapshot size": {
			volOpts: controller.VolumeOptions{
				PVName: "test-name",
				PVC: &v1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						UID: "testid",
					},
					Spec: v1.PersistentVolumeClaimSpec{
						Selector: nil,
						Resources: v1.ResourceRequirements{
							Requests: v1.ResourceList{
								v1.ResourceName(v1.ResourceStorage): resource.MustParse(strconv.FormatInt(100, 10)),
							},
						},
						AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
						DataSource: &v1.TypedLocalObjectReference{
							Name:     snapName,
							Kind:     "VolumeSnapshot",
							APIGroup: &apiGrp,
						},
					},
				},
				Parameters: map[string]string{},
			},
			restoredVolSizeSmall: true,
			snapshotStatusReady:  true,
			expectErr:            true,
		},
		"fail empty snapshot name": {
			volOpts: controller.VolumeOptions{
				PVName: "test-name",
				PVC: &v1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						UID: "testid",
					},
					Spec: v1.PersistentVolumeClaimSpec{
						Selector: nil,
						Resources: v1.ResourceRequirements{
							Requests: v1.ResourceList{
								v1.ResourceName(v1.ResourceStorage): resource.MustParse(strconv.FormatInt(requestedBytes, 10)),
							},
						},
						AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
						DataSource: &v1.TypedLocalObjectReference{
							Name:     "",
							Kind:     "VolumeSnapshot",
							APIGroup: &apiGrp,
						},
					},
				},
				Parameters: map[string]string{},
			},
			wrongDataSource: true,
			expectErr:       true,
		},
		"fail unsupported datasource kind": {
			volOpts: controller.VolumeOptions{
				PVName: "test-name",
				PVC: &v1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						UID: "testid",
					},
					Spec: v1.PersistentVolumeClaimSpec{
						Selector: nil,
						Resources: v1.ResourceRequirements{
							Requests: v1.ResourceList{
								v1.ResourceName(v1.ResourceStorage): resource.MustParse(strconv.FormatInt(requestedBytes, 10)),
							},
						},
						AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
						DataSource: &v1.TypedLocalObjectReference{
							Name:     "",
							Kind:     "UnsupportedKind",
							APIGroup: &apiGrp,
						},
					},
				},
				Parameters: map[string]string{},
			},
			wrongDataSource: true,
			expectErr:       true,
		},
		"fail unsupported apigroup": {
			volOpts: controller.VolumeOptions{
				PVName: "test-name",
				PVC: &v1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						UID: "testid",
					},
					Spec: v1.PersistentVolumeClaimSpec{
						Selector: nil,
						Resources: v1.ResourceRequirements{
							Requests: v1.ResourceList{
								v1.ResourceName(v1.ResourceStorage): resource.MustParse(strconv.FormatInt(requestedBytes, 10)),
							},
						},
						AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
						DataSource: &v1.TypedLocalObjectReference{
							Name:     snapName,
							Kind:     "VolumeSnapshot",
							APIGroup: &unsupportedAPIGrp,
						},
					},
				},
				Parameters: map[string]string{},
			},
			wrongDataSource: true,
			expectErr:       true,
		},
		"fail invalid snapshot status": {
			volOpts: controller.VolumeOptions{
				PVName: "test-name",
				PVC: &v1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						UID: "testid",
					},
					Spec: v1.PersistentVolumeClaimSpec{
						Selector: nil,
						Resources: v1.ResourceRequirements{
							Requests: v1.ResourceList{
								v1.ResourceName(v1.ResourceStorage): resource.MustParse(strconv.FormatInt(100, 10)),
							},
						},
						AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
						DataSource: &v1.TypedLocalObjectReference{
							Name:     snapName,
							Kind:     "VolumeSnapshot",
							APIGroup: &apiGrp,
						},
					},
				},
				Parameters: map[string]string{},
			},
			snapshotStatusReady: false,
			expectErr:           true,
		},
	}

	mockController, driver, identityServer, controllerServer, csiConn, err := createMockServer(t)
	if err != nil {
		t.Fatal(err)
	}
	defer mockController.Finish()
	defer driver.Stop()

	for k, tc := range testcases {
		var clientSet kubernetes.Interface
		clientSet = fakeclientset.NewSimpleClientset()
		client := &fake.Clientset{}

		client.AddReactor("get", "volumesnapshots", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
			snap := newSnapshot(snapName, snapClassName, "snapcontent-snapuid", "snapuid", "claim", tc.snapshotStatusReady, nil, metaTimeNowUnix, resource.NewQuantity(requestedBytes, resource.BinarySI))
			return true, snap, nil
		})

		client.AddReactor("get", "volumesnapshotcontents", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
			content := newContent("snapcontent-snapuid", snapClassName, "sid", "pv-uid", "volume", "snapuid", snapName, &requestedBytes, &timeNow)
			return true, content, nil
		})

		csiProvisioner := NewCSIProvisioner(clientSet, nil, driver.Address(), 5*time.Second, "test-provisioner", "test", 5, csiConn.conn, client)

		out := &csi.CreateVolumeResponse{
			Volume: &csi.Volume{
				CapacityBytes: requestedBytes,
				VolumeId:      "test-volume-id",
			},
		}

		// Setup mock call expectations. If tc.wrongDataSource is false, DataSource is valid
		// and the controller will proceed to check whether the plugin supports snapshot.
		// So in this case, we need the plugin to report snapshot support capabilities;
		// Otherwise, the controller will fail the operation so it won't check the capabilities.
		if tc.wrongDataSource == false {
			provisionFromSnapshotMockServerSetupExpectations(identityServer, controllerServer)
		}
		// If tc.restoredVolSizeSmall is true, or tc.wrongDataSource is true, or
		// tc.snapshotStatusReady is false,  create volume from snapshot operation will fail
		// early and therefore CreateVolume is not expected to be called.
		// When the following if condition is met, it is a valid create volume from snapshot
		// operation and CreateVolume is expected to be called.
		if tc.restoredVolSizeSmall == false && tc.wrongDataSource == false && tc.snapshotStatusReady {
			controllerServer.EXPECT().CreateVolume(gomock.Any(), gomock.Any()).Return(out, nil).Times(1)
		}

		pv, err := csiProvisioner.Provision(tc.volOpts)
		if tc.expectErr && err == nil {
			t.Errorf("test %q: Expected error, got none", k)
		}

		if !tc.expectErr && err != nil {
			t.Errorf("test %q: got error: %v", k, err)
		}

		if tc.expectedPVSpec != nil {
			if pv.Name != tc.expectedPVSpec.Name {
				t.Errorf("test %q: expected PV name: %q, got: %q", k, tc.expectedPVSpec.Name, pv.Name)
			}

			if !reflect.DeepEqual(pv.Spec.Capacity, tc.expectedPVSpec.Capacity) {
				t.Errorf("test %q: expected capacity: %v, got: %v", k, tc.expectedPVSpec.Capacity, pv.Spec.Capacity)
			}

			if tc.expectedPVSpec.CSIPVS != nil {
				if !reflect.DeepEqual(pv.Spec.PersistentVolumeSource.CSI, tc.expectedPVSpec.CSIPVS) {
					t.Errorf("test %q: expected PV: %v, got: %v", k, tc.expectedPVSpec.CSIPVS, pv.Spec.PersistentVolumeSource.CSI)
				}
			}
		}
	}
}

// TestProvisionWithTopology is a basic test of provisioner integration with topology functions.
func TestProvisionWithTopology(t *testing.T) {
	defer utilfeaturetesting.SetFeatureGateDuringTest(t, utilfeature.DefaultFeatureGate, features.Topology, true)()

	accessibleTopology := []*csi.Topology{
		{
			Segments: map[string]string{
				"com.example.csi/zone": "zone1",
				"com.example.csi/rack": "rack2",
			},
		},
	}
	expectedNodeAffinity := &v1.VolumeNodeAffinity{
		Required: &v1.NodeSelector{
			NodeSelectorTerms: []v1.NodeSelectorTerm{
				{
					MatchExpressions: []v1.NodeSelectorRequirement{
						{
							Key:      "com.example.csi/zone",
							Operator: v1.NodeSelectorOpIn,
							Values:   []string{"zone1"},
						},
						{
							Key:      "com.example.csi/rack",
							Operator: v1.NodeSelectorOpIn,
							Values:   []string{"rack2"},
						},
					},
				},
			},
		},
	}

	const requestBytes = 100
	mockController, driver, identityServer, controllerServer, csiConn, err := createMockServer(t)
	if err != nil {
		t.Fatal(err)
	}
	defer mockController.Finish()
	defer driver.Stop()

	clientSet := fakeclientset.NewSimpleClientset()
	csiClientSet := fakecsiclientset.NewSimpleClientset()
	csiProvisioner := NewCSIProvisioner(clientSet, csiClientSet, driver.Address(), 5*time.Second, "test-provisioner", "test", 5, csiConn.conn, nil)

	out := &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			CapacityBytes:      requestBytes,
			VolumeId:           "test-volume-id",
			AccessibleTopology: accessibleTopology,
		},
	}

	provisionWithTopologyMockServerSetupExpectations(identityServer, controllerServer)
	controllerServer.EXPECT().CreateVolume(gomock.Any(), gomock.Any()).Return(out, nil).Times(1)

	pv, err := csiProvisioner.Provision(controller.VolumeOptions{
		PVC: createFakePVC(requestBytes), // dummy PVC
	})
	if err != nil {
		t.Errorf("got error from Provision call: %v", err)
	}

	if !volumeNodeAffinitiesEqual(pv.Spec.NodeAffinity, expectedNodeAffinity) {
		t.Errorf("expected node affinity %v; got: %v", expectedNodeAffinity, pv.Spec.NodeAffinity)
	}
}

// TestProvisionWithMountOptions is a test of provisioner integration with mount options.
func TestProvisionWithMountOptions(t *testing.T) {
	expectedOptions := []string{"foo=bar", "baz=qux"}
	const requestBytes = 100
	mockController, driver, identityServer, controllerServer, csiConn, err := createMockServer(t)
	if err != nil {
		t.Fatal(err)
	}
	defer mockController.Finish()
	defer driver.Stop()

	clientSet := fakeclientset.NewSimpleClientset()
	csiClientSet := fakecsiclientset.NewSimpleClientset()
	csiProvisioner := NewCSIProvisioner(clientSet, csiClientSet, driver.Address(), 5*time.Second, "test-provisioner", "test", 5, csiConn.conn, nil)

	out := &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			CapacityBytes: requestBytes,
			VolumeId:      "test-volume-id",
		},
	}

	provisionWithTopologyMockServerSetupExpectations(identityServer, controllerServer)
	controllerServer.EXPECT().CreateVolume(gomock.Any(), gomock.Any()).Return(out, nil).Times(1)

	pv, err := csiProvisioner.Provision(controller.VolumeOptions{
		PVC:          createFakePVC(requestBytes), // dummy PVC
		MountOptions: expectedOptions,
	})
	if err != nil {
		t.Errorf("got error from Provision call: %v", err)
	}

	if !reflect.DeepEqual(pv.Spec.MountOptions, expectedOptions) {
		t.Errorf("expected mount options %v; got: %v", expectedOptions, pv.Spec.MountOptions)
	}
}
