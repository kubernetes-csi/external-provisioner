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
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"testing"
	"time"

	csi "github.com/container-storage-interface/spec/lib/go/csi/v0"
	"github.com/golang/mock/gomock"
	"github.com/kubernetes-csi/csi-test/driver"
	"github.com/kubernetes-incubator/external-storage/lib/controller"
	"google.golang.org/grpc"
	"k8s.io/api/core/v1"
	"k8s.io/api/storage/v1beta1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	fakeclientset "k8s.io/client-go/kubernetes/fake"
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
	oldName, err := getDriverName(csiConn.conn, timeout)
	if err != nil {
		t.Errorf("test %q: Failed to get driver's name", test.name)
	}
	if oldName != test.output[0].Name {
		t.Errorf("test %s: failed, expected %s got %s", test.name, test.output[0].Name, oldName)
	}

	out = test.output[1]
	identityServer.EXPECT().GetPluginInfo(gomock.Any(), in).Return(out, nil).Times(1)
	newName, err := getDriverName(csiConn.conn, timeout)
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

	csiProvisioner := NewCSIProvisioner(nil, driver.Address(), 5*time.Second, "test-provisioner", "test", 5, csiConn.conn)

	// Requested PVC with requestedBytes storage
	opts := controller.VolumeOptions{
		PersistentVolumeReclaimPolicy: v1.PersistentVolumeReclaimDelete,
		PVName:     "test-name",
		PVC:        createFakePVC(requestedBytes),
		Parameters: map[string]string{},
	}

	// Drivers CreateVolume response with lower capacity bytes than request
	out := &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			CapacityBytes: requestedBytes - 1,
			Id:            "test-volume-id",
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

func TestDelete(t *testing.T) {
	var requestedBytes int64 = 100
	var storageClassName = "test-storageclass"

	testcases := map[string]struct {
		noVol               bool
		noCSI               bool
		ctrlDeleteErr       bool // Error from CSI Controller's DeleteVolume()
		driverNotReady      bool
		missingStorageClass bool // StorageClass object not found.
		expectErr           bool
	}{
		"normal delete": {},
		"no volume": {
			noVol:     true,
			expectErr: true,
		},
		"no CSI": {
			noCSI:     true,
			expectErr: true,
		},
		"error from controller": {
			ctrlDeleteErr: true,
			expectErr:     true,
		},
		"driver not ready": {
			driverNotReady: true,
			expectErr:      true,
		},
		"missing storageclass": {
			missingStorageClass: true,
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

		if tc.missingStorageClass {
			clientSet = fakeclientset.NewSimpleClientset()
		} else {
			clientSet = fakeclientset.NewSimpleClientset(&v1beta1.StorageClass{
				ObjectMeta: metav1.ObjectMeta{
					Name: storageClassName,
				},
			})
		}

		csiProvisioner := NewCSIProvisioner(clientSet, driver.Address(), 5*time.Second, "test-provisioner", "test", 5, csiConn.conn)

		opts := controller.VolumeOptions{
			PersistentVolumeReclaimPolicy: v1.PersistentVolumeReclaimDelete,
			PVName:     "test-name",
			PVC:        createFakePVC(requestedBytes),
			Parameters: map[string]string{},
		}

		opts.PVC.Spec.StorageClassName = &storageClassName

		out := &csi.CreateVolumeResponse{
			Volume: &csi.Volume{
				CapacityBytes: requestedBytes,
				Id:            "test-volume-id",
			},
		}

		var pv *v1.PersistentVolume

		if !tc.noVol {
			// Set up Mocks
			provisionMockServerSetupExpectations(identityServer, controllerServer)
			controllerServer.EXPECT().CreateVolume(gomock.Any(), gomock.Any()).Return(out, nil).Times(1)

			// Call provision
			pv, err = csiProvisioner.Provision(opts)
			if err != nil {
				t.Fatal(err)
			}

			if tc.noCSI {
				pv.Spec.CSI = nil
			} else {
				if tc.driverNotReady {
					identityServer.EXPECT().GetPluginCapabilities(gomock.Any(), gomock.Any()).Return(nil, errors.New("driver not ready")).Times(1)
				} else {
					provisionMockServerSetupExpectations(identityServer, controllerServer)
				}

				if tc.ctrlDeleteErr {
					controllerServer.EXPECT().DeleteVolume(gomock.Any(), gomock.Any()).Return(nil, errors.New("failed to delete")).Times(1)
				} else if tc.expectErr {
					controllerServer.EXPECT().DeleteVolume(gomock.Any(), gomock.Any()).Times(0)
				} else {
					controllerServer.EXPECT().DeleteVolume(gomock.Any(), &csi.DeleteVolumeRequest{
						VolumeId: "test-volume-id",
					}).Return(&csi.DeleteVolumeResponse{}, nil).Times(1)
				}
			}
		}

		err = csiProvisioner.Delete(pv)
		if tc.expectErr && err == nil {
			t.Errorf("test %q: Expected error, got none", k)
		}
		if !tc.expectErr && err != nil {
			t.Errorf("test %q: got error: %v", k, err)
		}
	}
}
