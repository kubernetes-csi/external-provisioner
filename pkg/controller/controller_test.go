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
	"github.com/kubernetes-csi/csi-lib-utils/metrics"
	"github.com/kubernetes-csi/csi-lib-utils/rpc"
	"github.com/kubernetes-csi/csi-test/driver"
	"github.com/kubernetes-csi/external-provisioner/pkg/features"
	crdv1 "github.com/kubernetes-csi/external-snapshotter/pkg/apis/volumesnapshot/v1beta1"
	"github.com/kubernetes-csi/external-snapshotter/pkg/client/clientset/versioned/fake"
	"google.golang.org/grpc"
	v1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/client-go/kubernetes"
	fakeclientset "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
	utilfeaturetesting "k8s.io/component-base/featuregate/testing"
	csitrans "k8s.io/csi-translation-lib"
	"sigs.k8s.io/sig-storage-lib-external-provisioner/controller"
)

const (
	timeout    = 10 * time.Second
	driverName = "test-driver"
)

var (
	volumeModeFileSystem = v1.PersistentVolumeFilesystem
	volumeModeBlock      = v1.PersistentVolumeBlock

	driverNameAnnotation = map[string]string{annStorageProvisioner: driverName}
	translatedKey        = "translated"
)

type csiConnection struct {
	conn *grpc.ClientConn
}

func New(address string) (csiConnection, error) {
	metricsManager := metrics.NewCSIMetricsManager("fake.csi.driver.io" /* driverName */)
	conn, err := connection.Connect(address, metricsManager)
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
				prefixedDefaultSecretNameKey:                "csiBar",
				prefixedDefaultSecretNamespaceKey:           "csiBar",
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
	csiProvisioner := NewCSIProvisioner(nil, 5*time.Second, "test-provisioner", "test",
		5, csiConn.conn, nil, driverName, pluginCaps, controllerCaps, "", false, csitrans.New(), nil, nil, nil, nil, false)

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

func provisionCapabilities() (rpc.PluginCapabilitySet, rpc.ControllerCapabilitySet) {
	return rpc.PluginCapabilitySet{
			csi.PluginCapability_Service_CONTROLLER_SERVICE: true,
		}, rpc.ControllerCapabilitySet{
			csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME: true,
		}
}

func provisionFromSnapshotCapabilities() (rpc.PluginCapabilitySet, rpc.ControllerCapabilitySet) {
	return rpc.PluginCapabilitySet{
			csi.PluginCapability_Service_CONTROLLER_SERVICE: true,
		}, rpc.ControllerCapabilitySet{
			csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME:   true,
			csi.ControllerServiceCapability_RPC_CREATE_DELETE_SNAPSHOT: true,
		}
}

func provisionWithTopologyCapabilities() (rpc.PluginCapabilitySet, rpc.ControllerCapabilitySet) {
	return rpc.PluginCapabilitySet{
			csi.PluginCapability_Service_CONTROLLER_SERVICE:               true,
			csi.PluginCapability_Service_VOLUME_ACCESSIBILITY_CONSTRAINTS: true,
		}, rpc.ControllerCapabilitySet{
			csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME: true,
		}
}

func provisionFromPVCCapabilities() (rpc.PluginCapabilitySet, rpc.ControllerCapabilitySet) {
	return rpc.PluginCapabilitySet{
			csi.PluginCapability_Service_CONTROLLER_SERVICE: true,
		}, rpc.ControllerCapabilitySet{
			csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME: true,
			csi.ControllerServiceCapability_RPC_CLONE_VOLUME:         true,
		}
}

func createFakeNamedPVC(requestBytes int64, name string, userAnnotations map[string]string) *v1.PersistentVolumeClaim {
	annotations := map[string]string{annStorageProvisioner: driverName}
	for k, v := range userAnnotations {
		annotations[k] = v
	}

	return &v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			UID:         "testid",
			Name:        name,
			Namespace:   "fake-ns",
			Annotations: annotations,
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

// Minimal PVC required for tests to function
func createFakePVC(requestBytes int64) *v1.PersistentVolumeClaim {
	return createFakeNamedPVC(requestBytes, "fake-pvc", nil)
}

// createFakePVCWithVolumeMode returns PVC with VolumeMode
func createFakePVCWithVolumeMode(requestBytes int64, volumeMode v1.PersistentVolumeMode) *v1.PersistentVolumeClaim {
	claim := createFakePVC(requestBytes)
	claim.Spec.VolumeMode = &volumeMode
	return claim
}

// fakeClaim returns a valid PVC with the requested settings
func fakeClaim(name, namespace, claimUID string, capacity int64, boundToVolume string, phase v1.PersistentVolumeClaimPhase, class *string, mode string) *v1.PersistentVolumeClaim {
	claim := v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			Namespace:       namespace,
			UID:             types.UID(claimUID),
			ResourceVersion: "1",
			SelfLink:        "/api/v1/namespaces/testns/persistentvolumeclaims/" + name,
		},
		Spec: v1.PersistentVolumeClaimSpec{
			AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce, v1.ReadOnlyMany},
			Resources: v1.ResourceRequirements{
				Requests: v1.ResourceList{
					v1.ResourceName(v1.ResourceStorage): *resource.NewQuantity(capacity, resource.BinarySI),
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

	switch mode {
	case "block":
		claim.Spec.VolumeMode = &volumeModeBlock
	case "filesystem":
		claim.Spec.VolumeMode = &volumeModeFileSystem
	default:
		// leave it undefined/nil to maintaint the current defaults for test cases
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
	volWithZeroCap    bool
	expectedPVSpec    *pvSpec
	clientSetObjects  []runtime.Object
	createVolumeError error
	expectErr         bool
	expectState       controller.ProvisioningState
	expectCreateVolDo interface{}
	withExtraMetadata bool
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

const defaultSecretNsName = "default"

func getDefaultStorageClassSecretParameters() map[string]string {
	return map[string]string{
		controllerPublishSecretNameKey:             "ctrlpublishsecret",
		controllerPublishSecretNamespaceKey:        defaultSecretNsName,
		nodeStageSecretNameKey:                     "nodestagesecret",
		nodeStageSecretNamespaceKey:                defaultSecretNsName,
		nodePublishSecretNameKey:                   "nodepublishsecret",
		nodePublishSecretNamespaceKey:              defaultSecretNsName,
		prefixedControllerExpandSecretNameKey:      "controllerexpandsecret",
		prefixedControllerExpandSecretNamespaceKey: defaultSecretNsName,
	}
}

func getDefaultSecretObjects() []runtime.Object {
	return []runtime.Object{
		&v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "ctrlpublishsecret",
				Namespace: defaultSecretNsName,
			},
		}, &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "nodestagesecret",
				Namespace: defaultSecretNsName,
			},
		}, &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "nodepublishsecret",
				Namespace: defaultSecretNsName,
			},
		}, &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "controllerexpandsecret",
				Namespace: defaultSecretNsName,
			},
		},
	}
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
		"normal provision with extra metadata": {
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
			withExtraMetadata: true,
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
				pvc := createFakePVC(requestedBytes)
				expectedParams := map[string]string{
					pvcNameKey:      pvc.GetName(),
					pvcNamespaceKey: pvc.GetNamespace(),
					pvNameKey:       "test-testi",
					"fstype":        "ext3",
				}
				if fmt.Sprintf("%v", req.Parameters) != fmt.Sprintf("%v", expectedParams) { // only pvc name/namespace left
					t.Errorf("Unexpected parameters: %v", req.Parameters)
				}
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
						UID:         "testid",
						Annotations: driverNameAnnotation,
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
						UID:         "testid",
						Annotations: driverNameAnnotation,
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
						UID:         "testid",
						Annotations: driverNameAnnotation,
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
						UID:         "testid",
						Annotations: driverNameAnnotation,
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
					Parameters:    getDefaultStorageClassSecretParameters(),
				},
				PVName: "test-name",
				PVC:    createFakePVC(requestedBytes),
			},
			clientSetObjects: getDefaultSecretObjects(),
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
		"provision with default secrets": {
			volOpts: controller.ProvisionOptions{
				StorageClass: &storagev1.StorageClass{
					ReclaimPolicy: &deletePolicy,
					Parameters: map[string]string{
						prefixedDefaultSecretNameKey:      "default-secret",
						prefixedDefaultSecretNamespaceKey: "default-ns",
					},
				},
				PVName: "test-name",
				PVC:    createFakePVC(requestedBytes),
			},
			clientSetObjects: []runtime.Object{&v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "default-secret",
					Namespace: "default-ns",
				},
			}},
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
						Name:      "default-secret",
						Namespace: "default-ns",
					},
					NodeStageSecretRef: &v1.SecretReference{
						Name:      "default-secret",
						Namespace: "default-ns",
					},
					NodePublishSecretRef: &v1.SecretReference{
						Name:      "default-secret",
						Namespace: "default-ns",
					},
					ControllerExpandSecretRef: &v1.SecretReference{
						Name:      "default-secret",
						Namespace: "default-ns",
					},
				},
			},
			expectState: controller.ProvisioningFinished,
		},
		"provision with default secrets with template": {
			volOpts: controller.ProvisionOptions{
				StorageClass: &storagev1.StorageClass{
					ReclaimPolicy: &deletePolicy,
					Parameters: map[string]string{
						prefixedDefaultSecretNameKey:      "${pvc.name}",
						prefixedDefaultSecretNamespaceKey: "default-ns",
					},
				},
				PVName: "test-name",
				PVC:    createFakeNamedPVC(requestedBytes, "my-pvc", nil),
			},
			clientSetObjects: []runtime.Object{&v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-pvc",
					Namespace: "default-ns",
				},
			}},
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
						Name:      "my-pvc",
						Namespace: "default-ns",
					},
					NodeStageSecretRef: &v1.SecretReference{
						Name:      "my-pvc",
						Namespace: "default-ns",
					},
					NodePublishSecretRef: &v1.SecretReference{
						Name:      "my-pvc",
						Namespace: "default-ns",
					},
					ControllerExpandSecretRef: &v1.SecretReference{
						Name:      "my-pvc",
						Namespace: "default-ns",
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
		"fail to get secret reference for invalid default secret parameter template": {
			volOpts: controller.ProvisionOptions{
				StorageClass: &storagev1.StorageClass{
					ReclaimPolicy: &deletePolicy,
					Parameters: map[string]string{
						prefixedDefaultSecretNameKey:      "default-${pvc.annotations['team.example.com/key']}",
						prefixedDefaultSecretNamespaceKey: "default-ns",
					},
				},
				PVName: "test-name",
				PVC: createFakeNamedPVC(
					requestedBytes,
					"fake-pvc",
					map[string]string{"team.example.com/key": "secret-from-annotation"},
				),
			},
			clientSetObjects: []runtime.Object{&v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "default-secret-from-annotation",
					Namespace: "default-ns",
				},
			}},
			getSecretRefErr: true,
			expectErr:       true,
			expectState:     controller.ProvisioningNoChange,
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
						UID:         "testid",
						Annotations: driverNameAnnotation,
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
						UID:         "testid",
						Annotations: driverNameAnnotation,
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
						UID:         "testid",
						Annotations: driverNameAnnotation,
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
		"provision with size 0": {
			volOpts: controller.ProvisionOptions{
				StorageClass: &storagev1.StorageClass{
					Parameters: map[string]string{
						"fstype": "ext3",
					},
					ReclaimPolicy: &deletePolicy,
				},
				PVName: "test-name",
				PVC:    createFakePVC(requestedBytes),
			},
			volWithZeroCap: true,
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
	}

	for k, tc := range testcases {
		runProvisionTest(t, k, tc, requestedBytes, driverName, "" /* no migration */)
	}
}

// newSnapshot returns a new snapshot object
func newSnapshot(name, className, boundToContent, snapshotUID, claimName string, ready bool, err *crdv1.VolumeSnapshotError, creationTime *metav1.Time, size *resource.Quantity) *crdv1.VolumeSnapshot {
	snapshot := crdv1.VolumeSnapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			Namespace:       "default",
			UID:             types.UID(snapshotUID),
			ResourceVersion: "1",
			SelfLink:        "/apis/snapshot.storage.k8s.io/v1beta1/namespaces/" + "default" + "/volumesnapshots/" + name,
		},
		Spec: crdv1.VolumeSnapshotSpec{
			Source: crdv1.VolumeSnapshotSource{
				PersistentVolumeClaimName: &claimName,
			},
			VolumeSnapshotClassName: &className,
		},
		Status: &crdv1.VolumeSnapshotStatus{
			BoundVolumeSnapshotContentName: &boundToContent,
			CreationTime:                   creationTime,
			ReadyToUse:                     &ready,
			Error:                          err,
			RestoreSize:                    size,
		},
	}

	return &snapshot
}

func runProvisionTest(t *testing.T, k string, tc provisioningTestcase, requestedBytes int64, provisionDriverName, supportsMigrationFromInTreePluginName string) {
	t.Logf("Running test: %v", k)

	tmpdir := tempDir(t)
	defer os.RemoveAll(tmpdir)
	mockController, driver, _, controllerServer, csiConn, err := createMockServer(t, tmpdir)
	if err != nil {
		t.Fatal(err)
	}
	defer mockController.Finish()
	defer driver.Stop()

	clientSet := fakeclientset.NewSimpleClientset(tc.clientSetObjects...)

	pluginCaps, controllerCaps := provisionCapabilities()
	csiProvisioner := NewCSIProvisioner(clientSet, 5*time.Second, "test-provisioner", "test", 5, csiConn.conn,
		nil, provisionDriverName, pluginCaps, controllerCaps, supportsMigrationFromInTreePluginName, false, csitrans.New(), nil, nil, nil, nil, tc.withExtraMetadata)

	out := &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			CapacityBytes: requestedBytes,
			VolumeId:      "test-volume-id",
		},
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
	} else if tc.volWithZeroCap {
		out.Volume.CapacityBytes = int64(0)
		controllerServer.EXPECT().CreateVolume(gomock.Any(), gomock.Any()).Return(out, nil).Times(1)
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
		t.Fatalf("test %q: got error: %v", k, err)
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
	ready := true
	content := crdv1.VolumeSnapshotContent{
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			ResourceVersion: "1",
		},
		Spec: crdv1.VolumeSnapshotContentSpec{
			Driver: "test-driver",
			Source: crdv1.VolumeSnapshotContentSource{
				SnapshotHandle: &snapshotHandle,
			},
			VolumeSnapshotClassName: &className,
		},
	}

	if boundToSnapshotName != "" {
		content.Spec.VolumeSnapshotRef = v1.ObjectReference{
			Kind:       "VolumeSnapshot",
			APIVersion: "snapshot.storage.k8s.io/v1beta1",
			UID:        types.UID(boundToSnapshotUID),
			Namespace:  "default",
			Name:       boundToSnapshotName,
		}
		content.Status = &crdv1.VolumeSnapshotContentStatus{
			RestoreSize:    size,
			SnapshotHandle: &snapshotHandle,
			CreationTime:   creationTime,
			ReadyToUse:     &ready,
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
		volOpts                           controller.ProvisionOptions
		restoredVolSizeSmall              bool
		wrongDataSource                   bool
		snapshotStatusReady               bool
		expectedPVSpec                    *pvSpec
		expectErr                         bool
		expectCSICall                     bool
		notPopulated                      bool
		misBoundSnapshotContentUID        bool
		misBoundSnapshotContentNamespace  bool
		misBoundSnapshotContentName       bool
		nilBoundVolumeSnapshotContentName bool
		nilSnapshotStatus                 bool
		nilReadyToUse                     bool
		nilContentStatus                  bool
		nilSnapshotHandle                 bool
	}{
		"provision with volume snapshot data source": {
			volOpts: controller.ProvisionOptions{
				StorageClass: &storagev1.StorageClass{
					ReclaimPolicy: &deletePolicy,
					Parameters:    map[string]string{},
					Provisioner:   "test-driver",
				},
				PVName: "test-name",
				PVC: &v1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						UID:         "testid",
						Annotations: driverNameAnnotation,
					},
					Spec: v1.PersistentVolumeClaimSpec{
						StorageClassName: &snapClassName,
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
			expectCSICall: true,
		},
		"fail vol size less than snapshot size": {
			volOpts: controller.ProvisionOptions{
				StorageClass: &storagev1.StorageClass{
					Parameters:  map[string]string{},
					Provisioner: "test-driver",
				},
				PVName: "test-name",
				PVC: &v1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						UID:         "testid",
						Annotations: driverNameAnnotation,
					},
					Spec: v1.PersistentVolumeClaimSpec{
						StorageClassName: &snapClassName,
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
					Parameters:  map[string]string{},
					Provisioner: "test-driver",
				},
				PVName: "test-name",
				PVC: &v1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						UID:         "testid",
						Annotations: driverNameAnnotation,
					},
					Spec: v1.PersistentVolumeClaimSpec{
						StorageClassName: &snapClassName,
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
					Parameters:  map[string]string{},
					Provisioner: "test-driver",
				},
				PVName: "test-name",
				PVC: &v1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						UID:         "testid",
						Annotations: driverNameAnnotation,
					},
					Spec: v1.PersistentVolumeClaimSpec{
						StorageClassName: &snapClassName,
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
					Parameters:  map[string]string{},
					Provisioner: "test-driver",
				},
				PVName: "test-name",
				PVC: &v1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						UID:         "testid",
						Annotations: driverNameAnnotation,
					},
					Spec: v1.PersistentVolumeClaimSpec{
						StorageClassName: &snapClassName,
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
					Parameters:  map[string]string{},
					Provisioner: "test-driver",
				},
				PVName: "test-name",
				PVC: &v1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						UID:         "testid",
						Annotations: driverNameAnnotation,
					},
					Spec: v1.PersistentVolumeClaimSpec{
						StorageClassName: &snapClassName,
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
		"fail not populated volume content source": {
			volOpts: controller.ProvisionOptions{
				StorageClass: &storagev1.StorageClass{
					Parameters:  map[string]string{},
					Provisioner: "test-driver",
				},
				PVName: "test-name",
				PVC: &v1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						UID:         "testid",
						Annotations: driverNameAnnotation,
					},
					Spec: v1.PersistentVolumeClaimSpec{
						StorageClassName: &snapClassName,
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
			expectErr:           true,
			expectCSICall:       true,
			notPopulated:        true,
		},
		"fail snapshotContent bound to a different snapshot (by UID)": {
			volOpts: controller.ProvisionOptions{
				StorageClass: &storagev1.StorageClass{
					Parameters:  map[string]string{},
					Provisioner: "test-driver",
				},
				PVName: "test-name",
				PVC: &v1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						UID:         "testid",
						Annotations: driverNameAnnotation,
					},
					Spec: v1.PersistentVolumeClaimSpec{
						StorageClassName: &snapClassName,
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
			snapshotStatusReady:        true,
			expectErr:                  true,
			misBoundSnapshotContentUID: true,
		},
		"fail snapshotContent bound to a different snapshot (by namespace)": {
			volOpts: controller.ProvisionOptions{
				StorageClass: &storagev1.StorageClass{
					Parameters:  map[string]string{},
					Provisioner: "test-driver",
				},
				PVName: "test-name",
				PVC: &v1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						UID:         "testid",
						Annotations: driverNameAnnotation,
					},
					Spec: v1.PersistentVolumeClaimSpec{
						StorageClassName: &snapClassName,
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
			snapshotStatusReady:              true,
			expectErr:                        true,
			misBoundSnapshotContentNamespace: true,
		},
		"fail snapshotContent bound to a different snapshot (by name)": {
			volOpts: controller.ProvisionOptions{
				StorageClass: &storagev1.StorageClass{
					Parameters:  map[string]string{},
					Provisioner: "test-driver",
				},
				PVName: "test-name",
				PVC: &v1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						UID:         "testid",
						Annotations: driverNameAnnotation,
					},
					Spec: v1.PersistentVolumeClaimSpec{
						StorageClassName: &snapClassName,
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
			snapshotStatusReady:         true,
			expectErr:                   true,
			misBoundSnapshotContentName: true,
		},
		"fail snapshotContent uses different driver than StorageClass": {
			volOpts: controller.ProvisionOptions{
				StorageClass: &storagev1.StorageClass{
					Parameters:  map[string]string{},
					Provisioner: "another-driver",
				},
				PVName: "test-name",
				PVC: &v1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						UID:         "testid",
						Annotations: driverNameAnnotation,
					},
					Spec: v1.PersistentVolumeClaimSpec{
						StorageClassName: &snapClassName,
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
			expectErr:           true,
		},
		"fail provision with no volume snapshot content status": {
			volOpts: controller.ProvisionOptions{
				StorageClass: &storagev1.StorageClass{
					ReclaimPolicy: &deletePolicy,
					Parameters:    map[string]string{},
					Provisioner:   "test-driver",
				},
				PVName: "test-name",
				PVC: &v1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						UID:         "testid",
						Annotations: driverNameAnnotation,
					},
					Spec: v1.PersistentVolumeClaimSpec{
						StorageClassName: &snapClassName,
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
			nilContentStatus:    true,
			expectErr:           true,
		},
		"fail provision with no volume snapshot handle in content status": {
			volOpts: controller.ProvisionOptions{
				StorageClass: &storagev1.StorageClass{
					ReclaimPolicy: &deletePolicy,
					Parameters:    map[string]string{},
					Provisioner:   "test-driver",
				},
				PVName: "test-name",
				PVC: &v1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						UID:         "testid",
						Annotations: driverNameAnnotation,
					},
					Spec: v1.PersistentVolumeClaimSpec{
						StorageClassName: &snapClassName,
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
			nilSnapshotHandle:   true,
			expectErr:           true,
		},
		"fail provision with no volume snapshot status": {
			volOpts: controller.ProvisionOptions{
				StorageClass: &storagev1.StorageClass{
					ReclaimPolicy: &deletePolicy,
					Parameters:    map[string]string{},
					Provisioner:   "test-driver",
				},
				PVName: "test-name",
				PVC: &v1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						UID:         "testid",
						Annotations: driverNameAnnotation,
					},
					Spec: v1.PersistentVolumeClaimSpec{
						StorageClassName: &snapClassName,
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
			nilSnapshotStatus: true,
			expectErr:         true,
		},
		"fail provision with no BoundVolumeSnapshotContentName in snapshot status": {
			volOpts: controller.ProvisionOptions{
				StorageClass: &storagev1.StorageClass{
					ReclaimPolicy: &deletePolicy,
					Parameters:    map[string]string{},
					Provisioner:   "test-driver",
				},
				PVName: "test-name",
				PVC: &v1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						UID:         "testid",
						Annotations: driverNameAnnotation,
					},
					Spec: v1.PersistentVolumeClaimSpec{
						StorageClassName: &snapClassName,
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
			nilBoundVolumeSnapshotContentName: true,
			expectErr:                         true,
		},
		"fail provision with nil ReadyToUse in snapshot status": {
			volOpts: controller.ProvisionOptions{
				StorageClass: &storagev1.StorageClass{
					ReclaimPolicy: &deletePolicy,
					Parameters:    map[string]string{},
					Provisioner:   "test-driver",
				},
				PVName: "test-name",
				PVC: &v1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						UID:         "testid",
						Annotations: driverNameAnnotation,
					},
					Spec: v1.PersistentVolumeClaimSpec{
						StorageClassName: &snapClassName,
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
			nilReadyToUse: true,
			expectErr:     true,
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
			if tc.nilSnapshotStatus {
				snap.Status = nil
			}
			if tc.nilBoundVolumeSnapshotContentName {
				snap.Status.BoundVolumeSnapshotContentName = nil
			}
			if tc.nilReadyToUse {
				snap.Status.ReadyToUse = nil
			}
			return true, snap, nil
		})

		client.AddReactor("get", "volumesnapshotcontents", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
			content := newContent("snapcontent-snapuid", snapClassName, "sid", "pv-uid", "volume", "snapuid", snapName, &requestedBytes, &timeNow)
			if tc.misBoundSnapshotContentUID {
				content.Spec.VolumeSnapshotRef.UID = "another-snapshot-uid"
			}
			if tc.misBoundSnapshotContentName {
				content.Spec.VolumeSnapshotRef.Name = "another-snapshot-name"
			}
			if tc.misBoundSnapshotContentNamespace {
				content.Spec.VolumeSnapshotRef.Namespace = "another-snapshot-namespace"
			}
			if tc.nilContentStatus {
				content.Status = nil
			}
			if tc.nilSnapshotHandle {
				content.Status.SnapshotHandle = nil
			}
			return true, content, nil
		})

		pluginCaps, controllerCaps := provisionFromSnapshotCapabilities()
		csiProvisioner := NewCSIProvisioner(clientSet, 5*time.Second, "test-provisioner", "test", 5, csiConn.conn,
			client, driverName, pluginCaps, controllerCaps, "", false, csitrans.New(), nil, nil, nil, nil, false)

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
		if tc.expectCSICall {
			if tc.notPopulated {
				out.Volume.ContentSource = nil
				controllerServer.EXPECT().CreateVolume(gomock.Any(), gomock.Any()).Return(out, nil).Times(1)
				controllerServer.EXPECT().DeleteVolume(gomock.Any(), &csi.DeleteVolumeRequest{
					VolumeId: "test-volume-id",
				}).Return(&csi.DeleteVolumeResponse{}, nil).Times(1)
			} else {
				snapshotSource := csi.VolumeContentSource_Snapshot{
					Snapshot: &csi.VolumeContentSource_SnapshotSource{
						SnapshotId: "sid",
					},
				}
				out.Volume.ContentSource = &csi.VolumeContentSource{
					Type: &snapshotSource,
				}
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
			if pv != nil {
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
			csiNodes := buildCSINodes(tc.topologyKeys)

			var (
				pluginCaps     rpc.PluginCapabilitySet
				controllerCaps rpc.ControllerCapabilitySet
			)

			if tc.driverSupportsTopology {
				pluginCaps, controllerCaps = provisionWithTopologyCapabilities()
			} else {
				pluginCaps, controllerCaps = provisionCapabilities()
			}

			clientSet := fakeclientset.NewSimpleClientset(nodes, csiNodes)

			scLister, csiNodeLister, nodeLister, claimLister, stopChan := listers(clientSet)
			defer close(stopChan)

			csiProvisioner := NewCSIProvisioner(clientSet, 5*time.Second, "test-provisioner", "test", 5,
				csiConn.conn, nil, driverName, pluginCaps, controllerCaps, "", false, csitrans.New(), scLister, csiNodeLister, nodeLister, claimLister, false)

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
	csiProvisioner := NewCSIProvisioner(clientSet, 5*time.Second, "test-provisioner", "test", 5,
		csiConn.conn, nil, driverName, pluginCaps, controllerCaps, "", false, csitrans.New(), nil, nil, nil, nil, false)

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
		"fail - nil PV": {
			persistentVolume: nil,
			expectErr:        true,
		},
		"fail - nil volume.Spec.CSI": {
			persistentVolume: &v1.PersistentVolume{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pv",
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
		"fail - pvc.annotations not supported for Create/Delete Volume Secret": {
			persistentVolume: &v1.PersistentVolume{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pv",
				},
				Spec: v1.PersistentVolumeSpec{
					PersistentVolumeSource: v1.PersistentVolumeSource{
						CSI: &v1.CSIPersistentVolumeSource{
							VolumeHandle: "vol-id-1",
						},
					},
					ClaimRef: &v1.ObjectReference{
						Name: "sc-name",
					},
					StorageClassName: "sc-name",
				},
			},
			storageClass: &storagev1.StorageClass{
				ObjectMeta: metav1.ObjectMeta{
					Name: "sc-name",
				},
				Parameters: map[string]string{
					prefixedProvisionerSecretNameKey: "static-${pv.name}-${pvc.namespace}-${pvc.name}-${pvc.annotations['akey']}",
				},
			},
			expectErr: true,
		},
		"simple - valid case": {
			persistentVolume: &v1.PersistentVolume{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pv",
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
					Name: "sc-name",
				},
				Parameters: map[string]string{
					prefixedProvisionerSecretNameKey: "static-${pv.name}-${pvc.namespace}-${pvc.name}",
				},
			},
			expectErr:  false,
			mockDelete: true,
		},
		"simple - valid case with ClaimRef set": {
			persistentVolume: &v1.PersistentVolume{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pv",
				},
				Spec: v1.PersistentVolumeSpec{
					PersistentVolumeSource: v1.PersistentVolumeSource{
						CSI: &v1.CSIPersistentVolumeSource{
							VolumeHandle: "vol-id-1",
						},
					},
					ClaimRef: &v1.ObjectReference{
						Name: "pvc-name",
					},
				},
			},
			storageClass: &storagev1.StorageClass{
				ObjectMeta: metav1.ObjectMeta{
					Name: "sc-name",
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
	scLister, _, _, _, _ := listers(clientSet)
	csiProvisioner := NewCSIProvisioner(clientSet, 5*time.Second, "test-provisioner", "test", 5,
		csiConn.conn, nil, driverName, pluginCaps, controllerCaps, "", false, csitrans.New(), scLister, nil, nil, nil, false)

	err = csiProvisioner.Delete(tc.persistentVolume)
	if tc.expectErr && err == nil {
		t.Errorf("test %q: Expected error, got none", k)
	}
	if !tc.expectErr && err != nil {
		t.Errorf("test %q: got error: %v", k, err)
	}
}

// generatePVCForProvisionFromPVC returns a ProvisionOptions with the requested settings
func generatePVCForProvisionFromPVC(srcNamespace, srcName, scName string, requestedBytes int64, volumeMode string) controller.ProvisionOptions {
	deletePolicy := v1.PersistentVolumeReclaimDelete

	provisionRequest := controller.ProvisionOptions{
		StorageClass: &storagev1.StorageClass{
			ReclaimPolicy: &deletePolicy,
			Parameters:    map[string]string{},
			Provisioner:   driverName,
		},
		PVName: "new-pv-name",
		PVC: &v1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "my-pvc",
				Namespace:   srcNamespace,
				UID:         "testid",
				Annotations: driverNameAnnotation,
			},
			Spec: v1.PersistentVolumeClaimSpec{
				Selector: nil,
				Resources: v1.ResourceRequirements{
					Requests: v1.ResourceList{
						v1.ResourceName(v1.ResourceStorage): *resource.NewQuantity(requestedBytes, resource.BinarySI),
					},
				},
				AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
				DataSource: &v1.TypedLocalObjectReference{
					Name: srcName,
					Kind: "PersistentVolumeClaim",
				},
			},
		},
	}

	if scName != "" {
		provisionRequest.PVC.Spec.StorageClassName = &scName
	}

	switch volumeMode {
	case "block":
		provisionRequest.PVC.Spec.VolumeMode = &volumeModeBlock
	case "filesystem":
		provisionRequest.PVC.Spec.VolumeMode = &volumeModeFileSystem
	default:
		// leave it undefined/nil to maintaint the current defaults for test cases
	}

	return provisionRequest
}

// TestProvisionFromPVC tests create volume clone
func TestProvisionFromPVC(t *testing.T) {
	var requestedBytes int64 = 1000
	fakeSc1 := "fake-sc-1"
	fakeSc2 := "fake-sc-2"
	srcName := "fake-pvc"
	srcNamespace := "fake-pvc-namespace"
	invalidPVC := "invalid-pv"
	lostPVC := "lost-pvc"
	pendingPVC := "pending-pvc"
	pvName := "test-testi"
	unboundPVName := "unbound-pv"
	anotherDriverPVName := "another-class"
	filesystemPVName := "filesystem-pv"
	blockModePVName := "block-pv"
	wrongPVCName := "pv-bound-to-another-pvc-by-name"
	wrongPVCNamespace := "pv-bound-to-another-pvc-by-namespace"
	wrongPVCUID := "pv-bound-to-another-pvc-by-UID"

	type pvSpec struct {
		Name          string
		ReclaimPolicy v1.PersistentVolumeReclaimPolicy
		AccessModes   []v1.PersistentVolumeAccessMode
		Capacity      v1.ResourceList
		CSIPVS        *v1.CSIPersistentVolumeSource
	}

	testcases := map[string]struct {
		volOpts              controller.ProvisionOptions
		clonePVName          string                   // name of the PV that srcName PVC has a claim on
		restoredVolSizeSmall bool                     // set to request a larger volSize than source PVC, default false
		restoredVolSizeBig   bool                     // set to request a smaller volSize than source PVC, default false
		expectedPVSpec       *pvSpec                  // set to expected PVSpec on success, for deep comparison, default nil
		cloneUnsupported     bool                     // set to state clone feature not supported in capabilities, default false
		sourcePVStatusPhase  v1.PersistentVolumePhase // set to change source PV Status.Phase, default "Bound"
		expectErr            bool                     // set to state, test is expected to return errors, default false
	}{
		"provision with pvc data source": {
			clonePVName: pvName,
			volOpts:     generatePVCForProvisionFromPVC(srcNamespace, srcName, fakeSc1, requestedBytes, ""),
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
		},
		"provision with pvc data source no clone capability": {
			clonePVName:      pvName,
			volOpts:          generatePVCForProvisionFromPVC(srcNamespace, srcName, fakeSc1, requestedBytes, ""),
			cloneUnsupported: true,
			expectErr:        true,
		},
		"provision with pvc data source different storage classes": {
			clonePVName: pvName,
			volOpts:     generatePVCForProvisionFromPVC(srcNamespace, srcName, fakeSc2, requestedBytes, ""),
			expectErr:   true,
		},
		"provision with pvc data source destination too small": {
			clonePVName:          pvName,
			volOpts:              generatePVCForProvisionFromPVC(srcNamespace, srcName, fakeSc1, requestedBytes+1, ""),
			restoredVolSizeSmall: true,
			expectErr:            true,
		},
		"provision with pvc data source destination too large": {
			clonePVName:        pvName,
			volOpts:            generatePVCForProvisionFromPVC(srcNamespace, srcName, fakeSc1, requestedBytes-1, ""),
			restoredVolSizeBig: true,
			expectErr:          true,
		},
		"provision with pvc data source not found": {
			clonePVName: pvName,
			volOpts:     generatePVCForProvisionFromPVC(srcNamespace, "source-not-found", fakeSc1, requestedBytes, ""),
			expectErr:   true,
		},
		"provision with source pvc storageclass nil": {
			clonePVName: pvName,
			volOpts:     generatePVCForProvisionFromPVC(srcNamespace, "pvc-sc-nil", fakeSc1, requestedBytes, ""),
			expectErr:   true,
		},
		"provision with requested pvc storageclass nil": {
			clonePVName: pvName,
			volOpts:     generatePVCForProvisionFromPVC(srcNamespace, srcName, "", requestedBytes, ""),
			expectErr:   true,
		},
		"provision with pvc data source when source pv not found": {
			clonePVName: "invalid-pv",
			volOpts:     generatePVCForProvisionFromPVC(srcNamespace, invalidPVC, fakeSc1, requestedBytes, ""),
			expectErr:   true,
		},
		"provision with pvc data source when pvc status is claim pending": {
			clonePVName: pvName,
			volOpts:     generatePVCForProvisionFromPVC(srcNamespace, pendingPVC, fakeSc1, requestedBytes, ""),
			expectErr:   true,
		},
		"provision with pvc data source when pvc status is claim lost": {
			clonePVName: pvName,
			volOpts:     generatePVCForProvisionFromPVC(srcNamespace, lostPVC, fakeSc1, requestedBytes, ""),
			expectErr:   true,
		},
		"provision with pvc data source when clone pv has released status": {
			clonePVName:         pvName,
			volOpts:             generatePVCForProvisionFromPVC(srcNamespace, srcName, fakeSc1, requestedBytes, ""),
			sourcePVStatusPhase: v1.VolumeReleased,
			expectErr:           true,
		},
		"provision with pvc data source when clone pv has failed status": {
			clonePVName:         pvName,
			volOpts:             generatePVCForProvisionFromPVC(srcNamespace, srcName, fakeSc1, requestedBytes, ""),
			sourcePVStatusPhase: v1.VolumeFailed,
			expectErr:           true,
		},
		"provision with pvc data source when clone pv has pending status": {
			clonePVName:         pvName,
			volOpts:             generatePVCForProvisionFromPVC(srcNamespace, srcName, fakeSc1, requestedBytes, ""),
			sourcePVStatusPhase: v1.VolumePending,
			expectErr:           true,
		},
		"provision with pvc data source when clone pv has available status": {
			clonePVName:         pvName,
			volOpts:             generatePVCForProvisionFromPVC(srcNamespace, srcName, fakeSc1, requestedBytes, ""),
			sourcePVStatusPhase: v1.VolumeAvailable,
			expectErr:           true,
		},
		"provision with PVC using unbound PV": {
			clonePVName: unboundPVName,
			volOpts:     generatePVCForProvisionFromPVC(srcNamespace, srcName, fakeSc1, requestedBytes, ""),
			expectErr:   true,
		},
		"provision with PVC using PV bound to another PVC (with wrong UID)": {
			clonePVName: wrongPVCUID,
			volOpts:     generatePVCForProvisionFromPVC(srcNamespace, srcName, fakeSc1, requestedBytes, ""),
			expectErr:   true,
		},
		"provision with PVC using PV bound to another PVC (with wrong namespace)": {
			clonePVName: wrongPVCNamespace,
			volOpts:     generatePVCForProvisionFromPVC(srcNamespace, srcName, fakeSc1, requestedBytes, ""),
			expectErr:   true,
		},
		"provision with PVC using PV bound to another PVC (with wrong name)": {
			clonePVName: wrongPVCName,
			volOpts:     generatePVCForProvisionFromPVC(srcNamespace, srcName, fakeSc1, requestedBytes, ""),
			expectErr:   true,
		},
		"provision with PVC bound to PV with wrong provisioner": {
			clonePVName: anotherDriverPVName,
			volOpts:     generatePVCForProvisionFromPVC(srcNamespace, srcName, fakeSc1, requestedBytes, ""),
			expectErr:   true,
		},
		"provision block data source is block": {
			clonePVName: blockModePVName,
			volOpts:     generatePVCForProvisionFromPVC(srcNamespace, blockModePVName, fakeSc1, requestedBytes, "block"),
			expectErr:   false,
		},
		"provision block but data source is filesystem": {
			clonePVName: filesystemPVName,
			volOpts:     generatePVCForProvisionFromPVC(srcNamespace, filesystemPVName, fakeSc1, requestedBytes, "block"),
			expectErr:   true,
		},
		"provision block but data source is nil": {
			clonePVName: pvName,
			volOpts:     generatePVCForProvisionFromPVC(srcNamespace, srcName, fakeSc1, requestedBytes, "block"),
			expectErr:   true,
		},
		"provision nil mode data source is nil": {
			clonePVName: pvName,
			volOpts:     generatePVCForProvisionFromPVC(srcNamespace, srcName, fakeSc1, requestedBytes, ""),
			expectErr:   false,
		},
		"provision filesystem data source is filesystem": {
			clonePVName: filesystemPVName,
			volOpts:     generatePVCForProvisionFromPVC(srcNamespace, filesystemPVName, fakeSc1, requestedBytes, "filesystem"),
			expectErr:   false,
		},
		"provision filesystem but data source is block": {
			clonePVName: blockModePVName,
			volOpts:     generatePVCForProvisionFromPVC(srcNamespace, blockModePVName, fakeSc1, requestedBytes, "filesystem"),
			expectErr:   true,
		},
		"provision filesystem data source is nil": {
			clonePVName: pvName,
			volOpts:     generatePVCForProvisionFromPVC(srcNamespace, srcName, fakeSc1, requestedBytes, "filesystem"),
			expectErr:   false,
		},
	}

	for k, tc := range testcases {
		tc := tc
		t.Run(k, func(t *testing.T) {
			t.Parallel()
			var clientSet *fakeclientset.Clientset

			// Phase: setup mock server
			tmpdir := tempDir(t)
			defer os.RemoveAll(tmpdir)
			mockController, driver, _, controllerServer, csiConn, err := createMockServer(t, tmpdir)
			if err != nil {
				t.Fatal(err)
			}
			defer mockController.Finish()
			defer driver.Stop()

			// Phase: setup fake objects for test
			pvPhase := v1.VolumeBound
			if tc.sourcePVStatusPhase != "" {
				pvPhase = tc.sourcePVStatusPhase
			}
			pv := &v1.PersistentVolume{
				ObjectMeta: metav1.ObjectMeta{
					Name: pvName,
				},
				Spec: v1.PersistentVolumeSpec{
					PersistentVolumeSource: v1.PersistentVolumeSource{
						CSI: &v1.CSIPersistentVolumeSource{
							Driver:       driverName,
							VolumeHandle: "test-volume-id",
							FSType:       "ext3",
							VolumeAttributes: map[string]string{
								"storage.kubernetes.io/csiProvisionerIdentity": "test-provisioner",
							},
						},
					},
					ClaimRef: &v1.ObjectReference{
						Kind:      "PersistentVolumeClaim",
						Namespace: srcNamespace,
						Name:      srcName,
						UID:       types.UID("fake-claim-uid"),
					},
					StorageClassName: fakeSc1,
				},
				Status: v1.PersistentVolumeStatus{
					Phase: pvPhase,
				},
			}

			blkModePV := &v1.PersistentVolume{
				ObjectMeta: metav1.ObjectMeta{
					Name: blockModePVName,
				},
				Spec: v1.PersistentVolumeSpec{
					VolumeMode: &volumeModeBlock,
					PersistentVolumeSource: v1.PersistentVolumeSource{
						CSI: &v1.CSIPersistentVolumeSource{
							Driver: driverName,
							VolumeAttributes: map[string]string{
								"storage.kubernetes.io/csiProvisionerIdentity": "test-provisioner",
							},
						},
					},
					ClaimRef: &v1.ObjectReference{
						Kind:      "PersistentVolumeClaim",
						Namespace: srcNamespace,
						Name:      blockModePVName,
						UID:       types.UID("fake-block-claim-uid"),
					},
					StorageClassName: fakeSc1,
				},
				Status: v1.PersistentVolumeStatus{
					Phase: pvPhase,
				},
			}

			unboundPV := pv.DeepCopy()
			unboundPV.Name = unboundPVName
			unboundPV.Spec.ClaimRef = nil

			anotherDriverPV := pv.DeepCopy()
			anotherDriverPV.Name = anotherDriverPVName
			anotherDriverPV.Spec.CSI.Driver = "wrong.com"

			pvBoundToAnotherPVCUID := pv.DeepCopy()
			pvBoundToAnotherPVCUID.Name = wrongPVCUID
			pvBoundToAnotherPVCUID.Spec.ClaimRef.UID = "another-claim-uid"

			pvBoundToAnotherPVCNamespace := pv.DeepCopy()
			pvBoundToAnotherPVCNamespace.Name = wrongPVCNamespace
			pvBoundToAnotherPVCNamespace.Spec.ClaimRef.Namespace = "another-claim-namespace"

			pvBoundToAnotherPVCName := pv.DeepCopy()
			pvBoundToAnotherPVCName.Name = wrongPVCName
			pvBoundToAnotherPVCName.Spec.ClaimRef.Name = "another-claim-name"

			pvUsingFilesystemMode := pv.DeepCopy()
			pvUsingFilesystemMode.Name = filesystemPVName
			pvUsingFilesystemMode.Spec.VolumeMode = &volumeModeFileSystem
			pvUsingFilesystemMode.Spec.ClaimRef.Name = filesystemPVName

			// Create a fake claim as our PVC DataSource
			claim := fakeClaim(srcName, srcNamespace, "fake-claim-uid", requestedBytes, tc.clonePVName, v1.ClaimBound, &fakeSc1, "")
			// Create a fake claim with invalid PV
			invalidClaim := fakeClaim(invalidPVC, srcNamespace, "fake-claim-uid", requestedBytes, "pv-not-present", v1.ClaimBound, &fakeSc1, "")
			// Create a fake claim as source PVC storageclass nil
			scNilClaim := fakeClaim("pvc-sc-nil", srcNamespace, "fake-claim-uid", requestedBytes, pvName, v1.ClaimBound, nil, "")
			// Create a fake claim, with source PVC having a lost claim status
			lostClaim := fakeClaim(lostPVC, srcNamespace, "fake-claim-uid", requestedBytes, tc.clonePVName, v1.ClaimLost, &fakeSc1, "")
			// Create a fake claim, with source PVC having a pending claim status
			pendingClaim := fakeClaim(pendingPVC, srcNamespace, "fake-claim-uid", requestedBytes, tc.clonePVName, v1.ClaimPending, &fakeSc1, "")
			// Create a fake claim with filesystem mode on our PVC DataSource
			filesystemClaim := fakeClaim(filesystemPVName, srcNamespace, "fake-claim-uid", requestedBytes, tc.clonePVName, v1.ClaimBound, &fakeSc1, "filesystem")
			// Create a fake claim with block mode on our PVC DataSource
			blockClaim := fakeClaim(blockModePVName, srcNamespace, "fake-block-claim-uid", requestedBytes, tc.clonePVName, v1.ClaimBound, &fakeSc1, "block")

			clientSet = fakeclientset.NewSimpleClientset(claim, scNilClaim, pv, invalidClaim, filesystemClaim, blockClaim, unboundPV, anotherDriverPV, pvBoundToAnotherPVCUID, pvBoundToAnotherPVCNamespace, pvBoundToAnotherPVCName, lostClaim, pendingClaim, pvUsingFilesystemMode, blkModePV)

			// Phase: setup responses based on test case parameters
			out := &csi.CreateVolumeResponse{
				Volume: &csi.Volume{
					CapacityBytes: requestedBytes,
					VolumeId:      "test-volume-id",
				},
			}

			pluginCaps, controllerCaps := provisionFromPVCCapabilities()
			if tc.cloneUnsupported {
				pluginCaps, controllerCaps = provisionCapabilities()
			}
			if !tc.expectErr {
				volumeSource := csi.VolumeContentSource_Volume{
					Volume: &csi.VolumeContentSource_VolumeSource{
						VolumeId: tc.volOpts.PVC.Spec.DataSource.Name,
					},
				}
				out.Volume.ContentSource = &csi.VolumeContentSource{
					Type: &volumeSource,
				}
				controllerServer.EXPECT().CreateVolume(gomock.Any(), gomock.Any()).Return(out, nil).Times(1)
			}

			if tc.restoredVolSizeSmall && tc.restoredVolSizeBig {
				t.Errorf("test %q: cannot contain requestVolSize to be both small and big", k)
			}
			if tc.restoredVolSizeSmall {
				controllerServer.EXPECT().CreateVolume(gomock.Any(), gomock.Any()).Return(out, nil).Times(1)
				// if the volume created is less than the requested size,
				// deletevolume will be called
				controllerServer.EXPECT().DeleteVolume(gomock.Any(), &csi.DeleteVolumeRequest{
					VolumeId: "test-volume-id",
				}).Return(&csi.DeleteVolumeResponse{}, nil).Times(1)
			}

			_, _, _, claimLister, _ := listers(clientSet)

			// Phase: execute the test
			csiProvisioner := NewCSIProvisioner(clientSet, 5*time.Second, "test-provisioner", "test", 5, csiConn.conn,
				nil, driverName, pluginCaps, controllerCaps, "", false, csitrans.New(), nil, nil, nil, claimLister, false)

			pv, err = csiProvisioner.Provision(tc.volOpts)
			if tc.expectErr && err == nil {
				t.Errorf("test %q: Expected error, got none", k)
			}

			// Phase: process test responses
			// process responses
			if !tc.expectErr && err != nil {
				t.Errorf("test %q: got error: %v", k, err)
			}

			if tc.expectedPVSpec != nil {
				if pv != nil {
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
		})
	}
}

func TestProvisionWithMigration(t *testing.T) {
	var requestBytes int64 = 100000
	var inTreePluginName = "in-tree-plugin"

	deletePolicy := v1.PersistentVolumeReclaimDelete
	testcases := []struct {
		name              string
		scProvisioner     string
		annotation        map[string]string
		expectTranslation bool
		expectErr         bool
	}{
		{
			name:              "provision with migration on",
			scProvisioner:     inTreePluginName,
			annotation:        map[string]string{annStorageProvisioner: driverName},
			expectTranslation: true,
		},
		{
			name:              "provision without migration for native CSI",
			scProvisioner:     driverName,
			annotation:        map[string]string{annStorageProvisioner: driverName},
			expectTranslation: false,
		},
		{
			name:              "provision with migration for migrated-to CSI",
			scProvisioner:     inTreePluginName,
			annotation:        map[string]string{annStorageProvisioner: inTreePluginName, annMigratedTo: driverName},
			expectTranslation: true,
		},
		{
			name:          "provision with migration-to some random driver",
			scProvisioner: inTreePluginName,
			annotation:    map[string]string{annStorageProvisioner: inTreePluginName, annMigratedTo: "foo"},
			expectErr:     true,
		},
		{
			name:          "provision with migration-to some random driver with random storageProvisioner",
			scProvisioner: inTreePluginName,
			annotation:    map[string]string{annStorageProvisioner: "foo", annMigratedTo: "foo"},
			expectErr:     true,
		},
		{
			name:              "provision with migration for migrated-to CSI with CSI Provisioner",
			scProvisioner:     inTreePluginName,
			annotation:        map[string]string{annStorageProvisioner: driverName, annMigratedTo: driverName},
			expectTranslation: true,
		},
		{
			name:          "ignore in-tree PVC when provisioned by in-tree",
			scProvisioner: inTreePluginName,
			annotation:    map[string]string{annStorageProvisioner: inTreePluginName},
			expectErr:     true,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			// Set up test
			tmpdir := tempDir(t)
			defer os.RemoveAll(tmpdir)
			mockController, driver, _, controllerServer, csiConn, err := createMockServer(t, tmpdir)
			if err != nil {
				t.Fatal(err)
			}
			mockTranslator := NewMockProvisionerCSITranslator(mockController)
			defer mockController.Finish()
			defer driver.Stop()
			clientSet := fakeclientset.NewSimpleClientset()
			pluginCaps, controllerCaps := provisionCapabilities()
			csiProvisioner := NewCSIProvisioner(clientSet, 5*time.Second, "test-provisioner",
				"test", 5, csiConn.conn, nil, driverName, pluginCaps, controllerCaps,
				inTreePluginName, false, mockTranslator, nil, nil, nil, nil, false)

			// Set up return values (AnyTimes to avoid overfitting on implementation)

			// Have fake translation provide same as input SC but with
			// Parameter indicating it has been translated
			mockTranslator.EXPECT().TranslateInTreeStorageClassToCSI(gomock.Any(), gomock.Any()).DoAndReturn(
				func(_ string, sc *storagev1.StorageClass) (*storagev1.StorageClass, error) {
					newSC := sc.DeepCopy()
					newSC.Parameters[translatedKey] = "foo"
					return newSC, nil
				},
			).AnyTimes()

			// Have fake translation provide same as input PV but with
			// Annotation indicating it has been translated
			mockTranslator.EXPECT().TranslateCSIPVToInTree(gomock.Any()).DoAndReturn(
				func(pv *v1.PersistentVolume) (*v1.PersistentVolume, error) {
					newPV := pv.DeepCopy()
					if newPV.Annotations == nil {
						newPV.Annotations = map[string]string{}
					}
					newPV.Annotations[translatedKey] = "foo"
					return newPV, nil
				},
			).AnyTimes()

			if !tc.expectErr {
				// Set an expectation that the Create should be called
				expectParams := map[string]string{"fstype": "ext3"} // Default
				if tc.expectTranslation {
					// If translation is expected we check that the CreateVolume
					// is called on the expected volume with a translated param
					expectParams[translatedKey] = "foo"
				}
				controllerServer.EXPECT().CreateVolume(gomock.Any(),
					&csi.CreateVolumeRequest{
						Name:               "test-testi",
						Parameters:         expectParams,
						VolumeCapabilities: nil,
						CapacityRange: &csi.CapacityRange{
							RequiredBytes: int64(requestBytes),
						},
					}).Return(
					&csi.CreateVolumeResponse{
						Volume: &csi.Volume{
							CapacityBytes: requestBytes,
							VolumeId:      "test-volume-id",
						},
					}, nil).Times(1)
			}

			// Make a Provision call
			volOpts := controller.ProvisionOptions{
				StorageClass: &storagev1.StorageClass{
					Provisioner:   tc.scProvisioner,
					Parameters:    map[string]string{"fstype": "ext3"},
					ReclaimPolicy: &deletePolicy,
				},
				PVName: "test-name",
				PVC:    createPVCWithAnnotation(tc.annotation, requestBytes),
			}

			pv, state, err := csiProvisioner.(controller.ProvisionerExt).ProvisionExt(volOpts)
			if tc.expectErr && err == nil {
				t.Errorf("Expected error, got none")
			}
			if err != nil {
				if !tc.expectErr {
					t.Errorf("got error: %v", err)
				}
				return
			}

			if controller.ProvisioningFinished != state {
				t.Errorf("expected ProvisioningState %s, got %s", controller.ProvisioningFinished, state)
			}

			if tc.expectTranslation {
				if _, ok := pv.Annotations[translatedKey]; !ok {
					t.Errorf("got no translated annotation %s on the pv, expected PV to be translated to in-tree", translatedKey)
				}
			} else {
				if _, ok := pv.Annotations[translatedKey]; ok {
					t.Errorf("got translated annotation %s on the pv, expected PV not to be translated to in-tree", translatedKey)
				}
			}
		})

	}
}

func createPVCWithAnnotation(ann map[string]string, requestBytes int64) *v1.PersistentVolumeClaim {
	return &v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			UID:         "testid",
			Name:        "fake-pvc",
			Annotations: ann,
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

func TestDeleteMigration(t *testing.T) {
	const (
		translatedHandle = "translated-handle"
		normalHandle     = "no-translation-handle"
	)

	testCases := []struct {
		name              string
		pv                *v1.PersistentVolume
		expectTranslation bool
		expectErr         bool
	}{
		{
			name: "normal migration",
			// The PV could be any random in-tree plugin - it doesn't really
			// matter here. We only care that the translation is called and the
			// function will work after some CSI volume is created
			pv:                &v1.PersistentVolume{},
			expectTranslation: true,
		},
		{
			name: "no migration",
			pv: &v1.PersistentVolume{
				Spec: v1.PersistentVolumeSpec{
					PersistentVolumeSource: v1.PersistentVolumeSource{
						CSI: &v1.CSIPersistentVolumeSource{
							VolumeHandle: normalHandle,
						},
					},
				},
			},
			expectTranslation: false,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Set up test
			tmpdir := tempDir(t)
			defer os.RemoveAll(tmpdir)
			mockController, driver, _, controllerServer, csiConn, err := createMockServer(t, tmpdir)
			if err != nil {
				t.Fatal(err)
			}
			mockTranslator := NewMockProvisionerCSITranslator(mockController)
			defer mockController.Finish()
			defer driver.Stop()
			clientSet := fakeclientset.NewSimpleClientset()
			pluginCaps, controllerCaps := provisionCapabilities()
			csiProvisioner := NewCSIProvisioner(clientSet, 5*time.Second, "test-provisioner",
				"test", 5, csiConn.conn, nil, driverName, pluginCaps, controllerCaps, "",
				false, mockTranslator, nil, nil, nil, nil, false)

			// Set mock return values (AnyTimes to avoid overfitting on implementation details)
			mockTranslator.EXPECT().IsPVMigratable(gomock.Any()).Return(tc.expectTranslation).AnyTimes()
			if tc.expectTranslation {
				// In the translation case we translate to CSI we return a fake
				// PV with a different handle
				mockTranslator.EXPECT().TranslateInTreePVToCSI(gomock.Any()).Return(createFakeCSIPV(translatedHandle), nil).AnyTimes()
			}

			volID := normalHandle
			if tc.expectTranslation {
				volID = translatedHandle
			}

			// We assert that the Delete is called on the driver with either the
			// normal or the translated handle
			controllerServer.EXPECT().DeleteVolume(gomock.Any(),
				&csi.DeleteVolumeRequest{
					VolumeId: volID,
				}).Return(&csi.DeleteVolumeResponse{}, nil).Times(1)

			// Run Delete
			err = csiProvisioner.Delete(tc.pv)
			if tc.expectErr && err == nil {
				t.Error("Got no error, expected one")
			}
			if !tc.expectErr && err != nil {
				t.Errorf("Got error: %v, expected none", err)
			}
		})

	}
}

func createFakeCSIPV(volumeHandle string) *v1.PersistentVolume {
	return &v1.PersistentVolume{
		Spec: v1.PersistentVolumeSpec{
			PersistentVolumeSource: v1.PersistentVolumeSource{
				CSI: &v1.CSIPersistentVolumeSource{
					VolumeHandle: volumeHandle,
				},
			},
		},
	}
}
