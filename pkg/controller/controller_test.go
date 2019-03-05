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
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/golang/mock/gomock"
	"github.com/kubernetes-csi/csi-lib-utils/connection"
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
	timeout    = 10 * time.Second
	driverName = "test-driver"
)

var (
	volumeModeFileSystem = v1.PersistentVolumeFilesystem
	volumeModeBlock      = v1.PersistentVolumeBlock
)

type csiConnection struct {
	conn *grpc.ClientConn
}

func New(address string) (csiConnection, error) {
	conn, err := connection.Connect(address)
	if err != nil {
		return csiConnection{}, err
	}
	return csiConnection{
		conn: conn,
	}, nil
}

func createMockServer(t *testing.T, tmpdir string) (*gomock.Controller,
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
	drv.StartOnAddress("unix", filepath.Join(tmpdir, "csi.sock"))

	// Create a client connection to it
	addr := drv.Address()
	csiConn, err := New(addr)
	if err != nil {
		return nil, nil, nil, nil, csiConnection{}, err
	}

	return mockController, drv, identityServer, controllerServer, csiConn, nil
}

func tempDir(t *testing.T) string {
	dir, err := ioutil.TempDir("", "external-attacher-test-")
	if err != nil {
		t.Fatalf("Cannot create temporary directory: %s", err)
	}
	return dir
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

	tmpdir := tempDir(t)
	defer os.RemoveAll(tmpdir)
	mockController, driver, identityServer, _, csiConn, err := createMockServer(t, tmpdir)
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

func TestStripPrefixedCSIParams(t *testing.T) {
	testcases := []struct {
		name           string
		params         map[string]string
		expectedParams map[string]string
		expectErr      bool
	}{
		{
			name:           "no prefix",
			params:         map[string]string{"csiFoo": "bar", "bim": "baz"},
			expectedParams: map[string]string{"csiFoo": "bar", "bim": "baz"},
		},
		{
			name:           "one prefixed",
			params:         map[string]string{prefixedControllerPublishSecretNameKey: "bar", "bim": "baz"},
			expectedParams: map[string]string{"bim": "baz"},
		},
		{
			name:           "prefix in value",
			params:         map[string]string{"foo": prefixedFsTypeKey, "bim": "baz"},
			expectedParams: map[string]string{"foo": prefixedFsTypeKey, "bim": "baz"},
		},
		{
			name: "all known prefixed",
			params: map[string]string{
				prefixedFsTypeKey:                           "csiBar",
				prefixedProvisionerSecretNameKey:            "csiBar",
				prefixedProvisionerSecretNamespaceKey:       "csiBar",
				prefixedControllerPublishSecretNameKey:      "csiBar",
				prefixedControllerPublishSecretNamespaceKey: "csiBar",
				prefixedNodeStageSecretNameKey:              "csiBar",
				prefixedNodeStageSecretNamespaceKey:         "csiBar",
				prefixedNodePublishSecretNameKey:            "csiBar",
				prefixedNodePublishSecretNamespaceKey:       "csiBar",
			},
			expectedParams: map[string]string{},
		},
		{
			name: "all known deprecated params not stripped",
			params: map[string]string{
				"fstype":                            "csiBar",
				provisionerSecretNameKey:            "csiBar",
				provisionerSecretNamespaceKey:       "csiBar",
				controllerPublishSecretNameKey:      "csiBar",
				controllerPublishSecretNamespaceKey: "csiBar",
				nodeStageSecretNameKey:              "csiBar",
				nodeStageSecretNamespaceKey:         "csiBar",
				nodePublishSecretNameKey:            "csiBar",
				nodePublishSecretNamespaceKey:       "csiBar",
			},
			expectedParams: map[string]string{
				"fstype":                            "csiBar",
				provisionerSecretNameKey:            "csiBar",
				provisionerSecretNamespaceKey:       "csiBar",
				controllerPublishSecretNameKey:      "csiBar",
				controllerPublishSecretNamespaceKey: "csiBar",
				nodeStageSecretNameKey:              "csiBar",
				nodeStageSecretNamespaceKey:         "csiBar",
				nodePublishSecretNameKey:            "csiBar",
				nodePublishSecretNamespaceKey:       "csiBar",
			},
		},

		{
			name:      "unknown prefixed var",
			params:    map[string]string{csiParameterPrefix + "bim": "baz"},
			expectErr: true,
		},
		{
			name:           "empty",
			params:         map[string]string{},
			expectedParams: map[string]string{},
		},
	}

	for _, tc := range testcases {
		t.Logf("test: %v", tc.name)

		newParams, err := removePrefixedParameters(tc.params)
		if err != nil {
			if tc.expectErr {
				continue
			} else {
				t.Fatalf("Encountered unexpected error: %v", err)
			}
		} else {
			if tc.expectErr {
				t.Fatalf("Did not get error when one was expected")
			}
		}

		eq := reflect.DeepEqual(newParams, tc.expectedParams)
		if !eq {
			t.Fatalf("Stripped parameters: %v not equal to expected parameters: %v", newParams, tc.expectedParams)
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

	tmpdir := tempDir(t)
	defer os.RemoveAll(tmpdir)
	mockController, driver, identityServer, _, csiConn, err := createMockServer(t, tmpdir)
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

	tmpdir := tempDir(t)
	defer os.RemoveAll(tmpdir)
	mockController, driver, _, controllerServer, csiConn, err := createMockServer(t, tmpdir)
	if err != nil {
		t.Fatal(err)
	}
	defer mockController.Finish()
	defer driver.Stop()

	pluginCaps, controllerCaps := provisionCapabilities()
	csiProvisioner := NewCSIProvisioner(nil, nil, 5*time.Second, "test-provisioner", "test", 5, csiConn.conn, nil, driverName, pluginCaps, controllerCaps)

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

func provisionCapabilities() (connection.PluginCapabilitySet, connection.ControllerCapabilitySet) {
	return connection.PluginCapabilitySet{
			csi.PluginCapability_Service_CONTROLLER_SERVICE: true,
		}, connection.ControllerCapabilitySet{
			csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME: true,
		}
}

func provisionFromSnapshotCapabilities() (connection.PluginCapabilitySet, connection.ControllerCapabilitySet) {
	return connection.PluginCapabilitySet{
			csi.PluginCapability_Service_CONTROLLER_SERVICE: true,
		}, connection.ControllerCapabilitySet{
			csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME:   true,
			csi.ControllerServiceCapability_RPC_CREATE_DELETE_SNAPSHOT: true,
		}
}

func provisionWithTopologyCapabilities() (connection.PluginCapabilitySet, connection.ControllerCapabilitySet) {
	return connection.PluginCapabilitySet{
			csi.PluginCapability_Service_CONTROLLER_SERVICE:               true,
			csi.PluginCapability_Service_VOLUME_ACCESSIBILITY_CONSTRAINTS: true,
		}, connection.ControllerCapabilitySet{
			csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME: true,
		}
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
		secretParams deprecatedSecretParamsMap
		params       map[string]string
		pvName       string
		pvc          *v1.PersistentVolumeClaim

		expectRef *v1.SecretReference
		expectErr bool
	}{
		"no params": {
			secretParams: nodePublishSecretParams,
			params:       nil,
			expectRef:    nil,
		},
		"empty err": {
			secretParams: nodePublishSecretParams,
			params:       map[string]string{nodePublishSecretNameKey: "", nodePublishSecretNamespaceKey: ""},
			expectErr:    true,
		},
		"[deprecated] name, no namespace": {
			secretParams: nodePublishSecretParams,
			params:       map[string]string{nodePublishSecretNameKey: "foo"},
			expectErr:    true,
		},
		"name, no namespace": {
			secretParams: nodePublishSecretParams,
			params:       map[string]string{prefixedNodePublishSecretNameKey: "foo"},
			expectErr:    true,
		},
		"[deprecated] namespace, no name": {
			secretParams: nodePublishSecretParams,
			params:       map[string]string{nodePublishSecretNamespaceKey: "foo"},
			expectErr:    true,
		},
		"namespace, no name": {
			secretParams: nodePublishSecretParams,
			params:       map[string]string{prefixedNodePublishSecretNamespaceKey: "foo"},
			expectErr:    true,
		},
		"[deprecated] simple - valid": {
			secretParams: nodePublishSecretParams,
			params:       map[string]string{nodePublishSecretNameKey: "name", nodePublishSecretNamespaceKey: "ns"},
			pvc:          &v1.PersistentVolumeClaim{},
			expectRef:    &v1.SecretReference{Name: "name", Namespace: "ns"},
		},
		"deprecated and new both": {
			secretParams: nodePublishSecretParams,
			params:       map[string]string{nodePublishSecretNameKey: "name", nodePublishSecretNamespaceKey: "ns", prefixedNodePublishSecretNameKey: "name", prefixedNodePublishSecretNamespaceKey: "ns"},
			expectErr:    true,
		},
		"deprecated and new names": {
			secretParams: nodePublishSecretParams,
			params:       map[string]string{nodePublishSecretNameKey: "name", nodePublishSecretNamespaceKey: "ns", prefixedNodePublishSecretNameKey: "name"},
			expectErr:    true,
		},
		"deprecated and new namespace": {
			secretParams: nodePublishSecretParams,
			params:       map[string]string{nodePublishSecretNameKey: "name", nodePublishSecretNamespaceKey: "ns", prefixedNodePublishSecretNamespaceKey: "ns"},
			expectErr:    true,
		},
		"deprecated and new mixed": {
			secretParams: nodePublishSecretParams,
			params:       map[string]string{nodePublishSecretNameKey: "name", prefixedNodePublishSecretNamespaceKey: "ns"},
			pvc:          &v1.PersistentVolumeClaim{},
			expectRef:    &v1.SecretReference{Name: "name", Namespace: "ns"},
		},
		"simple - valid": {
			secretParams: nodePublishSecretParams,
			params:       map[string]string{prefixedNodePublishSecretNameKey: "name", prefixedNodePublishSecretNamespaceKey: "ns"},
			pvc:          &v1.PersistentVolumeClaim{},
			expectRef:    &v1.SecretReference{Name: "name", Namespace: "ns"},
		},
		"simple - valid, no pvc": {
			secretParams: provisionerSecretParams,
			params:       map[string]string{provisionerSecretNameKey: "name", provisionerSecretNamespaceKey: "ns"},
			pvc:          nil,
			expectRef:    &v1.SecretReference{Name: "name", Namespace: "ns"},
		},
		"simple - invalid name": {
			secretParams: nodePublishSecretParams,
			params:       map[string]string{nodePublishSecretNameKey: "bad name", nodePublishSecretNamespaceKey: "ns"},
			pvc:          &v1.PersistentVolumeClaim{},
			expectRef:    nil,
			expectErr:    true,
		},
		"simple - invalid namespace": {
			secretParams: nodePublishSecretParams,
			params:       map[string]string{nodePublishSecretNameKey: "name", nodePublishSecretNamespaceKey: "bad ns"},
			pvc:          &v1.PersistentVolumeClaim{},
			expectRef:    nil,
			expectErr:    true,
		},
		"template - valid": {
			secretParams: nodePublishSecretParams,
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
		},
		"template - invalid namespace tokens": {
			secretParams: nodePublishSecretParams,
			params: map[string]string{
				nodePublishSecretNameKey:      "myname",
				nodePublishSecretNamespaceKey: "mynamespace${bar}",
			},
			pvc:       &v1.PersistentVolumeClaim{},
			expectRef: nil,
			expectErr: true,
		},
		"template - invalid name tokens": {
			secretParams: nodePublishSecretParams,
			params: map[string]string{
				nodePublishSecretNameKey:      "myname${foo}",
				nodePublishSecretNamespaceKey: "mynamespace",
			},
			pvc:       &v1.PersistentVolumeClaim{},
			expectRef: nil,
			expectErr: true,
		},
	}

	for k, tc := range testcases {
		t.Run(k, func(t *testing.T) {
			ref, err := getSecretReference(tc.secretParams, tc.params, tc.pvName, tc.pvc)
			if err != nil {
				if tc.expectErr {
					return
				}
				t.Fatalf("Did not expect error but got: %v", err)
			} else {
				if tc.expectErr {
					t.Fatalf("Expected error but got none")
				}
			}
			if !reflect.DeepEqual(ref, tc.expectRef) {
				t.Errorf("Expected %v, got %v", tc.expectRef, ref)
			}
		})
	}
}

type provisioningTestcase struct {
	volOpts           controller.VolumeOptions
	notNilSelector    bool
	makeVolumeNameErr bool
	getSecretRefErr   bool
	getCredentialsErr bool
	volWithLessCap    bool
	expectedPVSpec    *pvSpec
	withSecretRefs    bool
	expectErr         bool
	expectCreateVolDo interface{}
}

type pvSpec struct {
	Name          string
	ReclaimPolicy v1.PersistentVolumeReclaimPolicy
	AccessModes   []v1.PersistentVolumeAccessMode
	VolumeMode    *v1.PersistentVolumeMode
	Capacity      v1.ResourceList
	CSIPVS        *v1.CSIPersistentVolumeSource
}

func TestProvision(t *testing.T) {
	var requestedBytes int64 = 100
	testcases := map[string]provisioningTestcase{
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
		"multiple fsType provision": {
			volOpts: controller.VolumeOptions{
				PersistentVolumeReclaimPolicy: v1.PersistentVolumeReclaimDelete,
				PVName:                        "test-name",
				PVC:                           createFakePVC(requestedBytes),
				Parameters: map[string]string{
					"fstype":          "ext3",
					prefixedFsTypeKey: "ext4",
				},
			},
			expectErr: true,
		},
		"provision with prefixed FS Type key": {
			volOpts: controller.VolumeOptions{
				PersistentVolumeReclaimPolicy: v1.PersistentVolumeReclaimDelete,
				PVName:                        "test-name",
				PVC:                           createFakePVC(requestedBytes),
				Parameters: map[string]string{
					prefixedFsTypeKey: "ext3",
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
			expectCreateVolDo: func(ctx context.Context, req *csi.CreateVolumeRequest) {
				if len(req.Parameters) != 0 {
					t.Errorf("Parameters should have been stripped")
				}
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

	for k, tc := range testcases {
		runProvisionTest(t, k, tc, requestedBytes)
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

func runProvisionTest(t *testing.T, k string, tc provisioningTestcase, requestedBytes int64) {
	t.Logf("Running test: %v", k)

	tmpdir := tempDir(t)
	defer os.RemoveAll(tmpdir)
	mockController, driver, _, controllerServer, csiConn, err := createMockServer(t, tmpdir)
	if err != nil {
		t.Fatal(err)
	}
	defer mockController.Finish()
	defer driver.Stop()

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

	pluginCaps, controllerCaps := provisionCapabilities()
	csiProvisioner := NewCSIProvisioner(clientSet, nil, 5*time.Second, "test-provisioner", "test", 5, csiConn.conn, nil, driverName, pluginCaps, controllerCaps)

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
	} else if tc.makeVolumeNameErr {
		tc.volOpts.PVC.ObjectMeta.UID = ""
	} else if tc.getSecretRefErr {
		tc.volOpts.Parameters[provisionerSecretNameKey] = ""
	} else if tc.getCredentialsErr {
		tc.volOpts.Parameters[provisionerSecretNameKey] = "secretx"
		tc.volOpts.Parameters[provisionerSecretNamespaceKey] = "default"
	} else if tc.volWithLessCap {
		out.Volume.CapacityBytes = int64(80)
		controllerServer.EXPECT().CreateVolume(gomock.Any(), gomock.Any()).Return(out, nil).Times(1)
		controllerServer.EXPECT().DeleteVolume(gomock.Any(), gomock.Any()).Return(&csi.DeleteVolumeResponse{}, nil).Times(1)
	} else if tc.expectCreateVolDo != nil {
		controllerServer.EXPECT().CreateVolume(gomock.Any(), gomock.Any()).Do(tc.expectCreateVolDo).Return(out, nil).Times(1)
	} else {
		// Setup regular mock call expectations.
		if !tc.expectErr {
			controllerServer.EXPECT().CreateVolume(gomock.Any(), gomock.Any()).Return(out, nil).Times(1)
		}
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
	var apiGrp = "snapshot.storage.k8s.io"
	var unsupportedAPIGrp = "unsupported.group.io"
	var requestedBytes int64 = 1000
	var snapName = "test-snapshot"
	var snapClassName = "test-snapclass"
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

	tmpdir := tempDir(t)
	defer os.RemoveAll(tmpdir)
	mockController, driver, _, controllerServer, csiConn, err := createMockServer(t, tmpdir)
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

		pluginCaps, controllerCaps := provisionFromSnapshotCapabilities()
		csiProvisioner := NewCSIProvisioner(clientSet, nil, 5*time.Second, "test-provisioner", "test", 5, csiConn.conn, client, driverName, pluginCaps, controllerCaps)

		out := &csi.CreateVolumeResponse{
			Volume: &csi.Volume{
				CapacityBytes: requestedBytes,
				VolumeId:      "test-volume-id",
			},
		}

		// Setup mock call expectations.
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

	tmpdir := tempDir(t)
	defer os.RemoveAll(tmpdir)
	mockController, driver, _, controllerServer, csiConn, err := createMockServer(t, tmpdir)
	if err != nil {
		t.Fatal(err)
	}
	defer mockController.Finish()
	defer driver.Stop()

	clientSet := fakeclientset.NewSimpleClientset()
	csiClientSet := fakecsiclientset.NewSimpleClientset()
	pluginCaps, controllerCaps := provisionWithTopologyCapabilities()
	csiProvisioner := NewCSIProvisioner(clientSet, csiClientSet, 5*time.Second, "test-provisioner", "test", 5, csiConn.conn, nil, driverName, pluginCaps, controllerCaps)

	out := &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			CapacityBytes:      requestBytes,
			VolumeId:           "test-volume-id",
			AccessibleTopology: accessibleTopology,
		},
	}

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

	tmpdir := tempDir(t)
	defer os.RemoveAll(tmpdir)
	mockController, driver, _, controllerServer, csiConn, err := createMockServer(t, tmpdir)
	if err != nil {
		t.Fatal(err)
	}
	defer mockController.Finish()
	defer driver.Stop()

	clientSet := fakeclientset.NewSimpleClientset()
	csiClientSet := fakecsiclientset.NewSimpleClientset()
	pluginCaps, controllerCaps := provisionCapabilities()
	csiProvisioner := NewCSIProvisioner(clientSet, csiClientSet, 5*time.Second, "test-provisioner", "test", 5, csiConn.conn, nil, driverName, pluginCaps, controllerCaps)

	out := &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			CapacityBytes: requestBytes,
			VolumeId:      "test-volume-id",
		},
	}

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
