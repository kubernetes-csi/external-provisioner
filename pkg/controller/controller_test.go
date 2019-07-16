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

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/golang/mock/gomock"
	"github.com/kubernetes-csi/csi-lib-utils/connection"
	"github.com/kubernetes-csi/csi-test/driver"
	"github.com/kubernetes-csi/external-provisioner/pkg/features"
	crdv1 "github.com/kubernetes-csi/external-snapshotter/pkg/apis/volumesnapshot/v1alpha1"
	"github.com/kubernetes-csi/external-snapshotter/pkg/client/clientset/versioned/fake"
	"google.golang.org/grpc"
	v1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	storagev1beta1 "k8s.io/api/storage/v1beta1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	utilfeaturetesting "k8s.io/apiserver/pkg/util/feature/testing"
	"k8s.io/client-go/kubernetes"
	fakeclientset "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
	"sigs.k8s.io/sig-storage-lib-external-provisioner/controller"
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
				prefixedControllerExpandSecretNameKey:       "csiBar",
				prefixedControllerExpandSecretNamespaceKey:  "csiBar",
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
	csiProvisioner := NewCSIProvisioner(nil, 5*time.Second, "test-provisioner", "test", 5, csiConn.conn, nil, driverName, pluginCaps, controllerCaps, "", false)

	// Requested PVC with requestedBytes storage
	deletePolicy := v1.PersistentVolumeReclaimDelete
	opts := controller.ProvisionOptions{
		StorageClass: &storagev1.StorageClass{
			ReclaimPolicy: &deletePolicy,
			Parameters:    map[string]string{},
		},
		PVName: "test-name",
		PVC:    createFakePVC(requestedBytes),
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

func provisionFromPVCCapabilities() (connection.PluginCapabilitySet, connection.ControllerCapabilitySet) {
	return connection.PluginCapabilitySet{
			csi.PluginCapability_Service_CONTROLLER_SERVICE: true,
		}, connection.ControllerCapabilitySet{
			csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME: true,
			csi.ControllerServiceCapability_RPC_CLONE_VOLUME:         true,
		}
}

// Minimal PVC required for tests to function
func createFakePVC(requestBytes int64) *v1.PersistentVolumeClaim {
	return &v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			UID:  "testid",
			Name: "fake-pvc",
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

// fakeClaim returns a valid PVC with the requested settings
func fakeClaim(name, claimUID, capacity, boundToVolume string, phase v1.PersistentVolumeClaimPhase, class *string) *v1.PersistentVolumeClaim {
	claim := v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			Namespace:       "testns",
			UID:             types.UID(claimUID),
			ResourceVersion: "1",
			SelfLink:        "/api/v1/namespaces/testns/persistentvolumeclaims/" + name,
		},
		Spec: v1.PersistentVolumeClaimSpec{
			AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce, v1.ReadOnlyMany},
			Resources: v1.ResourceRequirements{
				Requests: v1.ResourceList{
					v1.ResourceName(v1.ResourceStorage): resource.MustParse(capacity),
				},
			},
			VolumeName:       boundToVolume,
			StorageClassName: class,
		},
		Status: v1.PersistentVolumeClaimStatus{
			Phase: phase,
		},
	}

	if phase == v1.ClaimBound {
		claim.Status.AccessModes = claim.Spec.AccessModes
		claim.Status.Capacity = claim.Spec.Resources.Requests
	}

	return &claim

}
func TestGetSecretReference(t *testing.T) {
	testcases := map[string]struct {
		secretParams secretParamsMap
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
		"simple - valid, pvc name and namespace": {
			secretParams: provisionerSecretParams,
			params: map[string]string{
				provisionerSecretNameKey:      "param-name",
				provisionerSecretNamespaceKey: "param-ns",
			},
			pvc: &v1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "ns",
				},
			},
			expectRef: &v1.SecretReference{Name: "param-name", Namespace: "param-ns"},
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
		"template - PVC name annotations not supported for Provision and Delete": {
			secretParams: provisionerSecretParams,
			params: map[string]string{
				prefixedProvisionerSecretNameKey: "static-${pv.name}-${pvc.namespace}-${pvc.name}-${pvc.annotations['akey']}",
			},
			pvc: &v1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "ns",
				},
			},
			expectErr: true,
		},
		"template - valid nodepublish secret ref": {
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
		"template - valid provisioner secret ref": {
			secretParams: provisionerSecretParams,
			params: map[string]string{
				provisionerSecretNameKey:      "static-provisioner-${pv.name}-${pvc.namespace}-${pvc.name}",
				provisionerSecretNamespaceKey: "static-provisioner-${pv.name}-${pvc.namespace}",
			},
			pvName: "pvname",
			pvc: &v1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pvcname",
					Namespace: "pvcnamespace",
				},
			},
			expectRef: &v1.SecretReference{Name: "static-provisioner-pvname-pvcnamespace-pvcname", Namespace: "static-provisioner-pvname-pvcnamespace"},
		},
		"template - valid, with pvc.name": {
			secretParams: provisionerSecretParams,
			params: map[string]string{
				provisionerSecretNameKey:      "${pvc.name}",
				provisionerSecretNamespaceKey: "ns",
			},
			pvc: &v1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pvcname",
					Namespace: "pvcns",
				},
			},
			expectRef: &v1.SecretReference{Name: "pvcname", Namespace: "ns"},
		},
		"template - valid, provisioner with pvc name and namepsace": {
			secretParams: provisionerSecretParams,
			params: map[string]string{
				provisionerSecretNameKey:      "${pvc.name}",
				provisionerSecretNamespaceKey: "${pvc.namespace}",
			},
			pvc: &v1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pvcname",
					Namespace: "pvcns",
				},
			},
			expectRef: &v1.SecretReference{Name: "pvcname", Namespace: "pvcns"},
		},
		"template - valid, static pvc name and templated namespace": {
			secretParams: provisionerSecretParams,
			params: map[string]string{
				provisionerSecretNameKey:      "static-name-1",
				provisionerSecretNamespaceKey: "${pvc.namespace}",
			},
			pvc: &v1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "ns",
				},
			},
			expectRef: &v1.SecretReference{Name: "static-name-1", Namespace: "ns"},
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
	volOpts           controller.ProvisionOptions
	notNilSelector    bool
	makeVolumeNameErr bool
	getSecretRefErr   bool
	getCredentialsErr bool
	volWithLessCap    bool
	expectedPVSpec    *pvSpec
	withSecretRefs    bool
	createVolumeError error
	expectErr         bool
	expectState       controller.ProvisioningState
	expectCreateVolDo interface{}
}

type pvSpec struct {
	Name          string
	ReclaimPolicy v1.PersistentVolumeReclaimPolicy
	AccessModes   []v1.PersistentVolumeAccessMode
	MountOptions  []string
	VolumeMode    *v1.PersistentVolumeMode
	Capacity      v1.ResourceList
	CSIPVS        *v1.CSIPersistentVolumeSource
}

func TestProvision(t *testing.T) {
	var requestedBytes int64 = 100
	deletePolicy := v1.PersistentVolumeReclaimDelete
	testcases := map[string]provisioningTestcase{
		"normal provision": {
			volOpts: controller.ProvisionOptions{
				StorageClass: &storagev1.StorageClass{
					ReclaimPolicy: &deletePolicy,
					Parameters: map[string]string{
						"fstype": "ext3",
					},
				},
				PVName: "test-name",
				PVC:    createFakePVC(requestedBytes),
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
			expectState: controller.ProvisioningFinished,
		},
		"multiple fsType provision": {
			volOpts: controller.ProvisionOptions{
				StorageClass: &storagev1.StorageClass{
					ReclaimPolicy: &deletePolicy,
					Parameters: map[string]string{
						"fstype":          "ext3",
						prefixedFsTypeKey: "ext4",
					},
				},
				PVName: "test-name",
				PVC:    createFakePVC(requestedBytes),
			},
			expectErr:   true,
			expectState: controller.ProvisioningFinished,
		},
		"provision with prefixed FS Type key": {
			volOpts: controller.ProvisionOptions{
				StorageClass: &storagev1.StorageClass{
					ReclaimPolicy: &deletePolicy,
					Parameters: map[string]string{
						prefixedFsTypeKey: "ext3",
					},
				},
				PVName: "test-name",
				PVC:    createFakePVC(requestedBytes),
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
			expectState: controller.ProvisioningFinished,
		},
		"provision with access mode multi node multi writer": {
			volOpts: controller.ProvisionOptions{
				StorageClass: &storagev1.StorageClass{
					ReclaimPolicy: &deletePolicy,
					Parameters:    map[string]string{},
				},
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
						AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadWriteMany},
					},
				},
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
			expectState: controller.ProvisioningFinished,
		},
		"provision with access mode multi node multi readonly": {
			volOpts: controller.ProvisionOptions{
				StorageClass: &storagev1.StorageClass{
					ReclaimPolicy: &deletePolicy,
					Parameters:    map[string]string{},
				},
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
						AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadOnlyMany},
					},
				},
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
			expectState: controller.ProvisioningFinished,
		},
		"provision with access mode single writer": {
			volOpts: controller.ProvisionOptions{
				StorageClass: &storagev1.StorageClass{
					ReclaimPolicy: &deletePolicy,
					Parameters:    map[string]string{},
				},
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
					},
				},
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
			expectState: controller.ProvisioningFinished,
		},
		"provision with multiple access modes": {
			volOpts: controller.ProvisionOptions{
				StorageClass: &storagev1.StorageClass{
					ReclaimPolicy: &deletePolicy,
					Parameters:    map[string]string{},
				},
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
						AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadOnlyMany, v1.ReadWriteOnce},
					},
				},
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
			expectState: controller.ProvisioningFinished,
		},
		"provision with secrets": {
			volOpts: controller.ProvisionOptions{
				StorageClass: &storagev1.StorageClass{
					ReclaimPolicy: &deletePolicy,
					Parameters:    map[string]string{},
				},
				PVName: "test-name",
				PVC:    createFakePVC(requestedBytes),
			},
			withSecretRefs: true,
			expectedPVSpec: &pvSpec{
				Name: "test-testi",
				Capacity: v1.ResourceList{
					v1.ResourceName(v1.ResourceStorage): bytesToGiQuantity(requestedBytes),
				},
				ReclaimPolicy: v1.PersistentVolumeReclaimDelete,
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
					ControllerExpandSecretRef: &v1.SecretReference{
						Name:      "controllerexpandsecret",
						Namespace: "default",
					},
				},
			},
			expectState: controller.ProvisioningFinished,
		},
		"provision with volume mode(Filesystem)": {
			volOpts: controller.ProvisionOptions{
				StorageClass: &storagev1.StorageClass{
					ReclaimPolicy: &deletePolicy,
					Parameters:    map[string]string{},
				},
				PVName: "test-name",
				PVC:    createFakePVCWithVolumeMode(requestedBytes, volumeModeFileSystem),
			},
			expectedPVSpec: &pvSpec{
				Name: "test-testi",
				Capacity: v1.ResourceList{
					v1.ResourceName(v1.ResourceStorage): bytesToGiQuantity(requestedBytes),
				},
				ReclaimPolicy: v1.PersistentVolumeReclaimDelete,
				VolumeMode:    &volumeModeFileSystem,
				CSIPVS: &v1.CSIPersistentVolumeSource{
					Driver:       "test-driver",
					VolumeHandle: "test-volume-id",
					FSType:       "ext4",
					VolumeAttributes: map[string]string{
						"storage.kubernetes.io/csiProvisionerIdentity": "test-provisioner",
					},
				},
			},
			expectState: controller.ProvisioningFinished,
		},
		"provision with volume mode(Block)": {
			volOpts: controller.ProvisionOptions{
				StorageClass: &storagev1.StorageClass{
					ReclaimPolicy: &deletePolicy,
					Parameters:    map[string]string{},
				},
				PVName: "test-name",
				PVC:    createFakePVCWithVolumeMode(requestedBytes, volumeModeBlock),
			},
			expectedPVSpec: &pvSpec{
				Name: "test-testi",
				Capacity: v1.ResourceList{
					v1.ResourceName(v1.ResourceStorage): bytesToGiQuantity(requestedBytes),
				},
				ReclaimPolicy: v1.PersistentVolumeReclaimDelete,
				VolumeMode:    &volumeModeBlock,
				CSIPVS: &v1.CSIPersistentVolumeSource{
					Driver:       "test-driver",
					VolumeHandle: "test-volume-id",
					VolumeAttributes: map[string]string{
						"storage.kubernetes.io/csiProvisionerIdentity": "test-provisioner",
					},
				},
			},
			expectState: controller.ProvisioningFinished,
		},
		"fail to get secret reference": {
			volOpts: controller.ProvisionOptions{
				StorageClass: &storagev1.StorageClass{
					Parameters: map[string]string{},
				},
				PVName: "test-name",
				PVC:    createFakePVC(requestedBytes),
			},
			getSecretRefErr: true,
			expectErr:       true,
			expectState:     controller.ProvisioningNoChange,
		},
		"fail not nil selector": {
			volOpts: controller.ProvisionOptions{
				PVName: "test-name",
				PVC:    createFakePVC(requestedBytes),
			},
			notNilSelector: true,
			expectErr:      true,
			expectState:    controller.ProvisioningFinished,
		},
		"fail to make volume name": {
			volOpts: controller.ProvisionOptions{
				PVName: "test-name",
				PVC:    createFakePVC(requestedBytes),
			},
			makeVolumeNameErr: true,
			expectErr:         true,
			expectState:       controller.ProvisioningFinished,
		},
		"fail to get credentials": {
			volOpts: controller.ProvisionOptions{
				StorageClass: &storagev1.StorageClass{
					Parameters: map[string]string{},
				},
				PVName: "test-name",
				PVC:    createFakePVC(requestedBytes),
			},
			getCredentialsErr: true,
			expectErr:         true,
			expectState:       controller.ProvisioningNoChange,
		},
		"fail vol with less capacity": {
			volOpts: controller.ProvisionOptions{
				StorageClass: &storagev1.StorageClass{
					Parameters: map[string]string{},
				},
				PVName: "test-name",
				PVC:    createFakePVC(requestedBytes),
			},
			volWithLessCap: true,
			expectErr:      true,
			expectState:    controller.ProvisioningInBackground,
		},
		"provision with mount options": {
			volOpts: controller.ProvisionOptions{
				StorageClass: &storagev1.StorageClass{
					Parameters:    map[string]string{},
					MountOptions:  []string{"foo=bar", "baz=qux"},
					ReclaimPolicy: &deletePolicy,
				},
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
					},
				},
			},
			expectedPVSpec: &pvSpec{
				Name:          "test-testi",
				ReclaimPolicy: v1.PersistentVolumeReclaimDelete,
				AccessModes:   []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
				MountOptions:  []string{"foo=bar", "baz=qux"},
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
			expectState: controller.ProvisioningFinished,
		},
		"provision with final error": {
			volOpts: controller.ProvisionOptions{
				StorageClass: &storagev1.StorageClass{
					Parameters:    map[string]string{},
					ReclaimPolicy: &deletePolicy,
				},
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
					},
				},
			},
			createVolumeError: status.Error(codes.Unauthenticated, "Mock final error"),
			expectCreateVolDo: func(ctx context.Context, req *csi.CreateVolumeRequest) {
				// intentionally empty
			},
			expectErr:   true,
			expectState: controller.ProvisioningFinished,
		},
		"provision with transient error": {
			volOpts: controller.ProvisionOptions{
				StorageClass: &storagev1.StorageClass{
					Parameters:    map[string]string{},
					ReclaimPolicy: &deletePolicy,
				},
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
					},
				},
			},
			createVolumeError: status.Error(codes.DeadlineExceeded, "Mock timeout"),
			expectCreateVolDo: func(ctx context.Context, req *csi.CreateVolumeRequest) {
				// intentionally empty
			},
			expectErr:   true,
			expectState: controller.ProvisioningInBackground,
		},
	}

	for k, tc := range testcases {
		runProvisionTest(t, k, tc, requestedBytes)
	}
}

// newSnapshot returns a new snapshot object
func newSnapshot(name, className, boundToContent, snapshotUID, claimName string, ready bool, err *storagev1beta1.VolumeError, creationTime *metav1.Time, size *resource.Quantity) *crdv1.VolumeSnapshot {
	snapshot := crdv1.VolumeSnapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			Namespace:       "default",
			UID:             types.UID(snapshotUID),
			ResourceVersion: "1",
			SelfLink:        "/apis/snapshot.storage.k8s.io/v1alpha1/namespaces/" + "default" + "/volumesnapshots/" + name,
		},
		Spec: crdv1.VolumeSnapshotSpec{
			Source: &v1.TypedLocalObjectReference{
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
		}, &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "controllerexpandsecret",
				Namespace: "default",
			},
		})
	} else {
		clientSet = fakeclientset.NewSimpleClientset()
	}

	pluginCaps, controllerCaps := provisionCapabilities()
	csiProvisioner := NewCSIProvisioner(clientSet, 5*time.Second, "test-provisioner", "test", 5, csiConn.conn, nil, driverName, pluginCaps, controllerCaps, "", false)

	out := &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			CapacityBytes: requestedBytes,
			VolumeId:      "test-volume-id",
		},
	}

	if tc.withSecretRefs {
		tc.volOpts.StorageClass.Parameters[controllerPublishSecretNameKey] = "ctrlpublishsecret"
		tc.volOpts.StorageClass.Parameters[controllerPublishSecretNamespaceKey] = "default"
		tc.volOpts.StorageClass.Parameters[nodeStageSecretNameKey] = "nodestagesecret"
		tc.volOpts.StorageClass.Parameters[nodeStageSecretNamespaceKey] = "default"
		tc.volOpts.StorageClass.Parameters[nodePublishSecretNameKey] = "nodepublishsecret"
		tc.volOpts.StorageClass.Parameters[nodePublishSecretNamespaceKey] = "default"
		tc.volOpts.StorageClass.Parameters[prefixedControllerExpandSecretNameKey] = "controllerexpandsecret"
		tc.volOpts.StorageClass.Parameters[prefixedControllerExpandSecretNamespaceKey] = "default"
	}

	if tc.notNilSelector {
		tc.volOpts.PVC.Spec.Selector = &metav1.LabelSelector{}
	} else if tc.makeVolumeNameErr {
		tc.volOpts.PVC.ObjectMeta.UID = ""
	} else if tc.getSecretRefErr {
		tc.volOpts.StorageClass.Parameters[provisionerSecretNameKey] = ""
	} else if tc.getCredentialsErr {
		tc.volOpts.StorageClass.Parameters[provisionerSecretNameKey] = "secretx"
		tc.volOpts.StorageClass.Parameters[provisionerSecretNamespaceKey] = "default"
	} else if tc.volWithLessCap {
		out.Volume.CapacityBytes = int64(80)
		controllerServer.EXPECT().CreateVolume(gomock.Any(), gomock.Any()).Return(out, tc.createVolumeError).Times(1)
		controllerServer.EXPECT().DeleteVolume(gomock.Any(), gomock.Any()).Return(&csi.DeleteVolumeResponse{}, tc.createVolumeError).Times(1)
	} else if tc.expectCreateVolDo != nil {
		controllerServer.EXPECT().CreateVolume(gomock.Any(), gomock.Any()).Do(tc.expectCreateVolDo).Return(out, tc.createVolumeError).Times(1)
	} else {
		// Setup regular mock call expectations.
		if !tc.expectErr {
			controllerServer.EXPECT().CreateVolume(gomock.Any(), gomock.Any()).Return(out, tc.createVolumeError).Times(1)
		}
	}

	pv, state, err := csiProvisioner.(controller.ProvisionerExt).ProvisionExt(tc.volOpts)
	if tc.expectErr && err == nil {
		t.Errorf("test %q: Expected error, got none", k)
	}
	if !tc.expectErr && err != nil {
		t.Errorf("test %q: got error: %v", k, err)
	}

	if tc.expectState == "" {
		tc.expectState = controller.ProvisioningFinished
	}
	if tc.expectState != state {
		t.Errorf("test %q: expected ProvisioningState %s, got %s", k, tc.expectState, state)
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

		if !reflect.DeepEqual(pv.Spec.MountOptions, tc.expectedPVSpec.MountOptions) {
			t.Errorf("test %q: expected mount options: %v, got: %v", k, tc.expectedPVSpec.MountOptions, pv.Spec.MountOptions)
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
	deletePolicy := v1.PersistentVolumeReclaimDelete

	type pvSpec struct {
		Name          string
		ReclaimPolicy v1.PersistentVolumeReclaimPolicy
		AccessModes   []v1.PersistentVolumeAccessMode
		Capacity      v1.ResourceList
		CSIPVS        *v1.CSIPersistentVolumeSource
	}

	testcases := map[string]struct {
		volOpts              controller.ProvisionOptions
		restoredVolSizeSmall bool
		wrongDataSource      bool
		snapshotStatusReady  bool
		expectedPVSpec       *pvSpec
		expectErr            bool
	}{
		"provision with volume snapshot data source": {
			volOpts: controller.ProvisionOptions{
				StorageClass: &storagev1.StorageClass{
					ReclaimPolicy: &deletePolicy,
					Parameters:    map[string]string{},
				},
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
							APIGroup: &apiGrp,
						},
					},
				},
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
			volOpts: controller.ProvisionOptions{
				StorageClass: &storagev1.StorageClass{
					Parameters: map[string]string{},
				},
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
			},
			restoredVolSizeSmall: true,
			snapshotStatusReady:  true,
			expectErr:            true,
		},
		"fail empty snapshot name": {
			volOpts: controller.ProvisionOptions{
				StorageClass: &storagev1.StorageClass{
					Parameters: map[string]string{},
				},
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
			},
			wrongDataSource: true,
			expectErr:       true,
		},
		"fail unsupported datasource kind": {
			volOpts: controller.ProvisionOptions{
				StorageClass: &storagev1.StorageClass{
					Parameters: map[string]string{},
				},
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
			},
			wrongDataSource: true,
			expectErr:       true,
		},
		"fail unsupported apigroup": {
			volOpts: controller.ProvisionOptions{
				StorageClass: &storagev1.StorageClass{
					Parameters: map[string]string{},
				},
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
			},
			wrongDataSource: true,
			expectErr:       true,
		},
		"fail invalid snapshot status": {
			volOpts: controller.ProvisionOptions{
				StorageClass: &storagev1.StorageClass{
					Parameters: map[string]string{},
				},
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
		csiProvisioner := NewCSIProvisioner(clientSet, 5*time.Second, "test-provisioner", "test", 5, csiConn.conn, client, driverName, pluginCaps, controllerCaps, "", false)

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
func TestProvisionWithTopologyEnabled(t *testing.T) {
	defer utilfeaturetesting.SetFeatureGateDuringTest(t, utilfeature.DefaultFeatureGate, features.Topology, true)()

	const requestBytes = 100

	testcases := map[string]struct {
		driverSupportsTopology bool
		nodeLabels             []map[string]string
		topologyKeys           []map[string][]string
		expectedNodeAffinity   *v1.VolumeNodeAffinity
		expectError            bool
	}{
		"topology success": {
			driverSupportsTopology: true,
			nodeLabels: []map[string]string{
				{"com.example.csi/zone": "zone1", "com.example.csi/rack": "rack1"},
				{"com.example.csi/zone": "zone1", "com.example.csi/rack": "rack2"},
			},
			topologyKeys: []map[string][]string{
				{driverName: []string{"com.example.csi/zone", "com.example.csi/rack"}},
				{driverName: []string{"com.example.csi/zone", "com.example.csi/rack"}},
			},
			expectedNodeAffinity: &v1.VolumeNodeAffinity{
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
			},
		},
		"topology fail": {
			driverSupportsTopology: true,
			topologyKeys: []map[string][]string{
				{driverName: []string{"com.example.csi/zone", "com.example.csi/rack"}},
				{driverName: []string{"com.example.csi/zone", "com.example.csi/rack"}},
			},
			expectError: true,
		},
		"driver doesn't support topology": {
			driverSupportsTopology: false,
			expectError:            false,
		},
	}

	accessibleTopology := []*csi.Topology{
		{
			Segments: map[string]string{
				"com.example.csi/zone": "zone1",
				"com.example.csi/rack": "rack2",
			},
		},
	}

	createVolumeOut := &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			CapacityBytes:      requestBytes,
			VolumeId:           "test-volume-id",
			AccessibleTopology: accessibleTopology,
		},
	}

	for name, tc := range testcases {
		t.Run(name, func(t *testing.T) {
			tmpdir := tempDir(t)
			defer os.RemoveAll(tmpdir)
			mockController, driver, _, controllerServer, csiConn, err := createMockServer(t, tmpdir)
			if err != nil {
				t.Fatal(err)
			}
			defer mockController.Finish()
			defer driver.Stop()

			if !tc.expectError {
				controllerServer.EXPECT().CreateVolume(gomock.Any(), gomock.Any()).Return(createVolumeOut, nil).Times(1)
			}

			nodes := buildNodes(tc.nodeLabels, k8sTopologyBetaVersion.String())
			nodeInfos := buildNodeInfos(tc.topologyKeys)

			var (
				pluginCaps     connection.PluginCapabilitySet
				controllerCaps connection.ControllerCapabilitySet
			)

			if tc.driverSupportsTopology {
				pluginCaps, controllerCaps = provisionWithTopologyCapabilities()
			} else {
				pluginCaps, controllerCaps = provisionCapabilities()
			}

			clientSet := fakeclientset.NewSimpleClientset(nodes, nodeInfos)
			csiProvisioner := NewCSIProvisioner(clientSet, 5*time.Second, "test-provisioner", "test", 5, csiConn.conn, nil, driverName, pluginCaps, controllerCaps, "", false)

			pv, err := csiProvisioner.Provision(controller.ProvisionOptions{
				StorageClass: &storagev1.StorageClass{},
				PVC:          createFakePVC(requestBytes),
			})
			if !tc.expectError {
				if err != nil {
					t.Fatalf("test %q failed: got error from Provision call: %v", name, err)
				}

				if !volumeNodeAffinitiesEqual(pv.Spec.NodeAffinity, tc.expectedNodeAffinity) {
					t.Errorf("test %q failed: expected node affinity %+v; got: %+v", name, tc.expectedNodeAffinity, pv.Spec.NodeAffinity)
				}
			}
			if tc.expectError {
				if err == nil {
					t.Errorf("test %q failed: expected error from Provision call, got success", name)
				}
				if pv != nil {
					t.Errorf("test %q failed: expected nil PV, got %+v", name, pv)
				}
			}
		})
	}
}

// TestProvisionWithTopologyDisabled checks that missing Node and CSINode objects, selectedNode
// are ignored and topology is not set on the PV
func TestProvisionWithTopologyDisabled(t *testing.T) {
	defer utilfeaturetesting.SetFeatureGateDuringTest(t, utilfeature.DefaultFeatureGate, features.Topology, false)()

	accessibleTopology := []*csi.Topology{
		{
			Segments: map[string]string{
				"com.example.csi/zone": "zone1",
				"com.example.csi/rack": "rack2",
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
	pluginCaps, controllerCaps := provisionWithTopologyCapabilities()
	csiProvisioner := NewCSIProvisioner(clientSet, 5*time.Second, "test-provisioner", "test", 5, csiConn.conn, nil, driverName, pluginCaps, controllerCaps, "", false)

	out := &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			CapacityBytes:      requestBytes,
			VolumeId:           "test-volume-id",
			AccessibleTopology: accessibleTopology,
		},
	}

	controllerServer.EXPECT().CreateVolume(gomock.Any(), gomock.Any()).Return(out, nil).Times(1)

	pv, err := csiProvisioner.Provision(controller.ProvisionOptions{
		StorageClass: &storagev1.StorageClass{},
		PVC:          createFakePVC(requestBytes),
		SelectedNode: &v1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: "some-node",
			},
		},
	})

	if err != nil {
		t.Fatalf("got error from Provision call: %v", err)
	}

	if pv.Spec.NodeAffinity != nil {
		t.Errorf("expected nil PV node affinity; got: %v", pv.Spec.NodeAffinity)
	}
}

type deleteTestcase struct {
	persistentVolume *v1.PersistentVolume
	storageClass     *storagev1.StorageClass
	mockDelete       bool
	expectErr        bool
}

// TestDelete is a test of the delete operation
func TestDelete(t *testing.T) {
	tt := map[string]deleteTestcase{
		"fail - nil PV": deleteTestcase{
			persistentVolume: nil,
			expectErr:        true,
		},
		"fail - nil volume.Spec.CSI": deleteTestcase{
			persistentVolume: &v1.PersistentVolume{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pv",
					Namespace: "ns",
					Annotations: map[string]string{
						prefixedProvisionerSecretNameKey: "static-${pv.name}-${pvc.namespace}-${pvc.name}-${pvc.annotations['akey']}",
					},
				},
				Spec: v1.PersistentVolumeSpec{
					PersistentVolumeSource: v1.PersistentVolumeSource{},
				},
			},
			expectErr: true,
		},
		"fail - pvc.annotations not supported": deleteTestcase{
			persistentVolume: &v1.PersistentVolume{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pv",
					Namespace: "ns",
				},
				Spec: v1.PersistentVolumeSpec{
					PersistentVolumeSource: v1.PersistentVolumeSource{
						CSI: &v1.CSIPersistentVolumeSource{
							VolumeHandle: "vol-id-1",
						},
					},
					ClaimRef: &v1.ObjectReference{
						Name:      "sc-name",
						Namespace: "ns",
					},
					StorageClassName: "sc-name",
				},
			},
			storageClass: &storagev1.StorageClass{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "sc-name",
					Namespace: "ns",
				},
				Parameters: map[string]string{
					prefixedProvisionerSecretNameKey: "static-${pv.name}-${pvc.namespace}-${pvc.name}-${pvc.annotations['akey']}",
				},
			},
			expectErr: true,
		},
		"simple - valid case": deleteTestcase{
			persistentVolume: &v1.PersistentVolume{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pv",
					Namespace: "ns",
				},
				Spec: v1.PersistentVolumeSpec{
					PersistentVolumeSource: v1.PersistentVolumeSource{
						CSI: &v1.CSIPersistentVolumeSource{
							VolumeHandle: "vol-id-1",
						},
					},
				},
			},
			storageClass: &storagev1.StorageClass{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "sc-name",
					Namespace: "ns",
				},
				Parameters: map[string]string{
					prefixedProvisionerSecretNameKey: "static-${pv.name}-${pvc.namespace}-${pvc.name}",
				},
			},
			expectErr:  false,
			mockDelete: true,
		},
		"simple - valid case with ClaimRef set": deleteTestcase{
			persistentVolume: &v1.PersistentVolume{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pv",
					Namespace: "ns",
				},
				Spec: v1.PersistentVolumeSpec{
					PersistentVolumeSource: v1.PersistentVolumeSource{
						CSI: &v1.CSIPersistentVolumeSource{
							VolumeHandle: "vol-id-1",
						},
					},
					ClaimRef: &v1.ObjectReference{
						Name:      "pvc-name",
						Namespace: "ns",
					},
				},
			},
			storageClass: &storagev1.StorageClass{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "sc-name",
					Namespace: "ns",
				},
				Parameters: map[string]string{
					prefixedProvisionerSecretNameKey: "static-${pv.name}-${pvc.namespace}-${pvc.name}",
				},
			},
			expectErr:  false,
			mockDelete: true,
		},
	}

	for k, tc := range tt {
		runDeleteTest(t, k, tc)
	}
}

func runDeleteTest(t *testing.T, k string, tc deleteTestcase) {
	t.Logf("Running test: %v", k)

	tmpdir := tempDir(t)

	defer os.RemoveAll(tmpdir)
	mockController, driver, _, controllerServer, csiConn, err := createMockServer(t, tmpdir)
	if err != nil {
		t.Fatal(err)
	}
	defer mockController.Finish()
	defer driver.Stop()

	var clientSet *fakeclientset.Clientset

	if tc.storageClass != nil {
		clientSet = fakeclientset.NewSimpleClientset(tc.storageClass)
	} else {
		clientSet = fakeclientset.NewSimpleClientset()
	}

	if tc.mockDelete {
		controllerServer.EXPECT().DeleteVolume(gomock.Any(), gomock.Any()).Return(&csi.DeleteVolumeResponse{}, nil).Times(1)
	}

	pluginCaps, controllerCaps := provisionCapabilities()
	csiProvisioner := NewCSIProvisioner(clientSet, 5*time.Second, "test-provisioner", "test", 5, csiConn.conn, nil, driverName, pluginCaps, controllerCaps, "", false)

	err = csiProvisioner.Delete(tc.persistentVolume)
	if tc.expectErr && err == nil {
		t.Errorf("test %q: Expected error, got none", k)
	}
	if !tc.expectErr && err != nil {
		t.Errorf("test %q: got error: %v", k, err)
	}
}

// TestProvisionFromPVC tests create volume clone
func TestProvisionFromPVC(t *testing.T) {
	var requestedBytes int64 = 1000
	invalidSCName := "invalid-sc"
	srcName := "fake-pvc"
	pvName := "test-testi"
	deletePolicy := v1.PersistentVolumeReclaimDelete

	type pvSpec struct {
		Name          string
		ReclaimPolicy v1.PersistentVolumeReclaimPolicy
		AccessModes   []v1.PersistentVolumeAccessMode
		Capacity      v1.ResourceList
		CSIPVS        *v1.CSIPersistentVolumeSource
	}

	testcases := map[string]struct {
		volOpts              controller.ProvisionOptions
		restoredVolSizeSmall bool
		wrongDataSource      bool
		pvcStatusReady       bool
		expectedPVSpec       *pvSpec
		cloneSupport         bool
		expectErr            bool
	}{
		"provision with pvc data source": {
			volOpts: controller.ProvisionOptions{
				StorageClass: &storagev1.StorageClass{
					ReclaimPolicy: &deletePolicy,
					Parameters:    map[string]string{},
				},
				PVName: pvName,
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
							Name: srcName,
							Kind: "PersistentVolumeClaim",
						},
					},
				},
			},
			pvcStatusReady: true,
			expectedPVSpec: &pvSpec{
				Name:          pvName,
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
			cloneSupport: true,
		},
		"provision with pvc data source no clone capability": {
			volOpts: controller.ProvisionOptions{
				StorageClass: &storagev1.StorageClass{
					ReclaimPolicy: &deletePolicy,
					Parameters:    map[string]string{},
				},
				PVName: pvName,
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
							Name: srcName,
							Kind: "PersistentVolumeClaim",
						},
					},
				},
			},
			pvcStatusReady: true,
			expectedPVSpec: nil,
			cloneSupport:   false,
			expectErr:      true,
		},
		"provision with pvc data source different storage classes": {
			volOpts: controller.ProvisionOptions{
				StorageClass: &storagev1.StorageClass{
					ReclaimPolicy: &deletePolicy,
					Parameters:    map[string]string{},
				},
				PVName: pvName,
				PVC: &v1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						UID: "testid",
					},
					Spec: v1.PersistentVolumeClaimSpec{
						StorageClassName: &invalidSCName,
						Selector:         nil,
						Resources: v1.ResourceRequirements{
							Requests: v1.ResourceList{
								v1.ResourceName(v1.ResourceStorage): resource.MustParse(strconv.FormatInt(requestedBytes, 10)),
							},
						},
						AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
						DataSource: &v1.TypedLocalObjectReference{
							Name: srcName,
							Kind: "PersistentVolumeClaim",
						},
					},
				},
			},
			pvcStatusReady: true,
			expectedPVSpec: nil,
			cloneSupport:   true,
			expectErr:      true,
		},
		"provision with pvc data source destination too small": {
			volOpts: controller.ProvisionOptions{
				StorageClass: &storagev1.StorageClass{
					ReclaimPolicy: &deletePolicy,
					Parameters:    map[string]string{},
				},
				PVName: pvName,
				PVC: &v1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						UID: "testid",
					},
					Spec: v1.PersistentVolumeClaimSpec{
						StorageClassName: &invalidSCName,
						Selector:         nil,
						Resources: v1.ResourceRequirements{
							Requests: v1.ResourceList{
								v1.ResourceName(v1.ResourceStorage): resource.MustParse(strconv.FormatInt(requestedBytes-1, 10)),
							},
						},
						AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
						DataSource: &v1.TypedLocalObjectReference{
							Name: srcName,
							Kind: "PersistentVolumeClaim",
						},
					},
				},
			},
			pvcStatusReady: true,
			expectedPVSpec: nil,
			cloneSupport:   true,
			expectErr:      true,
		},
		"provision with pvc data source not found": {
			volOpts: controller.ProvisionOptions{
				StorageClass: &storagev1.StorageClass{
					ReclaimPolicy: &deletePolicy,
					Parameters:    map[string]string{},
				},
				PVName: pvName,
				PVC: &v1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						UID: "testid",
					},
					Spec: v1.PersistentVolumeClaimSpec{
						StorageClassName: &invalidSCName,
						Selector:         nil,
						Resources: v1.ResourceRequirements{
							Requests: v1.ResourceList{
								v1.ResourceName(v1.ResourceStorage): resource.MustParse(strconv.FormatInt(requestedBytes-1, 10)),
							},
						},
						AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
						DataSource: &v1.TypedLocalObjectReference{
							Name: "source-not-found",
							Kind: "PersistentVolumeClaim",
						},
					},
				},
			},
			pvcStatusReady: true,
			expectedPVSpec: nil,
			cloneSupport:   true,
			expectErr:      true,
		},
		"provision with pvc data source destination too large": {
			volOpts: controller.ProvisionOptions{
				StorageClass: &storagev1.StorageClass{
					ReclaimPolicy: &deletePolicy,
					Parameters:    map[string]string{},
				},
				PVName: pvName,
				PVC: &v1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						UID: "testid",
					},
					Spec: v1.PersistentVolumeClaimSpec{
						StorageClassName: &invalidSCName,
						Selector:         nil,
						Resources: v1.ResourceRequirements{
							Requests: v1.ResourceList{
								v1.ResourceName(v1.ResourceStorage): resource.MustParse(strconv.FormatInt(requestedBytes+1, 10)),
							},
						},
						AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
						DataSource: &v1.TypedLocalObjectReference{
							Name: srcName,
							Kind: "PersistentVolumeClaim",
						},
					},
				},
			},
			pvcStatusReady: true,
			expectedPVSpec: nil,
			cloneSupport:   true,
			expectErr:      true,
		},
		"provision with pvc data source when source pv not found": {
			volOpts: controller.ProvisionOptions{
				StorageClass: &storagev1.StorageClass{
					ReclaimPolicy: &deletePolicy,
					Parameters:    map[string]string{},
				},
				PVName: "invalid-pv",
				PVC: &v1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						UID: "testid",
					},
					Spec: v1.PersistentVolumeClaimSpec{
						StorageClassName: &invalidSCName,
						Selector:         nil,
						Resources: v1.ResourceRequirements{
							Requests: v1.ResourceList{
								v1.ResourceName(v1.ResourceStorage): resource.MustParse(strconv.FormatInt(requestedBytes+1, 10)),
							},
						},
						AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
						DataSource: &v1.TypedLocalObjectReference{
							Name: srcName,
							Kind: "PersistentVolumeClaim",
						},
					},
				},
			},
			pvcStatusReady: true,
			expectedPVSpec: nil,
			cloneSupport:   true,
			expectErr:      true,
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
		var requestedBytes int64 = 1000
		var clientSet *fakeclientset.Clientset

		out := &csi.CreateVolumeResponse{
			Volume: &csi.Volume{
				CapacityBytes: requestedBytes,
				VolumeId:      "test-volume-id",
			},
		}

		clientSet = fakeclientset.NewSimpleClientset()

		// Create a fake claim as our PVC DataSource
		claim := fakeClaim(srcName, "fake-claim-uid", "1Gi", pvName, v1.ClaimBound, nil)

		pv := &v1.PersistentVolume{
			ObjectMeta: metav1.ObjectMeta{
				Name: pvName,
			},
			Spec: v1.PersistentVolumeSpec{
				PersistentVolumeSource: v1.PersistentVolumeSource{
					CSI: &v1.CSIPersistentVolumeSource{
						Driver:       "test-driver",
						VolumeHandle: "test-volume-id",
						FSType:       "ext3",
						VolumeAttributes: map[string]string{
							"storage.kubernetes.io/csiProvisionerIdentity": "test-provisioner",
						},
					},
				},
			},
		}
		clientSet = fakeclientset.NewSimpleClientset(claim, pv)

		pluginCaps, controllerCaps := provisionFromPVCCapabilities()
		if !tc.cloneSupport {
			pluginCaps, controllerCaps = provisionCapabilities()

		}
		if !tc.expectErr {
			controllerServer.EXPECT().CreateVolume(gomock.Any(), gomock.Any()).Return(out, nil).Times(1)

		}
		csiProvisioner := NewCSIProvisioner(clientSet, 5*time.Second, "test-provisioner", "test", 5, csiConn.conn, nil, driverName, pluginCaps, controllerCaps, "", false)

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
