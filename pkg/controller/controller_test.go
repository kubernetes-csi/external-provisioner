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
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/golang/mock/gomock"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	v1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	storagev1alpha1 "k8s.io/api/storage/v1alpha1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	fakeclientset "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
	"k8s.io/component-base/featuregate"
	utilfeaturetesting "k8s.io/component-base/featuregate/testing"
	csitrans "k8s.io/csi-translation-lib"
	klog "k8s.io/klog/v2"
	"sigs.k8s.io/sig-storage-lib-external-provisioner/v10/controller"

	"github.com/kubernetes-csi/csi-lib-utils/connection"
	"github.com/kubernetes-csi/csi-lib-utils/metrics"
	"github.com/kubernetes-csi/csi-lib-utils/rpc"
	"github.com/kubernetes-csi/csi-test/v5/driver"
	"github.com/kubernetes-csi/external-provisioner/pkg/features"
	crdv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	"github.com/kubernetes-csi/external-snapshotter/client/v6/clientset/versioned/fake"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
	fakegateway "sigs.k8s.io/gateway-api/pkg/client/clientset/versioned/fake"
	gatewayInformers "sigs.k8s.io/gateway-api/pkg/client/informers/externalversions"
	referenceGrantv1beta1 "sigs.k8s.io/gateway-api/pkg/client/listers/apis/v1beta1"
)

func init() {
	klog.InitFlags(nil)
}

const (
	timeout                = 10 * time.Second
	driverName             = "test-driver"
	driverTopologyKey      = "test-driver-node"
	inTreePluginName       = "test-in-tree-plugin"
	inTreeStorageClassName = "test-in-tree-sc"
)

var (
	volumeModeFileSystem = v1.PersistentVolumeFilesystem
	volumeModeBlock      = v1.PersistentVolumeBlock

	driverNameAnnotation = map[string]string{annBetaStorageProvisioner: driverName}
	translatedKey        = "translated"
	defaultfsType        = "ext4"
)

type csiConnection struct {
	conn *grpc.ClientConn
}

func New(address string) (csiConnection, error) {
	metricsManager := metrics.NewCSIMetricsManager("fake.csi.driver.io" /* driverName */)
	ctx := context.Background()
	conn, err := connection.Connect(ctx, address, metricsManager)
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
	err := drv.StartOnAddress("unix", filepath.Join(tmpdir, "csi.sock"))
	if err != nil {
		return nil, nil, nil, nil, csiConnection{}, err
	}
	// Create a client connection to it
	addr := drv.Address()
	csiConn, err := New(addr)
	if err != nil {
		return nil, nil, nil, nil, csiConnection{}, err
	}

	return mockController, drv, identityServer, controllerServer, csiConn, nil
}

func tempDir(t *testing.T) string {
	dir, err := os.MkdirTemp("", "external-provisioner-test-")
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
				prefixedNodeExpandSecretNameKey:             "csiBar",
				prefixedNodeExpandSecretNamespaceKey:        "csiBar",
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
			"Gibibyte that cannot be put into any nice format without loss precision",
			5.56 * 1024 * 1024 * 1024,
			"5970004541",
		},
		{
			"Gibibyte that can be parsed nicer",
			5.5 * 1024 * 1024 * 1024,
			"5632Mi",
		},
		{
			"Gibibyte exact",
			5 * 1024 * 1024 * 1024,
			"5Gi",
		},
		{
			"Mebibyte that cannot be parsed nicer",
			5.23 * 1024 * 1024,
			"5484052",
		},
		{
			"Kibibyte that can be parsed nicer",
			// (100 * 1024)
			102400,
			"100Ki",
		},
	}

	for _, test := range tests {
		q := bytesToQuantity(int64(test.bytes))
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

	var clientSetObjects []runtime.Object
	clientSet := fakeclientset.NewSimpleClientset(clientSetObjects...)

	pluginCaps, controllerCaps := provisionCapabilities()
	csiProvisioner := NewCSIProvisioner(clientSet, 5*time.Second, "test-provisioner", "test",
		5, csiConn.conn, nil, driverName, pluginCaps, controllerCaps, "", false, true, csitrans.New(), nil, nil, nil, nil, nil, nil, false, "", defaultfsType, nil, true, false)

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
	_, _, err = csiProvisioner.Provision(context.Background(), opts)
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

func provisionWithSingleNodeMultiWriterCapabilities() (rpc.PluginCapabilitySet, rpc.ControllerCapabilitySet) {
	return rpc.PluginCapabilitySet{
			csi.PluginCapability_Service_CONTROLLER_SERVICE: true,
		}, rpc.ControllerCapabilitySet{
			csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME:     true,
			csi.ControllerServiceCapability_RPC_SINGLE_NODE_MULTI_WRITER: true,
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

func provisionWithVACCapabilities() (rpc.PluginCapabilitySet, rpc.ControllerCapabilitySet) {
	return rpc.PluginCapabilitySet{
			csi.PluginCapability_Service_CONTROLLER_SERVICE: true,
		}, rpc.ControllerCapabilitySet{
			csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME: true,
			csi.ControllerServiceCapability_RPC_MODIFY_VOLUME:        true,
		}
}

var fakeSCName = "fake-test-sc"

func createFakeNamedPVC(requestBytes int64, name string, userAnnotations map[string]string) *v1.PersistentVolumeClaim {
	annotations := map[string]string{annBetaStorageProvisioner: driverName}
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
			Resources: v1.VolumeResourceRequirements{
				Requests: v1.ResourceList{
					v1.ResourceName(v1.ResourceStorage): resource.MustParse(strconv.FormatInt(requestBytes, 10)),
				},
			},
			StorageClassName: &fakeSCName,
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

// createFakePVCWithVAC returns PVC with VolumeAttributesClassName
func createFakePVCWithVAC(requestBytes int64, vacName string) *v1.PersistentVolumeClaim {
	claim := createFakePVC(requestBytes)
	claim.Spec.VolumeAttributesClassName = &vacName
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
			Resources: v1.VolumeResourceRequirements{
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
		// leave it undefined/nil to maintain the current defaults for test cases
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
		"template - valid PVC annotations for Provision and Delete": {
			secretParams: provisionerSecretParams,
			params: map[string]string{
				prefixedProvisionerSecretNamespaceKey: "static-${pvc.namespace}",
				prefixedProvisionerSecretNameKey:      "static-${pvc.name}-${pvc.annotations['akey']}",
			},
			pvName: "pvname",
			pvc: &v1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "name",
					Namespace:   "pvcnamespace",
					Annotations: map[string]string{"akey": "avalue"},
				},
			},
			expectErr: false,
			expectRef: &v1.SecretReference{Name: "static-name-avalue", Namespace: "static-pvcnamespace"},
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
		"template - valid, provisioner with pvc name and namespace": {
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
	capacity                  int64 // if zero, default capacity, otherwise available bytes
	volOpts                   controller.ProvisionOptions
	notNilSelector            bool
	makeVolumeNameErr         bool
	getSecretRefErr           bool
	getCredentialsErr         bool
	volWithLessCap            bool
	volWithZeroCap            bool
	expectedPVSpec            *pvSpec
	clientSetObjects          []runtime.Object
	createVolumeError         error
	expectErr                 bool
	expectState               controller.ProvisioningState
	expectCreateVolDo         func(t *testing.T, ctx context.Context, req *csi.CreateVolumeRequest)
	withExtraMetadata         bool
	withExtraMetadataPrefix   string
	skipCreateVolume          bool
	deploymentNode            string // fake distributed provisioning with this node as host
	immediateBinding          bool   // enable immediate binding support for distributed provisioning
	expectSelectedNode        string // a specific selected-node of the PVC in the apiserver after the test, same as before if empty
	expectNoProvision         bool   // if true, then ShouldProvision should return false
	controllerPublishReadOnly bool
	featureGates              map[featuregate.Feature]bool
	pluginCapabilities        func() (rpc.PluginCapabilitySet, rpc.ControllerCapabilitySet)
}

type provisioningFSTypeTestcase struct {
	volOpts controller.ProvisionOptions

	expectedPVSpec    *pvSpec
	clientSetObjects  []runtime.Object
	createVolumeError error
	expectErr         bool
	expectState       controller.ProvisioningState

	skipDefaultFSType bool
}

type pvSpec struct {
	Name                      string
	Annotations               map[string]string
	ReclaimPolicy             v1.PersistentVolumeReclaimPolicy
	AccessModes               []v1.PersistentVolumeAccessMode
	MountOptions              []string
	VolumeMode                *v1.PersistentVolumeMode
	Capacity                  v1.ResourceList
	CSIPVS                    *v1.CSIPersistentVolumeSource
	VolumeAttributesClassName *string
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
		prefixedProvisionerSecretNameKey:           "provisionersecret",
		prefixedProvisionerSecretNamespaceKey:      defaultSecretNsName,
		prefixedNodeExpandSecretNameKey:            "nodeexpandsecret",
		prefixedNodeExpandSecretNamespaceKey:       defaultSecretNsName,
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
		}, &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "provisionersecret",
				Namespace: defaultSecretNsName,
			},
		}, &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "nodeexpandsecret",
				Namespace: defaultSecretNsName,
			},
		},
	}
}

func TestFSTypeProvision(t *testing.T) {
	var requestedBytes int64 = 100
	deletePolicy := v1.PersistentVolumeReclaimDelete
	testcases := map[string]provisioningFSTypeTestcase{
		"fstype not set/'nil' in SC to provision": {
			volOpts: controller.ProvisionOptions{
				StorageClass: &storagev1.StorageClass{
					ReclaimPolicy: &deletePolicy,
					Parameters:    map[string]string{
						// We deliberately skip fsType in sc param
						//	"fstype": "",
					},
				},
				PVName: "test-name",
				PVC:    createFakePVC(requestedBytes),
			},
			expectedPVSpec: &pvSpec{
				Name:          "test-testi",
				ReclaimPolicy: v1.PersistentVolumeReclaimDelete,
				Capacity: v1.ResourceList{
					v1.ResourceName(v1.ResourceStorage): bytesToQuantity(requestedBytes),
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
			expectState: controller.ProvisioningFinished,
		},
		"Other fstype(ex:'xfs') set in SC": {
			volOpts: controller.ProvisionOptions{
				StorageClass: &storagev1.StorageClass{
					ReclaimPolicy: &deletePolicy,
					Parameters: map[string]string{
						"fstype": "xfs",
					},
				},
				PVName: "test-name",
				PVC:    createFakePVC(requestedBytes),
			},
			expectedPVSpec: &pvSpec{
				Name:          "test-testi",
				ReclaimPolicy: v1.PersistentVolumeReclaimDelete,
				Capacity: v1.ResourceList{
					v1.ResourceName(v1.ResourceStorage): bytesToQuantity(requestedBytes),
				},
				CSIPVS: &v1.CSIPersistentVolumeSource{
					Driver:       "test-driver",
					VolumeHandle: "test-volume-id",
					FSType:       "xfs",
					VolumeAttributes: map[string]string{
						"storage.kubernetes.io/csiProvisionerIdentity": "test-provisioner",
					},
				},
			},
			expectState: controller.ProvisioningFinished,
		},

		"fstype not set/Nil in SC and defaultFSType arg unset for provisioner": {
			volOpts: controller.ProvisionOptions{
				StorageClass: &storagev1.StorageClass{
					ReclaimPolicy: &deletePolicy,
					Parameters:    map[string]string{
						// We deliberately skip fsType in sc param
						//	"fstype": "xfs",
					},
				},
				PVName: "test-name",
				PVC:    createFakePVC(requestedBytes),
			},
			skipDefaultFSType: true,
			expectedPVSpec: &pvSpec{
				Name:          "test-testi",
				ReclaimPolicy: v1.PersistentVolumeReclaimDelete,
				Capacity: v1.ResourceList{
					v1.ResourceName(v1.ResourceStorage): bytesToQuantity(requestedBytes),
				},
				CSIPVS: &v1.CSIPersistentVolumeSource{
					Driver:       "test-driver",
					VolumeHandle: "test-volume-id",
					FSType:       "",
					VolumeAttributes: map[string]string{
						"storage.kubernetes.io/csiProvisionerIdentity": "test-provisioner",
					},
				},
			},
			expectState: controller.ProvisioningFinished,
		},

		"fstype set in SC and defaultFSType arg unset for provisioner": {
			volOpts: controller.ProvisionOptions{
				StorageClass: &storagev1.StorageClass{
					ReclaimPolicy: &deletePolicy,
					Parameters: map[string]string{
						"fstype": "xfs",
					},
				},
				PVName: "test-name",
				PVC:    createFakePVC(requestedBytes),
			},
			skipDefaultFSType: true,
			expectedPVSpec: &pvSpec{
				Name:          "test-testi",
				ReclaimPolicy: v1.PersistentVolumeReclaimDelete,
				Capacity: v1.ResourceList{
					v1.ResourceName(v1.ResourceStorage): bytesToQuantity(requestedBytes),
				},
				CSIPVS: &v1.CSIPersistentVolumeSource{
					Driver:       "test-driver",
					VolumeHandle: "test-volume-id",
					FSType:       "xfs",
					VolumeAttributes: map[string]string{
						"storage.kubernetes.io/csiProvisionerIdentity": "test-provisioner",
					},
				},
			},
			expectState: controller.ProvisioningFinished,
		},
	}

	for k, tc := range testcases {
		t.Run(k, func(t *testing.T) {
			runFSTypeProvisionTest(t, k, tc, requestedBytes, driverName, "" /* no migration */)
		})
	}
}

func provisionTestcases() (int64, map[string]provisioningTestcase) {
	var requestedBytes int64 = 100
	deletePolicy := v1.PersistentVolumeReclaimDelete
	immediateBinding := storagev1.VolumeBindingImmediate
	apiGrp := "my.example.io"
	nodeFoo := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "foo",
		},
	}
	nodeBar := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "bar",
		},
	}
	vacName := "test-vac"
	return requestedBytes, map[string]provisioningTestcase{
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
				Name: "test-testi",
				Annotations: map[string]string{
					annDeletionProvisionerSecretRefName:      "",
					annDeletionProvisionerSecretRefNamespace: "",
				},
				ReclaimPolicy: v1.PersistentVolumeReclaimDelete,
				Capacity: v1.ResourceList{
					v1.ResourceName(v1.ResourceStorage): bytesToQuantity(requestedBytes),
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
					v1.ResourceName(v1.ResourceStorage): bytesToQuantity(requestedBytes),
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
			expectCreateVolDo: func(t *testing.T, ctx context.Context, req *csi.CreateVolumeRequest) {
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
		"normal provision with extra metadata prefix": {
			volOpts: controller.ProvisionOptions{
				StorageClass: &storagev1.StorageClass{
					ReclaimPolicy: &deletePolicy,
					Parameters: map[string]string{
						"fstype": "ext3",
					},
				},
				PVName: "test-name",
				PVC:    createFakeNamedPVC(requestedBytes, "fake-pvc", map[string]string{"csi.my.company.org/some_annotation": "1234"}),
			},
			withExtraMetadataPrefix: "csi.my.company.org",
			expectedPVSpec: &pvSpec{
				Name:          "test-testi",
				ReclaimPolicy: v1.PersistentVolumeReclaimDelete,
				Capacity: v1.ResourceList{
					v1.ResourceName(v1.ResourceStorage): bytesToQuantity(requestedBytes),
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
			expectCreateVolDo: func(t *testing.T, ctx context.Context, req *csi.CreateVolumeRequest) {
				expectedParams := map[string]string{
					"csi.my.company.org/some_annotation": "1234",
					"fstype":                             "ext3",
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
					v1.ResourceName(v1.ResourceStorage): bytesToQuantity(requestedBytes),
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
			expectCreateVolDo: func(t *testing.T, ctx context.Context, req *csi.CreateVolumeRequest) {
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
						Resources: v1.VolumeResourceRequirements{
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
					v1.ResourceName(v1.ResourceStorage): bytesToQuantity(requestedBytes),
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
			expectCreateVolDo: func(t *testing.T, ctx context.Context, req *csi.CreateVolumeRequest) {
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
		"provision with access mode multi node multi readonly with sidecar arg false": {
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
						Resources: v1.VolumeResourceRequirements{
							Requests: v1.ResourceList{
								v1.ResourceName(v1.ResourceStorage): resource.MustParse(strconv.FormatInt(requestedBytes, 10)),
							},
						},
						AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadOnlyMany},
					},
				},
			},
			controllerPublishReadOnly: false,
			expectedPVSpec: &pvSpec{
				Name:          "test-testi",
				ReclaimPolicy: v1.PersistentVolumeReclaimDelete,
				AccessModes:   []v1.PersistentVolumeAccessMode{v1.ReadOnlyMany},
				Capacity: v1.ResourceList{
					v1.ResourceName(v1.ResourceStorage): bytesToQuantity(requestedBytes),
				},
				CSIPVS: &v1.CSIPersistentVolumeSource{
					Driver:       "test-driver",
					VolumeHandle: "test-volume-id",
					FSType:       "ext4",
					ReadOnly:     false,
					VolumeAttributes: map[string]string{
						"storage.kubernetes.io/csiProvisionerIdentity": "test-provisioner",
					},
				},
			},
			expectCreateVolDo: func(t *testing.T, ctx context.Context, req *csi.CreateVolumeRequest) {
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
		"provision with access mode multi node multi readonly with sidecar arg true": {
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
						Resources: v1.VolumeResourceRequirements{
							Requests: v1.ResourceList{
								v1.ResourceName(v1.ResourceStorage): resource.MustParse(strconv.FormatInt(requestedBytes, 10)),
							},
						},
						AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadOnlyMany},
					},
				},
			},
			controllerPublishReadOnly: true,
			expectedPVSpec: &pvSpec{
				Name:          "test-testi",
				ReclaimPolicy: v1.PersistentVolumeReclaimDelete,
				AccessModes:   []v1.PersistentVolumeAccessMode{v1.ReadOnlyMany},
				Capacity: v1.ResourceList{
					v1.ResourceName(v1.ResourceStorage): bytesToQuantity(requestedBytes),
				},
				CSIPVS: &v1.CSIPersistentVolumeSource{
					Driver:       "test-driver",
					VolumeHandle: "test-volume-id",
					FSType:       "ext4",
					ReadOnly:     true,
					VolumeAttributes: map[string]string{
						"storage.kubernetes.io/csiProvisionerIdentity": "test-provisioner",
					},
				},
			},
			expectCreateVolDo: func(t *testing.T, ctx context.Context, req *csi.CreateVolumeRequest) {
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
		"provision with access mode single node writer": {
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
						Resources: v1.VolumeResourceRequirements{
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
					v1.ResourceName(v1.ResourceStorage): bytesToQuantity(requestedBytes),
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
			expectCreateVolDo: func(t *testing.T, ctx context.Context, req *csi.CreateVolumeRequest) {
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
		"provision with access mode single node writer and single node multi writer capability": {
			pluginCapabilities: provisionWithSingleNodeMultiWriterCapabilities,
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
						Resources: v1.VolumeResourceRequirements{
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
					v1.ResourceName(v1.ResourceStorage): bytesToQuantity(requestedBytes),
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
			expectCreateVolDo: func(t *testing.T, ctx context.Context, req *csi.CreateVolumeRequest) {
				if len(req.GetVolumeCapabilities()) != 1 {
					t.Errorf("Incorrect length in volume capabilities")
				}
				if req.GetVolumeCapabilities()[0].GetAccessMode() == nil {
					t.Errorf("Expected access mode to be set")
				}
				if req.GetVolumeCapabilities()[0].GetAccessMode().GetMode() != csi.VolumeCapability_AccessMode_SINGLE_NODE_MULTI_WRITER {
					t.Errorf("Expected single_node_multi_writer")
				}
			},
			expectState: controller.ProvisioningFinished,
		},
		"provision with access mode single node single writer and single node multi writer capability": {
			pluginCapabilities: provisionWithSingleNodeMultiWriterCapabilities,
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
						Resources: v1.VolumeResourceRequirements{
							Requests: v1.ResourceList{
								v1.ResourceName(v1.ResourceStorage): resource.MustParse(strconv.FormatInt(requestedBytes, 10)),
							},
						},
						AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadWriteOncePod},
					},
				},
			},
			expectedPVSpec: &pvSpec{
				Name:          "test-testi",
				ReclaimPolicy: v1.PersistentVolumeReclaimDelete,
				AccessModes:   []v1.PersistentVolumeAccessMode{v1.ReadWriteOncePod},
				Capacity: v1.ResourceList{
					v1.ResourceName(v1.ResourceStorage): bytesToQuantity(requestedBytes),
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
			expectCreateVolDo: func(t *testing.T, ctx context.Context, req *csi.CreateVolumeRequest) {
				if len(req.GetVolumeCapabilities()) != 1 {
					t.Errorf("Incorrect length in volume capabilities")
				}
				if req.GetVolumeCapabilities()[0].GetAccessMode() == nil {
					t.Errorf("Expected access mode to be set")
				}
				if req.GetVolumeCapabilities()[0].GetAccessMode().GetMode() != csi.VolumeCapability_AccessMode_SINGLE_NODE_SINGLE_WRITER {
					t.Errorf("Expected single_node_multi_writer")
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
						Resources: v1.VolumeResourceRequirements{
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
					v1.ResourceName(v1.ResourceStorage): bytesToQuantity(requestedBytes),
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
			expectCreateVolDo: func(t *testing.T, ctx context.Context, req *csi.CreateVolumeRequest) {
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
				Annotations: map[string]string{
					annDeletionProvisionerSecretRefName:      "provisionersecret",
					annDeletionProvisionerSecretRefNamespace: defaultSecretNsName,
				},
				Capacity: v1.ResourceList{
					v1.ResourceName(v1.ResourceStorage): bytesToQuantity(requestedBytes),
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
					NodeExpandSecretRef: &v1.SecretReference{
						Name:      "nodeexpandsecret",
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
				Annotations: map[string]string{
					annDeletionProvisionerSecretRefName:      "default-secret",
					annDeletionProvisionerSecretRefNamespace: "default-ns",
				},
				Capacity: v1.ResourceList{
					v1.ResourceName(v1.ResourceStorage): bytesToQuantity(requestedBytes),
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
					NodeExpandSecretRef: &v1.SecretReference{
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
				Annotations: map[string]string{
					annDeletionProvisionerSecretRefName:      "my-pvc",
					annDeletionProvisionerSecretRefNamespace: "default-ns",
				},
				Capacity: v1.ResourceList{
					v1.ResourceName(v1.ResourceStorage): bytesToQuantity(requestedBytes),
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
					NodeExpandSecretRef: &v1.SecretReference{
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
					v1.ResourceName(v1.ResourceStorage): bytesToQuantity(requestedBytes),
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
					v1.ResourceName(v1.ResourceStorage): bytesToQuantity(requestedBytes),
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
						Resources: v1.VolumeResourceRequirements{
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
					v1.ResourceName(v1.ResourceStorage): bytesToQuantity(requestedBytes),
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
			expectCreateVolDo: func(t *testing.T, ctx context.Context, req *csi.CreateVolumeRequest) {
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
						Resources: v1.VolumeResourceRequirements{
							Requests: v1.ResourceList{
								v1.ResourceName(v1.ResourceStorage): resource.MustParse(strconv.FormatInt(requestedBytes, 10)),
							},
						},
						AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
					},
				},
			},
			createVolumeError: status.Error(codes.Unauthenticated, "Mock final error"),
			expectCreateVolDo: func(t *testing.T, ctx context.Context, req *csi.CreateVolumeRequest) {
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
						Resources: v1.VolumeResourceRequirements{
							Requests: v1.ResourceList{
								v1.ResourceName(v1.ResourceStorage): resource.MustParse(strconv.FormatInt(requestedBytes, 10)),
							},
						},
						AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
					},
				},
			},
			createVolumeError: status.Error(codes.DeadlineExceeded, "Mock timeout"),
			expectCreateVolDo: func(t *testing.T, ctx context.Context, req *csi.CreateVolumeRequest) {
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
					v1.ResourceName(v1.ResourceStorage): bytesToQuantity(requestedBytes),
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
		"provision with any volume data source": {
			volOpts: controller.ProvisionOptions{
				StorageClass: &storagev1.StorageClass{
					ReclaimPolicy: &deletePolicy,
					Parameters:    map[string]string{},
					Provisioner:   "test-driver",
				},
				PVC: &v1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						UID:         "testid",
						Annotations: driverNameAnnotation,
					},
					Spec: v1.PersistentVolumeClaimSpec{
						Resources: v1.VolumeResourceRequirements{
							Requests: v1.ResourceList{
								v1.ResourceName(v1.ResourceStorage): resource.MustParse(strconv.FormatInt(requestedBytes, 10)),
							},
						},
						AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
						DataSource: &v1.TypedLocalObjectReference{
							Name:     "testPopulator",
							Kind:     "MyPopulator",
							APIGroup: &apiGrp,
						},
					},
				},
			},
			expectState:      controller.ProvisioningFinished,
			expectErr:        true,
			skipCreateVolume: true,
		},
		"distributed, right node selected": {
			deploymentNode: "foo",
			volOpts: controller.ProvisionOptions{
				SelectedNode: nodeFoo,
				StorageClass: &storagev1.StorageClass{
					ObjectMeta: metav1.ObjectMeta{
						Name: fakeSCName,
					},
					ReclaimPolicy: &deletePolicy,
					Parameters: map[string]string{
						"fstype": "ext3",
					},
				},
				PVName: "test-name",
				PVC: func() *v1.PersistentVolumeClaim {
					claim := createFakePVC(requestedBytes)
					claim.Annotations[annSelectedNode] = nodeFoo.Name
					return claim
				}(),
			},
			expectedPVSpec: &pvSpec{
				Name:          "test-testi",
				ReclaimPolicy: v1.PersistentVolumeReclaimDelete,
				Capacity: v1.ResourceList{
					v1.ResourceName(v1.ResourceStorage): bytesToQuantity(requestedBytes),
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
		"distributed, no node selected": {
			deploymentNode: "foo",
			volOpts: controller.ProvisionOptions{
				StorageClass: &storagev1.StorageClass{
					ObjectMeta: metav1.ObjectMeta{
						Name: fakeSCName,
					},
					ReclaimPolicy: &deletePolicy,
					Parameters: map[string]string{
						"fstype": "ext3",
					},
				},
				PVName: "test-name",
				PVC:    createFakePVC(requestedBytes),
			},
			expectErr:         true,
			expectState:       controller.ProvisioningNoChange,
			expectNoProvision: true, // not owner
		},
		"distributed, wrong node selected": {
			deploymentNode: "foo",
			volOpts: controller.ProvisionOptions{
				SelectedNode: nodeBar,
				StorageClass: &storagev1.StorageClass{
					ObjectMeta: metav1.ObjectMeta{
						Name: fakeSCName,
					},
					ReclaimPolicy: &deletePolicy,
					Parameters: map[string]string{
						"fstype": "ext3",
					},
				},
				PVName: "test-name",
				PVC: func() *v1.PersistentVolumeClaim {
					claim := createFakePVC(requestedBytes)
					claim.Annotations[annSelectedNode] = nodeBar.Name
					return claim
				}(),
			},
			expectErr:         true,
			expectState:       controller.ProvisioningNoChange,
			expectNoProvision: true, // not owner
		},
		"distributed immediate, right node selected": {
			deploymentNode:   "foo",
			immediateBinding: true,
			volOpts: controller.ProvisionOptions{
				SelectedNode: nodeFoo,
				StorageClass: &storagev1.StorageClass{
					ObjectMeta: metav1.ObjectMeta{
						Name: fakeSCName,
					},
					ReclaimPolicy: &deletePolicy,
					Parameters: map[string]string{
						"fstype": "ext3",
					},
				},
				PVName: "test-name",
				PVC: func() *v1.PersistentVolumeClaim {
					claim := createFakePVC(requestedBytes)
					claim.Annotations[annSelectedNode] = nodeFoo.Name
					return claim
				}(),
			},
			expectedPVSpec: &pvSpec{
				Name:          "test-testi",
				ReclaimPolicy: v1.PersistentVolumeReclaimDelete,
				Capacity: v1.ResourceList{
					v1.ResourceName(v1.ResourceStorage): bytesToQuantity(requestedBytes),
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
		"distributed immediate, no node selected": {
			deploymentNode:   "foo",
			immediateBinding: true,
			volOpts: controller.ProvisionOptions{
				StorageClass: &storagev1.StorageClass{
					ObjectMeta: metav1.ObjectMeta{
						Name: fakeSCName,
					},
					ReclaimPolicy: &deletePolicy,
					Parameters: map[string]string{
						"fstype": "ext3",
					},
					VolumeBindingMode: &immediateBinding,
				},
				PVName: "test-name",
				PVC:    createFakePVC(requestedBytes),
			},
			expectErr:          true,
			expectState:        controller.ProvisioningNoChange,
			expectNoProvision:  true,         // not owner yet
			expectSelectedNode: nodeFoo.Name, // changed by ShouldProvision
		},
		"distributed immediate, no capacity ": {
			deploymentNode:   "foo",
			immediateBinding: true,
			capacity:         requestedBytes - 1,
			volOpts: controller.ProvisionOptions{
				StorageClass: &storagev1.StorageClass{
					ObjectMeta: metav1.ObjectMeta{
						Name: fakeSCName,
					},
					ReclaimPolicy: &deletePolicy,
					Parameters: map[string]string{
						"fstype": "ext3",
					},
					VolumeBindingMode: &immediateBinding,
				},
				PVName: "test-name",
				PVC:    createFakePVC(requestedBytes),
			},
			expectErr:          true,
			expectState:        controller.ProvisioningNoChange,
			expectNoProvision:  true, // not owner yet and not becoming it
			expectSelectedNode: "",   // not changed by ShouldProvision
		},
		"distributed immediate, allowed topologies okay": {
			deploymentNode:   "foo",
			immediateBinding: true,
			volOpts: controller.ProvisionOptions{
				StorageClass: &storagev1.StorageClass{
					ObjectMeta: metav1.ObjectMeta{
						Name: fakeSCName,
					},
					ReclaimPolicy: &deletePolicy,
					Parameters: map[string]string{
						"fstype": "ext3",
					},
					VolumeBindingMode: &immediateBinding,
					AllowedTopologies: []v1.TopologySelectorTerm{
						{
							MatchLabelExpressions: []v1.TopologySelectorLabelRequirement{
								{
									Key:    driverTopologyKey,
									Values: []string{"foo"},
								},
							},
						},
					},
				},
				PVName: "test-name",
				PVC:    createFakePVC(requestedBytes),
			},
			expectErr:          true,
			expectState:        controller.ProvisioningNoChange,
			expectNoProvision:  true,         // not owner yet
			expectSelectedNode: nodeFoo.Name, // changed by ShouldProvision
		},
		"distributed immediate, allowed topologies not okay": {
			// This is the same as "distributed immediate, allowed topologies okay"
			// except that the node names do now not match. The expected outcome
			// then is that the controller does not attempt to become
			// the owner (= leaves the selected node annotation unset) because
			// it would not be able to provision the volume if it was
			// the owner (generating accessibility requirements would fail).
			deploymentNode:   "foo",
			immediateBinding: true,
			volOpts: controller.ProvisionOptions{
				StorageClass: &storagev1.StorageClass{
					ObjectMeta: metav1.ObjectMeta{
						Name: fakeSCName,
					},
					ReclaimPolicy: &deletePolicy,
					Parameters: map[string]string{
						"fstype": "ext3",
					},
					VolumeBindingMode: &immediateBinding,
					AllowedTopologies: []v1.TopologySelectorTerm{
						{
							MatchLabelExpressions: []v1.TopologySelectorLabelRequirement{
								{
									Key:    driverTopologyKey,
									Values: []string{"bar"},
								},
							},
						},
					},
				},
				PVName: "test-name",
				PVC:    createFakePVC(requestedBytes),
			},
			expectErr:          true,
			expectState:        controller.ProvisioningNoChange,
			skipCreateVolume:   true,
			expectNoProvision:  true, // not owner and will not change that either
			expectSelectedNode: "",   // not changed by ShouldProvision
		},
		"normal provision with VolumeAttributesClass": {
			featureGates: map[featuregate.Feature]bool{
				features.VolumeAttributesClass: true,
			},
			pluginCapabilities: provisionWithVACCapabilities,
			clientSetObjects: []runtime.Object{&storagev1alpha1.VolumeAttributesClass{
				ObjectMeta: metav1.ObjectMeta{
					Name: vacName,
				},
				DriverName: driverName,
				Parameters: map[string]string{
					"test-param": "from-vac",
				},
			}},
			expectCreateVolDo: func(t *testing.T, ctx context.Context, req *csi.CreateVolumeRequest) {
				// TODO: define a constant for this? kind of ugly
				if !reflect.DeepEqual(req.MutableParameters, map[string]string{"test-param": "from-vac"}) {
					t.Errorf("Missing or incorrect VolumeAttributesClass (mutable) parameters")
				}
			},
			volOpts: controller.ProvisionOptions{
				StorageClass: &storagev1.StorageClass{
					ReclaimPolicy: &deletePolicy,
					Parameters: map[string]string{
						"fstype":     "ext3",
						"test-param": "from-sc",
					},
				},
				PVName: "test-name",
				PVC:    createFakePVCWithVAC(requestedBytes, vacName),
			},
			expectedPVSpec: &pvSpec{
				Name: "test-testi",
				Annotations: map[string]string{
					annDeletionProvisionerSecretRefName:      "",
					annDeletionProvisionerSecretRefNamespace: "",
				},
				ReclaimPolicy: v1.PersistentVolumeReclaimDelete,
				Capacity: v1.ResourceList{
					v1.ResourceName(v1.ResourceStorage): bytesToQuantity(requestedBytes),
				},
				CSIPVS: &v1.CSIPersistentVolumeSource{
					Driver:       "test-driver",
					VolumeHandle: "test-volume-id",
					FSType:       "ext3",
					VolumeAttributes: map[string]string{
						"storage.kubernetes.io/csiProvisionerIdentity": "test-provisioner",
					},
				},
				VolumeAttributesClassName: &vacName,
			},
			expectState: controller.ProvisioningFinished,
		},
		"normal provision with VolumeAttributesClass but feature gate is disabled": {
			featureGates: map[featuregate.Feature]bool{
				features.VolumeAttributesClass: false,
			},
			pluginCapabilities: provisionWithVACCapabilities,
			clientSetObjects: []runtime.Object{&storagev1alpha1.VolumeAttributesClass{
				ObjectMeta: metav1.ObjectMeta{
					Name: vacName,
				},
				DriverName: driverName,
				Parameters: map[string]string{
					"test-param": "from-vac",
				},
			}},
			expectCreateVolDo: func(t *testing.T, ctx context.Context, req *csi.CreateVolumeRequest) {
				if req.MutableParameters != nil {
					t.Errorf("VolumeAttributesClass (mutable) parameters present when they should not be")
				}
			},
			volOpts: controller.ProvisionOptions{
				StorageClass: &storagev1.StorageClass{
					ReclaimPolicy: &deletePolicy,
					Parameters: map[string]string{
						"fstype":     "ext3",
						"test-param": "from-sc",
					},
				},
				PVName: "test-name",
				PVC:    createFakePVCWithVAC(requestedBytes, vacName),
			},
			expectedPVSpec: &pvSpec{
				Name: "test-testi",
				Annotations: map[string]string{
					annDeletionProvisionerSecretRefName:      "",
					annDeletionProvisionerSecretRefNamespace: "",
				},
				ReclaimPolicy: v1.PersistentVolumeReclaimDelete,
				Capacity: v1.ResourceList{
					v1.ResourceName(v1.ResourceStorage): bytesToQuantity(requestedBytes),
				},
				CSIPVS: &v1.CSIPersistentVolumeSource{
					Driver:       "test-driver",
					VolumeHandle: "test-volume-id",
					FSType:       "ext3",
					VolumeAttributes: map[string]string{
						"storage.kubernetes.io/csiProvisionerIdentity": "test-provisioner",
					},
				},
				VolumeAttributesClassName: nil,
			},
			expectState: controller.ProvisioningFinished,
		},
		"fail with VolumeAttributesClass but driver does not support MODIFY_VOLUME": {
			featureGates: map[featuregate.Feature]bool{
				features.VolumeAttributesClass: true,
			},
			volOpts: controller.ProvisionOptions{
				StorageClass: &storagev1.StorageClass{
					ReclaimPolicy: &deletePolicy,
					Parameters: map[string]string{
						"fstype": "ext3",
					},
				},
				PVName: "test-name",
				PVC:    createFakePVCWithVAC(requestedBytes, vacName),
			},
			expectErr:   true,
			expectState: controller.ProvisioningFinished,
		},
		"fail with VolumeAttributesClass but VAC does not exist": {
			featureGates: map[featuregate.Feature]bool{
				features.VolumeAttributesClass: true,
			},
			pluginCapabilities: provisionWithVACCapabilities,
			clientSetObjects: []runtime.Object{&storagev1alpha1.VolumeAttributesClass{
				ObjectMeta: metav1.ObjectMeta{
					Name: vacName,
				},
				DriverName: driverName,
				Parameters: map[string]string{
					"test-param": "from-vac",
				},
			}},
			volOpts: controller.ProvisionOptions{
				StorageClass: &storagev1.StorageClass{
					ReclaimPolicy: &deletePolicy,
					Parameters: map[string]string{
						"fstype": "ext3",
					},
				},
				PVName: "test-name",
				PVC:    createFakePVCWithVAC(requestedBytes, "does-not-exist"),
			},
			expectErr:   true,
			expectState: controller.ProvisioningNoChange,
		},
		"fail with VolumeAttributesClass but driver name does not match": {
			featureGates: map[featuregate.Feature]bool{
				features.VolumeAttributesClass: true,
			},
			pluginCapabilities: provisionWithVACCapabilities,
			clientSetObjects: []runtime.Object{&storagev1alpha1.VolumeAttributesClass{
				ObjectMeta: metav1.ObjectMeta{
					Name: vacName,
				},
				DriverName: "not-" + driverName,
				Parameters: map[string]string{
					"test-param": "from-vac",
				},
			}},
			volOpts: controller.ProvisionOptions{
				StorageClass: &storagev1.StorageClass{
					ReclaimPolicy: &deletePolicy,
					Parameters: map[string]string{
						"fstype": "ext3",
					},
				},
				PVName: "test-name",
				PVC:    createFakePVCWithVAC(requestedBytes, vacName),
			},
			expectErr:   true,
			expectState: controller.ProvisioningFinished,
		},
	}
}

func TestProvision(t *testing.T) {
	requestedBytes, testcases := provisionTestcases()
	for k, tc := range testcases {
		t.Run(k, func(t *testing.T) {
			runProvisionTest(t, tc, requestedBytes, driverName, "" /* no migration */, true /* Provision() */)
		})
	}
}

func TestShouldProvision(t *testing.T) {
	requestedBytes, testcases := provisionTestcases()
	for k, tc := range testcases {
		t.Run(k, func(t *testing.T) {
			runProvisionTest(t, tc, requestedBytes, driverName, "" /* no migration */, false /* ShouldProvision() */)
		})
	}
}

// newSnapshot returns a new snapshot object
func newSnapshot(name, namespace, className, boundToContent, snapshotUID, claimName string, ready bool, err *crdv1.VolumeSnapshotError, creationTime *metav1.Time, size *resource.Quantity) *crdv1.VolumeSnapshot {
	snapshot := crdv1.VolumeSnapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			Namespace:       namespace,
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

func runFSTypeProvisionTest(t *testing.T, k string, tc provisioningFSTypeTestcase, requestedBytes int64, provisionDriverName, supportsMigrationFromInTreePluginName string) {
	t.Logf("Running test: %v", k)
	myDefaultfsType := "ext4"
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

	if tc.skipDefaultFSType {
		myDefaultfsType = ""
	}
	csiProvisioner := NewCSIProvisioner(clientSet, 5*time.Second, "test-provisioner", "test", 5, csiConn.conn,
		nil, provisionDriverName, pluginCaps, controllerCaps, supportsMigrationFromInTreePluginName, false, true, csitrans.New(), nil, nil, nil, nil, nil, nil, false, "", myDefaultfsType, nil, false, false)
	out := &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			CapacityBytes: requestedBytes,
			VolumeId:      "test-volume-id",
		},
	}

	// Setup regular mock call expectations.
	if !tc.expectErr {
		controllerServer.EXPECT().CreateVolume(gomock.Any(), gomock.Any()).Return(out, tc.createVolumeError).Times(1)
	}

	pv, state, err := csiProvisioner.Provision(context.Background(), tc.volOpts)
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

func runProvisionTest(t *testing.T, tc provisioningTestcase, requestedBytes int64, provisionDriverName, supportsMigrationFromInTreePluginName string, testProvision bool) {
	for featureName, featureValue := range tc.featureGates {
		defer utilfeaturetesting.SetFeatureGateDuringTest(t, utilfeature.DefaultFeatureGate, featureName, featureValue)()
	}

	tmpdir := tempDir(t)
	defer os.RemoveAll(tmpdir)
	mockController, driver, _, controllerServer, csiConn, err := createMockServer(t, tmpdir)
	if err != nil {
		t.Fatal(err)
	}
	defer mockController.Finish()
	defer driver.Stop()

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
	} else if !testProvision {
		controllerServer.EXPECT().CreateVolume(gomock.Any(), gomock.Any()).Return(out, tc.createVolumeError).Times(0)
	} else if tc.volWithLessCap {
		out.Volume.CapacityBytes = int64(80)
		controllerServer.EXPECT().CreateVolume(gomock.Any(), gomock.Any()).Return(out, tc.createVolumeError).Times(1)
		controllerServer.EXPECT().DeleteVolume(gomock.Any(), gomock.Any()).Return(&csi.DeleteVolumeResponse{}, tc.createVolumeError).Times(1)
	} else if tc.volWithZeroCap {
		out.Volume.CapacityBytes = int64(0)
		controllerServer.EXPECT().CreateVolume(gomock.Any(), gomock.Any()).Return(out, nil).Times(1)
	} else if tc.expectCreateVolDo != nil {
		controllerServer.EXPECT().CreateVolume(gomock.Any(), gomock.Any()).Do(func(ctx context.Context, req *csi.CreateVolumeRequest) {
			tc.expectCreateVolDo(t, ctx, req)
		}).Return(out, tc.createVolumeError).Times(1)
	} else {
		// Setup regular mock call expectations.
		if !tc.expectErr && !tc.skipCreateVolume {
			controllerServer.EXPECT().CreateVolume(gomock.Any(), gomock.Any()).Return(out, tc.createVolumeError).Times(1)
		}
	}

	getCapacityOut := &csi.GetCapacityResponse{
		AvailableCapacity: 1024 * 1024 * 1024 * 1024,
	}
	if tc.capacity != 0 {
		getCapacityOut.AvailableCapacity = tc.capacity
	}
	controllerServer.EXPECT().GetCapacity(gomock.Any(), gomock.Any()).Return(getCapacityOut, nil).AnyTimes()

	expectSelectedNode := tc.expectSelectedNode
	objects := tc.clientSetObjects
	var node *v1.Node
	var csiNode *storagev1.CSINode
	if tc.deploymentNode != "" {
		node = &v1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: tc.deploymentNode,
				Labels: map[string]string{
					driverTopologyKey: tc.deploymentNode,
				},
			},
		}
		csiNode = &storagev1.CSINode{
			ObjectMeta: metav1.ObjectMeta{
				Name: tc.deploymentNode,
			},
			Spec: storagev1.CSINodeSpec{
				Drivers: []storagev1.CSINodeDriver{
					{
						Name:         driverName,
						NodeID:       tc.deploymentNode,
						TopologyKeys: []string{driverTopologyKey},
					},
				},
			},
		}
		objects = append(objects, node, csiNode)
	}
	if tc.volOpts.PVC != nil {
		tc.volOpts.PVC = tc.volOpts.PVC.DeepCopy()
		objects = append(objects, tc.volOpts.PVC)
		if expectSelectedNode == "" && tc.volOpts.PVC.Annotations != nil {
			expectSelectedNode = tc.volOpts.PVC.Annotations[annSelectedNode]
		}
	}
	if tc.volOpts.StorageClass != nil {
		tc.volOpts.StorageClass = tc.volOpts.StorageClass.DeepCopy()
		objects = append(objects, tc.volOpts.StorageClass)
	}
	clientSet := fakeclientset.NewSimpleClientset(objects...)
	informerFactory := informers.NewSharedInformerFactory(clientSet, 0)
	claimInformer := informerFactory.Core().V1().PersistentVolumeClaims()
	scInformer := informerFactory.Storage().V1().StorageClasses()
	nodeInformer := informerFactory.Core().V1().Nodes()
	csiNodeInformer := informerFactory.Storage().V1().CSINodes()

	var nodeDeployment *NodeDeployment
	if tc.deploymentNode != "" {
		nodeDeployment = &NodeDeployment{
			NodeName:         tc.deploymentNode,
			ClaimInformer:    claimInformer,
			ImmediateBinding: tc.immediateBinding,
		}
	}

	var pluginCaps rpc.PluginCapabilitySet
	var controllerCaps rpc.ControllerCapabilitySet
	if tc.pluginCapabilities != nil {
		pluginCaps, controllerCaps = tc.pluginCapabilities()
	} else {
		pluginCaps, controllerCaps = provisionCapabilities()
	}
	mycontrollerPublishReadOnly := tc.controllerPublishReadOnly
	csiProvisioner := NewCSIProvisioner(clientSet, 5*time.Second, "test-provisioner", "test", 5, csiConn.conn,
		nil, provisionDriverName, pluginCaps, controllerCaps, supportsMigrationFromInTreePluginName, false, true, csitrans.New(), scInformer.Lister(), csiNodeInformer.Lister(), nodeInformer.Lister(), nil, nil, nil, tc.withExtraMetadata, tc.withExtraMetadataPrefix, defaultfsType, nodeDeployment, mycontrollerPublishReadOnly, false)

	// Adding objects to the informer ensures that they are consistent with
	// the fake storage without having to start the informers.
	claimInformer.Informer().GetStore().Add(tc.volOpts.PVC)
	if node != nil {
		nodeInformer.Informer().GetStore().Add(node)
	}
	if csiNode != nil {
		csiNodeInformer.Informer().GetStore().Add(csiNode)
	}
	if tc.volOpts.StorageClass != nil {
		scInformer.Informer().GetStore().Add(tc.volOpts.StorageClass)
	}

	if testProvision {
		pv, state, err := csiProvisioner.Provision(context.Background(), tc.volOpts)
		if tc.expectErr && err == nil {
			t.Error("expected error, got none")
		}
		if !tc.expectErr && err != nil {
			t.Fatalf("got error: %v", err)
		}

		if tc.expectState == "" {
			tc.expectState = controller.ProvisioningFinished
		}
		if tc.expectState != state {
			t.Errorf("expected ProvisioningState %s, got %s", tc.expectState, state)
		}

		if tc.expectedPVSpec != nil {
			if pv.Name != tc.expectedPVSpec.Name {
				t.Errorf("expected PV name: %q, got: %q", tc.expectedPVSpec.Name, pv.Name)
			}

			if tc.expectedPVSpec.Annotations != nil && !reflect.DeepEqual(pv.Annotations, tc.expectedPVSpec.Annotations) {
				t.Errorf("expected PV annotations: %v, got: %v", tc.expectedPVSpec.Annotations, pv.Annotations)
			}

			if pv.Spec.PersistentVolumeReclaimPolicy != tc.expectedPVSpec.ReclaimPolicy {
				t.Errorf("expected reclaim policy: %v, got: %v", tc.expectedPVSpec.ReclaimPolicy, pv.Spec.PersistentVolumeReclaimPolicy)
			}

			if !reflect.DeepEqual(pv.Spec.AccessModes, tc.expectedPVSpec.AccessModes) {
				t.Errorf("expected access modes: %v, got: %v", tc.expectedPVSpec.AccessModes, pv.Spec.AccessModes)
			}

			if !reflect.DeepEqual(pv.Spec.VolumeMode, tc.expectedPVSpec.VolumeMode) {
				t.Errorf("expected volumeMode: %v, got: %v", tc.expectedPVSpec.VolumeMode, pv.Spec.VolumeMode)
			}

			if !reflect.DeepEqual(pv.Spec.Capacity, tc.expectedPVSpec.Capacity) {
				t.Errorf("expected capacity: %v, got: %v", tc.expectedPVSpec.Capacity, pv.Spec.Capacity)
			}

			if !reflect.DeepEqual(pv.Spec.MountOptions, tc.expectedPVSpec.MountOptions) {
				t.Errorf("expected mount options: %v, got: %v", tc.expectedPVSpec.MountOptions, pv.Spec.MountOptions)
			}

			if tc.expectedPVSpec.CSIPVS != nil {
				if !reflect.DeepEqual(pv.Spec.PersistentVolumeSource.CSI, tc.expectedPVSpec.CSIPVS) {
					t.Errorf("expected PV: %v, got: %v", tc.expectedPVSpec.CSIPVS, pv.Spec.PersistentVolumeSource.CSI)
				}
			}

			if !reflect.DeepEqual(pv.Spec.VolumeAttributesClassName, tc.expectedPVSpec.VolumeAttributesClassName) {
				t.Errorf("expected VAC name: %v, got: %v", tc.expectedPVSpec.VolumeAttributesClassName, pv.Spec.VolumeAttributesClassName)
			}
		}
	} else {
		provision := csiProvisioner.(controller.Qualifier).ShouldProvision(context.Background(), tc.volOpts.PVC)
		if provision != !tc.expectNoProvision {
			t.Fatalf("expect ShouldProvision result %v, got %v", !tc.expectNoProvision, provision)
		}
	}

	if tc.volOpts.PVC != nil {
		claim, err := clientSet.CoreV1().PersistentVolumeClaims(tc.volOpts.PVC.Namespace).Get(context.Background(), tc.volOpts.PVC.Name, metav1.GetOptions{})
		if err != nil {
			t.Errorf("PVC %s not found: %v", tc.volOpts.PVC.Name, err)
		} else {
			if expectSelectedNode != "" {
				if claim.Annotations == nil {
					t.Errorf("PVC %s has no annotations", claim.Name)
				} else if claim.Annotations[annSelectedNode] != expectSelectedNode {
					t.Errorf("expected selected node %q, got %q", expectSelectedNode, claim.Annotations[annSelectedNode])
				}
			} else {
				if claim.Annotations[annSelectedNode] != "" {
					t.Errorf("expected no selected node, got %q", claim.Annotations[annSelectedNode])
				}
			}
		}
	}
}

// newContent returns a new content with given attributes
func newContent(name, namespace, className, snapshotHandle, volumeUID, volumeName, boundToSnapshotUID, boundToSnapshotName string, size *int64, creationTime *int64) *crdv1.VolumeSnapshotContent {
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
			SourceVolumeMode:        &volumeModeFileSystem,
		},
	}

	if boundToSnapshotName != "" {
		content.Spec.VolumeSnapshotRef = v1.ObjectReference{
			Kind:       "VolumeSnapshot",
			APIVersion: "snapshot.storage.k8s.io/v1beta1",
			UID:        types.UID(boundToSnapshotUID),
			Namespace:  namespace,
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
	apiGrp := "snapshot.storage.k8s.io"
	unsupportedAPIGrp := "unsupported.group.io"
	var requestedBytes int64 = 1000
	snapName := "test-snapshot"
	snapClassName := "test-snapclass"
	timeNow := time.Now().UnixNano()
	metaTimeNowUnix := &metav1.Time{
		Time: time.Unix(0, timeNow),
	}
	deletePolicy := v1.PersistentVolumeReclaimDelete
	dataSourceNamespace := "ns1"
	xnsNamespace := "ns2"
	type pvSpec struct {
		Name          string
		ReclaimPolicy v1.PersistentVolumeReclaimPolicy
		AccessModes   []v1.PersistentVolumeAccessMode
		Capacity      v1.ResourceList
		CSIPVS        *v1.CSIPersistentVolumeSource
	}

	type testcase struct {
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
		allowVolumeModeChange             bool
		xnsEnabled                        bool // set to use CrossNamespaceVolumeDataSource feature, default false
		snapNamespace                     string
		withreferenceGrants               bool // set to use ReferenceGrant, default false
		refGrantsrcNamespace              string
		referenceGrantFrom                []gatewayv1beta1.ReferenceGrantFrom
		referenceGrantTo                  []gatewayv1beta1.ReferenceGrantTo
	}
	testcases := map[string]testcase{
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
						Resources: v1.VolumeResourceRequirements{
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
					v1.ResourceName(v1.ResourceStorage): bytesToQuantity(requestedBytes),
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
						Resources: v1.VolumeResourceRequirements{
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
						Resources: v1.VolumeResourceRequirements{
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
						Resources: v1.VolumeResourceRequirements{
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
						Resources: v1.VolumeResourceRequirements{
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
						Resources: v1.VolumeResourceRequirements{
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
						Resources: v1.VolumeResourceRequirements{
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
						Resources: v1.VolumeResourceRequirements{
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
						Resources: v1.VolumeResourceRequirements{
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
						Resources: v1.VolumeResourceRequirements{
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
						Resources: v1.VolumeResourceRequirements{
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
						Resources: v1.VolumeResourceRequirements{
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
						Resources: v1.VolumeResourceRequirements{
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
						Resources: v1.VolumeResourceRequirements{
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
						Resources: v1.VolumeResourceRequirements{
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
						Resources: v1.VolumeResourceRequirements{
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
		"allow source volume mode conversion": {
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
						Resources: v1.VolumeResourceRequirements{
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
						VolumeMode: &volumeModeBlock,
					},
				},
			},
			expectedPVSpec: &pvSpec{
				Name:          "test-testi",
				ReclaimPolicy: v1.PersistentVolumeReclaimDelete,
				AccessModes:   []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
				Capacity: v1.ResourceList{
					v1.ResourceName(v1.ResourceStorage): bytesToQuantity(requestedBytes),
				},
				CSIPVS: &v1.CSIPersistentVolumeSource{
					Driver:       "test-driver",
					VolumeHandle: "test-volume-id",
					VolumeAttributes: map[string]string{
						"storage.kubernetes.io/csiProvisionerIdentity": "test-provisioner",
					},
				},
			},
			allowVolumeModeChange: true,
			snapshotStatusReady:   true,
			expectCSICall:         true,
			expectErr:             false,
		},
		"fail source volume mode conversion": {
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
						Resources: v1.VolumeResourceRequirements{
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
						VolumeMode: &volumeModeBlock,
					},
				},
			},
			allowVolumeModeChange: false,
			snapshotStatusReady:   true,
			expectErr:             true,
		},
		"provision with volume snapshot data source when CrossNamespaceVolumeDataSource feature enabled": {
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
						Resources: v1.VolumeResourceRequirements{
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
					v1.ResourceName(v1.ResourceStorage): bytesToQuantity(requestedBytes),
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
			xnsEnabled:    true,
		},
		"provision with xns volume snapshot data source with refgrant when CrossNamespaceVolumeDataSource feature enabled": {
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
						Namespace:   xnsNamespace,
					},
					Spec: v1.PersistentVolumeClaimSpec{
						StorageClassName: &snapClassName,
						Resources: v1.VolumeResourceRequirements{
							Requests: v1.ResourceList{
								v1.ResourceName(v1.ResourceStorage): resource.MustParse(strconv.FormatInt(requestedBytes, 10)),
							},
						},
						AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
						DataSourceRef: &v1.TypedObjectReference{
							Name:      snapName,
							Kind:      "VolumeSnapshot",
							APIGroup:  &apiGrp,
							Namespace: &dataSourceNamespace,
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
					v1.ResourceName(v1.ResourceStorage): bytesToQuantity(requestedBytes),
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
			expectCSICall:        true,
			xnsEnabled:           true,
			snapNamespace:        dataSourceNamespace,
			withreferenceGrants:  true,
			refGrantsrcNamespace: dataSourceNamespace,
			referenceGrantFrom: []gatewayv1beta1.ReferenceGrantFrom{
				{
					Group:     gatewayv1beta1.Group(""),
					Kind:      gatewayv1beta1.Kind("PersistentVolumeClaim"),
					Namespace: gatewayv1beta1.Namespace(xnsNamespace),
				},
			},
			referenceGrantTo: []gatewayv1beta1.ReferenceGrantTo{
				{
					Group: gatewayv1beta1.Group(apiGrp),
					Kind:  gatewayv1beta1.Kind("VolumeSnapshot"),
				},
			},
		},
		"provision with xns volume snapshot data source with refgrant of specify toName when CrossNamespaceVolumeDataSource feature enabled": {
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
						Namespace:   xnsNamespace,
					},
					Spec: v1.PersistentVolumeClaimSpec{
						StorageClassName: &snapClassName,
						Resources: v1.VolumeResourceRequirements{
							Requests: v1.ResourceList{
								v1.ResourceName(v1.ResourceStorage): resource.MustParse(strconv.FormatInt(requestedBytes, 10)),
							},
						},
						AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
						DataSourceRef: &v1.TypedObjectReference{
							Name:      snapName,
							Kind:      "VolumeSnapshot",
							APIGroup:  &apiGrp,
							Namespace: &dataSourceNamespace,
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
					v1.ResourceName(v1.ResourceStorage): bytesToQuantity(requestedBytes),
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
			expectCSICall:        true,
			xnsEnabled:           true,
			snapNamespace:        dataSourceNamespace,
			withreferenceGrants:  true,
			refGrantsrcNamespace: dataSourceNamespace,
			referenceGrantFrom: []gatewayv1beta1.ReferenceGrantFrom{
				{
					Group:     gatewayv1beta1.Group(""),
					Kind:      gatewayv1beta1.Kind("PersistentVolumeClaim"),
					Namespace: gatewayv1beta1.Namespace(xnsNamespace),
				},
			},
			referenceGrantTo: []gatewayv1beta1.ReferenceGrantTo{
				{
					Group: gatewayv1beta1.Group(apiGrp),
					Kind:  gatewayv1beta1.Kind("VolumeSnapshot"),
					Name:  ObjectNamePtr(snapName),
				},
			},
		},
		"provision with xns volume snapshot data source with refgrant of specify nil when CrossNamespaceVolumeDataSource feature enabled": {
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
						Namespace:   xnsNamespace,
					},
					Spec: v1.PersistentVolumeClaimSpec{
						StorageClassName: &snapClassName,
						Resources: v1.VolumeResourceRequirements{
							Requests: v1.ResourceList{
								v1.ResourceName(v1.ResourceStorage): resource.MustParse(strconv.FormatInt(requestedBytes, 10)),
							},
						},
						AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
						DataSourceRef: &v1.TypedObjectReference{
							Name:      snapName,
							Kind:      "VolumeSnapshot",
							APIGroup:  &apiGrp,
							Namespace: &dataSourceNamespace,
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
					v1.ResourceName(v1.ResourceStorage): bytesToQuantity(requestedBytes),
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
			expectCSICall:        true,
			xnsEnabled:           true,
			snapNamespace:        dataSourceNamespace,
			withreferenceGrants:  true,
			refGrantsrcNamespace: dataSourceNamespace,
			referenceGrantFrom: []gatewayv1beta1.ReferenceGrantFrom{
				{
					Group:     gatewayv1beta1.Group(""),
					Kind:      gatewayv1beta1.Kind("PersistentVolumeClaim"),
					Namespace: gatewayv1beta1.Namespace(xnsNamespace),
				},
			},
			referenceGrantTo: []gatewayv1beta1.ReferenceGrantTo{
				{
					Group: gatewayv1beta1.Group(apiGrp),
					Kind:  gatewayv1beta1.Kind("VolumeSnapshot"),
					Name:  nil,
				},
			},
		},
		"provision with xns volume snapshot data source with refgrant of specify non when CrossNamespaceVolumeDataSource feature enabled": {
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
						Namespace:   xnsNamespace,
					},
					Spec: v1.PersistentVolumeClaimSpec{
						StorageClassName: &snapClassName,
						Resources: v1.VolumeResourceRequirements{
							Requests: v1.ResourceList{
								v1.ResourceName(v1.ResourceStorage): resource.MustParse(strconv.FormatInt(requestedBytes, 10)),
							},
						},
						AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
						DataSourceRef: &v1.TypedObjectReference{
							Name:      snapName,
							Kind:      "VolumeSnapshot",
							APIGroup:  &apiGrp,
							Namespace: &dataSourceNamespace,
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
					v1.ResourceName(v1.ResourceStorage): bytesToQuantity(requestedBytes),
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
			expectCSICall:        true,
			xnsEnabled:           true,
			snapNamespace:        dataSourceNamespace,
			withreferenceGrants:  true,
			refGrantsrcNamespace: dataSourceNamespace,
			referenceGrantFrom: []gatewayv1beta1.ReferenceGrantFrom{
				{
					Group:     gatewayv1beta1.Group(""),
					Kind:      gatewayv1beta1.Kind("PersistentVolumeClaim"),
					Namespace: gatewayv1beta1.Namespace(xnsNamespace),
				},
			},
			referenceGrantTo: []gatewayv1beta1.ReferenceGrantTo{
				{
					Group: gatewayv1beta1.Group(apiGrp),
					Kind:  gatewayv1beta1.Kind("VolumeSnapshot"),
					Name:  ObjectNamePtr(""),
				},
			},
		},
		"provision with same ns volume snapshot data source without refgrant when CrossNamespaceVolumeDataSource feature enabled": {
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
						Namespace:   dataSourceNamespace,
					},
					Spec: v1.PersistentVolumeClaimSpec{
						StorageClassName: &snapClassName,
						Resources: v1.VolumeResourceRequirements{
							Requests: v1.ResourceList{
								v1.ResourceName(v1.ResourceStorage): resource.MustParse(strconv.FormatInt(requestedBytes, 10)),
							},
						},
						AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
						DataSourceRef: &v1.TypedObjectReference{
							APIGroup:  &apiGrp,
							Kind:      "VolumeSnapshot",
							Name:      snapName,
							Namespace: &dataSourceNamespace,
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
					v1.ResourceName(v1.ResourceStorage): bytesToQuantity(requestedBytes),
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
			xnsEnabled:    true,
			snapNamespace: dataSourceNamespace,
		},
		"provision with non-ns volume snapshot data source without refgrant when CrossNamespaceVolumeDataSource feature enabled": {
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
						Namespace:   dataSourceNamespace,
					},
					Spec: v1.PersistentVolumeClaimSpec{
						StorageClassName: &snapClassName,
						Resources: v1.VolumeResourceRequirements{
							Requests: v1.ResourceList{
								v1.ResourceName(v1.ResourceStorage): resource.MustParse(strconv.FormatInt(requestedBytes, 10)),
							},
						},
						AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
						DataSourceRef: &v1.TypedObjectReference{
							APIGroup: &apiGrp,
							Kind:     "VolumeSnapshot",
							Name:     snapName,
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
					v1.ResourceName(v1.ResourceStorage): bytesToQuantity(requestedBytes),
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
			xnsEnabled:    true,
			snapNamespace: dataSourceNamespace,
		},
		"fail provision with xns volume snapshot data source without refgrant when CrossNamespaceVolumeDataSource feature enabled": {
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
						Namespace:   xnsNamespace,
					},
					Spec: v1.PersistentVolumeClaimSpec{
						StorageClassName: &snapClassName,
						Resources: v1.VolumeResourceRequirements{
							Requests: v1.ResourceList{
								v1.ResourceName(v1.ResourceStorage): resource.MustParse(strconv.FormatInt(requestedBytes, 10)),
							},
						},
						AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
						DataSourceRef: &v1.TypedObjectReference{
							Name:      snapName,
							Kind:      "VolumeSnapshot",
							APIGroup:  &apiGrp,
							Namespace: &dataSourceNamespace,
						},
					},
				},
			},
			snapshotStatusReady: true,
			expectErr:           true,
			xnsEnabled:          true,
			snapNamespace:       dataSourceNamespace,
		},
		"fail provision with xns volume snapshot data source with refgrant when CrossNamespaceVolumeDataSource feature disabled": {
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
						Namespace:   xnsNamespace,
					},
					Spec: v1.PersistentVolumeClaimSpec{
						StorageClassName: &snapClassName,
						Resources: v1.VolumeResourceRequirements{
							Requests: v1.ResourceList{
								v1.ResourceName(v1.ResourceStorage): resource.MustParse(strconv.FormatInt(requestedBytes, 10)),
							},
						},
						AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
						DataSourceRef: &v1.TypedObjectReference{
							Name:      snapName,
							Kind:      "VolumeSnapshot",
							APIGroup:  &apiGrp,
							Namespace: &dataSourceNamespace,
						},
					},
				},
			},
			snapshotStatusReady:  true,
			expectErr:            true,
			snapNamespace:        dataSourceNamespace,
			withreferenceGrants:  true,
			refGrantsrcNamespace: dataSourceNamespace,
			referenceGrantFrom: []gatewayv1beta1.ReferenceGrantFrom{
				{
					Group:     gatewayv1beta1.Group(""),
					Kind:      gatewayv1beta1.Kind("PersistentVolumeClaim"),
					Namespace: gatewayv1beta1.Namespace(xnsNamespace),
				},
			},
			referenceGrantTo: []gatewayv1beta1.ReferenceGrantTo{
				{
					Group: gatewayv1beta1.Group(apiGrp),
					Kind:  gatewayv1beta1.Kind("VolumeSnapshot"),
				},
			},
		},
		"fail provision with xns volume snapshot data source with refgrant of wrong create namespace when CrossNamespaceVolumeDataSource feature enabled": {
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
						Namespace:   xnsNamespace,
					},
					Spec: v1.PersistentVolumeClaimSpec{
						StorageClassName: &snapClassName,
						Resources: v1.VolumeResourceRequirements{
							Requests: v1.ResourceList{
								v1.ResourceName(v1.ResourceStorage): resource.MustParse(strconv.FormatInt(requestedBytes, 10)),
							},
						},
						AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
						DataSourceRef: &v1.TypedObjectReference{
							Name:      snapName,
							Kind:      "VolumeSnapshot",
							APIGroup:  &apiGrp,
							Namespace: &dataSourceNamespace,
						},
					},
				},
			},
			snapshotStatusReady:  true,
			expectErr:            true,
			xnsEnabled:           true,
			snapNamespace:        dataSourceNamespace,
			withreferenceGrants:  true,
			refGrantsrcNamespace: "wrong-reference-namespace",
			referenceGrantFrom: []gatewayv1beta1.ReferenceGrantFrom{
				{
					Group:     gatewayv1beta1.Group(""),
					Kind:      gatewayv1beta1.Kind("PersistentVolumeClaim"),
					Namespace: gatewayv1beta1.Namespace(xnsNamespace),
				},
			},
			referenceGrantTo: []gatewayv1beta1.ReferenceGrantTo{
				{
					Group: gatewayv1beta1.Group(apiGrp),
					Kind:  gatewayv1beta1.Kind("VolumeSnapshot"),
				},
			},
		},
		"fail provision with xns volume snapshot data source with refgrant of wrong fromaGroup when CrossNamespaceVolumeDataSource feature enabled": {
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
						Namespace:   xnsNamespace,
					},
					Spec: v1.PersistentVolumeClaimSpec{
						StorageClassName: &snapClassName,
						Resources: v1.VolumeResourceRequirements{
							Requests: v1.ResourceList{
								v1.ResourceName(v1.ResourceStorage): resource.MustParse(strconv.FormatInt(requestedBytes, 10)),
							},
						},
						AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
						DataSourceRef: &v1.TypedObjectReference{
							Name:      snapName,
							Kind:      "VolumeSnapshot",
							APIGroup:  &apiGrp,
							Namespace: &dataSourceNamespace,
						},
					},
				},
			},
			snapshotStatusReady:  true,
			expectErr:            true,
			xnsEnabled:           true,
			snapNamespace:        dataSourceNamespace,
			withreferenceGrants:  true,
			refGrantsrcNamespace: dataSourceNamespace,
			referenceGrantFrom: []gatewayv1beta1.ReferenceGrantFrom{
				{
					Group:     gatewayv1beta1.Group("wrong.apiGroup"),
					Kind:      gatewayv1beta1.Kind("PersistentVolumeClaim"),
					Namespace: gatewayv1beta1.Namespace(xnsNamespace),
				},
			},
			referenceGrantTo: []gatewayv1beta1.ReferenceGrantTo{
				{
					Group: gatewayv1beta1.Group(apiGrp),
					Kind:  gatewayv1beta1.Kind("VolumeSnapshot"),
				},
			},
		},
		"fail provision with xns volume snapshot data source with refgrant of wrong fromKind when CrossNamespaceVolumeDataSource feature enabled": {
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
						Namespace:   xnsNamespace,
					},
					Spec: v1.PersistentVolumeClaimSpec{
						StorageClassName: &snapClassName,
						Resources: v1.VolumeResourceRequirements{
							Requests: v1.ResourceList{
								v1.ResourceName(v1.ResourceStorage): resource.MustParse(strconv.FormatInt(requestedBytes, 10)),
							},
						},
						AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
						DataSourceRef: &v1.TypedObjectReference{
							Name:      snapName,
							Kind:      "VolumeSnapshot",
							APIGroup:  &apiGrp,
							Namespace: &dataSourceNamespace,
						},
					},
				},
			},
			snapshotStatusReady:  true,
			expectErr:            true,
			xnsEnabled:           true,
			snapNamespace:        dataSourceNamespace,
			withreferenceGrants:  true,
			refGrantsrcNamespace: dataSourceNamespace,
			referenceGrantFrom: []gatewayv1beta1.ReferenceGrantFrom{
				{
					Group:     gatewayv1beta1.Group(""),
					Kind:      gatewayv1beta1.Kind("WrongKind"),
					Namespace: gatewayv1beta1.Namespace(xnsNamespace),
				},
			},
			referenceGrantTo: []gatewayv1beta1.ReferenceGrantTo{
				{
					Group: gatewayv1beta1.Group(apiGrp),
					Kind:  gatewayv1beta1.Kind("VolumeSnapshot"),
				},
			},
		},
		"fail provision with xns volume snapshot data source with refgrant of wrong fromNamespace when CrossNamespaceVolumeDataSource feature enabled": {
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
						Namespace:   xnsNamespace,
					},
					Spec: v1.PersistentVolumeClaimSpec{
						StorageClassName: &snapClassName,
						Resources: v1.VolumeResourceRequirements{
							Requests: v1.ResourceList{
								v1.ResourceName(v1.ResourceStorage): resource.MustParse(strconv.FormatInt(requestedBytes, 10)),
							},
						},
						AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
						DataSourceRef: &v1.TypedObjectReference{
							Name:      snapName,
							Kind:      "VolumeSnapshot",
							APIGroup:  &apiGrp,
							Namespace: &dataSourceNamespace,
						},
					},
				},
			},
			snapshotStatusReady:  true,
			expectErr:            true,
			xnsEnabled:           true,
			snapNamespace:        dataSourceNamespace,
			withreferenceGrants:  true,
			refGrantsrcNamespace: dataSourceNamespace,
			referenceGrantFrom: []gatewayv1beta1.ReferenceGrantFrom{
				{
					Group:     gatewayv1beta1.Group(""),
					Kind:      gatewayv1beta1.Kind("PersistentVolumeClaim"),
					Namespace: gatewayv1beta1.Namespace("wrong-namespace"),
				},
			},
			referenceGrantTo: []gatewayv1beta1.ReferenceGrantTo{
				{
					Group: gatewayv1beta1.Group(apiGrp),
					Kind:  gatewayv1beta1.Kind("VolumeSnapshot"),
				},
			},
		},
		"fail provision with xns volume snapshot data source with refgrant of wrong toKind when CrossNamespaceVolumeDataSource feature enabled": {
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
						Namespace:   xnsNamespace,
					},
					Spec: v1.PersistentVolumeClaimSpec{
						StorageClassName: &snapClassName,
						Resources: v1.VolumeResourceRequirements{
							Requests: v1.ResourceList{
								v1.ResourceName(v1.ResourceStorage): resource.MustParse(strconv.FormatInt(requestedBytes, 10)),
							},
						},
						AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
						DataSourceRef: &v1.TypedObjectReference{
							Name:      snapName,
							Kind:      "VolumeSnapshot",
							APIGroup:  &apiGrp,
							Namespace: &dataSourceNamespace,
						},
					},
				},
			},
			snapshotStatusReady:  true,
			expectErr:            true,
			xnsEnabled:           true,
			snapNamespace:        dataSourceNamespace,
			withreferenceGrants:  true,
			refGrantsrcNamespace: dataSourceNamespace,
			referenceGrantFrom: []gatewayv1beta1.ReferenceGrantFrom{
				{
					Group:     gatewayv1beta1.Group(""),
					Kind:      gatewayv1beta1.Kind("PersistentVolumeClaim"),
					Namespace: gatewayv1beta1.Namespace(xnsNamespace),
				},
			},
			referenceGrantTo: []gatewayv1beta1.ReferenceGrantTo{
				{
					Group: gatewayv1beta1.Group(apiGrp),
					Kind:  gatewayv1beta1.Kind("WrongKind"),
				},
			},
		},
		"fail provision with xns volume snapshot data source with refgrant of wrong toGroup when CrossNamespaceVolumeDataSource feature enabled": {
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
						Namespace:   xnsNamespace,
					},
					Spec: v1.PersistentVolumeClaimSpec{
						StorageClassName: &snapClassName,
						Resources: v1.VolumeResourceRequirements{
							Requests: v1.ResourceList{
								v1.ResourceName(v1.ResourceStorage): resource.MustParse(strconv.FormatInt(requestedBytes, 10)),
							},
						},
						AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
						DataSourceRef: &v1.TypedObjectReference{
							Name:      snapName,
							Kind:      "VolumeSnapshot",
							APIGroup:  &apiGrp,
							Namespace: &dataSourceNamespace,
						},
					},
				},
			},
			snapshotStatusReady:  true,
			expectErr:            true,
			xnsEnabled:           true,
			snapNamespace:        dataSourceNamespace,
			withreferenceGrants:  true,
			refGrantsrcNamespace: dataSourceNamespace,
			referenceGrantFrom: []gatewayv1beta1.ReferenceGrantFrom{
				{
					Group:     gatewayv1beta1.Group(""),
					Kind:      gatewayv1beta1.Kind("PersistentVolumeClaim"),
					Namespace: gatewayv1beta1.Namespace(xnsNamespace),
				},
			},
			referenceGrantTo: []gatewayv1beta1.ReferenceGrantTo{
				{
					Group: gatewayv1beta1.Group("wrong.toGroup"),
					Kind:  gatewayv1beta1.Kind("VolumeSnapshot"),
				},
			},
		},
		"fail provision with xns volume snapshot data source with refgrant of wrong toName when CrossNamespaceVolumeDataSource feature enabled": {
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
						Namespace:   xnsNamespace,
					},
					Spec: v1.PersistentVolumeClaimSpec{
						StorageClassName: &snapClassName,
						Resources: v1.VolumeResourceRequirements{
							Requests: v1.ResourceList{
								v1.ResourceName(v1.ResourceStorage): resource.MustParse(strconv.FormatInt(requestedBytes, 10)),
							},
						},
						AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
						DataSourceRef: &v1.TypedObjectReference{
							Name:      snapName,
							Kind:      "VolumeSnapshot",
							APIGroup:  &apiGrp,
							Namespace: &dataSourceNamespace,
						},
					},
				},
			},
			snapshotStatusReady:  true,
			expectErr:            true,
			xnsEnabled:           true,
			snapNamespace:        dataSourceNamespace,
			withreferenceGrants:  true,
			refGrantsrcNamespace: dataSourceNamespace,
			referenceGrantFrom: []gatewayv1beta1.ReferenceGrantFrom{
				{
					Group:     gatewayv1beta1.Group(""),
					Kind:      gatewayv1beta1.Kind("PersistentVolumeClaim"),
					Namespace: gatewayv1beta1.Namespace(xnsNamespace),
				},
			},
			referenceGrantTo: []gatewayv1beta1.ReferenceGrantTo{
				{
					Group: gatewayv1beta1.Group(apiGrp),
					Kind:  gatewayv1beta1.Kind("VolumeSnapshot"),
					Name:  ObjectNamePtr("wrong-dataSourceName"),
				},
			},
		},
		"fail provision with xns volume snapshot data source with refgrant of referenceGrantFrom and referenceGrantTo empty when CrossNamespaceVolumeDataSource feature enabled": {
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
						Namespace:   xnsNamespace,
					},
					Spec: v1.PersistentVolumeClaimSpec{
						StorageClassName: &snapClassName,
						Resources: v1.VolumeResourceRequirements{
							Requests: v1.ResourceList{
								v1.ResourceName(v1.ResourceStorage): resource.MustParse(strconv.FormatInt(requestedBytes, 10)),
							},
						},
						AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
						DataSourceRef: &v1.TypedObjectReference{
							Name:      snapName,
							Kind:      "VolumeSnapshot",
							APIGroup:  &apiGrp,
							Namespace: &dataSourceNamespace,
						},
					},
				},
			},
			snapshotStatusReady:  true,
			expectErr:            true,
			xnsEnabled:           true,
			snapNamespace:        dataSourceNamespace,
			withreferenceGrants:  true,
			refGrantsrcNamespace: dataSourceNamespace,
			referenceGrantFrom:   []gatewayv1beta1.ReferenceGrantFrom{},
			referenceGrantTo:     []gatewayv1beta1.ReferenceGrantTo{},
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

	doit := func(t *testing.T, tc testcase) {
		var clientSet kubernetes.Interface
		clientSet = fakeclientset.NewSimpleClientset()
		client := &fake.Clientset{}

		client.AddReactor("get", "volumesnapshots", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
			namespace := "default"
			if tc.snapNamespace != "" {
				namespace = tc.snapNamespace
			}
			snap := newSnapshot(snapName, namespace, snapClassName, "snapcontent-snapuid", "snapuid", "claim", tc.snapshotStatusReady, nil, metaTimeNowUnix, resource.NewQuantity(requestedBytes, resource.BinarySI))
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
			namespace := "default"
			if tc.snapNamespace != "" {
				namespace = tc.snapNamespace
			}
			content := newContent("snapcontent-snapuid", namespace, snapClassName, "sid", "pv-uid", "volume", "snapuid", snapName, &requestedBytes, &timeNow)
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
			if tc.allowVolumeModeChange {
				content.Annotations = map[string]string{
					annAllowVolumeModeChange: "true",
				}
			}
			return true, content, nil
		})

		var refGrantLister referenceGrantv1beta1.ReferenceGrantLister
		var stopChan chan struct{}
		var gatewayClient *fakegateway.Clientset
		if tc.withreferenceGrants {
			referenceGrant := generateReferenceGrant(tc.refGrantsrcNamespace, tc.referenceGrantFrom, tc.referenceGrantTo)
			gatewayClient = fakegateway.NewSimpleClientset(referenceGrant)
		} else {
			gatewayClient = fakegateway.NewSimpleClientset()
		}

		if tc.xnsEnabled {
			gatewayFactory := gatewayInformers.NewSharedInformerFactory(gatewayClient, ResyncPeriodOfReferenceGrantInformer)
			referenceGrants := gatewayFactory.Gateway().V1beta1().ReferenceGrants()
			refGrantLister = referenceGrants.Lister()

			stopChan := make(chan struct{})
			gatewayFactory.Start(stopChan)
			gatewayFactory.WaitForCacheSync(stopChan)
		}
		defer func() {
			if stopChan != nil {
				close(stopChan)
			}
		}()

		defer utilfeaturetesting.SetFeatureGateDuringTest(t, utilfeature.DefaultFeatureGate, features.CrossNamespaceVolumeDataSource, tc.xnsEnabled)()

		pluginCaps, controllerCaps := provisionFromSnapshotCapabilities()
		csiProvisioner := NewCSIProvisioner(clientSet, 5*time.Second, "test-provisioner", "test", 5, csiConn.conn,
			client, driverName, pluginCaps, controllerCaps, "", false, true, csitrans.New(), nil, nil, nil, nil, nil, refGrantLister, false, "", defaultfsType, nil, true, true)

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

		pv, _, err := csiProvisioner.Provision(context.Background(), tc.volOpts)
		if tc.expectErr && err == nil {
			t.Errorf("Expected error, got none")
		}

		if !tc.expectErr && err != nil {
			t.Errorf("got error: %v", err)
		}

		if tc.expectedPVSpec != nil {
			if pv != nil {
				if pv.Name != tc.expectedPVSpec.Name {
					t.Errorf("expected PV name: %q, got: %q", tc.expectedPVSpec.Name, pv.Name)
				}

				if !reflect.DeepEqual(pv.Spec.Capacity, tc.expectedPVSpec.Capacity) {
					t.Errorf("expected capacity: %v, got: %v", tc.expectedPVSpec.Capacity, pv.Spec.Capacity)
				}

				if tc.expectedPVSpec.CSIPVS != nil {
					if !reflect.DeepEqual(pv.Spec.PersistentVolumeSource.CSI, tc.expectedPVSpec.CSIPVS) {
						t.Errorf("expected PV: %v, got: %v", tc.expectedPVSpec.CSIPVS, pv.Spec.PersistentVolumeSource.CSI)
					}
				}
			}
		}
	}

	for k, tc := range testcases {
		t.Run(k, func(t *testing.T) {
			doit(t, tc)
		})
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

			nodes := buildNodes(tc.nodeLabels)
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

			scLister, csiNodeLister, nodeLister, claimLister, vaLister, stopChan := listers(clientSet)
			defer close(stopChan)

			csiProvisioner := NewCSIProvisioner(clientSet, 5*time.Second, "test-provisioner", "test", 5,
				csiConn.conn, nil, driverName, pluginCaps, controllerCaps, "", false, true, csitrans.New(), scLister, csiNodeLister, nodeLister, claimLister, vaLister, nil, false, "", defaultfsType, nil, true, false)

			pv, _, err := csiProvisioner.Provision(context.Background(), controller.ProvisionOptions{
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

// TestProvisionErrorHandling checks how different errors are handled by the provisioner.
func TestProvisionErrorHandling(t *testing.T) {
	const requestBytes = 100

	testcases := map[codes.Code]controller.ProvisioningState{
		codes.Canceled:         controller.ProvisioningInBackground,
		codes.DeadlineExceeded: controller.ProvisioningInBackground,
		codes.Unavailable:      controller.ProvisioningInBackground,
		codes.Aborted:          controller.ProvisioningInBackground,

		codes.ResourceExhausted:  controller.ProvisioningFinished,
		codes.Unknown:            controller.ProvisioningFinished,
		codes.InvalidArgument:    controller.ProvisioningFinished,
		codes.NotFound:           controller.ProvisioningFinished,
		codes.AlreadyExists:      controller.ProvisioningFinished,
		codes.PermissionDenied:   controller.ProvisioningFinished,
		codes.FailedPrecondition: controller.ProvisioningFinished,
		codes.OutOfRange:         controller.ProvisioningFinished,
		codes.Unimplemented:      controller.ProvisioningFinished,
		codes.Internal:           controller.ProvisioningFinished,
		codes.DataLoss:           controller.ProvisioningFinished,
		codes.Unauthenticated:    controller.ProvisioningFinished,
	}
	nodeLabels := []map[string]string{
		{"com.example.csi/zone": "zone1", "com.example.csi/rack": "rack1"},
		{"com.example.csi/zone": "zone1", "com.example.csi/rack": "rack2"},
	}
	topologyKeys := []map[string][]string{
		{driverName: []string{"com.example.csi/zone", "com.example.csi/rack"}},
		{driverName: []string{"com.example.csi/zone", "com.example.csi/rack"}},
	}

	test := func(driverSupportsTopology, nodeSelected bool) {
		t.Run(fmt.Sprintf("topology=%v node=%v", driverSupportsTopology, nodeSelected), func(t *testing.T) {
			for code, expectedState := range testcases {
				t.Run(code.String(), func(t *testing.T) {
					var (
						pluginCaps     rpc.PluginCapabilitySet
						controllerCaps rpc.ControllerCapabilitySet
					)
					if driverSupportsTopology {
						defer utilfeaturetesting.SetFeatureGateDuringTest(t, utilfeature.DefaultFeatureGate, features.Topology, true)()
						pluginCaps, controllerCaps = provisionWithTopologyCapabilities()
					} else {
						pluginCaps, controllerCaps = provisionCapabilities()
					}

					tmpdir := tempDir(t)
					defer os.RemoveAll(tmpdir)
					mockController, driver, _, controllerServer, csiConn, err := createMockServer(t, tmpdir)
					if err != nil {
						t.Fatal(err)
					}
					defer mockController.Finish()
					defer driver.Stop()

					// Always return some error.
					errOut := status.Error(code, "fake error")
					controllerServer.EXPECT().CreateVolume(gomock.Any(), gomock.Any()).Return(nil, errOut).Times(1)

					nodes := buildNodes(nodeLabels)
					csiNodes := buildCSINodes(topologyKeys)
					clientSet := fakeclientset.NewSimpleClientset(nodes, csiNodes)
					scLister, csiNodeLister, nodeLister, claimLister, vaLister, stopChan := listers(clientSet)
					defer close(stopChan)

					csiProvisioner := NewCSIProvisioner(clientSet, 5*time.Second, "test-provisioner", "test", 5,
						csiConn.conn, nil, driverName, pluginCaps, controllerCaps, "", false, true, csitrans.New(), scLister, csiNodeLister, nodeLister, claimLister, vaLister, nil, false, "", defaultfsType, nil, true, false)

					options := controller.ProvisionOptions{
						StorageClass: &storagev1.StorageClass{},
						PVC:          createFakePVC(requestBytes),
					}
					if nodeSelected {
						options.SelectedNode = &nodes.Items[0]
					}
					pv, state, err := csiProvisioner.Provision(context.Background(), options)

					if pv != nil {
						t.Errorf("expected no PV, got %v", pv)
					}
					if err == nil {
						t.Fatal("expected error, got nil")
					}
					st, ok := status.FromError(err)
					if !ok {
						t.Errorf("expected status %s, got error without status: %v", code, err)
					} else if st.Code() != code {
						t.Errorf("expected status %s, got %s", code, st.Code())
					}

					// This is the only situation where we request rescheduling.
					if driverSupportsTopology && nodeSelected && code == codes.ResourceExhausted {
						expectedState = controller.ProvisioningReschedule
					}

					if expectedState != state {
						t.Errorf("expected provisioning state %s, got %s", expectedState, state)
					}
				})
			}
		})
	}

	// Try all four combinations. For most of them there's no
	// difference, but better check...
	test(false, false)
	test(false, true)
	test(true, false)
	test(true, true)
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
		csiConn.conn, nil, driverName, pluginCaps, controllerCaps, "", false, true, csitrans.New(), nil, nil, nil, nil, nil, nil, false, "", defaultfsType, nil, true, false)

	out := &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			CapacityBytes:      requestBytes,
			VolumeId:           "test-volume-id",
			AccessibleTopology: accessibleTopology,
		},
	}

	controllerServer.EXPECT().CreateVolume(gomock.Any(), gomock.Any()).Return(out, nil).Times(1)

	pv, _, err := csiProvisioner.Provision(context.Background(), controller.ProvisionOptions{
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

type expectedSecret struct {
	exist   bool
	secrets map[string]string
}

type deleteTestcase struct {
	persistentVolume          *v1.PersistentVolume
	storageClass              *storagev1.StorageClass
	secrets                   []runtime.Object
	volumeAttachment          *storagev1.VolumeAttachment
	mockDelete                bool
	expectedProvisionerSecret *expectedSecret
	deploymentNode            string // fake distributed provisioning with this node as host
	expectErr                 bool
}

func getDefaultProvisinerSecrets() []runtime.Object {
	return []runtime.Object{
		&v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "provisionersecret",
				Namespace: defaultSecretNsName,
			},
			Data: map[string][]byte{
				"provisionersecret-key": []byte("provisionersecret-val"),
			},
		},
	}
}

// TestDelete is a test of the delete operation
func TestDelete(t *testing.T) {
	pvName := "pv"
	deletionTimestamp := metav1.NewTime(time.Now())
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
		"fail - delete when attached to node": {
			persistentVolume: &v1.PersistentVolume{
				ObjectMeta: metav1.ObjectMeta{
					Name: pvName,
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
					prefixedProvisionerSecretNameKey: "static-${pv.name}-${pvc.namespace}-${pvc.name}",
				},
			},
			volumeAttachment: &storagev1.VolumeAttachment{
				ObjectMeta: metav1.ObjectMeta{
					Name: "va",
				},
				Spec: storagev1.VolumeAttachmentSpec{
					Source: storagev1.VolumeAttachmentSource{
						PersistentVolumeName: &pvName,
					},
					NodeName: "node",
				},
				Status: storagev1.VolumeAttachmentStatus{
					Attached: true,
				},
			},
			expectErr: true,
		},
		"fail - delete when volumeattachment exists but not attached to node": {
			persistentVolume: &v1.PersistentVolume{
				ObjectMeta: metav1.ObjectMeta{
					Name: pvName,
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
					prefixedProvisionerSecretNameKey: "static-${pv.name}-${pvc.namespace}-${pvc.name}",
				},
			},
			volumeAttachment: &storagev1.VolumeAttachment{
				ObjectMeta: metav1.ObjectMeta{
					Name: "va",
				},
				Spec: storagev1.VolumeAttachmentSpec{
					Source: storagev1.VolumeAttachmentSource{
						PersistentVolumeName: &pvName,
					},
					NodeName: "node",
				},
				Status: storagev1.VolumeAttachmentStatus{
					Attached: false,
				},
			},
			expectErr: true,
		},
		"fail - delete when volumeattachment exists with deletionTimestamp set": {
			persistentVolume: &v1.PersistentVolume{
				ObjectMeta: metav1.ObjectMeta{
					Name: pvName,
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
					prefixedProvisionerSecretNameKey: "static-${pv.name}-${pvc.namespace}-${pvc.name}",
				},
			},
			volumeAttachment: &storagev1.VolumeAttachment{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "va",
					DeletionTimestamp: &deletionTimestamp,
				},
				Spec: storagev1.VolumeAttachmentSpec{
					Source: storagev1.VolumeAttachmentSource{
						PersistentVolumeName: &pvName,
					},
					NodeName: "node",
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
		"simple - valid case with existing volumeattachment on different pv": {
			persistentVolume: &v1.PersistentVolume{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pv-without-attachment",
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
			volumeAttachment: &storagev1.VolumeAttachment{
				ObjectMeta: metav1.ObjectMeta{
					Name: "va",
				},
				Spec: storagev1.VolumeAttachmentSpec{
					Source: storagev1.VolumeAttachmentSource{
						PersistentVolumeName: &pvName,
					},
					NodeName: "node",
				},
				Status: storagev1.VolumeAttachmentStatus{
					Attached: true,
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
		"Empty provisioner secret is set as PV annotation and StorageClass exists": {
			persistentVolume: &v1.PersistentVolume{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pv",
					Annotations: map[string]string{
						annDeletionProvisionerSecretRefName:      "",
						annDeletionProvisionerSecretRefNamespace: "",
					},
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
				Parameters: map[string]string{},
			},
			expectErr:                 false,
			mockDelete:                true,
			expectedProvisionerSecret: &expectedSecret{exist: false},
		},
		"Empty provisioner secret is set as PV annotation and StorageClass doesn't exist on deletion": {
			persistentVolume: &v1.PersistentVolume{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pv",
					Annotations: map[string]string{
						annDeletionProvisionerSecretRefName:      "",
						annDeletionProvisionerSecretRefNamespace: "",
					},
				},
				Spec: v1.PersistentVolumeSpec{
					PersistentVolumeSource: v1.PersistentVolumeSource{
						CSI: &v1.CSIPersistentVolumeSource{
							VolumeHandle: "vol-id-1",
						},
					},
				},
			},
			expectErr:                 false,
			mockDelete:                true,
			expectedProvisionerSecret: &expectedSecret{exist: false},
		},
		"Non-empty provisioner secret is set as PV annotation and StorageClass exists": {
			persistentVolume: &v1.PersistentVolume{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pv",
					Annotations: map[string]string{
						annDeletionProvisionerSecretRefName:      "provisionersecret",
						annDeletionProvisionerSecretRefNamespace: defaultSecretNsName,
					},
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
					prefixedProvisionerSecretNameKey:      "provisionersecret",
					prefixedProvisionerSecretNamespaceKey: defaultSecretNsName,
				},
			},
			secrets:    getDefaultProvisinerSecrets(),
			expectErr:  false,
			mockDelete: true,
			expectedProvisionerSecret: &expectedSecret{
				exist:   true,
				secrets: map[string]string{"provisionersecret-key": "provisionersecret-val"},
			},
		},
		"Non-empty provisioner secret is set as PV annotation and StorageClass doesn't exist": {
			persistentVolume: &v1.PersistentVolume{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pv",
					Annotations: map[string]string{
						annDeletionProvisionerSecretRefName:      "provisionersecret",
						annDeletionProvisionerSecretRefNamespace: defaultSecretNsName,
					},
				},
				Spec: v1.PersistentVolumeSpec{
					PersistentVolumeSource: v1.PersistentVolumeSource{
						CSI: &v1.CSIPersistentVolumeSource{
							VolumeHandle: "vol-id-1",
						},
					},
				},
			},
			secrets:    getDefaultProvisinerSecrets(),
			expectErr:  false,
			mockDelete: true,
			expectedProvisionerSecret: &expectedSecret{
				exist:   true,
				secrets: map[string]string{"provisionersecret-key": "provisionersecret-val"},
			},
		},
		"Non-empty provisioner secret is set as PV annotation and modified StorageClass exists": {
			persistentVolume: &v1.PersistentVolume{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pv",
					Annotations: map[string]string{
						annDeletionProvisionerSecretRefName:      "provisionersecret",
						annDeletionProvisionerSecretRefNamespace: defaultSecretNsName,
					},
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
					prefixedProvisionerSecretNameKey:      "modified-provisionersecret",
					prefixedProvisionerSecretNamespaceKey: "modified-namespace",
				},
			},
			secrets:    getDefaultProvisinerSecrets(),
			expectErr:  false,
			mockDelete: true,
			expectedProvisionerSecret: &expectedSecret{
				exist:   true,
				secrets: map[string]string{"provisionersecret-key": "provisionersecret-val"},
			},
		},
		"Non-existent provisioner secret is set as PV annotation": {
			persistentVolume: &v1.PersistentVolume{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pv",
					Annotations: map[string]string{
						annDeletionProvisionerSecretRefName:      "non-existent-provisionersecret",
						annDeletionProvisionerSecretRefNamespace: defaultSecretNsName,
					},
				},
				Spec: v1.PersistentVolumeSpec{
					PersistentVolumeSource: v1.PersistentVolumeSource{
						CSI: &v1.CSIPersistentVolumeSource{
							VolumeHandle: "vol-id-1",
						},
					},
				},
			},
			secrets:                   getDefaultProvisinerSecrets(),
			expectErr:                 false, // Deletion doesn't fail even if the Secret is not found
			mockDelete:                true,
			expectedProvisionerSecret: &expectedSecret{exist: false}, // No secret is passed to csi DeleteVolume call
		},
		"Provisioner secret isn't set as PV annotation": {
			persistentVolume: &v1.PersistentVolume{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pv",
					Annotations: map[string]string{
						v1.BetaStorageClassAnnotation: "sc-name",
					},
				},
				Spec: v1.PersistentVolumeSpec{
					PersistentVolumeSource: v1.PersistentVolumeSource{
						CSI: &v1.CSIPersistentVolumeSource{
							VolumeHandle: "vol-id-1",
						},
					},
					ClaimRef: &v1.ObjectReference{
						Namespace: "pvc-namespace",
						Name:      "pvc-name",
					},
				},
			},
			storageClass: &storagev1.StorageClass{
				ObjectMeta: metav1.ObjectMeta{
					Name: "sc-name",
				},
				Parameters: map[string]string{
					prefixedProvisionerSecretNameKey:      "provisionersecret",
					prefixedProvisionerSecretNamespaceKey: defaultSecretNsName,
				},
			},
			secrets:    getDefaultProvisinerSecrets(),
			expectErr:  false,
			mockDelete: true,
			expectedProvisionerSecret: &expectedSecret{
				exist:   true,
				secrets: map[string]string{"provisionersecret-key": "provisionersecret-val"},
			},
		},
		"distributed, ignore PV from other host": {
			deploymentNode: "foo",
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
					NodeAffinity: &v1.VolumeNodeAffinity{
						Required: &v1.NodeSelector{
							NodeSelectorTerms: []v1.NodeSelectorTerm{
								{
									MatchExpressions: []v1.NodeSelectorRequirement{
										{
											Key:      driverTopologyKey,
											Operator: v1.NodeSelectorOpIn,
											Values: []string{
												"bar",
											},
										},
									},
								},
							},
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
			expectErr: true,
		},
		"distributed, delete PV from our host": {
			deploymentNode: "foo",
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
					NodeAffinity: &v1.VolumeNodeAffinity{
						Required: &v1.NodeSelector{
							NodeSelectorTerms: []v1.NodeSelectorTerm{
								{
									MatchExpressions: []v1.NodeSelectorRequirement{
										{
											Key:      driverTopologyKey,
											Operator: v1.NodeSelectorOpIn,
											Values: []string{
												"foo",
											},
										},
									},
								},
							},
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
	}

	for k, tc := range tt {
		t.Run(k, func(t *testing.T) {
			runDeleteTest(t, k, tc)
		})
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

	var clientSetObjects []runtime.Object
	if tc.storageClass != nil {
		clientSetObjects = append(clientSetObjects, tc.storageClass)
	}
	if tc.secrets != nil {
		clientSetObjects = append(clientSetObjects, tc.secrets...)
	}
	clientSet = fakeclientset.NewSimpleClientset(clientSetObjects...)

	informerFactory := informers.NewSharedInformerFactory(clientSet, 0)
	claimInformer := informerFactory.Core().V1().PersistentVolumeClaims()

	var nodeDeployment *NodeDeployment
	if tc.deploymentNode != "" {
		nodeDeployment = &NodeDeployment{
			NodeName:      tc.deploymentNode,
			ClaimInformer: claimInformer,
			NodeInfo: csi.NodeGetInfoResponse{
				NodeId: tc.deploymentNode,
				AccessibleTopology: &csi.Topology{
					Segments: map[string]string{
						driverTopologyKey: tc.deploymentNode,
					},
				},
			},
		}
	}

	if tc.mockDelete {
		if tc.expectedProvisionerSecret != nil {
			if tc.expectedProvisionerSecret.exist {
				controllerServer.EXPECT().DeleteVolume(gomock.Any(), gomock.Any()).
					Return(&csi.DeleteVolumeResponse{}, nil).
					Do(func(ctx context.Context, req *csi.DeleteVolumeRequest) {
						if len(req.Secrets) != len(tc.expectedProvisionerSecret.secrets) {
							t.Errorf("expected DeleteVolumeRequest %#v, got: %#v", tc.expectedProvisionerSecret.secrets, req.Secrets)
						}
						for k, v := range tc.expectedProvisionerSecret.secrets {
							if string(req.Secrets[k]) != v {
								t.Errorf("expected DeleteVolumeRequest %#v, got: %#v", tc.expectedProvisionerSecret.secrets, req.Secrets)
							}
						}
					}).Times(1)
			} else {
				controllerServer.EXPECT().DeleteVolume(gomock.Any(), gomock.Any()).
					Return(&csi.DeleteVolumeResponse{}, nil).
					Do(func(ctx context.Context, req *csi.DeleteVolumeRequest) {
						if len(req.Secrets) > 0 {
							t.Errorf("expected empty DeleteVolumeRequest, got: %#v", req.Secrets)
						}
					}).Times(1)
			}
		} else {
			controllerServer.EXPECT().DeleteVolume(gomock.Any(), gomock.Any()).Return(&csi.DeleteVolumeResponse{}, nil).Times(1)
		}
	}

	pluginCaps, controllerCaps := provisionCapabilities()
	scLister, _, _, _, vaLister, _ := listers(clientSet)
	csiProvisioner := NewCSIProvisioner(clientSet, 5*time.Second, "test-provisioner", "test", 5,
		csiConn.conn, nil, driverName, pluginCaps, controllerCaps, "", false, true, csitrans.New(), scLister, nil, nil, nil, vaLister, nil, false, "", defaultfsType, nodeDeployment, true, false)

	err = csiProvisioner.Delete(context.Background(), tc.persistentVolume)
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
			Parameters:    map[string]string{"fstype": "ext4"},
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
				Resources: v1.VolumeResourceRequirements{
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
		// leave it undefined/nil to maintain the current defaults for test cases
	}

	return provisionRequest
}

// generatePVCForProvisionFromXnsdataSource returns a ProvisionOptions with the requested settings
func generatePVCForProvisionFromXnsdataSource(scName, namespace string, dataSourceRef *v1.TypedObjectReference, requestedBytes int64, volumeMode string) controller.ProvisionOptions {
	deletePolicy := v1.PersistentVolumeReclaimDelete

	provisionRequest := controller.ProvisionOptions{
		StorageClass: &storagev1.StorageClass{
			ReclaimPolicy: &deletePolicy,
			Parameters:    map[string]string{"csi.storage.k8s.io/fstype": "ext4"},
			Provisioner:   driverName,
		},
		PVName: "new-pv-name",
		PVC: &v1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "my-pvc",
				Namespace:   namespace,
				UID:         "testid",
				Annotations: driverNameAnnotation,
			},
			Spec: v1.PersistentVolumeClaimSpec{
				Selector: nil,
				Resources: v1.VolumeResourceRequirements{
					Requests: v1.ResourceList{
						v1.ResourceName(v1.ResourceStorage): *resource.NewQuantity(requestedBytes, resource.BinarySI),
					},
				},
				AccessModes:   []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
				DataSourceRef: dataSourceRef,
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
		// leave it undefined/nil to maintain the current defaults for test cases
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
	coreapiGrp := ""
	xnsNamespace := "ns2"
	dataSourceName := srcName
	dataSourceNamespace := srcNamespace

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
		expectFinalizers     bool                     // while set, expects clone protection finalizers to be set on a PVC
		sourcePVStatusPhase  v1.PersistentVolumePhase // set to change source PV Status.Phase, default "Bound"
		expectErr            bool                     // set to state, test is expected to return errors, default false
		xnsEnabled           bool                     // set to use CrossNamespaceVolumeDataSource feature, default false
		withreferenceGrants  bool                     // set to use ReferenceGrant, default false
		refGrantsrcNamespace string
		referenceGrantFrom   []gatewayv1beta1.ReferenceGrantFrom
		referenceGrantTo     []gatewayv1beta1.ReferenceGrantTo
	}{
		"provision with pvc data source": {
			clonePVName:      pvName,
			volOpts:          generatePVCForProvisionFromPVC(srcNamespace, srcName, fakeSc1, requestedBytes, ""),
			expectFinalizers: true,
			expectedPVSpec: &pvSpec{
				Name:          pvName,
				ReclaimPolicy: v1.PersistentVolumeReclaimDelete,
				AccessModes:   []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
				Capacity: v1.ResourceList{
					v1.ResourceName(v1.ResourceStorage): bytesToQuantity(requestedBytes),
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
			clonePVName:      pvName,
			volOpts:          generatePVCForProvisionFromPVC(srcNamespace, srcName, fakeSc2, requestedBytes, ""),
			expectFinalizers: true,
			expectErr:        false,
		},
		"provision with pvc data source destination too small": {
			clonePVName:          pvName,
			volOpts:              generatePVCForProvisionFromPVC(srcNamespace, srcName, fakeSc1, requestedBytes+1, ""),
			expectFinalizers:     true,
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
			clonePVName:      blockModePVName,
			volOpts:          generatePVCForProvisionFromPVC(srcNamespace, blockModePVName, fakeSc1, requestedBytes, "block"),
			expectFinalizers: true,
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
			clonePVName:      pvName,
			volOpts:          generatePVCForProvisionFromPVC(srcNamespace, srcName, fakeSc1, requestedBytes, ""),
			expectFinalizers: true,
			expectErr:        false,
		},
		"provision filesystem data source is filesystem": {
			clonePVName:      filesystemPVName,
			volOpts:          generatePVCForProvisionFromPVC(srcNamespace, filesystemPVName, fakeSc1, requestedBytes, "filesystem"),
			expectFinalizers: true,
			expectErr:        false,
		},
		"provision filesystem but data source is block": {
			clonePVName: blockModePVName,
			volOpts:     generatePVCForProvisionFromPVC(srcNamespace, blockModePVName, fakeSc1, requestedBytes, "filesystem"),
			expectErr:   true,
		},
		"provision filesystem data source is nil": {
			clonePVName:      pvName,
			volOpts:          generatePVCForProvisionFromPVC(srcNamespace, srcName, fakeSc1, requestedBytes, "filesystem"),
			expectFinalizers: true,
			expectErr:        false,
		},
		"provision with pvc data source when CrossNamespaceVolumeDataSource feature enabled": {
			clonePVName:      pvName,
			volOpts:          generatePVCForProvisionFromPVC(srcNamespace, srcName, fakeSc1, requestedBytes, ""),
			expectFinalizers: true,
			xnsEnabled:       true,
			expectedPVSpec: &pvSpec{
				Name:          pvName,
				ReclaimPolicy: v1.PersistentVolumeReclaimDelete,
				AccessModes:   []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
				Capacity: v1.ResourceList{
					v1.ResourceName(v1.ResourceStorage): bytesToQuantity(requestedBytes),
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
		"provision with pvc data source different storage classes when CrossNamespaceVolumeDataSource feature enabled": {
			clonePVName:      pvName,
			volOpts:          generatePVCForProvisionFromPVC(srcNamespace, srcName, fakeSc2, requestedBytes, ""),
			expectFinalizers: true,
			expectErr:        false,
		},
		"provision filesystem data source is filesystem when CrossNamespaceVolumeDataSource feature enabled": {
			clonePVName:      filesystemPVName,
			volOpts:          generatePVCForProvisionFromPVC(srcNamespace, filesystemPVName, fakeSc1, requestedBytes, "filesystem"),
			expectFinalizers: true,
			expectErr:        false,
		},
		"provision nil mode data source is nil when CrossNamespaceVolumeDataSource feature enabled": {
			clonePVName:      pvName,
			volOpts:          generatePVCForProvisionFromPVC(srcNamespace, srcName, fakeSc1, requestedBytes, ""),
			expectFinalizers: true,
			expectErr:        false,
		},
		"provision with xns PersitentVolumeClaim data source with refgrant when CrossNamespaceVolumeDataSource feature enabled": {
			clonePVName: pvName,
			volOpts: generatePVCForProvisionFromXnsdataSource(fakeSc1, xnsNamespace,
				&v1.TypedObjectReference{
					APIGroup:  &coreapiGrp,
					Kind:      "PersistentVolumeClaim",
					Name:      dataSourceName,
					Namespace: &dataSourceNamespace,
				},
				requestedBytes, ""),
			expectFinalizers:     true,
			xnsEnabled:           true,
			withreferenceGrants:  true,
			refGrantsrcNamespace: dataSourceNamespace,
			referenceGrantFrom: []gatewayv1beta1.ReferenceGrantFrom{
				{
					Group:     gatewayv1beta1.Group(coreapiGrp),
					Kind:      gatewayv1beta1.Kind("PersistentVolumeClaim"),
					Namespace: gatewayv1beta1.Namespace(xnsNamespace),
				},
			},
			referenceGrantTo: []gatewayv1beta1.ReferenceGrantTo{
				{
					Group: gatewayv1beta1.Group(coreapiGrp),
					Kind:  gatewayv1beta1.Kind("PersistentVolumeClaim"),
				},
			},
			expectedPVSpec: &pvSpec{
				Name:          pvName,
				ReclaimPolicy: v1.PersistentVolumeReclaimDelete,
				AccessModes:   []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
				Capacity: v1.ResourceList{
					v1.ResourceName(v1.ResourceStorage): bytesToQuantity(requestedBytes),
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
		"provision with xns PersitentVolumeClaim data source with refgrant of specify toName when CrossNamespaceVolumeDataSource feature enabled": {
			clonePVName: pvName,
			volOpts: generatePVCForProvisionFromXnsdataSource(fakeSc1, xnsNamespace,
				&v1.TypedObjectReference{
					APIGroup:  &coreapiGrp,
					Kind:      "PersistentVolumeClaim",
					Name:      dataSourceName,
					Namespace: &dataSourceNamespace,
				},
				requestedBytes, ""),
			expectFinalizers:     true,
			xnsEnabled:           true,
			withreferenceGrants:  true,
			refGrantsrcNamespace: dataSourceNamespace,
			referenceGrantFrom: []gatewayv1beta1.ReferenceGrantFrom{
				{
					Group:     gatewayv1beta1.Group(coreapiGrp),
					Kind:      gatewayv1beta1.Kind("PersistentVolumeClaim"),
					Namespace: gatewayv1beta1.Namespace(xnsNamespace),
				},
			},
			referenceGrantTo: []gatewayv1beta1.ReferenceGrantTo{
				{
					Group: gatewayv1beta1.Group(coreapiGrp),
					Kind:  gatewayv1beta1.Kind("PersistentVolumeClaim"),
					Name:  ObjectNamePtr(dataSourceName),
				},
			},
			expectedPVSpec: &pvSpec{
				Name:          pvName,
				ReclaimPolicy: v1.PersistentVolumeReclaimDelete,
				AccessModes:   []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
				Capacity: v1.ResourceList{
					v1.ResourceName(v1.ResourceStorage): bytesToQuantity(requestedBytes),
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
		"provision with same ns PersitentVolumeClaim data source without refgrant when CrossNamespaceVolumeDataSource feature enabled": {
			clonePVName: pvName,
			volOpts: generatePVCForProvisionFromXnsdataSource(fakeSc1, dataSourceNamespace,
				&v1.TypedObjectReference{
					APIGroup:  &coreapiGrp,
					Kind:      "PersistentVolumeClaim",
					Name:      dataSourceName,
					Namespace: &dataSourceNamespace,
				},
				requestedBytes, ""),
			expectFinalizers: true,
			xnsEnabled:       true,
			expectedPVSpec: &pvSpec{
				Name:          pvName,
				ReclaimPolicy: v1.PersistentVolumeReclaimDelete,
				AccessModes:   []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
				Capacity: v1.ResourceList{
					v1.ResourceName(v1.ResourceStorage): bytesToQuantity(requestedBytes),
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
		"provision with PersitentVolumeClaim data source without ns without refgrant when CrossNamespaceVolumeDataSource feature enabled": {
			clonePVName: pvName,
			volOpts: generatePVCForProvisionFromXnsdataSource(fakeSc1, dataSourceNamespace,
				&v1.TypedObjectReference{
					APIGroup: &coreapiGrp,
					Kind:     "PersistentVolumeClaim",
					Name:     dataSourceName,
				},
				requestedBytes, ""),
			expectFinalizers: true,
			xnsEnabled:       true,
			expectedPVSpec: &pvSpec{
				Name:          pvName,
				ReclaimPolicy: v1.PersistentVolumeReclaimDelete,
				AccessModes:   []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
				Capacity: v1.ResourceList{
					v1.ResourceName(v1.ResourceStorage): bytesToQuantity(requestedBytes),
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
		"provision PersitentVolumeClaim data source of nil ns without refgrant when CrossNamespaceVolumeDataSource feature enabled": {
			clonePVName: pvName,
			volOpts: generatePVCForProvisionFromXnsdataSource(fakeSc1, dataSourceNamespace,
				&v1.TypedObjectReference{
					APIGroup:  &coreapiGrp,
					Kind:      "PersistentVolumeClaim",
					Name:      dataSourceName,
					Namespace: nil,
				},
				requestedBytes, ""),
			expectFinalizers: true,
			xnsEnabled:       true,
			expectedPVSpec: &pvSpec{
				Name:          pvName,
				ReclaimPolicy: v1.PersistentVolumeReclaimDelete,
				AccessModes:   []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
				Capacity: v1.ResourceList{
					v1.ResourceName(v1.ResourceStorage): bytesToQuantity(requestedBytes),
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
		"provision with xns PersitentVolumeClaim data source of nil apiGroup with refgrant when CrossNamespaceVolumeDataSource feature enabled": {
			clonePVName: pvName,
			volOpts: generatePVCForProvisionFromXnsdataSource(fakeSc1, xnsNamespace,
				&v1.TypedObjectReference{
					APIGroup:  nil,
					Kind:      "PersistentVolumeClaim",
					Name:      dataSourceName,
					Namespace: &dataSourceNamespace,
				},
				requestedBytes, ""),
			expectFinalizers:     true,
			xnsEnabled:           true,
			withreferenceGrants:  true,
			refGrantsrcNamespace: dataSourceNamespace,
			referenceGrantFrom: []gatewayv1beta1.ReferenceGrantFrom{
				{
					Group:     gatewayv1beta1.Group(coreapiGrp),
					Kind:      gatewayv1beta1.Kind("PersistentVolumeClaim"),
					Namespace: gatewayv1beta1.Namespace(xnsNamespace),
				},
			},
			referenceGrantTo: []gatewayv1beta1.ReferenceGrantTo{
				{
					Group: gatewayv1beta1.Group(coreapiGrp),
					Kind:  gatewayv1beta1.Kind("PersistentVolumeClaim"),
				},
			},
			expectedPVSpec: &pvSpec{
				Name:          pvName,
				ReclaimPolicy: v1.PersistentVolumeReclaimDelete,
				AccessModes:   []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
				Capacity: v1.ResourceList{
					v1.ResourceName(v1.ResourceStorage): bytesToQuantity(requestedBytes),
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
		"provision with xns PersitentVolumeClaim data source without apiGroup with refgrant when CrossNamespaceVolumeDataSource feature enabled": {
			clonePVName: pvName,
			volOpts: generatePVCForProvisionFromXnsdataSource(fakeSc1, xnsNamespace,
				&v1.TypedObjectReference{
					Kind:      "PersistentVolumeClaim",
					Name:      dataSourceName,
					Namespace: &dataSourceNamespace,
				},
				requestedBytes, ""),
			expectFinalizers:     true,
			xnsEnabled:           true,
			withreferenceGrants:  true,
			refGrantsrcNamespace: dataSourceNamespace,
			referenceGrantFrom: []gatewayv1beta1.ReferenceGrantFrom{
				{
					Group:     gatewayv1beta1.Group(coreapiGrp),
					Kind:      gatewayv1beta1.Kind("PersistentVolumeClaim"),
					Namespace: gatewayv1beta1.Namespace(xnsNamespace),
				},
			},
			referenceGrantTo: []gatewayv1beta1.ReferenceGrantTo{
				{
					Group: gatewayv1beta1.Group(coreapiGrp),
					Kind:  gatewayv1beta1.Kind("PersistentVolumeClaim"),
				},
			},
			expectedPVSpec: &pvSpec{
				Name:          pvName,
				ReclaimPolicy: v1.PersistentVolumeReclaimDelete,
				AccessModes:   []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
				Capacity: v1.ResourceList{
					v1.ResourceName(v1.ResourceStorage): bytesToQuantity(requestedBytes),
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
		"fail provision xns PersistentVolumeClaim data source of nil apiGroup and wrong Kind with refgrant when CrossNamespaceVolumeDataSource feature enabled": {
			clonePVName: pvName,
			volOpts: generatePVCForProvisionFromXnsdataSource(fakeSc1, xnsNamespace,
				&v1.TypedObjectReference{
					APIGroup:  nil,
					Kind:      "WrongKind",
					Name:      dataSourceName,
					Namespace: &dataSourceNamespace,
				},
				requestedBytes, ""),
			expectErr:            true,
			xnsEnabled:           true,
			withreferenceGrants:  true,
			refGrantsrcNamespace: dataSourceNamespace,
			referenceGrantFrom: []gatewayv1beta1.ReferenceGrantFrom{
				{
					Group:     gatewayv1beta1.Group(coreapiGrp),
					Kind:      gatewayv1beta1.Kind("PersistentVolumeClaim"),
					Namespace: gatewayv1beta1.Namespace(xnsNamespace),
				},
			},
			referenceGrantTo: []gatewayv1beta1.ReferenceGrantTo{
				{
					Group: gatewayv1beta1.Group(coreapiGrp),
					Kind:  gatewayv1beta1.Kind("PersistentVolumeClaim"),
				},
			},
		},
		"fail provision with xns PersistentVolumeClaim data source with refgrant of wrong create namespace when CrossNamespaceVolumeDataSource feature enabled": {
			clonePVName: pvName,
			volOpts: generatePVCForProvisionFromXnsdataSource(fakeSc1, xnsNamespace,
				&v1.TypedObjectReference{
					APIGroup:  &coreapiGrp,
					Kind:      "PersistentVolumeClaim",
					Name:      dataSourceName,
					Namespace: &dataSourceNamespace,
				},
				requestedBytes, ""),
			expectErr:            true,
			xnsEnabled:           true,
			refGrantsrcNamespace: "wrong-reference-namespace",
			referenceGrantFrom: []gatewayv1beta1.ReferenceGrantFrom{
				{
					Group:     gatewayv1beta1.Group(coreapiGrp),
					Kind:      gatewayv1beta1.Kind("PersistentVolumeClaim"),
					Namespace: gatewayv1beta1.Namespace(xnsNamespace),
				},
			},
			referenceGrantTo: []gatewayv1beta1.ReferenceGrantTo{
				{
					Group: gatewayv1beta1.Group(coreapiGrp),
					Kind:  gatewayv1beta1.Kind("PersistentVolumeClaim"),
				},
			},
		},
		"fail provision with xns PersistentVolumeClaim data source with refgrant of wrong fromGroup when CrossNamespaceVolumeDataSource feature enabled": {
			clonePVName: pvName,
			volOpts: generatePVCForProvisionFromXnsdataSource(fakeSc1, xnsNamespace,
				&v1.TypedObjectReference{
					APIGroup:  &coreapiGrp,
					Kind:      "PersistentVolumeClaim",
					Name:      dataSourceName,
					Namespace: &dataSourceNamespace,
				},
				requestedBytes, ""),
			expectErr:            true,
			xnsEnabled:           true,
			refGrantsrcNamespace: dataSourceNamespace,
			referenceGrantFrom: []gatewayv1beta1.ReferenceGrantFrom{
				{
					Group:     gatewayv1beta1.Group("wrong.fromGroup"),
					Kind:      gatewayv1beta1.Kind("PersistentVolumeClaim"),
					Namespace: gatewayv1beta1.Namespace(xnsNamespace),
				},
			},
			referenceGrantTo: []gatewayv1beta1.ReferenceGrantTo{
				{
					Group: gatewayv1beta1.Group(coreapiGrp),
					Kind:  gatewayv1beta1.Kind("PersistentVolumeClaim"),
				},
			},
		},
		"fail provision with xns PersistentVolumeClaim data source with refgrant of wrong fromKind when CrossNamespaceVolumeDataSource feature enabled": {
			clonePVName: pvName,
			volOpts: generatePVCForProvisionFromXnsdataSource(fakeSc1, xnsNamespace,
				&v1.TypedObjectReference{
					APIGroup:  &coreapiGrp,
					Kind:      "PersistentVolumeClaim",
					Name:      dataSourceName,
					Namespace: &dataSourceNamespace,
				},
				requestedBytes, ""),
			expectErr:            true,
			xnsEnabled:           true,
			refGrantsrcNamespace: dataSourceNamespace,
			referenceGrantFrom: []gatewayv1beta1.ReferenceGrantFrom{
				{
					Group:     gatewayv1beta1.Group(coreapiGrp),
					Kind:      gatewayv1beta1.Kind("WrongfromKind"),
					Namespace: gatewayv1beta1.Namespace(xnsNamespace),
				},
			},
			referenceGrantTo: []gatewayv1beta1.ReferenceGrantTo{
				{
					Group: gatewayv1beta1.Group(coreapiGrp),
					Kind:  gatewayv1beta1.Kind("PersistentVolumeClaim"),
				},
			},
		},
		"fail provision with xns PersistentVolumeClaim data source with refgrant of wrong fromNamespace when CrossNamespaceVolumeDataSource feature enabled": {
			clonePVName: pvName,
			volOpts: generatePVCForProvisionFromXnsdataSource(fakeSc1, xnsNamespace,
				&v1.TypedObjectReference{
					APIGroup:  &coreapiGrp,
					Kind:      "PersistentVolumeClaim",
					Name:      dataSourceName,
					Namespace: &dataSourceNamespace,
				},
				requestedBytes, ""),
			expectErr:            true,
			xnsEnabled:           true,
			refGrantsrcNamespace: dataSourceNamespace,
			referenceGrantFrom: []gatewayv1beta1.ReferenceGrantFrom{
				{
					Group:     gatewayv1beta1.Group(coreapiGrp),
					Kind:      gatewayv1beta1.Kind("PersistentVolumeClaim"),
					Namespace: gatewayv1beta1.Namespace("wrong-namespace"),
				},
			},
			referenceGrantTo: []gatewayv1beta1.ReferenceGrantTo{
				{
					Group: gatewayv1beta1.Group(coreapiGrp),
					Kind:  gatewayv1beta1.Kind("PersistentVolumeClaim"),
				},
			},
		},
		"fail provision with xns PersistentVolumeClaim data source with refgrant of wrong toGroup when CrossNamespaceVolumeDataSource feature enabled": {
			clonePVName: pvName,
			volOpts: generatePVCForProvisionFromXnsdataSource(fakeSc1, xnsNamespace,
				&v1.TypedObjectReference{
					APIGroup:  &coreapiGrp,
					Kind:      "PersistentVolumeClaim",
					Name:      dataSourceName,
					Namespace: &dataSourceNamespace,
				},
				requestedBytes, ""),
			expectErr:            true,
			xnsEnabled:           true,
			refGrantsrcNamespace: dataSourceNamespace,
			referenceGrantFrom: []gatewayv1beta1.ReferenceGrantFrom{
				{
					Group:     gatewayv1beta1.Group(coreapiGrp),
					Kind:      gatewayv1beta1.Kind("PersistentVolumeClaim"),
					Namespace: gatewayv1beta1.Namespace(xnsNamespace),
				},
			},
			referenceGrantTo: []gatewayv1beta1.ReferenceGrantTo{
				{
					Group: gatewayv1beta1.Group("wrong.toGroup"),
					Kind:  gatewayv1beta1.Kind("PersistentVolumeClaim"),
				},
			},
		},
		"fail provision with xns PersistentVolumeClaim data source with refgrant of wrong toKind when CrossNamespaceVolumeDataSource feature enabled": {
			clonePVName: pvName,
			volOpts: generatePVCForProvisionFromXnsdataSource(fakeSc1, xnsNamespace,
				&v1.TypedObjectReference{
					APIGroup:  &coreapiGrp,
					Kind:      "PersistentVolumeClaim",
					Name:      dataSourceName,
					Namespace: &dataSourceNamespace,
				},
				requestedBytes, ""),
			expectErr:            true,
			xnsEnabled:           true,
			refGrantsrcNamespace: dataSourceNamespace,
			referenceGrantFrom: []gatewayv1beta1.ReferenceGrantFrom{
				{
					Group:     gatewayv1beta1.Group(coreapiGrp),
					Kind:      gatewayv1beta1.Kind("PersistentVolumeClaim"),
					Namespace: gatewayv1beta1.Namespace(xnsNamespace),
				},
			},
			referenceGrantTo: []gatewayv1beta1.ReferenceGrantTo{
				{
					Group: gatewayv1beta1.Group(coreapiGrp),
					Kind:  gatewayv1beta1.Kind("WrongtoKind"),
				},
			},
		},
		"fail provision with xns PersistentVolumeClaim data source with refgrant of wrong toName when CrossNamespaceVolumeDataSource feature enabled": {
			clonePVName: pvName,
			volOpts: generatePVCForProvisionFromXnsdataSource(fakeSc1, xnsNamespace,
				&v1.TypedObjectReference{
					APIGroup:  &coreapiGrp,
					Kind:      "PersistentVolumeClaim",
					Name:      dataSourceName,
					Namespace: &dataSourceNamespace,
				},
				requestedBytes, ""),
			expectErr:            true,
			xnsEnabled:           true,
			refGrantsrcNamespace: dataSourceNamespace,
			referenceGrantFrom: []gatewayv1beta1.ReferenceGrantFrom{
				{
					Group:     gatewayv1beta1.Group(coreapiGrp),
					Kind:      gatewayv1beta1.Kind("PersistentVolumeClaim"),
					Namespace: gatewayv1beta1.Namespace(xnsNamespace),
				},
			},
			referenceGrantTo: []gatewayv1beta1.ReferenceGrantTo{
				{
					Group: gatewayv1beta1.Group(coreapiGrp),
					Kind:  gatewayv1beta1.Kind("PersistentVolumeClaim"),
					Name:  ObjectNamePtr("wrong-toName"),
				},
			},
		},
		"fail provision with xns PersistentVolumeClaim data source with refgrant referenceGrantFrom and referenceGrantTo empty when CrossNamespaceVolumeDataSource feature enabled": {
			clonePVName: pvName,
			volOpts: generatePVCForProvisionFromXnsdataSource(fakeSc1, xnsNamespace,
				&v1.TypedObjectReference{
					APIGroup:  &coreapiGrp,
					Kind:      "PersistentVolumeClaim",
					Name:      dataSourceName,
					Namespace: &dataSourceNamespace,
				},
				requestedBytes, ""),
			expectErr:            true,
			xnsEnabled:           true,
			refGrantsrcNamespace: dataSourceNamespace,
			referenceGrantFrom:   []gatewayv1beta1.ReferenceGrantFrom{},
			referenceGrantTo:     []gatewayv1beta1.ReferenceGrantTo{},
		},
		"fail provision with xns PersistentVolumeClaim PersitentVolumeClaim data source without refgrant when CrossNamespaceVolumeDataSource feature enabled": {
			clonePVName: pvName,
			volOpts: generatePVCForProvisionFromXnsdataSource(fakeSc1, xnsNamespace,
				&v1.TypedObjectReference{
					APIGroup:  &coreapiGrp,
					Kind:      "PersistentVolumeClaim",
					Name:      srcName,
					Namespace: &dataSourceNamespace,
				},
				requestedBytes, ""),
			expectErr:  true,
			xnsEnabled: true,
		},
		"fail provision with xns PersistentVolumeClaim data source with refgrant when CrossNamespaceVolumeDataSource feature disabled": {
			clonePVName: pvName,
			volOpts: generatePVCForProvisionFromXnsdataSource(fakeSc1, xnsNamespace,
				&v1.TypedObjectReference{
					APIGroup:  &coreapiGrp,
					Kind:      "PersistentVolumeClaim",
					Name:      dataSourceName,
					Namespace: &dataSourceNamespace,
				},
				requestedBytes, ""),
			expectErr:            true,
			refGrantsrcNamespace: dataSourceNamespace,
			referenceGrantFrom: []gatewayv1beta1.ReferenceGrantFrom{
				{
					Group:     gatewayv1beta1.Group(coreapiGrp),
					Kind:      gatewayv1beta1.Kind("PersistentVolumeClaim"),
					Namespace: gatewayv1beta1.Namespace(xnsNamespace),
				},
			},
			referenceGrantTo: []gatewayv1beta1.ReferenceGrantTo{
				{
					Group: gatewayv1beta1.Group(coreapiGrp),
					Kind:  gatewayv1beta1.Kind("PersistentVolumeClaim"),
				},
			},
		},
	}

	for k, tc := range testcases {
		tc := tc
		t.Run(k, func(t *testing.T) {
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

			var refGrantLister referenceGrantv1beta1.ReferenceGrantLister
			var stopChan chan struct{}
			var gatewayClient *fakegateway.Clientset
			if tc.withreferenceGrants {
				referenceGrant := generateReferenceGrant(tc.refGrantsrcNamespace, tc.referenceGrantFrom, tc.referenceGrantTo)
				gatewayClient = fakegateway.NewSimpleClientset(referenceGrant)
			} else {
				gatewayClient = fakegateway.NewSimpleClientset()
			}
			if tc.xnsEnabled {
				gatewayFactory := gatewayInformers.NewSharedInformerFactory(gatewayClient, ResyncPeriodOfReferenceGrantInformer)
				referenceGrants := gatewayFactory.Gateway().V1beta1().ReferenceGrants()
				refGrantLister = referenceGrants.Lister()

				stopChan := make(chan struct{})
				gatewayFactory.Start(stopChan)
				gatewayFactory.WaitForCacheSync(stopChan)
			}
			defer func() {
				if stopChan != nil {
					close(stopChan)
				}
			}()
			defer utilfeaturetesting.SetFeatureGateDuringTest(t, utilfeature.DefaultFeatureGate, features.CrossNamespaceVolumeDataSource, tc.xnsEnabled)()

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
			var volumeSource csi.VolumeContentSource_Volume
			if !tc.expectErr {
				if tc.xnsEnabled && tc.volOpts.PVC.Spec.DataSourceRef != nil {
					volumeSource = csi.VolumeContentSource_Volume{
						Volume: &csi.VolumeContentSource_VolumeSource{
							VolumeId: tc.volOpts.PVC.Spec.DataSourceRef.Name,
						},
					}
				} else if tc.volOpts.PVC.Spec.DataSource != nil {
					volumeSource = csi.VolumeContentSource_Volume{
						Volume: &csi.VolumeContentSource_VolumeSource{
							VolumeId: tc.volOpts.PVC.Spec.DataSource.Name,
						},
					}
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

			_, _, _, claimLister, _, _ := listers(clientSet)

			// Phase: execute the test
			csiProvisioner := NewCSIProvisioner(clientSet, 5*time.Second, "test-provisioner", "test", 5, csiConn.conn,
				nil, driverName, pluginCaps, controllerCaps, "", false, true, csitrans.New(), nil, nil, nil, claimLister, nil, refGrantLister, false, "", defaultfsType, nil, true, false)

			pv, _, err = csiProvisioner.Provision(context.Background(), tc.volOpts)
			if tc.expectErr && err == nil {
				t.Errorf("test %q: Expected error, got none", k)
			}

			if tc.volOpts.PVC.Spec.DataSourceRef != nil || tc.volOpts.PVC.Spec.DataSource != nil {
				var claim *v1.PersistentVolumeClaim
				if tc.volOpts.PVC.Spec.DataSourceRef != nil {
					claim, _ = claimLister.PersistentVolumeClaims(tc.volOpts.PVC.Namespace).Get(tc.volOpts.PVC.Spec.DataSourceRef.Name)
				} else if tc.volOpts.PVC.Spec.DataSource != nil {
					claim, _ = claimLister.PersistentVolumeClaims(tc.volOpts.PVC.Namespace).Get(tc.volOpts.PVC.Spec.DataSource.Name)
				}
				if claim != nil {
					set := checkFinalizer(claim, pvcCloneFinalizer)
					if tc.expectFinalizers && !set {
						t.Errorf("Claim %s does not have clone protection finalizer set", claim.Name)
					} else if !tc.expectFinalizers && set {
						t.Errorf("Claim %s should not have clone protection finalizer set", claim.Name)
					}
				}
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
	inTreePluginName := "in-tree-plugin"

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
			annotation:        map[string]string{annBetaStorageProvisioner: driverName},
			expectTranslation: true,
		},
		{
			name:              "provision with migration on with GA annStorageProvisioner annotation",
			scProvisioner:     inTreePluginName,
			annotation:        map[string]string{annStorageProvisioner: driverName},
			expectTranslation: true,
		},
		{
			name:              "provision without migration for native CSI",
			scProvisioner:     driverName,
			annotation:        map[string]string{annBetaStorageProvisioner: driverName},
			expectTranslation: false,
		},
		{
			name:              "provision without migration for native CSI with GA annStorageProvisioner annotation",
			scProvisioner:     driverName,
			annotation:        map[string]string{annStorageProvisioner: driverName},
			expectTranslation: false,
		},
		{
			name:              "provision with migration for migrated-to CSI",
			scProvisioner:     inTreePluginName,
			annotation:        map[string]string{annBetaStorageProvisioner: inTreePluginName, annMigratedTo: driverName},
			expectTranslation: true,
		},
		{
			name:          "provision with migration-to some random driver",
			scProvisioner: inTreePluginName,
			annotation:    map[string]string{annBetaStorageProvisioner: inTreePluginName, annMigratedTo: "foo"},
			expectErr:     true,
		},
		{
			name:          "provision with migration-to some random driver with random storageProvisioner",
			scProvisioner: inTreePluginName,
			annotation:    map[string]string{annBetaStorageProvisioner: "foo", annMigratedTo: "foo"},
			expectErr:     true,
		},
		{
			name:              "provision with migration for migrated-to CSI with CSI Provisioner",
			scProvisioner:     inTreePluginName,
			annotation:        map[string]string{annBetaStorageProvisioner: driverName, annMigratedTo: driverName},
			expectTranslation: true,
		},
		{
			name:          "ignore in-tree PVC when provisioned by in-tree",
			scProvisioner: inTreePluginName,
			annotation:    map[string]string{annBetaStorageProvisioner: inTreePluginName},
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
				inTreePluginName, false, true, mockTranslator, nil, nil, nil, nil, nil, nil, false, "", defaultfsType, nil, true, false)

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

			pv, state, err := csiProvisioner.Provision(context.Background(), volOpts)
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
			Resources: v1.VolumeResourceRequirements{
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
		sc                *storagev1.StorageClass
		expectTranslation bool
		expectErr         bool
	}{
		{
			name: "normal migration",
			// The PV could be any random in-tree plugin - it doesn't really
			// matter here. We only care that the translation is called and the
			// function will work after some CSI volume is created
			pv: &v1.PersistentVolume{},
			sc: &storagev1.StorageClass{
				ObjectMeta: metav1.ObjectMeta{
					Name: inTreeStorageClassName,
				},
				Provisioner: inTreePluginName,
				Parameters:  map[string]string{},
			},
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

			var clientSet *fakeclientset.Clientset
			if tc.sc != nil {
				clientSet = fakeclientset.NewSimpleClientset(tc.sc)
			} else {
				clientSet = fakeclientset.NewSimpleClientset()
			}

			pluginCaps, controllerCaps := provisionCapabilities()
			scLister, _, _, _, vaLister, stopCh := listers(clientSet)
			defer close(stopCh)
			csiProvisioner := NewCSIProvisioner(clientSet, 5*time.Second, "test-provisioner",
				"test", 5, csiConn.conn, nil, driverName, pluginCaps, controllerCaps, inTreePluginName,
				false, true, mockTranslator, scLister, nil, nil, nil, vaLister, nil, false, "", defaultfsType, nil, true, false)

			// Set mock return values (AnyTimes to avoid overfitting on implementation details)
			mockTranslator.EXPECT().IsPVMigratable(gomock.Any()).Return(tc.expectTranslation).AnyTimes()
			if tc.expectTranslation {
				// In the translation case we translate to CSI we return a fake
				// PV with a different handle
				mockTranslator.EXPECT().TranslateInTreePVToCSI(gomock.Any()).Return(createFakeCSIPV(translatedHandle), nil).AnyTimes()
				mockTranslator.EXPECT().TranslateInTreeStorageClassToCSI(gomock.Any(), gomock.Any()).Return(tc.sc, nil).AnyTimes()
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
			err = csiProvisioner.Delete(context.Background(), tc.pv)
			if tc.expectErr && err == nil {
				t.Error("Got no error, expected one")
			}
			if !tc.expectErr && err != nil {
				t.Errorf("Got error: %v, expected none", err)
			}
		})
	}
}

func TestMarkAsMigrated(t *testing.T) {
	t.Run("context has the migrated label for the migratable plugins", func(t *testing.T) {
		ctx := context.Background()
		migratedCtx := markAsMigrated(ctx, true)
		additionalInfo := migratedCtx.Value(connection.AdditionalInfoKey)
		if additionalInfo == nil {
			t.Errorf("test: %s, no migrated label found in the context", t.Name())
		}
		additionalInfoVal := additionalInfo.(connection.AdditionalInfo)
		migrated := additionalInfoVal.Migrated

		if migrated != "true" {
			t.Errorf("test: %s, expected: %v, got: %v", t.Name(), "true", migrated)
		}
	})
}

func createFakeCSIPV(volumeHandle string) *v1.PersistentVolume {
	return &v1.PersistentVolume{
		Spec: v1.PersistentVolumeSpec{
			PersistentVolumeSource: v1.PersistentVolumeSource{
				CSI: &v1.CSIPersistentVolumeSource{
					VolumeHandle: volumeHandle,
				},
			},
			ClaimRef: &v1.ObjectReference{
				Name:      "test-pvc",
				Namespace: "test-namespace",
			},
			StorageClassName: inTreeStorageClassName,
		},
	}
}
