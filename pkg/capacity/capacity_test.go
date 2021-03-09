/*
Copyright 2020 The Kubernetes Authors.

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

package capacity

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/kubernetes-csi/external-provisioner/pkg/capacity/topology"
	"google.golang.org/grpc"
	storagev1 "k8s.io/api/storage/v1"
	storagev1alpha1 "k8s.io/api/storage/v1alpha1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	krand "k8s.io/apimachinery/pkg/util/rand"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/informers"
	fakeclientset "k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/component-base/metrics"
	"k8s.io/component-base/metrics/testutil"
	"k8s.io/klog/v2"
)

func init() {
	klog.InitFlags(nil)
}

const (
	driverName     = "test-driver"
	ownerNamespace = "testns"
	csiscRev       = "CSISC-REV-"
)

var (
	yes          = true
	defaultOwner = metav1.OwnerReference{
		APIVersion: "apps/v1",
		Kind:       "statefulset",
		Name:       "test-driver",
		UID:        "309cd460-2d62-4f40-bbcf-b7765aac5a6d",
		Controller: &yes,
	}
	noOwner    = metav1.OwnerReference{}
	otherOwner = metav1.OwnerReference{
		APIVersion: "apps/v1",
		Kind:       "statefulset",
		Name:       "other-test-driver",
		UID:        "11111111-2d62-4f40-bbcf-b7765aac5a6d",
		Controller: &yes,
	}

	layer0 = topology.Segment{
		{Key: "layer0", Value: "foo"},
	}
	layer0other = topology.Segment{
		{Key: "layer0", Value: "bar"},
	}
	mb = resource.MustParse("1Mi")
)

type objects struct {
	goal, current, obsolete int64
}

func (o objects) verify(m metrics.Gatherer) error {
	if err := testutil.GatherAndCompare(m, bytes.NewBufferString(
		fmt.Sprintf(`# HELP csistoragecapacities_desired_goal [ALPHA] Number of CSIStorageCapacity objects that are supposed to be managed automatically.
# TYPE csistoragecapacities_desired_goal gauge
csistoragecapacities_desired_goal %d
# HELP csistoragecapacities_desired_current [ALPHA] Number of CSIStorageCapacity objects that exist and are supposed to be managed automatically.
# TYPE csistoragecapacities_desired_current gauge
csistoragecapacities_desired_current %d
# HELP csistoragecapacities_obsolete [ALPHA] Number of CSIStorageCapacity objects that exist and will be deleted automatically. Objects that exist and may need an update are not considered obsolete and therefore not included in this value.
# TYPE csistoragecapacities_obsolete gauge
csistoragecapacities_obsolete %d
`, o.goal, o.current, o.obsolete))); err != nil {
		return fmt.Errorf("expected goal/current/obsolete object numbers %d/%d/%d: %v",
			o.goal, o.current, o.obsolete,
			err,
		)
	}
	return nil
}

// TestCapacityController checks that the controller handles the initial state and
// several different changes at runtime correctly.
func TestCapacityController(t *testing.T) {
	testcases := map[string]struct {
		immediateBinding   bool
		topology           *topology.Mock
		storage            mockCapacity
		initialSCs         []testSC
		initialCapacities  []testCapacity
		expectedCapacities []testCapacity
		modify             func(ctx context.Context, clientSet *fakeclientset.Clientset, expected []testCapacity) (modifiedExpected []testCapacity, err error)
		capacityChange     func(ctx context.Context, storage *mockCapacity, expected []testCapacity) (modifiedExpected []testCapacity)
		topologyChange     func(ctx context.Context, topology *topology.Mock, expected []testCapacity) (modifiedExpected []testCapacity)

		expectedObjectsPrepared objects
		expectedTotalProcessed  int64
	}{
		"empty": {
			expectedCapacities: []testCapacity{},
		},
		"one segment": {
			topology:           topology.NewMock(&layer0),
			expectedCapacities: []testCapacity{},
		},
		"one class": {
			initialSCs: []testSC{
				{
					name:       "fast-sc",
					driverName: driverName,
				},
			},
			expectedCapacities: []testCapacity{},
		},
		"one capacity object": {
			topology: topology.NewMock(&layer0),
			storage: mockCapacity{
				capacity: map[string]interface{}{
					// This matches layer0.
					"foo": "1Gi",
				},
			},
			initialSCs: []testSC{
				{
					name:       "other-sc",
					driverName: driverName,
				},
			},
			expectedCapacities: []testCapacity{
				{
					segment:          layer0,
					storageClassName: "other-sc",
					quantity:         "1Gi",
				},
			},
			expectedObjectsPrepared: objects{
				goal: 1,
			},
			expectedTotalProcessed: 1,
		},
		"ignore SC with immediate binding": {
			topology: topology.NewMock(&layer0),
			storage: mockCapacity{
				capacity: map[string]interface{}{
					// This matches layer0.
					"foo": "1Gi",
				},
			},
			initialSCs: []testSC{
				{
					name:             "other-sc",
					driverName:       driverName,
					immediateBinding: true,
				},
			},
		},
		"support SC with immediate binding": {
			immediateBinding: true,
			topology:         topology.NewMock(&layer0),
			storage: mockCapacity{
				capacity: map[string]interface{}{
					// This matches layer0.
					"foo": "1Gi",
				},
			},
			initialSCs: []testSC{
				{
					name:             "other-sc",
					driverName:       driverName,
					immediateBinding: true,
				},
			},
			expectedCapacities: []testCapacity{
				{
					segment:          layer0,
					storageClassName: "other-sc",
					quantity:         "1Gi",
				},
			},
			expectedObjectsPrepared: objects{
				goal: 1,
			},
			expectedTotalProcessed: 1,
		},
		"reuse one capacity object, no changes": {
			topology: topology.NewMock(&layer0),
			storage: mockCapacity{
				capacity: map[string]interface{}{
					// This matches layer0.
					"foo": "1Gi",
				},
			},
			initialSCs: []testSC{
				{
					name:       "other-sc",
					driverName: driverName,
				},
			},
			initialCapacities: []testCapacity{
				{
					uid:              "test-capacity-1",
					segment:          layer0,
					storageClassName: "other-sc",
					quantity:         "1Gi",
				},
			},
			expectedCapacities: []testCapacity{
				{
					uid:              "test-capacity-1",
					resourceVersion:  csiscRev + "0",
					segment:          layer0,
					storageClassName: "other-sc",
					quantity:         "1Gi",
				},
			},
			expectedObjectsPrepared: objects{
				goal:    1,
				current: 1,
			},
			expectedTotalProcessed: 1,
		},
		"reuse one capacity object, update capacity": {
			topology: topology.NewMock(&layer0),
			storage: mockCapacity{
				capacity: map[string]interface{}{
					// This matches layer0.
					"foo": "2Gi",
				},
			},
			initialSCs: []testSC{
				{
					name:       "other-sc",
					driverName: driverName,
				},
			},
			initialCapacities: []testCapacity{
				{
					uid:              "test-capacity-1",
					segment:          layer0,
					storageClassName: "other-sc",
					quantity:         "1Gi",
				},
			},
			expectedCapacities: []testCapacity{
				{
					uid:              "test-capacity-1",
					resourceVersion:  csiscRev + "1",
					segment:          layer0,
					storageClassName: "other-sc",
					quantity:         "2Gi",
				},
			},
			expectedObjectsPrepared: objects{
				goal:    1,
				current: 1,
			},
			expectedTotalProcessed: 1,
		},
		"obsolete object, missing SC": {
			topology: topology.NewMock(&layer0),
			storage: mockCapacity{
				capacity: map[string]interface{}{
					// This matches layer0.
					"foo": "1Gi",
				},
			},
			initialCapacities: []testCapacity{
				{
					segment:          layer0,
					storageClassName: "other-sc",
					quantity:         "1Gi",
				},
			},
			expectedCapacities: []testCapacity{},
			expectedObjectsPrepared: objects{
				obsolete: 1,
			},
		},
		"obsolete object, missing segment": {
			storage: mockCapacity{
				capacity: map[string]interface{}{
					// This matches layer0.
					"foo": "1Gi",
				},
			},
			initialSCs: []testSC{
				{
					name:       "other-sc",
					driverName: driverName,
				},
			},
			initialCapacities: []testCapacity{
				{
					segment:          layer0,
					storageClassName: "other-sc",
					quantity:         "1Gi",
				},
			},
			expectedObjectsPrepared: objects{
				obsolete: 1,
			},
		},
		"ignore capacity with other owner": {
			initialCapacities: []testCapacity{
				{
					owner:            &otherOwner,
					uid:              "test-capacity-1",
					segment:          layer0,
					storageClassName: "other-sc",
					quantity:         "1Gi",
				},
			},
			expectedCapacities: []testCapacity{
				{
					owner:            &otherOwner,
					uid:              "test-capacity-1",
					segment:          layer0,
					storageClassName: "other-sc",
					quantity:         "1Gi",
				},
			},
		},
		"ignore capacity with no owner": {
			initialCapacities: []testCapacity{
				{
					owner:            &noOwner,
					uid:              "test-capacity-1",
					segment:          layer0,
					storageClassName: "other-sc",
					quantity:         "1Gi",
				},
			},
			expectedCapacities: []testCapacity{
				{
					owner:            &noOwner,
					uid:              "test-capacity-1",
					segment:          layer0,
					storageClassName: "other-sc",
					quantity:         "1Gi",
				},
			},
		},
		"two segments, two classes, four objects missing": {
			topology: topology.NewMock(&layer0, &layer0other),
			storage: mockCapacity{
				capacity: map[string]interface{}{
					// This matches layer0.
					"foo": "1Gi",
					"bar": "2Gi",
				},
			},
			initialSCs: []testSC{
				{
					name:       "direct-sc",
					driverName: driverName,
				},
				{
					name:       "triple-sc",
					driverName: driverName,
					parameters: map[string]string{
						mockMultiplier: "3",
					},
				},
			},
			expectedCapacities: []testCapacity{
				{
					resourceVersion:  csiscRev + "0",
					segment:          layer0,
					storageClassName: "direct-sc",
					quantity:         "1Gi",
				},
				{
					resourceVersion:  csiscRev + "0",
					segment:          layer0,
					storageClassName: "triple-sc",
					quantity:         "3Gi",
				},
				{
					resourceVersion:  csiscRev + "0",
					segment:          layer0other,
					storageClassName: "direct-sc",
					quantity:         "2Gi",
				},
				{
					resourceVersion:  csiscRev + "0",
					segment:          layer0other,
					storageClassName: "triple-sc",
					quantity:         "6Gi",
				},
			},
			expectedObjectsPrepared: objects{
				goal: 4,
			},
			expectedTotalProcessed: 4,
		},
		"two segments, two classes, four objects updated": {
			topology: topology.NewMock(&layer0, &layer0other),
			storage: mockCapacity{
				capacity: map[string]interface{}{
					// This matches layer0.
					"foo": "1Gi",
					"bar": "2Gi",
				},
			},
			initialSCs: []testSC{
				{
					name:       "direct-sc",
					driverName: driverName,
				},
				{
					name:       "triple-sc",
					driverName: driverName,
					parameters: map[string]string{
						mockMultiplier: "3",
					},
				},
			},
			initialCapacities: []testCapacity{
				{
					uid:              "test-capacity-1",
					segment:          layer0,
					storageClassName: "direct-sc",
					quantity:         "1Mi",
				},
				{
					uid:              "test-capacity-2",
					segment:          layer0,
					storageClassName: "triple-sc",
					quantity:         "3Mi",
				},
				{
					uid:              "test-capacity-3",
					segment:          layer0other,
					storageClassName: "direct-sc",
					quantity:         "2Mi",
				},
				{
					uid:              "test-capacity-4",
					segment:          layer0other,
					storageClassName: "triple-sc",
					quantity:         "6Mi",
				},
			},
			expectedCapacities: []testCapacity{
				{
					uid:              "test-capacity-1",
					resourceVersion:  csiscRev + "1",
					segment:          layer0,
					storageClassName: "direct-sc",
					quantity:         "1Gi",
				},
				{
					uid:              "test-capacity-2",
					resourceVersion:  csiscRev + "1",
					segment:          layer0,
					storageClassName: "triple-sc",
					quantity:         "3Gi",
				},
				{
					uid:              "test-capacity-3",
					resourceVersion:  csiscRev + "1",
					segment:          layer0other,
					storageClassName: "direct-sc",
					quantity:         "2Gi",
				},
				{
					uid:              "test-capacity-4",
					resourceVersion:  csiscRev + "1",
					segment:          layer0other,
					storageClassName: "triple-sc",
					quantity:         "6Gi",
				},
			},
			expectedObjectsPrepared: objects{
				goal:    4,
				current: 4,
			},
			expectedTotalProcessed: 4,
		},
		"two segments, two classes, two added, two removed": {
			topology: topology.NewMock(&layer0, &layer0other),
			storage: mockCapacity{
				capacity: map[string]interface{}{
					// This matches layer0.
					"foo": "1Gi",
					"bar": "2Gi",
				},
			},
			initialSCs: []testSC{
				{
					name:       "direct-sc",
					driverName: driverName,
				},
				{
					name:       "triple-sc",
					driverName: driverName,
					parameters: map[string]string{
						mockMultiplier: "3",
					},
				},
			},
			initialCapacities: []testCapacity{
				{
					uid:              "test-capacity-1",
					segment:          layer0,
					storageClassName: "old-direct-sc",
					quantity:         "1Mi",
				},
				{
					uid:              "test-capacity-2",
					segment:          layer0,
					storageClassName: "old-triple-sc",
					quantity:         "3Mi",
				},
				{
					uid:              "test-capacity-3",
					segment:          layer0other,
					storageClassName: "direct-sc",
					quantity:         "2Mi",
				},
				{
					uid:              "test-capacity-4",
					segment:          layer0other,
					storageClassName: "triple-sc",
					quantity:         "6Mi",
				},
			},
			expectedCapacities: []testCapacity{
				{
					resourceVersion:  csiscRev + "0",
					segment:          layer0,
					storageClassName: "direct-sc",
					quantity:         "1Gi",
				},
				{
					resourceVersion:  csiscRev + "0",
					segment:          layer0,
					storageClassName: "triple-sc",
					quantity:         "3Gi",
				},
				{
					uid:              "test-capacity-3",
					resourceVersion:  csiscRev + "1",
					segment:          layer0other,
					storageClassName: "direct-sc",
					quantity:         "2Gi",
				},
				{
					uid:              "test-capacity-4",
					resourceVersion:  csiscRev + "1",
					segment:          layer0other,
					storageClassName: "triple-sc",
					quantity:         "6Gi",
				},
			},
			expectedObjectsPrepared: objects{
				goal:     4,
				current:  2,
				obsolete: 2,
			},
			expectedTotalProcessed: 4,
		},
		"re-create capacity": {
			topology: topology.NewMock(&layer0),
			storage: mockCapacity{
				capacity: map[string]interface{}{
					// This matches layer0.
					"foo": "1Gi",
				},
			},
			initialSCs: []testSC{
				{
					name:       "other-sc",
					driverName: driverName,
				},
			},
			expectedCapacities: []testCapacity{
				{
					uid:              "CSISC-UID-1",
					resourceVersion:  csiscRev + "0",
					segment:          layer0,
					storageClassName: "other-sc",
					quantity:         "1Gi",
				},
			},
			modify: func(ctx context.Context, clientSet *fakeclientset.Clientset, expected []testCapacity) ([]testCapacity, error) {
				capacities, err := clientSet.StorageV1alpha1().CSIStorageCapacities(ownerNamespace).List(ctx, metav1.ListOptions{})
				if err != nil {
					return nil, err
				}
				capacity := capacities.Items[0]
				if err := clientSet.StorageV1alpha1().CSIStorageCapacities(ownerNamespace).Delete(ctx, capacity.Name, metav1.DeleteOptions{}); err != nil {
					return nil, err
				}
				expected[0].uid = "CSISC-UID-2"
				return expected, nil
			},
			expectedObjectsPrepared: objects{
				goal: 1,
			},
			expectedTotalProcessed: 1,
		},
		"delete redundant capacity": {
			modify: func(ctx context.Context, clientSet *fakeclientset.Clientset, expected []testCapacity) ([]testCapacity, error) {
				capacity := makeCapacity(testCapacity{quantity: "1Gi"})
				if _, err := clientSet.StorageV1alpha1().CSIStorageCapacities(ownerNamespace).Create(ctx, capacity, metav1.CreateOptions{}); err != nil {
					return nil, err
				}
				return expected, nil
			},
		},
		"ignore capacity after owner change": {
			topology: topology.NewMock(&layer0),
			storage: mockCapacity{
				capacity: map[string]interface{}{
					// This matches layer0.
					"foo": "1Gi",
				},
			},
			initialSCs: []testSC{
				{
					name:       "other-sc",
					driverName: driverName,
				},
			},
			expectedCapacities: []testCapacity{
				{
					uid:              "CSISC-UID-1",
					resourceVersion:  csiscRev + "0",
					segment:          layer0,
					storageClassName: "other-sc",
					quantity:         "1Gi",
				},
			},
			modify: func(ctx context.Context, clientSet *fakeclientset.Clientset, expected []testCapacity) ([]testCapacity, error) {
				capacities, err := clientSet.StorageV1alpha1().CSIStorageCapacities(ownerNamespace).List(ctx, metav1.ListOptions{})
				if err != nil {
					return nil, err
				}
				capacity := capacities.Items[0]
				// Unset owner. It's not clear why anyone would want to do that, but lets deal with it anyway:
				// - the now "foreign" object must be left alone
				// - an entry must be created anew
				capacity.OwnerReferences = []metav1.OwnerReference{}
				if _, err := clientSet.StorageV1alpha1().CSIStorageCapacities(ownerNamespace).Update(ctx, &capacity, metav1.UpdateOptions{}); err != nil {
					return nil, err
				}
				expected[0].owner = &noOwner
				expected[0].resourceVersion = csiscRev + "1"
				expected = append(expected, testCapacity{
					uid:              "CSISC-UID-2",
					resourceVersion:  csiscRev + "0",
					segment:          layer0,
					storageClassName: "other-sc",
					quantity:         "1Gi",
				})
				return expected, nil
			},
			expectedObjectsPrepared: objects{
				goal: 1,
			},
			expectedTotalProcessed: 1,
		},
		"delete and recreate by someone": {
			topology: topology.NewMock(&layer0),
			storage: mockCapacity{
				capacity: map[string]interface{}{
					// This matches layer0.
					"foo": "1Gi",
				},
			},
			initialSCs: []testSC{
				{
					name:       "other-sc",
					driverName: driverName,
				},
			},
			expectedCapacities: []testCapacity{
				{
					uid:              "CSISC-UID-1",
					resourceVersion:  csiscRev + "0",
					segment:          layer0,
					storageClassName: "other-sc",
					quantity:         "1Gi",
				},
			},
			modify: func(ctx context.Context, clientSet *fakeclientset.Clientset, expected []testCapacity) ([]testCapacity, error) {
				capacities, err := clientSet.StorageV1alpha1().CSIStorageCapacities(ownerNamespace).List(ctx, metav1.ListOptions{})
				if err != nil {
					return nil, err
				}
				capacity := capacities.Items[0]
				// Delete and recreate with wrong capacity. This changes the UID while keeping the name
				// the same. The capacity then must get corrected by the controller.
				if err := clientSet.StorageV1alpha1().CSIStorageCapacities(ownerNamespace).Delete(ctx, capacity.Name, metav1.DeleteOptions{}); err != nil {
					return nil, err
				}
				capacity.UID = "CSISC-UID-2"
				capacity.Capacity = &mb
				if _, err := clientSet.StorageV1alpha1().CSIStorageCapacities(ownerNamespace).Create(ctx, &capacity, metav1.CreateOptions{}); err != nil {
					return nil, err
				}
				expected[0].uid = capacity.UID
				expected[0].resourceVersion = csiscRev + "1"
				return expected, nil
			},
			expectedObjectsPrepared: objects{
				goal: 1,
			},
			expectedTotalProcessed: 1,
		},
		"storage capacity change": {
			topology: topology.NewMock(&layer0),
			storage: mockCapacity{
				capacity: map[string]interface{}{
					// This matches layer0.
					"foo": "1Gi",
				},
			},
			initialSCs: []testSC{
				{
					name:       "other-sc",
					driverName: driverName,
				},
			},
			expectedCapacities: []testCapacity{
				{
					uid:              "CSISC-UID-1",
					resourceVersion:  csiscRev + "0",
					segment:          layer0,
					storageClassName: "other-sc",
					quantity:         "1Gi",
				},
			},
			capacityChange: func(ctx context.Context, storage *mockCapacity, expected []testCapacity) []testCapacity {
				storage.capacity["foo"] = "2Gi"
				expected[0].quantity = "2Gi"
				expected[0].resourceVersion = csiscRev + "1"
				return expected
			},
			expectedObjectsPrepared: objects{
				goal: 1,
			},
			expectedTotalProcessed: 1,
		},
		"add storage topology segment": {
			storage: mockCapacity{
				capacity: map[string]interface{}{
					// This matches layer0.
					"foo": "1Gi",
				},
			},
			initialSCs: []testSC{
				// We intentionally create a SC with immediate binding first.
				// It needs to be skipped while still processing the other one.
				// Ordering of the objects is not guaranteed, but in practice
				// the informer seems to be "first in, first out", which is what
				// we need.
				{
					name:             "immediate-sc",
					driverName:       driverName,
					immediateBinding: true,
				},
				{
					name:       "late-sc",
					driverName: driverName,
				},
			},
			expectedCapacities: nil,
			topologyChange: func(ctx context.Context, topo *topology.Mock, expected []testCapacity) []testCapacity {
				topo.Modify([]*topology.Segment{&layer0} /* added */, nil /* removed */)
				return append(expected, testCapacity{
					uid:              "CSISC-UID-1",
					resourceVersion:  csiscRev + "0",
					segment:          layer0,
					storageClassName: "late-sc",
					quantity:         "1Gi",
				})
			},
			expectedTotalProcessed: 1,
		},
		"add storage topology segment, immediate binding": {
			immediateBinding: true,
			storage: mockCapacity{
				capacity: map[string]interface{}{
					// This matches layer0.
					"foo": "1Gi",
				},
			},
			initialSCs: []testSC{
				{
					name:             "immediate-sc",
					driverName:       driverName,
					immediateBinding: true,
				},
				{
					name:       "late-sc",
					driverName: driverName,
				},
			},
			expectedCapacities: nil,
			topologyChange: func(ctx context.Context, topo *topology.Mock, expected []testCapacity) []testCapacity {
				topo.Modify([]*topology.Segment{&layer0} /* added */, nil /* removed */)
				// We don't check the UID here because we don't want to fail when
				// ordering of storage classes isn't such that the "immediate-sc" is seen first.
				return append(expected, testCapacity{
					resourceVersion:  csiscRev + "0",
					segment:          layer0,
					storageClassName: "immediate-sc",
					quantity:         "1Gi",
				},
					testCapacity{
						resourceVersion:  csiscRev + "0",
						segment:          layer0,
						storageClassName: "late-sc",
						quantity:         "1Gi",
					},
				)
			},
			expectedTotalProcessed: 2,
		},
		"remove storage topology segment": {
			topology: topology.NewMock(&layer0),
			storage: mockCapacity{
				capacity: map[string]interface{}{
					// This matches layer0.
					"foo": "1Gi",
				},
			},
			initialSCs: []testSC{
				{
					name:             "immediate-sc",
					driverName:       driverName,
					immediateBinding: true,
				},
				{
					name:       "late-sc",
					driverName: driverName,
				},
			},
			expectedCapacities: []testCapacity{
				{
					uid:              "CSISC-UID-1",
					resourceVersion:  csiscRev + "0",
					segment:          layer0,
					storageClassName: "late-sc",
					quantity:         "1Gi",
				},
			},
			topologyChange: func(ctx context.Context, topo *topology.Mock, expected []testCapacity) []testCapacity {
				topo.Modify(nil /* added */, topo.List()[:] /* removed */)
				return nil
			},
			expectedObjectsPrepared: objects{
				goal: 1,
			},
		},
		"add and remove storage topology segment": {
			topology: topology.NewMock(&layer0),
			storage: mockCapacity{
				capacity: map[string]interface{}{
					// This matches layer0.
					"foo": "1Gi",
					"bar": "2Gi",
				},
			},
			initialSCs: []testSC{
				{
					name:             "immediate-sc",
					driverName:       driverName,
					immediateBinding: true,
				},
				{
					name:       "late-sc",
					driverName: driverName,
				},
			},
			expectedCapacities: []testCapacity{
				{
					uid:              "CSISC-UID-1",
					resourceVersion:  csiscRev + "0",
					segment:          layer0,
					storageClassName: "late-sc",
					quantity:         "1Gi",
				},
			},
			topologyChange: func(ctx context.Context, topo *topology.Mock, expected []testCapacity) []testCapacity {
				topo.Modify([]*topology.Segment{&layer0other}, /* added */
					topo.List()[:] /* removed */)
				return []testCapacity{
					{
						uid:              "CSISC-UID-2",
						resourceVersion:  csiscRev + "0",
						segment:          layer0other,
						storageClassName: "late-sc",
						quantity:         "2Gi",
					},
				}
			},
			expectedObjectsPrepared: objects{
				goal: 1,
			},
			expectedTotalProcessed: 1,
		},
	}

	for name, tc := range testcases {
		// Not run in parallel. That doesn't work well in combination with global logging
		// and global metrics instances.
		t.Run(name, func(t *testing.T) {
			// There is no good way to shut down the controller. It spawns
			// various goroutines and some of them (in particular shared informer)
			// become very unhappy ("close on closed channel") when using a context
			// that gets cancelled. Therefore we just keep everything running.
			ctx := context.Background()

			var initialObjects []runtime.Object
			initialObjects = append(initialObjects, makeSCs(tc.initialSCs)...)
			clientSet := fakeclientset.NewSimpleClientset(initialObjects...)
			clientSet.PrependReactor("create", "csistoragecapacities", createCSIStorageCapacityReactor())
			clientSet.PrependReactor("update", "csistoragecapacities", updateCSIStorageCapacityReactor())
			topo := tc.topology
			if topo == nil {
				topo = topology.NewMock()
			}
			c, registry := fakeController(ctx, clientSet, &tc.storage, topo, tc.immediateBinding)
			for _, testCapacity := range tc.initialCapacities {
				capacity := makeCapacity(testCapacity)
				_, err := clientSet.StorageV1alpha1().CSIStorageCapacities(ownerNamespace).Create(ctx, capacity, metav1.CreateOptions{})
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			}
			c.prepare(ctx)
			if err := tc.expectedObjectsPrepared.verify(registry); err != nil {
				t.Fatalf("metrics after prepare: %v", err)
			}
			if err := process(ctx, c); err != nil {
				t.Fatalf("unexpected processing error: %v", err)
			}
			err := validateCapacities(ctx, clientSet, tc.expectedCapacities)
			if err != nil {
				t.Fatalf("%v", err)
			}

			// Now (optionally) modify the state and
			// ensure that eventually the controller
			// catches up.
			expectedCapacities := tc.expectedCapacities
			if tc.modify != nil {
				klog.Info("modifying objects")
				ec, err := tc.modify(ctx, clientSet, expectedCapacities)
				if err != nil {
					t.Fatalf("modify objects: %v", err)
				}
				expectedCapacities = ec
				if err := validateCapacitiesEventually(ctx, c, clientSet, expectedCapacities); err != nil {
					t.Fatalf("modified objects: %v", err)
				}
			}
			if tc.capacityChange != nil {
				klog.Info("modifying capacity")
				expectedCapacities = tc.capacityChange(ctx, &tc.storage, expectedCapacities)
				c.pollCapacities()
				if err := validateCapacitiesEventually(ctx, c, clientSet, expectedCapacities); err != nil {
					t.Fatalf("modified capacity: %v", err)
				}
			}
			if tc.topologyChange != nil {
				klog.Info("modifying topology")
				expectedCapacities = tc.topologyChange(ctx, topo, expectedCapacities)
				if err := validateCapacitiesEventually(ctx, c, clientSet, expectedCapacities); err != nil {
					t.Fatalf("modified capacity: %v", err)
				}
			}

			// Processing the work queues may take some time.
			validateMetrics := func(ctx context.Context) error {
				return objects{
					goal:    tc.expectedTotalProcessed,
					current: tc.expectedTotalProcessed,
				}.verify(registry)
			}
			if err := validateEventually(ctx, c, validateMetrics); err != nil {
				t.Fatalf("metrics after processing: %v", err)
			}
			if err := validateConsistently(ctx, c, validateMetrics); err != nil {
				t.Fatalf("metrics not stable after processing: %v", err)
			}
		})
	}
}

func validateCapacities(ctx context.Context, clientSet *fakeclientset.Clientset, expectedCapacities []testCapacity) error {
	actualCapacities, err := clientSet.StorageV1alpha1().CSIStorageCapacities(ownerNamespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("unexpected error: %v", err)
	}
	var messages []string
	if len(actualCapacities.Items) != len(expectedCapacities) {
		messages = append(messages, fmt.Sprintf("expected %d CSIStorageCapacity objects, got %d", len(expectedCapacities), len(actualCapacities.Items)))
	}
nextActual:
	for _, actual := range actualCapacities.Items {
		for i, expected := range expectedCapacities {
			expectedOwnerReferences := makeCapacity(expected).OwnerReferences
			if reflect.DeepEqual(actual.NodeTopology, expected.segment.GetLabelSelector()) &&
				actual.StorageClassName == expected.storageClassName &&
				(len(actual.OwnerReferences) == 0 && len(expectedOwnerReferences) == 0 ||
					reflect.DeepEqual(actual.OwnerReferences, expectedOwnerReferences)) {
				var mismatches []string
				if expected.quantity != "" && actual.Capacity == nil {
					mismatches = append(mismatches, "unexpected nil quantity")
				}
				if expected.quantity == "" && actual.Capacity != nil {
					mismatches = append(mismatches, "unexpected quantity")
				}
				if expected.quantity != "" && actual.Capacity.Cmp(*expected.getCapacity()) != 0 {
					mismatches = append(mismatches, fmt.Sprintf("expected quantity %v, got %v", expected.quantity, *actual.Capacity))
				}
				if expected.uid != "" && actual.UID != expected.uid {
					mismatches = append(mismatches, fmt.Sprintf("expected UID %s, got %s", expected.uid, actual.UID))
				}
				if expected.resourceVersion != "" && actual.ResourceVersion != expected.resourceVersion {
					mismatches = append(mismatches, fmt.Sprintf("expected ResourceVersion %s, got %s", expected.resourceVersion, actual.ResourceVersion))
				}
				if len(mismatches) > 0 {
					messages = append(messages, fmt.Sprintf("CSIStorageCapacity %+v:\n    %s", actual, strings.Join(mismatches, "\n    ")))
				}
				// Never match against the same expected capacity twice. Also, the ones that remain are dumped below.
				expectedCapacities = append(expectedCapacities[:i], expectedCapacities[i+1:]...)
				continue nextActual
			}
		}
		messages = append(messages, fmt.Sprintf("unexpected CSIStorageCapacity %#v", actual))
	}
	for _, expected := range expectedCapacities {
		messages = append(messages, fmt.Sprintf("expected CSIStorageCapacity %+v not found", expected))
	}
	if len(messages) > 0 {
		return errors.New(strings.Join(messages, "\n"))
	}
	return nil
}

func validateCapacitiesEventually(ctx context.Context, c *Controller, clientSet *fakeclientset.Clientset, expectedCapacities []testCapacity) error {
	return validateEventually(ctx, c, func(ctx context.Context) error {
		return validateCapacities(ctx, clientSet, expectedCapacities)
	})
}

func validateEventually(ctx context.Context, c *Controller, validate func(ctx context.Context) error) error {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	deadline, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	var lastValidationError error
	klog.Info("waiting for controller to catch up")
	for {
		select {
		case <-ticker.C:
			if err := process(ctx, c); err != nil {
				return fmt.Errorf("unexpected processing error: %v", err)
			}
			lastValidationError = validate(ctx)
			if lastValidationError == nil {
				return nil
			}
		case <-deadline.Done():
			return fmt.Errorf("timed out waiting for controller, last unexpected state:\n%v", lastValidationError)
		}
	}
}

func validateConsistently(ctx context.Context, c *Controller, validate func(ctx context.Context) error) error {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	deadline, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	for {
		select {
		case <-deadline.Done():
			return nil
		case <-ticker.C:
			if err := process(ctx, c); err != nil {
				return fmt.Errorf("unexpected processing error: %v", err)
			}
			if err := validate(ctx); err != nil {
				return err
			}
		}
	}
}

// createCSIStorageCapacityReactor implements the logic required for the GenerateName and UID fields to work when using
// the fake client. Add it with client.PrependReactor to your fake client.
func createCSIStorageCapacityReactor() func(action ktesting.Action) (handled bool, ret runtime.Object, err error) {
	var uidCounter int
	var mutex sync.Mutex
	return func(action ktesting.Action) (handled bool, ret runtime.Object, err error) {
		s := action.(ktesting.CreateAction).GetObject().(*storagev1alpha1.CSIStorageCapacity)
		if s.Name == "" && s.GenerateName != "" {
			s.Name = fmt.Sprintf("%s-%s", s.GenerateName, krand.String(16))
		}
		if s.UID == "" {
			mutex.Lock()
			defer mutex.Unlock()
			uidCounter++
			s.UID = types.UID(fmt.Sprintf("CSISC-UID-%d", uidCounter))
		}
		s.ResourceVersion = csiscRev + "0"
		return false, nil, nil
	}
}

// updateCSIStorageCapacityReactor implements the logic required for the ResourceVersion field to work when using
// the fake client. Add it with client.PrependReactor to your fake client.
func updateCSIStorageCapacityReactor() func(action ktesting.Action) (handled bool, ret runtime.Object, err error) {
	return func(action ktesting.Action) (handled bool, ret runtime.Object, err error) {
		s := action.(ktesting.UpdateAction).GetObject().(*storagev1alpha1.CSIStorageCapacity)
		if !strings.HasPrefix(s.ResourceVersion, csiscRev) {
			return false, nil, fmt.Errorf("resource version %q should have prefix %s", s.ResourceVersion, csiscRev)
		}
		revCounter, err := strconv.Atoi(s.ResourceVersion[len(csiscRev):])
		if err != nil {
			return false, nil, fmt.Errorf("resource version %q should have formar %s<number>: %v", s.ResourceVersion, csiscRev, err)
		}
		s.ResourceVersion = csiscRev + fmt.Sprintf("%d", revCounter+1)
		return false, nil, nil
	}
}

func fakeController(ctx context.Context, client *fakeclientset.Clientset, storage CSICapacityClient, topologyInformer topology.Informer, immediateBinding bool) (*Controller, metrics.KubeRegistry) {
	utilruntime.ReallyCrash = false // avoids os.Exit after "close of closed channel" in shared informer code

	// We don't need resyncs, they just lead to confusing log output if they get triggered while already some
	// new test is running.
	resyncPeriod := time.Hour
	informerFactory := informers.NewSharedInformerFactory(client, resyncPeriod)
	scInformer := informerFactory.Storage().V1().StorageClasses()
	cInformer := informerFactory.Storage().V1alpha1().CSIStorageCapacities()
	rateLimiter := workqueue.NewItemExponentialFailureRateLimiter(time.Second, 2*time.Second)
	queue := workqueue.NewNamedRateLimitingQueue(rateLimiter, "items")

	c := NewCentralCapacityController(
		storage,
		driverName,
		client,
		queue,
		defaultOwner,
		ownerNamespace,
		topologyInformer,
		scInformer,
		cInformer,
		1000*time.Hour, // Not used, but even if it was, we wouldn't want automatic capacity polling while the test runs...
		immediateBinding,
	)

	// This ensures that the informers are running and up-to-date.
	go informerFactory.Start(ctx.Done())
	informerFactory.WaitForCacheSync(ctx.Done())

	registry := metrics.NewKubeRegistry()
	registry.CustomMustRegister(c)

	return c, registry
}

// process handles work items until the queue is empty and the informers are synced.
func process(ctx context.Context, c *Controller) error {
	for {
		if c.queue.Len() == 0 {
			done, err := storageClassesSynced(ctx, c)
			if err != nil {
				return fmt.Errorf("check storage classes: %v", err)
			}
			if done {
				return nil
			}
		}
		// There's no atomic "try to get a work item". Let's
		// check one more time before potentially blocking
		// in c.queue.Get().
		len := c.queue.Len()
		if len > 0 {
			klog.V(1).Infof("testing next work item, queue length %d", len)
			c.processNextWorkItem(ctx)
			klog.V(5).Infof("done testing next work item")
		}
	}
}

func storageClassesSynced(ctx context.Context, c *Controller) (bool, error) {
	actualStorageClasses, err := c.client.StorageV1().StorageClasses().List(ctx, metav1.ListOptions{})
	if err != nil {
		return false, err
	}
	informerStorageClasses, err := c.scInformer.Lister().List(labels.Everything())
	if len(informerStorageClasses) != len(actualStorageClasses.Items) {
		return false, nil
	}
	if len(informerStorageClasses) > 0 && !func() bool {
		for _, actualStorageClass := range actualStorageClasses.Items {
			for _, informerStorageClass := range informerStorageClasses {
				if reflect.DeepEqual(actualStorageClass, *informerStorageClass) {
					return true
				}
			}
		}
		return false
	}() {
		return false, nil
	}

	return true, nil
}

const (
	mockMultiplier = "multiplier"
)

// mockGetCapacity simulates a driver with a layered storage system:
// storage exists at each level with different quantities (one pool for all nodes,
// one pool for each data center, one pool for reach region).
//
// It uses "layer1", "layer2", ... etc. as topology keys to dive into
// the map, which then either has a string or another map.
// A fake "multiplier" parameter is applied to the resulting capacity.
type mockCapacity struct {
	capacity map[string]interface{}
}

func (mc *mockCapacity) GetCapacity(ctx context.Context, in *csi.GetCapacityRequest, opts ...grpc.CallOption) (*csi.GetCapacityResponse, error) {
	available := ""
	if in.AccessibleTopology != nil {
		var err error
		available, err = getCapacity(mc.capacity, in.AccessibleTopology.Segments, 0)
		if err != nil {
			return nil, err
		}
	}
	resp := &csi.GetCapacityResponse{}
	if available != "" {
		quantity := resource.MustParse(available)
		resp.AvailableCapacity = quantity.Value()
	}
	multiplierStr, ok := in.Parameters[mockMultiplier]
	if ok {
		multiplier, err := strconv.Atoi(multiplierStr)
		if err != nil {
			return nil, fmt.Errorf("invalid parameter %s -> %s: %v", mockMultiplier, multiplierStr, err)
		}
		resp.AvailableCapacity *= int64(multiplier)
	}
	return resp, nil
}

func getCapacity(capacity map[string]interface{}, segments map[string]string, layer int) (string, error) {
	if capacity == nil {
		return "", fmt.Errorf("no information found at layer %d", layer)
	}
	key := fmt.Sprintf("layer%d", layer)
	value := capacity[segments[key]]
	switch value := value.(type) {
	case string:
		return value, nil
	case map[string]interface{}:
		result, err := getCapacity(value, segments, layer+1)
		if err != nil {
			return "", fmt.Errorf("%s -> %s: %v", key, segments[key], err)
		}
		return result, nil
	}
	return "", nil
}

type testCapacity struct {
	uid              types.UID
	resourceVersion  string
	segment          topology.Segment
	storageClassName string
	quantity         string
	owner            *metav1.OwnerReference
}

func (tc testCapacity) getCapacity() *resource.Quantity {
	if tc.quantity == "" {
		return nil
	}
	quantity := resource.MustParse(tc.quantity)
	return &quantity
}

var capacityCounter int

func makeCapacity(in testCapacity) *storagev1alpha1.CSIStorageCapacity {
	capacityCounter++
	var owners []metav1.OwnerReference
	switch in.owner {
	case nil:
		owners = append(owners, defaultOwner)
	case &noOwner:
		// Don't add anything.
	default:
		owners = append(owners, *in.owner)
	}
	return &storagev1alpha1.CSIStorageCapacity{
		ObjectMeta: metav1.ObjectMeta{
			UID:             in.uid,
			ResourceVersion: in.resourceVersion,
			Name:            fmt.Sprintf("csisc-%d", capacityCounter),
			OwnerReferences: owners,
		},
		NodeTopology:     in.segment.GetLabelSelector(),
		StorageClassName: in.storageClassName,
		Capacity:         in.getCapacity(),
	}
}

type testSC struct {
	name             string
	driverName       string
	parameters       map[string]string
	immediateBinding bool
}

func makeSC(in testSC) *storagev1.StorageClass {
	volumeBinding := storagev1.VolumeBindingWaitForFirstConsumer
	if in.immediateBinding {
		volumeBinding = storagev1.VolumeBindingImmediate
	}
	return &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: in.name,
		},
		Provisioner:       in.driverName,
		Parameters:        in.parameters,
		VolumeBindingMode: &volumeBinding,
	}
}

func makeSCs(in []testSC) (items []runtime.Object) {
	for _, item := range in {
		items = append(items, makeSC(item))
	}
	return
}
