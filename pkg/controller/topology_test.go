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
	"slices"
	"testing"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/testing/protocmp"
	v1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/informers"
	fakeclientset "k8s.io/client-go/kubernetes/fake"
	corelisters "k8s.io/client-go/listers/core/v1"
	storagelistersv1 "k8s.io/client-go/listers/storage/v1"
)

const (
	testDriverName = "com.example.csi/test-driver"
)

var withWithout = map[bool]string{false: "without", true: "with"}

func TestGenerateVolumeNodeAffinity(t *testing.T) {
	// TODO (verult) more test cases
	testcases := map[string]struct {
		accessibleTopology   []*csi.Topology
		expectedNodeAffinity *v1.VolumeNodeAffinity
	}{
		"nil topology": {
			accessibleTopology:   nil,
			expectedNodeAffinity: nil,
		},
		"non-nil topology but nil segment": {
			accessibleTopology:   []*csi.Topology{},
			expectedNodeAffinity: nil,
		},
		"single segment": {
			accessibleTopology: []*csi.Topology{
				{
					Segments: map[string]string{"com.example.csi/zone": "zone1"},
				},
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
							},
						},
					},
				},
			},
		},
		"multiple segments": {
			accessibleTopology: []*csi.Topology{
				{
					Segments: map[string]string{
						"com.example.csi/zone": "zone1",
						"com.example.csi/rack": "rack2",
					},
				},
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
		"multiple topologies": {
			accessibleTopology: []*csi.Topology{
				{
					Segments: map[string]string{
						"com.example.csi/zone": "zone1",
					},
				},
				{
					Segments: map[string]string{
						"com.example.csi/zone": "zone2",
					},
				},
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
							},
						},
						{
							MatchExpressions: []v1.NodeSelectorRequirement{
								{
									Key:      "com.example.csi/zone",
									Operator: v1.NodeSelectorOpIn,
									Values:   []string{"zone2"},
								},
							},
						},
					},
				},
			},
		},
	}

	for name, tc := range testcases {
		t.Run(name, func(t *testing.T) {
			nodeAffinity := GenerateVolumeNodeAffinity(tc.accessibleTopology)
			if !volumeNodeAffinitiesEqual(nodeAffinity, tc.expectedNodeAffinity) {
				t.Errorf("expected node affinity %v; got: %v", tc.expectedNodeAffinity, nodeAffinity)
			}
		})
	}
}

func TestStatefulSetSpreading(t *testing.T) {
	nodeLabels := []map[string]string{
		{"com.example.csi/zone": "zone1", "com.example.csi/rack": "rackA"},
		{"com.example.csi/zone": "zone2", "com.example.csi/rack": "rackB"},
		{"com.example.csi/zone": "zone3", "com.example.csi/rack": "rackC"},
		{"com.example.csi/zone": "zone4", "com.example.csi/rack": "rackD"},
	}
	var topologyKeys []map[string][]string
	keys := map[string][]string{testDriverName: {"com.example.csi/zone", "com.example.csi/rack"}}
	for range nodeLabels {
		topologyKeys = append(topologyKeys, keys)
	}
	// Ordering of segments in preferred array is sensitive to statefulset name portion of pvcName.
	// In the tests below, name of the statefulset: testset is the portion whose hash determines ordering.
	// If statefulset name is changed, make sure expectedPreferred is kept in sync.
	// pvc prefix in pvcName does not have any effect on segment ordering
	testcases := map[string]struct {
		pvcUID            types.UID
		pvcName           string
		allowedTopologies []v1.TopologySelectorTerm
		expectedPreferred []*csi.Topology
	}{
		"select index 0 among nodes for pvc with statefulset name:testset and id:1; ignore claimname:testpvcA": {
			pvcUID:  "a9e2d5a3-1c64-4787-97b7-1a22d5a0b123",
			pvcName: "testpvcA-testset-1",
			expectedPreferred: []*csi.Topology{
				{
					Segments: map[string]string{
						"com.example.csi/rack": "rackA",
						"com.example.csi/zone": "zone1",
					},
				},
				{
					Segments: map[string]string{
						"com.example.csi/rack": "rackB",
						"com.example.csi/zone": "zone2",
					},
				},
				{
					Segments: map[string]string{
						"com.example.csi/rack": "rackC",
						"com.example.csi/zone": "zone3",
					},
				},
				{
					Segments: map[string]string{
						"com.example.csi/rack": "rackD",
						"com.example.csi/zone": "zone4",
					},
				},
			},
		},
		"select index 0 among nodes for pvc with statefulset name:testset and id:1; ignore claimname:testpvcB": {
			pvcUID:  "a9e2d5a3-1c64-4787-97b7-1a22d5a0b123",
			pvcName: "testpvcB-testset-1",
			expectedPreferred: []*csi.Topology{
				{
					Segments: map[string]string{
						"com.example.csi/rack": "rackA",
						"com.example.csi/zone": "zone1",
					},
				},
				{
					Segments: map[string]string{
						"com.example.csi/rack": "rackB",
						"com.example.csi/zone": "zone2",
					},
				},
				{
					Segments: map[string]string{
						"com.example.csi/rack": "rackC",
						"com.example.csi/zone": "zone3",
					},
				},
				{
					Segments: map[string]string{
						"com.example.csi/rack": "rackD",
						"com.example.csi/zone": "zone4",
					},
				},
			},
		},
		"select index 0 among allowedTopologies with single term/multiple requirements for pvc with statefulset name:testset and id:1; ignore claimname:testpvcC": {
			pvcUID:  "a9e2d5a3-1c64-4787-97b7-1a22d5a0b123",
			pvcName: "testpvcC-testset-1",
			allowedTopologies: []v1.TopologySelectorTerm{
				{
					MatchLabelExpressions: []v1.TopologySelectorLabelRequirement{
						{
							Key:    "com.example.csi/zone",
							Values: []string{"zone1"},
						},
						{
							Key:    "com.example.csi/rack",
							Values: []string{"rackA"},
						},
					},
				},
			},
			expectedPreferred: []*csi.Topology{
				{
					Segments: map[string]string{
						"com.example.csi/rack": "rackA",
						"com.example.csi/zone": "zone1",
					},
				},
			},
		},
		"select index 1 among nodes for pvc with statefulset name:testset and id:2": {
			pvcUID:  "a9e2d5a3-1c64-4787-97b7-1a22d5a0b123",
			pvcName: "testset-2",
			expectedPreferred: []*csi.Topology{
				{
					Segments: map[string]string{
						"com.example.csi/rack": "rackB",
						"com.example.csi/zone": "zone2",
					},
				},
				{
					Segments: map[string]string{
						"com.example.csi/rack": "rackC",
						"com.example.csi/zone": "zone3",
					},
				},
				{
					Segments: map[string]string{
						"com.example.csi/rack": "rackD",
						"com.example.csi/zone": "zone4",
					},
				},
				{
					Segments: map[string]string{
						"com.example.csi/rack": "rackA",
						"com.example.csi/zone": "zone1",
					},
				},
			},
		},
		"select index 1 among allowedTopologies with multiple terms/multiple requirements for pvc with statefulset name:testset and id:2; ignore claimname:testpvcB": {
			pvcUID:  "a9e2d5a3-1c64-4787-97b7-1a22d5a0b123",
			pvcName: "testpvcB-testset-2",
			allowedTopologies: []v1.TopologySelectorTerm{
				{
					MatchLabelExpressions: []v1.TopologySelectorLabelRequirement{
						{
							Key:    "com.example.csi/zone",
							Values: []string{"zone2"},
						},
						{
							Key:    "com.example.csi/rack",
							Values: []string{"rackB"},
						},
					},
				},
				{
					MatchLabelExpressions: []v1.TopologySelectorLabelRequirement{
						{
							Key:    "com.example.csi/zone",
							Values: []string{"zone1"},
						},
						{
							Key:    "com.example.csi/rack",
							Values: []string{"rackA"},
						},
					},
				},
			},
			expectedPreferred: []*csi.Topology{
				{
					Segments: map[string]string{
						"com.example.csi/rack": "rackB",
						"com.example.csi/zone": "zone2",
					},
				},
				{
					Segments: map[string]string{
						"com.example.csi/rack": "rackA",
						"com.example.csi/zone": "zone1",
					},
				},
			},
		},
		"select index 2 among nodes with statefulset name:testset and id:3; ignore claimname:testpvc": {
			pvcUID:  "a9e2d5a3-1c64-4787-97b7-1a22d5a0b123",
			pvcName: "testpvc-testset-3",
			expectedPreferred: []*csi.Topology{
				{
					Segments: map[string]string{
						"com.example.csi/rack": "rackC",
						"com.example.csi/zone": "zone3",
					},
				},
				{
					Segments: map[string]string{
						"com.example.csi/rack": "rackD",
						"com.example.csi/zone": "zone4",
					},
				},
				{
					Segments: map[string]string{
						"com.example.csi/rack": "rackA",
						"com.example.csi/zone": "zone1",
					},
				},
				{
					Segments: map[string]string{
						"com.example.csi/rack": "rackB",
						"com.example.csi/zone": "zone2",
					},
				},
			},
		},
		"select index 3 among nodes with statefulset name:testset and id:4; ignore claimname:testpvc": {
			pvcUID:  "a9e2d5a3-1c64-4787-97b7-1a22d5a0b123",
			pvcName: "testpvc-testset-4",
			expectedPreferred: []*csi.Topology{
				{
					Segments: map[string]string{
						"com.example.csi/rack": "rackD",
						"com.example.csi/zone": "zone4",
					},
				},
				{
					Segments: map[string]string{
						"com.example.csi/rack": "rackA",
						"com.example.csi/zone": "zone1",
					},
				},
				{
					Segments: map[string]string{
						"com.example.csi/rack": "rackB",
						"com.example.csi/zone": "zone2",
					},
				},
				{
					Segments: map[string]string{
						"com.example.csi/rack": "rackC",
						"com.example.csi/zone": "zone3",
					},
				},
			},
		},
	}

	nodes := buildNodes(nodeLabels)
	csiNodes := buildCSINodes(topologyKeys)

	kubeClient := fakeclientset.NewSimpleClientset(nodes, csiNodes)

	_, csiNodeLister, nodeLister, _, _, stopChan := listers(kubeClient)
	defer close(stopChan)

	for name, tc := range testcases {
		t.Run(name, func(t *testing.T) {
			for strictTopology, withOrWithout := range withWithout {
				t.Run(withOrWithout+" strict topology", func(t *testing.T) {
					pvcNodeStore := NewInMemoryStore()
					for immediateTopology, withOrWithout := range withWithout {
						t.Run(withOrWithout+" immediate topology", func(t *testing.T) {
							requirements, err := GenerateAccessibilityRequirements(
								kubeClient,
								testDriverName,
								tc.pvcUID,
								tc.pvcName,
								tc.allowedTopologies,
								"",
								strictTopology,
								immediateTopology,
								csiNodeLister,
								nodeLister,
								pvcNodeStore,
							)

							if err != nil {
								t.Fatalf("unexpected error found: %v", err)
							}

							expected := tc.expectedPreferred
							if !immediateTopology && tc.allowedTopologies == nil {
								expected = nil
							}

							if expected == nil && requirements != nil && requirements.Preferred != nil {
								t.Fatalf("expected no preferred but requirements is %v", requirements)
							}
							if expected != nil && requirements == nil {
								t.Fatalf("expected preferred to be %v but requirements is nil", expected)
							}
							if expected != nil && requirements.Preferred == nil {
								t.Fatalf("expected preferred to be %v but requirements.Preferred is nil", expected)
							}
							if expected != nil && !cmp.Equal(requirements.Preferred, expected, protocmp.Transform()) {
								t.Errorf("expected preferred requisite %v; got: %v", expected, requirements.Preferred)
							}
						})
					}
				})
			}
		})
	}
}

func TestAllowedTopologies(t *testing.T) {
	// TODO (verult) more AllowedTopologies unit tests
	testcases := map[string]struct {
		allowedTopologies []v1.TopologySelectorTerm
		expectedRequisite []*csi.Topology
	}{
		"single expression, single value": {
			allowedTopologies: []v1.TopologySelectorTerm{
				{
					MatchLabelExpressions: []v1.TopologySelectorLabelRequirement{
						{
							Key:    "com.example.csi/zone",
							Values: []string{"zone1"},
						},
					},
				},
			},
			expectedRequisite: []*csi.Topology{
				{
					Segments: map[string]string{
						"com.example.csi/zone": "zone1",
					},
				},
			},
		},
		"single expression, multiple values": {
			allowedTopologies: []v1.TopologySelectorTerm{
				{
					MatchLabelExpressions: []v1.TopologySelectorLabelRequirement{
						{
							Key:    "com.example.csi/zone",
							Values: []string{"zone1", "zone2"},
						},
					},
				},
			},
			expectedRequisite: []*csi.Topology{
				{
					Segments: map[string]string{
						"com.example.csi/zone": "zone1",
					},
				},
				{
					Segments: map[string]string{
						"com.example.csi/zone": "zone2",
					},
				},
			},
		},
		"multiple expressions, single value": {
			allowedTopologies: []v1.TopologySelectorTerm{
				{
					MatchLabelExpressions: []v1.TopologySelectorLabelRequirement{
						{
							Key:    "com.example.csi/zone",
							Values: []string{"zone1"},
						},
						{
							Key:    "com.example.csi/rack",
							Values: []string{"rackA"},
						},
					},
				},
			},
			expectedRequisite: []*csi.Topology{
				{
					Segments: map[string]string{
						"com.example.csi/zone": "zone1",
						"com.example.csi/rack": "rackA",
					},
				},
			},
		},
		"multiple expressions, 1 single value, 1 multiple values": {
			allowedTopologies: []v1.TopologySelectorTerm{
				{
					MatchLabelExpressions: []v1.TopologySelectorLabelRequirement{
						{
							Key:    "com.example.csi/zone",
							Values: []string{"zone1"},
						},
						{
							Key:    "com.example.csi/rack",
							Values: []string{"rackA", "rackB"},
						},
					},
				},
			},
			expectedRequisite: []*csi.Topology{
				{
					Segments: map[string]string{
						"com.example.csi/zone": "zone1",
						"com.example.csi/rack": "rackA",
					},
				},
				{
					Segments: map[string]string{
						"com.example.csi/zone": "zone1",
						"com.example.csi/rack": "rackB",
					},
				},
			},
		},
		"multiple expressions, both multiple values": {
			allowedTopologies: []v1.TopologySelectorTerm{
				{
					MatchLabelExpressions: []v1.TopologySelectorLabelRequirement{
						{
							Key:    "com.example.csi/zone",
							Values: []string{"zone1", "zone2"},
						},
						{
							Key:    "com.example.csi/rack",
							Values: []string{"rackA", "rackB"},
						},
					},
				},
			},
			expectedRequisite: []*csi.Topology{
				{
					Segments: map[string]string{
						"com.example.csi/zone": "zone1",
						"com.example.csi/rack": "rackA",
					},
				},
				{
					Segments: map[string]string{
						"com.example.csi/zone": "zone1",
						"com.example.csi/rack": "rackB",
					},
				},
				{
					Segments: map[string]string{
						"com.example.csi/zone": "zone2",
						"com.example.csi/rack": "rackA",
					},
				},
				{
					Segments: map[string]string{
						"com.example.csi/zone": "zone2",
						"com.example.csi/rack": "rackB",
					},
				},
			},
		},
		"multiple terms: single expression, single values": {
			allowedTopologies: []v1.TopologySelectorTerm{
				{
					MatchLabelExpressions: []v1.TopologySelectorLabelRequirement{
						{
							Key:    "com.example.csi/zone",
							Values: []string{"zone1"},
						},
					},
				},
				{
					MatchLabelExpressions: []v1.TopologySelectorLabelRequirement{
						{
							Key:    "com.example.csi/rack",
							Values: []string{"rackA"},
						},
					},
				},
			},
			expectedRequisite: []*csi.Topology{
				{
					Segments: map[string]string{
						"com.example.csi/zone": "zone1",
					},
				},
				{
					Segments: map[string]string{
						"com.example.csi/rack": "rackA",
					},
				},
			},
		},
		"multiple terms: single expression, overlapping keys, distinct values": {
			allowedTopologies: []v1.TopologySelectorTerm{
				{
					MatchLabelExpressions: []v1.TopologySelectorLabelRequirement{
						{
							Key:    "com.example.csi/zone",
							Values: []string{"zone1"},
						},
					},
				},
				{
					MatchLabelExpressions: []v1.TopologySelectorLabelRequirement{
						{
							Key:    "com.example.csi/zone",
							Values: []string{"zone2"},
						},
					},
				},
			},
			expectedRequisite: []*csi.Topology{
				{
					Segments: map[string]string{
						"com.example.csi/zone": "zone1",
					},
				},
				{
					Segments: map[string]string{
						"com.example.csi/zone": "zone2",
					},
				},
			},
		},
		"multiple terms: single expression, overlapping keys, overlapping values": {
			allowedTopologies: []v1.TopologySelectorTerm{
				{
					MatchLabelExpressions: []v1.TopologySelectorLabelRequirement{
						{
							Key:    "com.example.csi/zone",
							Values: []string{"zone1", "zone2"},
						},
					},
				},
				{
					MatchLabelExpressions: []v1.TopologySelectorLabelRequirement{
						{
							Key:    "com.example.csi/zone",
							Values: []string{"zone2", "zone3"},
						},
					},
				},
			},
			expectedRequisite: []*csi.Topology{
				{
					Segments: map[string]string{
						"com.example.csi/zone": "zone1",
					},
				},
				{
					Segments: map[string]string{
						"com.example.csi/zone": "zone2",
					},
				},
				{
					Segments: map[string]string{
						"com.example.csi/zone": "zone3",
					},
				},
			},
		},
		//// TODO (verult) advanced reduction could eliminate subset duplicates here
		"multiple terms: 1 single expression, 1 multiple expressions; contains a subset duplicate": {
			allowedTopologies: []v1.TopologySelectorTerm{
				{
					MatchLabelExpressions: []v1.TopologySelectorLabelRequirement{
						{
							Key:    "com.example.csi/zone",
							Values: []string{"zone1"},
						},
					},
				},
				{
					MatchLabelExpressions: []v1.TopologySelectorLabelRequirement{
						{
							Key:    "com.example.csi/zone",
							Values: []string{"zone1", "zone2"},
						},
						{
							Key:    "com.example.csi/rack",
							Values: []string{"rackA", "rackB"},
						},
					},
				},
			},
			expectedRequisite: []*csi.Topology{
				{
					Segments: map[string]string{
						"com.example.csi/zone": "zone1",
					},
				},
				{
					Segments: map[string]string{
						"com.example.csi/zone": "zone1",
						"com.example.csi/rack": "rackA",
					},
				},
				{
					Segments: map[string]string{
						"com.example.csi/zone": "zone1",
						"com.example.csi/rack": "rackB",
					},
				},
				{
					Segments: map[string]string{
						"com.example.csi/zone": "zone2",
						"com.example.csi/rack": "rackA",
					},
				},
				{
					Segments: map[string]string{
						"com.example.csi/zone": "zone2",
						"com.example.csi/rack": "rackB",
					},
				},
			},
		},
		"multiple terms: both contains multiple expressions; contains an identical duplicate": {
			allowedTopologies: []v1.TopologySelectorTerm{
				{
					MatchLabelExpressions: []v1.TopologySelectorLabelRequirement{
						{
							Key:    "com.example.csi/zone",
							Values: []string{"zone1"},
						},
						{
							Key:    "com.example.csi/rack",
							Values: []string{"rackB"},
						},
					},
				},
				{
					MatchLabelExpressions: []v1.TopologySelectorLabelRequirement{
						{
							Key:    "com.example.csi/zone",
							Values: []string{"zone1", "zone2"},
						},
						{
							Key:    "com.example.csi/rack",
							Values: []string{"rackA", "rackB"},
						},
					},
				},
			},
			expectedRequisite: []*csi.Topology{
				{
					Segments: map[string]string{
						"com.example.csi/zone": "zone1",
						"com.example.csi/rack": "rackA",
					},
				},
				{
					Segments: map[string]string{
						"com.example.csi/zone": "zone1",
						"com.example.csi/rack": "rackB",
					},
				},
				{
					Segments: map[string]string{
						"com.example.csi/zone": "zone2",
						"com.example.csi/rack": "rackA",
					},
				},
				{
					Segments: map[string]string{
						"com.example.csi/zone": "zone2",
						"com.example.csi/rack": "rackB",
					},
				},
			},
		},
	}

	for name, tc := range testcases {
		t.Run(name, func(t *testing.T) {
			pvcNodeStore := NewInMemoryStore()
			for strictTopology, withOrWithout := range withWithout {
				t.Run(withOrWithout+" strict topology", func(t *testing.T) {
					for immediateTopology, withOrWithout := range withWithout {
						t.Run(withOrWithout+" immediate topology", func(t *testing.T) {
							requirements, err := GenerateAccessibilityRequirements(
								nil,                                    /* kubeClient */
								"test-driver",                          /* driverName */
								"a9e2d5a3-1c64-4787-97b7-1a22d5a0b123", /* PVC UID */
								"testpvc",
								tc.allowedTopologies,
								"", /* selectedNode */
								strictTopology,
								immediateTopology,
								nil,
								nil,
								pvcNodeStore,
							)

							if err != nil {
								t.Fatalf("expected no error but got: %v", err)
							}
							if requirements == nil {
								t.Fatalf("expected requirements not to be nil")
							}
							if !requisiteEqual(requirements.Requisite, tc.expectedRequisite) {
								t.Errorf("expected requisite %v; got: %v", tc.expectedRequisite, requirements.Requisite)
							}
						})
					}
				})
			}
		})
	}
}

func TestTopologyAggregation(t *testing.T) {
	// Note: all test cases below include topology from another driver, in addition to the driver
	//       specified in the test case.
	testcases := map[string]struct {
		nodeLabels              []map[string]string
		topologyKeys            []map[string][]string
		hasSelectedNode         bool // if set, the first map in nodeLabels is for the selected node.
		expectedRequisite       []*csi.Topology
		expectedStrictRequisite []*csi.Topology
		expectError             bool
	}{
		"same keys and values across cluster": {
			nodeLabels: []map[string]string{
				{"com.example.csi/zone": "zone1"},
				{"com.example.csi/zone": "zone1"},
				{"com.example.csi/zone": "zone1"},
			},
			topologyKeys: []map[string][]string{
				{testDriverName: []string{"com.example.csi/zone"}},
				{testDriverName: []string{"com.example.csi/zone"}},
				{testDriverName: []string{"com.example.csi/zone"}},
			},
			expectedRequisite: []*csi.Topology{
				{Segments: map[string]string{"com.example.csi/zone": "zone1"}},
			},
		},
		"selected node; same keys and values across cluster": {
			hasSelectedNode: true,
			nodeLabels: []map[string]string{
				{"com.example.csi/zone": "zone1"},
				{"com.example.csi/zone": "zone1"},
				{"com.example.csi/zone": "zone1"},
			},
			topologyKeys: []map[string][]string{
				{testDriverName: []string{"com.example.csi/zone"}},
				{testDriverName: []string{"com.example.csi/zone"}},
				{testDriverName: []string{"com.example.csi/zone"}},
			},
			expectedRequisite: []*csi.Topology{
				{Segments: map[string]string{"com.example.csi/zone": "zone1"}},
			},
		},
		"different values across cluster": {
			nodeLabels: []map[string]string{
				{"com.example.csi/zone": "zone1"},
				{"com.example.csi/zone": "zone2"},
				{"com.example.csi/zone": "zone2"},
			},
			topologyKeys: []map[string][]string{
				{testDriverName: []string{"com.example.csi/zone"}},
				{testDriverName: []string{"com.example.csi/zone"}},
				{testDriverName: []string{"com.example.csi/zone"}},
			},
			expectedRequisite: []*csi.Topology{
				{Segments: map[string]string{"com.example.csi/zone": "zone1"}},
				{Segments: map[string]string{"com.example.csi/zone": "zone2"}},
			},
		},
		"selected node; different values across cluster": {
			hasSelectedNode: true,
			nodeLabels: []map[string]string{
				{"com.example.csi/zone": "zone1"},
				{"com.example.csi/zone": "zone2"},
				{"com.example.csi/zone": "zone2"},
			},
			topologyKeys: []map[string][]string{
				{testDriverName: []string{"com.example.csi/zone"}},
				{testDriverName: []string{"com.example.csi/zone"}},
				{testDriverName: []string{"com.example.csi/zone"}},
			},
			expectedRequisite: []*csi.Topology{
				{Segments: map[string]string{"com.example.csi/zone": "zone1"}},
				{Segments: map[string]string{"com.example.csi/zone": "zone2"}},
			},
			expectedStrictRequisite: []*csi.Topology{
				{Segments: map[string]string{"com.example.csi/zone": "zone1"}},
			},
		},
		//"different keys across cluster": {
		//	nodeLabels: []map[string]string{
		//		{ "com.example.csi/zone": "zone1" },
		//		{ "com.example.csi/zone": "zone1", "com.example.csi/rack": "rackA" },
		//		{ "com.example.csi/zone": "zone1", "com.example.csi/rack": "rackA" },
		//	},
		//	topologyKeys: []map[string][]string{
		//		{ testDriverName: []string{ "com.example.csi/zone" } },
		//		{ testDriverName: []string{ "com.example.csi/zone", "com.example.csi/rack" } },
		//		{ testDriverName: []string{ "com.example.csi/zone", "com.example.csi/rack" } },
		//	},
		//	expectedRequisite: &csi.TopologyRequirement{
		//		Requisite: []*csi.Topology{
		//			{ Segments: map[string]string{ "com.example.csi/zone": "zone1", "com.example.csi/rack": "rackA" } },
		//		},
		//	},
		//	// TODO (verult) mock Rand
		//},
		"selected node: different keys across cluster": {
			hasSelectedNode: true,
			nodeLabels: []map[string]string{
				{"com.example.csi/zone": "zone1"},
				{"com.example.csi/zone": "zone1", "com.example.csi/rack": "rackA"},
				{"com.example.csi/zone": "zone1", "com.example.csi/rack": "rackA"},
			},
			topologyKeys: []map[string][]string{
				{testDriverName: []string{"com.example.csi/zone"}},
				{testDriverName: []string{"com.example.csi/zone", "com.example.csi/rack"}},
				{testDriverName: []string{"com.example.csi/zone", "com.example.csi/rack"}},
			},
			expectedRequisite: []*csi.Topology{
				{Segments: map[string]string{"com.example.csi/zone": "zone1"}},
			},
		},
		"random node: no nodes": {
			nodeLabels: nil,
			topologyKeys: []map[string][]string{
				{testDriverName: []string{"com.example.csi/zone"}},
				{testDriverName: []string{"com.example.csi/zone"}},
				{testDriverName: []string{"com.example.csi/zone"}},
			},
			expectedRequisite: nil,
			expectError:       true,
		},
		"random node: missing matching node info": {
			nodeLabels: []map[string]string{
				{"com.example.csi/foo": "bar"},
			},
			topologyKeys: []map[string][]string{
				{testDriverName: []string{"com.example.csi/zone"}},
				{testDriverName: []string{"com.example.csi/zone"}},
				{testDriverName: []string{"com.example.csi/zone"}},
			},
			expectedRequisite: nil,
			expectError:       true,
		},
		// Driver has not been registered on any nodes
		"random node: no CSINodes": {
			nodeLabels: []map[string]string{
				{"com.example.csi/zone": "zone1"},
				{"com.example.csi/zone": "zone2"},
				{"com.example.csi/zone": "zone2"},
			},
			topologyKeys:      nil,
			expectedRequisite: nil,
			expectError:       true,
		},
		// Driver on node has not been updated to report topology keys
		"random node: missing keys": {
			nodeLabels: []map[string]string{
				{"com.example.csi/zone": "zone1"},
				{"com.example.csi/zone": "zone2"},
				{"com.example.csi/zone": "zone2"},
			},
			topologyKeys: []map[string][]string{
				{testDriverName: nil},
				{testDriverName: nil},
				{testDriverName: nil},
			},
			expectedRequisite: nil,
			expectError:       true,
		},
		"random node: one node has been upgraded": {
			nodeLabels: []map[string]string{
				{"com.example.another/zone": "zone1"},
				{"com.example.another/zone": "zone2"},
				{"com.example.csi/zone": "zone3"},
			},
			topologyKeys: []map[string][]string{
				{testDriverName: nil},
				{testDriverName: nil},
				{testDriverName: []string{"com.example.csi/zone"}},
			},
			expectedRequisite: []*csi.Topology{
				{Segments: map[string]string{"com.example.csi/zone": "zone3"}},
			},
		},
		"random node: node labels already exist without CSINode": {
			nodeLabels: []map[string]string{
				{"com.example.csi/zone": "zone1"},
				{"com.example.csi/zone": "zone2"},
				{"com.example.csi/zone": "zone3"},
			},
			topologyKeys: []map[string][]string{
				{testDriverName: nil},
				{testDriverName: nil},
				{testDriverName: []string{"com.example.csi/zone"}},
			},
			expectedRequisite: []*csi.Topology{
				{Segments: map[string]string{"com.example.csi/zone": "zone1"}},
				{Segments: map[string]string{"com.example.csi/zone": "zone2"}},
				{Segments: map[string]string{"com.example.csi/zone": "zone3"}},
			},
		},
		"selected node: missing matching node info": {
			hasSelectedNode: true,
			nodeLabels: []map[string]string{
				{"com.example.csi/foo": "bar"},
			},
			topologyKeys: []map[string][]string{
				{testDriverName: []string{"com.example.csi/zone"}},
				{testDriverName: []string{"com.example.csi/zone"}},
				{testDriverName: []string{"com.example.csi/zone"}},
			},
			expectedRequisite: nil,
			expectError:       true,
		},
		"selected node: no CSINode info": {
			hasSelectedNode: true,
			nodeLabels: []map[string]string{
				{"com.example.csi/zone": "zone1"},
				{"com.example.csi/zone": "zone2"},
				{"com.example.csi/zone": "zone2"},
			},
			topologyKeys:      nil,
			expectedRequisite: nil,
			expectError:       true,
		},
		"selected node is missing keys": {
			hasSelectedNode: true,
			nodeLabels: []map[string]string{
				{"com.example.csi/zone": "zone1"},
				{"com.example.csi/zone": "zone2"},
				{"com.example.csi/zone": "zone2"},
			},
			topologyKeys: []map[string][]string{
				{testDriverName: nil},
				{testDriverName: nil},
				{testDriverName: nil},
			},
			expectedRequisite: nil,
			expectError:       true,
		},
	}

	for name, tc := range testcases {
		t.Run(name, func(t *testing.T) {
			for strictTopology, withOrWithout := range withWithout {
				t.Run(withOrWithout+" strict topology", func(t *testing.T) {
					for immediateTopology, withOrWithout := range withWithout {
						t.Run(withOrWithout+" immediate topology", func(t *testing.T) {
							nodes := buildNodes(tc.nodeLabels)
							csiNodes := buildCSINodes(tc.topologyKeys)
							pvc := &v1.PersistentVolumeClaim{
								ObjectMeta: metav1.ObjectMeta{
									Name:      "testpvc",
									Namespace: "default",
								},
							}

							kubeClient := fakeclientset.NewSimpleClientset(nodes, csiNodes, pvc)

							_, csiNodeLister, nodeLister, _, _, stopChan := listers(kubeClient)
							pvcNodeStore := NewInMemoryStore()
							defer close(stopChan)

							var selectedNode *v1.Node
							var selectedNodeName string
							if tc.hasSelectedNode {
								selectedNode = &nodes.Items[0]
								selectedNodeName = selectedNode.Name
							}

							requirements, err := GenerateAccessibilityRequirements(
								kubeClient,
								testDriverName,
								"a9e2d5a3-1c64-4787-97b7-1a22d5a0b123", /* PVC UID */
								"testpvc",
								nil, /* allowedTopologies */
								selectedNodeName,
								strictTopology,
								immediateTopology,
								csiNodeLister,
								nodeLister,
								pvcNodeStore,
							)

							expectError := tc.expectError
							expectedRequisite := tc.expectedRequisite
							if strictTopology && tc.expectedStrictRequisite != nil {
								expectedRequisite = tc.expectedStrictRequisite
							}
							if !immediateTopology && selectedNode == nil {
								expectedRequisite = nil
								expectError = false
							}

							if expectError && err == nil {
								t.Fatalf("expected error but got none")
							}
							if !expectError && err != nil {
								t.Fatalf("expected no error but got: %v", err)
							}

							if expectedRequisite == nil {
								if requirements != nil {
									t.Fatalf("expected requirements to be nil but got requisite: %v", requirements.Requisite)
								}
							} else {
								if requirements == nil {
									t.Fatalf("expected requisite to be %v but requirements is nil", expectedRequisite)
								}
								if !requisiteEqual(requirements.Requisite, expectedRequisite) {
									t.Errorf("expected requisite %v; got: %v", expectedRequisite, requirements.Requisite)
								}
							}
						})
					}
				})
			}
		})
	}
}

func TestPreferredTopologies(t *testing.T) {
	testcases := map[string]struct {
		allowedTopologies       []v1.TopologySelectorTerm
		nodeLabels              []map[string]string   // first node is selected node
		topologyKeys            []map[string][]string // first entry is from the selected node
		expectedPreferred       []*csi.Topology
		expectedStrictPreferred []*csi.Topology
		expectError             bool
	}{
		"allowedTopologies specified": {
			allowedTopologies: []v1.TopologySelectorTerm{
				{
					MatchLabelExpressions: []v1.TopologySelectorLabelRequirement{
						{
							Key:    "com.example.csi/zone",
							Values: []string{"zone1", "zone2"},
						},
						{
							Key:    "com.example.csi/rack",
							Values: []string{"rackA", "rackB"},
						},
					},
				},
			},
			nodeLabels: []map[string]string{
				{"com.example.csi/zone": "zone2", "com.example.csi/rack": "rackA"},
				{"com.example.csi/zone": "zone1", "com.example.csi/rack": "rackA"},
				{"com.example.csi/zone": "zone1", "com.example.csi/rack": "rackB"},
			},
			topologyKeys: []map[string][]string{
				{testDriverName: []string{"com.example.csi/zone", "com.example.csi/rack"}},
				{testDriverName: []string{"com.example.csi/zone", "com.example.csi/rack"}},
				{testDriverName: []string{"com.example.csi/zone", "com.example.csi/rack"}},
			},
			expectedPreferred: []*csi.Topology{
				{
					Segments: map[string]string{
						"com.example.csi/rack": "rackA",
						"com.example.csi/zone": "zone2",
					},
				},
				{
					Segments: map[string]string{
						"com.example.csi/rack": "rackB",
						"com.example.csi/zone": "zone1",
					},
				},
				{
					Segments: map[string]string{
						"com.example.csi/rack": "rackB",
						"com.example.csi/zone": "zone2",
					},
				},
				{
					Segments: map[string]string{
						"com.example.csi/rack": "rackA",
						"com.example.csi/zone": "zone1",
					},
				},
			},
			expectedStrictPreferred: []*csi.Topology{
				{
					Segments: map[string]string{
						"com.example.csi/rack": "rackA",
						"com.example.csi/zone": "zone2",
					},
				},
			},
		},
		"allowedTopologies specified: no CSINode": {
			allowedTopologies: []v1.TopologySelectorTerm{
				{
					MatchLabelExpressions: []v1.TopologySelectorLabelRequirement{
						{
							Key:    "com.example.csi/zone",
							Values: []string{"zone1", "zone2"},
						},
						{
							Key:    "com.example.csi/rack",
							Values: []string{"rackA", "rackB"},
						},
					},
				},
			},
			nodeLabels: []map[string]string{
				{"com.example.csi/zone": "zone2", "com.example.csi/rack": "rackA"},
				{"com.example.csi/zone": "zone1", "com.example.csi/rack": "rackA"},
				{"com.example.csi/zone": "zone1", "com.example.csi/rack": "rackB"},
			},
			topologyKeys: nil,
			expectError:  true,
		},
		"allowedTopologies specified: mismatched key": {
			allowedTopologies: []v1.TopologySelectorTerm{
				{
					MatchLabelExpressions: []v1.TopologySelectorLabelRequirement{
						{
							Key:    "com.example.csi/zone",
							Values: []string{"zone1", "zone2"},
						},
						{
							Key:    "com.example.csi/rack",
							Values: []string{"rackA", "rackB"},
						},
					},
				},
			},
			nodeLabels: []map[string]string{
				{"com.example.csi/zone": "zone2", "com.example.csi/rack": "rackA"},
				{"com.example.csi/zone": "zone1", "com.example.csi/rack": "rackA"},
				{"com.example.csi/zone": "zone1", "com.example.csi/rack": "rackB"},
			},
			topologyKeys: []map[string][]string{
				{testDriverName: []string{"com.example.csi/foo", "com.example.csi/rack"}},
				{testDriverName: []string{"com.example.csi/foo", "com.example.csi/rack"}},
				{testDriverName: []string{"com.example.csi/foo", "com.example.csi/rack"}},
			},
			expectError: true,
		},
		"topology aggregation": {
			nodeLabels: []map[string]string{
				{"com.example.csi/zone": "zone2", "com.example.csi/rack": "rackA"},
				{"com.example.csi/zone": "zone1", "com.example.csi/rack": "rackA"},
				{"com.example.csi/zone": "zone1", "com.example.csi/rack": "rackB"},
			},
			topologyKeys: []map[string][]string{
				{testDriverName: []string{"com.example.csi/zone", "com.example.csi/rack"}},
				{testDriverName: []string{"com.example.csi/zone", "com.example.csi/rack"}},
				{testDriverName: []string{"com.example.csi/zone", "com.example.csi/rack"}},
			},
			expectedPreferred: []*csi.Topology{
				{
					Segments: map[string]string{
						"com.example.csi/rack": "rackA",
						"com.example.csi/zone": "zone2",
					},
				},
				{
					Segments: map[string]string{
						"com.example.csi/rack": "rackB",
						"com.example.csi/zone": "zone1",
					},
				},
				{
					Segments: map[string]string{
						"com.example.csi/rack": "rackA",
						"com.example.csi/zone": "zone1",
					},
				},
			},
			expectedStrictPreferred: []*csi.Topology{
				{
					Segments: map[string]string{
						"com.example.csi/rack": "rackA",
						"com.example.csi/zone": "zone2",
					},
				},
			},
		},
		"topology aggregation with no topology info": {
			nodeLabels:        []map[string]string{{}, {}, {}},
			topologyKeys:      []map[string][]string{{}, {}, {}},
			expectedPreferred: nil,
			expectError:       true,
		},
		// This case is never triggered in reality due to scheduler behavior
		"topology from selected node is not in allowedTopologies": {
			allowedTopologies: []v1.TopologySelectorTerm{
				{
					MatchLabelExpressions: []v1.TopologySelectorLabelRequirement{
						{
							Key:    "com.example.csi/zone",
							Values: []string{"zone1"},
						},
					},
				},
			},
			nodeLabels: []map[string]string{
				{"com.example.csi/zone": "zone2"},
				{"com.example.csi/zone": "zone1"},
				{"com.example.csi/zone": "zone1"},
			},
			topologyKeys: []map[string][]string{
				{testDriverName: []string{"com.example.csi/zone"}},
				{testDriverName: []string{"com.example.csi/zone"}},
				{testDriverName: []string{"com.example.csi/zone"}},
			},
			expectError: true,
		},
		"allowedTopologies is subset of selected node's topology": {
			allowedTopologies: []v1.TopologySelectorTerm{
				{
					MatchLabelExpressions: []v1.TopologySelectorLabelRequirement{
						{
							Key:    "com.example.csi/disk",
							Values: []string{"ssd"},
						},
						{
							Key:    "com.example.csi/zone",
							Values: []string{"zone1"},
						},
					},
				},
			},
			nodeLabels: []map[string]string{
				{"com.example.csi/zone": "zone1", "com.example.csi/rack": "rack1", "com.example.csi/disk": "ssd"},
				{"com.example.csi/zone": "zone2", "com.example.csi/rack": "rack2", "com.example.csi/disk": "ssd"},
				{"com.example.csi/zone": "zone2", "com.example.csi/rack": "rack1", "com.example.csi/disk": "nvme"},
			},
			topologyKeys: []map[string][]string{
				{testDriverName: []string{"com.example.csi/zone", "com.example.csi/rack", "com.example.csi/disk"}},
				{testDriverName: []string{"com.example.csi/zone", "com.example.csi/rack", "com.example.csi/disk"}},
				{testDriverName: []string{"com.example.csi/zone", "com.example.csi/rack", "com.example.csi/disk"}},
			},
			expectError: false,
			expectedPreferred: []*csi.Topology{
				{
					Segments: map[string]string{
						"com.example.csi/disk": "ssd",
						"com.example.csi/zone": "zone1",
					},
				},
			},
			expectedStrictPreferred: []*csi.Topology{
				{
					Segments: map[string]string{
						"com.example.csi/disk": "ssd",
						"com.example.csi/rack": "rack1",
						"com.example.csi/zone": "zone1",
					},
				},
			},
		},
		"allowedTopologies is not the subset of selected node's topology": {
			allowedTopologies: []v1.TopologySelectorTerm{
				{
					MatchLabelExpressions: []v1.TopologySelectorLabelRequirement{
						{
							Key:    "com.example.csi/zone",
							Values: []string{"zone1"},
						},
					},
				},
			},
			nodeLabels: []map[string]string{
				{"com.example.csi/zone": "zone10", "com.example.csi/rack": "rack1"},
				{"com.example.csi/zone": "zone11", "com.example.csi/rack": "rack2"},
				{"com.example.csi/zone": "zone12", "com.example.csi/rack": "rack1"},
			},
			topologyKeys: []map[string][]string{
				{testDriverName: []string{"com.example.csi/zone", "com.example.csi/rack"}},
				{testDriverName: []string{"com.example.csi/zone", "com.example.csi/rack"}},
				{testDriverName: []string{"com.example.csi/zone", "com.example.csi/rack"}},
			},
			expectError: true,
		},
	}

	for name, tc := range testcases {
		t.Run(name, func(t *testing.T) {
			for strictTopology, withOrWithout := range withWithout {
				t.Run(withOrWithout+" strict topology", func(t *testing.T) {
					for immediateTopology, withOrWithout := range withWithout {
						t.Run(withOrWithout+" immediate topology", func(t *testing.T) {
							nodes := buildNodes(tc.nodeLabels)
							csiNodes := buildCSINodes(tc.topologyKeys)

							pvc := &v1.PersistentVolumeClaim{
								ObjectMeta: metav1.ObjectMeta{
									Name:      "testpvc",
									Namespace: "default",
								},
							}

							kubeClient := fakeclientset.NewSimpleClientset(nodes, csiNodes, pvc)
							selectedNode := &nodes.Items[0]

							_, csiNodeLister, nodeLister, _, _, stopChan := listers(kubeClient)
							pvcNodeStore := NewInMemoryStore()
							defer close(stopChan)

							requirements, err := GenerateAccessibilityRequirements(
								kubeClient,
								testDriverName,
								"a9e2d5a3-1c64-4787-97b7-1a22d5a0b123", /* PVC UID */
								"testpvc",
								tc.allowedTopologies,
								selectedNode.Name,
								strictTopology,
								immediateTopology,
								csiNodeLister,
								nodeLister,
								pvcNodeStore,
							)

							if tc.expectError && err == nil {
								t.Fatalf("expected error but got none")
							}
							if !tc.expectError && err != nil {
								t.Fatalf("expected no error but got: %v", err)
							}
							expectedPreferred := tc.expectedPreferred
							if strictTopology && tc.expectedStrictPreferred != nil {
								expectedPreferred = tc.expectedStrictPreferred
							}
							if expectedPreferred == nil {
								if requirements != nil {
									t.Fatalf("expected requirements to be nil but got preferred: %v", requirements.Preferred)
								}
							} else {
								if requirements == nil {
									t.Fatalf("expected preferred to be %v but requirements is nil", expectedPreferred)
								}
								if !cmp.Equal(requirements.Preferred, expectedPreferred, protocmp.Transform()) {
									t.Errorf("expected requisite %v; got: %v", tc.expectedPreferred, requirements.Preferred)
								}
							}
						})
					}
				})
			}
		})
	}
}

func TestProvisionWithDeletedNodeFromCache(t *testing.T) {
	nodeLabels := []map[string]string{
		{"com.example.csi/zone": "zone1", "com.example.csi/rack": "rackA"},
		{"com.example.csi/zone": "zone2", "com.example.csi/rack": "rackB"},
		{"com.example.csi/zone": "zone3", "com.example.csi/rack": "rackC"},
	}
	topologyKeys := []map[string][]string{
		{testDriverName: {"com.example.csi/zone", "com.example.csi/rack"}},
		{testDriverName: {"com.example.csi/zone", "com.example.csi/rack"}},
		{testDriverName: {"com.example.csi/zone", "com.example.csi/rack"}},
	}
	pvcUID := "a9e2d5a3-1c64-4787-97b7-1a22d5a0b123"
	pvcName := "testpvc"

	testcases := []struct {
		name              string
		strictTopology    bool
		immediateTopology bool
		allowedTopologies []v1.TopologySelectorTerm
		selectedNode      string
		expectedRequisite []*csi.Topology
		expectedPreferred []*csi.Topology
	}{
		{
			name:              "delayed-binding, non-strict, without-allowed-topology",
			strictTopology:    false,
			immediateTopology: false,
			allowedTopologies: nil,
			selectedNode:      "node-0",
			expectedRequisite: []*csi.Topology{
				{Segments: map[string]string{"com.example.csi/zone": "zone1", "com.example.csi/rack": "rackA"}},
				{Segments: map[string]string{"com.example.csi/zone": "zone2", "com.example.csi/rack": "rackB"}},
				{Segments: map[string]string{"com.example.csi/zone": "zone3", "com.example.csi/rack": "rackC"}},
			},
			expectedPreferred: []*csi.Topology{
				{Segments: map[string]string{"com.example.csi/zone": "zone1", "com.example.csi/rack": "rackA"}},
				{Segments: map[string]string{"com.example.csi/zone": "zone2", "com.example.csi/rack": "rackB"}},
				{Segments: map[string]string{"com.example.csi/zone": "zone3", "com.example.csi/rack": "rackC"}},
			},
		},
		{
			name:              "delayed-binding, non-strict, without-allowed-topology, different-order",
			strictTopology:    false,
			immediateTopology: false,
			allowedTopologies: nil,
			selectedNode:      "node-1",
			expectedRequisite: []*csi.Topology{
				{Segments: map[string]string{"com.example.csi/zone": "zone1", "com.example.csi/rack": "rackA"}},
				{Segments: map[string]string{"com.example.csi/zone": "zone2", "com.example.csi/rack": "rackB"}},
				{Segments: map[string]string{"com.example.csi/zone": "zone3", "com.example.csi/rack": "rackC"}},
			},
			expectedPreferred: []*csi.Topology{
				{Segments: map[string]string{"com.example.csi/zone": "zone2", "com.example.csi/rack": "rackB"}},
				{Segments: map[string]string{"com.example.csi/zone": "zone3", "com.example.csi/rack": "rackC"}},
				{Segments: map[string]string{"com.example.csi/zone": "zone1", "com.example.csi/rack": "rackA"}},
			},
		},
		{
			name:              "delayed-binding, non-strict, with-allowed-topology",
			strictTopology:    false,
			immediateTopology: false,
			allowedTopologies: []v1.TopologySelectorTerm{
				{
					MatchLabelExpressions: []v1.TopologySelectorLabelRequirement{
						{Key: "com.example.csi/zone", Values: []string{"zone1"}},
						{Key: "com.example.csi/rack", Values: []string{"rackA"}},
					},
				},
				{
					MatchLabelExpressions: []v1.TopologySelectorLabelRequirement{
						{Key: "com.example.csi/zone", Values: []string{"zone2"}},
						{Key: "com.example.csi/rack", Values: []string{"rackB"}},
					},
				},
			},
			selectedNode: "node-0",
			expectedRequisite: []*csi.Topology{
				{Segments: map[string]string{"com.example.csi/zone": "zone1", "com.example.csi/rack": "rackA"}},
				{Segments: map[string]string{"com.example.csi/zone": "zone2", "com.example.csi/rack": "rackB"}},
			},
			expectedPreferred: []*csi.Topology{
				{Segments: map[string]string{"com.example.csi/zone": "zone1", "com.example.csi/rack": "rackA"}},
				{Segments: map[string]string{"com.example.csi/zone": "zone2", "com.example.csi/rack": "rackB"}},
			},
		},
		{
			name:              "delayed-binding, strict, without-allowed-topology",
			strictTopology:    true,
			immediateTopology: false,
			allowedTopologies: nil,
			selectedNode:      "node-0",
			expectedRequisite: []*csi.Topology{
				{Segments: map[string]string{"com.example.csi/zone": "zone1", "com.example.csi/rack": "rackA"}},
			},
			expectedPreferred: []*csi.Topology{
				{Segments: map[string]string{"com.example.csi/zone": "zone1", "com.example.csi/rack": "rackA"}},
			},
		},
		{
			name:              "delayed-binding, strict, with-allowed-topology",
			strictTopology:    true,
			immediateTopology: false,
			allowedTopologies: []v1.TopologySelectorTerm{
				{
					MatchLabelExpressions: []v1.TopologySelectorLabelRequirement{
						{Key: "com.example.csi/zone", Values: []string{"zone1"}},
						{Key: "com.example.csi/rack", Values: []string{"rackA"}},
					},
				},
				{
					MatchLabelExpressions: []v1.TopologySelectorLabelRequirement{
						{Key: "com.example.csi/zone", Values: []string{"zone2"}},
						{Key: "com.example.csi/rack", Values: []string{"rackB"}},
					},
				},
			},
			selectedNode: "node-0",
			expectedRequisite: []*csi.Topology{
				{Segments: map[string]string{"com.example.csi/zone": "zone1", "com.example.csi/rack": "rackA"}},
			},
			expectedPreferred: []*csi.Topology{
				{Segments: map[string]string{"com.example.csi/zone": "zone1", "com.example.csi/rack": "rackA"}},
			},
		},
		{
			name:              "immediate-no-selected-node, non-strict, without-allowed-topology",
			strictTopology:    false,
			immediateTopology: true,
			allowedTopologies: nil,
			selectedNode:      "",
			expectedRequisite: []*csi.Topology{
				{Segments: map[string]string{"com.example.csi/zone": "zone1", "com.example.csi/rack": "rackA"}},
				{Segments: map[string]string{"com.example.csi/zone": "zone2", "com.example.csi/rack": "rackB"}},
				{Segments: map[string]string{"com.example.csi/zone": "zone3", "com.example.csi/rack": "rackC"}},
			},
			expectedPreferred: []*csi.Topology{
				{Segments: map[string]string{"com.example.csi/zone": "zone3", "com.example.csi/rack": "rackC"}},
				{Segments: map[string]string{"com.example.csi/zone": "zone1", "com.example.csi/rack": "rackA"}},
				{Segments: map[string]string{"com.example.csi/zone": "zone2", "com.example.csi/rack": "rackB"}},
			},
		},
		{
			name:              "immediate-no-selected-node, non-strict, with-allowed-topology",
			strictTopology:    false,
			immediateTopology: true,
			allowedTopologies: []v1.TopologySelectorTerm{
				{
					MatchLabelExpressions: []v1.TopologySelectorLabelRequirement{
						{Key: "com.example.csi/zone", Values: []string{"zone1"}},
						{Key: "com.example.csi/rack", Values: []string{"rackA"}},
					},
				},
				{
					MatchLabelExpressions: []v1.TopologySelectorLabelRequirement{
						{Key: "com.example.csi/zone", Values: []string{"zone2"}},
						{Key: "com.example.csi/rack", Values: []string{"rackB"}},
					},
				},
			},
			selectedNode: "",
			expectedRequisite: []*csi.Topology{
				{Segments: map[string]string{"com.example.csi/zone": "zone1", "com.example.csi/rack": "rackA"}},
				{Segments: map[string]string{"com.example.csi/zone": "zone2", "com.example.csi/rack": "rackB"}},
			},
			expectedPreferred: []*csi.Topology{
				{Segments: map[string]string{"com.example.csi/zone": "zone1", "com.example.csi/rack": "rackA"}},
				{Segments: map[string]string{"com.example.csi/zone": "zone2", "com.example.csi/rack": "rackB"}},
			},
		},
		{
			name:              "immediate-no-selected-node, strict, without-allowed-topology",
			strictTopology:    true,
			immediateTopology: true,
			allowedTopologies: nil,
			selectedNode:      "",
			expectedRequisite: []*csi.Topology{
				{Segments: map[string]string{"com.example.csi/zone": "zone1", "com.example.csi/rack": "rackA"}},
				{Segments: map[string]string{"com.example.csi/zone": "zone2", "com.example.csi/rack": "rackB"}},
				{Segments: map[string]string{"com.example.csi/zone": "zone3", "com.example.csi/rack": "rackC"}},
			},
			expectedPreferred: []*csi.Topology{
				{Segments: map[string]string{"com.example.csi/zone": "zone3", "com.example.csi/rack": "rackC"}},
				{Segments: map[string]string{"com.example.csi/zone": "zone1", "com.example.csi/rack": "rackA"}},
				{Segments: map[string]string{"com.example.csi/zone": "zone2", "com.example.csi/rack": "rackB"}},
			},
		},
		{
			name:              "immediate-no-selected-node, strict, with-allowed-topology",
			strictTopology:    true,
			immediateTopology: true,
			allowedTopologies: []v1.TopologySelectorTerm{
				{
					MatchLabelExpressions: []v1.TopologySelectorLabelRequirement{
						{Key: "com.example.csi/zone", Values: []string{"zone1"}},
						{Key: "com.example.csi/rack", Values: []string{"rackA"}},
					},
				},
				{
					MatchLabelExpressions: []v1.TopologySelectorLabelRequirement{
						{Key: "com.example.csi/zone", Values: []string{"zone2"}},
						{Key: "com.example.csi/rack", Values: []string{"rackB"}},
					},
				},
			},
			selectedNode: "",
			expectedRequisite: []*csi.Topology{
				{Segments: map[string]string{"com.example.csi/zone": "zone1", "com.example.csi/rack": "rackA"}},
				{Segments: map[string]string{"com.example.csi/zone": "zone2", "com.example.csi/rack": "rackB"}},
			},
			expectedPreferred: []*csi.Topology{
				{Segments: map[string]string{"com.example.csi/zone": "zone1", "com.example.csi/rack": "rackA"}},
				{Segments: map[string]string{"com.example.csi/zone": "zone2", "com.example.csi/rack": "rackB"}},
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			nodes := buildNodes(nodeLabels)
			csiNodes := buildCSINodes(topologyKeys)
			kubeClient := fakeclientset.NewSimpleClientset(nodes, csiNodes)

			_, csiNodeLister, nodeLister, _, _, stopChan := listers(kubeClient)
			pvcNodeStore := NewInMemoryStore()
			defer close(stopChan)

			// First call to GenerateAccessibilityRequirements to populate the cache
			requirements, err := GenerateAccessibilityRequirements(
				kubeClient,
				testDriverName,
				types.UID(pvcUID),
				pvcName,
				tc.allowedTopologies,
				tc.selectedNode,
				tc.strictTopology,
				tc.immediateTopology,
				csiNodeLister,
				nodeLister,
				pvcNodeStore,
			)
			if err != nil {
				t.Fatalf("unexpected error found: %v", err)
			}
			if requirements == nil {
				t.Fatalf("expected requirements to be generated")
			}

			// Delete the node from the client
			selectedNode := tc.selectedNode
			if selectedNode == "" {
				selectedNode = "node-0"
			}

			err = kubeClient.CoreV1().Nodes().Delete(context.TODO(), selectedNode, metav1.DeleteOptions{})
			if err != nil {
				t.Fatalf("failed to delete node: %v", err)
			}
			err = kubeClient.StorageV1().CSINodes().Delete(context.TODO(), selectedNode, metav1.DeleteOptions{})
			if err != nil {
				t.Fatalf("failed to delete csinode: %v", err)
			}

			// Re-create listers to reflect the deleted node
			_, csiNodeLister, nodeLister, _, _, stopChan2 := listers(kubeClient)
			defer close(stopChan2)

			// Second call to GenerateAccessibilityRequirements, should use cache
			cachedRequirements, err := GenerateAccessibilityRequirements(
				kubeClient,
				testDriverName,
				types.UID(pvcUID),
				pvcName,
				tc.allowedTopologies,
				tc.selectedNode,
				tc.strictTopology,
				tc.immediateTopology,
				csiNodeLister,
				nodeLister,
				pvcNodeStore,
			)
			if err != nil {
				t.Fatalf("unexpected error on second call: %v", err)
			}

			if !cmp.Equal(requirements, cachedRequirements, protocmp.Transform()) {
				t.Errorf("expected requirements from cache to be the same. got %v, want %v", cachedRequirements, requirements)
			}

			// Also verify the values of Requisite and Preferred terms.
			if !requisiteEqual(cachedRequirements.Requisite, tc.expectedRequisite) {
				t.Errorf("expected requisite %v; got: %v", tc.expectedRequisite, cachedRequirements.Requisite)
			}

			if !cmp.Equal(cachedRequirements.Preferred, tc.expectedPreferred, protocmp.Transform()) {
				t.Errorf("expected preferred %v; got: %v", tc.expectedPreferred, cachedRequirements.Preferred)
			}
		})
	}
}

func BenchmarkDedupAndSortZone(b *testing.B) {
	terms := make([]topologyTerm, 0, 3000)
	for range 1000 {
		for _, zone := range [...]string{"zone1", "zone2", "zone3"} {
			terms = append(terms, topologyTerm{
				{"topology.kubernetes.io/region", "some-region"},
				{"topology.kubernetes.io/zone", zone},
			})
		}
	}
	benchmarkDedupAndSort(b, terms)
}

func BenchmarkDedupAndSortHost(b *testing.B) {
	terms := make([]topologyTerm, 0, 3000)
	for i := range 1000 {
		for j, zone := range [...]string{"zone1", "zone2", "zone3"} {
			terms = append(terms, topologyTerm{
				{"example.com/instance-id", fmt.Sprintf("i-%05d", i+j*10000)},
				{"topology.kubernetes.io/region", "some-region"},
				{"topology.kubernetes.io/zone", zone},
			})
		}
	}
	benchmarkDedupAndSort(b, terms)
}

func benchmarkDedupAndSort(b *testing.B, terms []topologyTerm) {
	for b.Loop() {
		terms := slices.Clone(terms)
		slices.SortFunc(terms, topologyTerm.compare)
		terms = slices.CompactFunc(terms, slices.Equal)
		toCSITopology(terms)
	}
}

func buildNodes(nodeLabels []map[string]string) *v1.NodeList {
	list := &v1.NodeList{}
	i := 0
	for _, l := range nodeLabels {
		node := v1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name:   fmt.Sprintf("node-%d", i),
				Labels: l,
			},
		}
		node.Labels["net.example.storage/rack"] = "rack1"
		list.Items = append(list.Items, node)
		i++
	}

	return list
}

func buildCSINodes(csiNodes []map[string][]string) *storagev1.CSINodeList {
	list := &storagev1.CSINodeList{}
	i := 0
	for _, csiNode := range csiNodes {
		nodeName := fmt.Sprintf("node-%d", i)
		n := storagev1.CSINode{
			ObjectMeta: metav1.ObjectMeta{
				Name: nodeName,
			},
		}
		var csiDrivers []storagev1.CSINodeDriver
		for driver, topologyKeys := range csiNode {
			driverInfos := []storagev1.CSINodeDriver{
				{
					Name:         driver,
					NodeID:       nodeName,
					TopologyKeys: topologyKeys,
				},
				{
					Name:         "net.example.storage/other-driver",
					NodeID:       nodeName,
					TopologyKeys: []string{"net.example.storage/rack"},
				},
			}
			csiDrivers = append(csiDrivers, driverInfos...)
		}
		n.Spec = storagev1.CSINodeSpec{Drivers: csiDrivers}
		list.Items = append(list.Items, n)
		i++
	}

	return list
}

func nodeSelectorRequirementsEqual(r1, r2 v1.NodeSelectorRequirement) bool {
	if r1.Key != r2.Key {
		return false
	}
	if r1.Operator != r2.Operator {
		return false
	}
	vals1 := sets.NewString(r1.Values...)
	vals2 := sets.NewString(r2.Values...)
	return vals1.Equal(vals2)
}

func nodeSelectorTermsEqual(t1, t2 v1.NodeSelectorTerm) bool {
	exprs1 := t1.MatchExpressions
	exprs2 := t2.MatchExpressions
	fields1 := t1.MatchFields
	fields2 := t2.MatchFields
	if len(exprs1) != len(exprs2) {
		return false
	}
	if len(fields1) != len(fields2) {
		return false
	}
	match := func(reqs1, reqs2 []v1.NodeSelectorRequirement) bool {
		for _, req1 := range reqs1 {
			reqMatched := false
			for _, req2 := range reqs2 {
				if nodeSelectorRequirementsEqual(req1, req2) {
					reqMatched = true
					break
				}
			}
			if !reqMatched {
				return false
			}
		}
		return true
	}
	return match(exprs1, exprs2) && match(exprs2, exprs1) && match(fields1, fields2) && match(fields2, fields1)
}

// volumeNodeAffinitiesEqual performs a highly semantic comparison of two VolumeNodeAffinity data structures
// It ignores ordering of instances of NodeSelectorRequirements in a VolumeNodeAffinity's NodeSelectorTerms as well as
// orderding of strings in Values of NodeSelectorRequirements when matching two VolumeNodeAffinity structures.
// Note that in most equality functions, Go considers two slices to be not equal if the order of elements in a slice do not
// match - so reflect.DeepEqual as well as Semantic.DeepEqual do not work for comparing VolumeNodeAffinity semantically.
// e.g. these two NodeSelectorTerms are considered semantically equal by volumeNodeAffinitiesEqual
// &VolumeNodeAffinity{Required:&NodeSelector{NodeSelectorTerms:[{[{a In [1]} {b In [2 3]}] []}],},}
// &VolumeNodeAffinity{Required:&NodeSelector{NodeSelectorTerms:[{[{b In [3 2]} {a In [1]}] []}],},}
// TODO: move to external controller lib utils so other can use it too
func volumeNodeAffinitiesEqual(n1, n2 *v1.VolumeNodeAffinity) bool {
	if (n1 == nil) != (n2 == nil) {
		return false
	}
	if n1 == nil || n2 == nil {
		return true
	}
	ns1 := n1.Required
	ns2 := n2.Required
	if (ns1 == nil) != (ns2 == nil) {
		return false
	}
	if (ns1 == nil) && (ns2 == nil) {
		return true
	}
	if len(ns1.NodeSelectorTerms) != len(ns2.NodeSelectorTerms) {
		return false
	}
	match := func(terms1, terms2 []v1.NodeSelectorTerm) bool {
		for _, term1 := range terms1 {
			termMatched := false
			for _, term2 := range terms2 {
				if nodeSelectorTermsEqual(term1, term2) {
					termMatched = true
					break
				}
			}
			if !termMatched {
				return false
			}
		}
		return true
	}
	return match(ns1.NodeSelectorTerms, ns2.NodeSelectorTerms) && match(ns2.NodeSelectorTerms, ns1.NodeSelectorTerms)
}

func requisiteEqual(t1, t2 []*csi.Topology) bool {
	// Requisite may contain duplicate topologies
	unchecked := make(sets.Int)
	for i := range t1 {
		unchecked.Insert(i)
	}
	for _, topology := range t2 {
		found := false
		for i := range unchecked {
			if cmp.Equal(t1[i], topology, protocmp.Transform()) {
				found = true
				unchecked.Delete(i)
				break
			}
		}
		if !found {
			return false
		}
	}

	return unchecked.Len() == 0
}

func listers(kubeClient *fakeclientset.Clientset) (
	storagelistersv1.StorageClassLister,
	storagelistersv1.CSINodeLister,
	corelisters.NodeLister,
	corelisters.PersistentVolumeClaimLister,
	storagelistersv1.VolumeAttachmentLister,
	chan struct{}) {
	factory := informers.NewSharedInformerFactory(kubeClient, ResyncPeriodOfCsiNodeInformer)
	stopChan := make(chan struct{})
	scLister := factory.Storage().V1().StorageClasses().Lister()
	csiNodeLister := factory.Storage().V1().CSINodes().Lister()
	nodeLister := factory.Core().V1().Nodes().Lister()
	claimLister := factory.Core().V1().PersistentVolumeClaims().Lister()
	vaLister := factory.Storage().V1().VolumeAttachments().Lister()
	factory.Start(stopChan)
	factory.WaitForCacheSync(stopChan)
	return scLister, csiNodeLister, nodeLister, claimLister, vaLister, stopChan
}

func TestTopologyTermSort(t *testing.T) {
	testCases := []struct {
		name            string
		terms, expected topologyTerm
	}{
		{
			name: "empty",
		},
		{
			name: "single-key",
			terms: topologyTerm{
				{"zone", "us-east-1a"},
			},
			expected: topologyTerm{
				{"zone", "us-east-1a"},
			},
		},
		{
			name: "multiple-keys",
			terms: topologyTerm{
				{"zone", "us-east-1a"},
				{"instance", "i-123"},
			},
			expected: topologyTerm{
				{"instance", "i-123"},
				{"zone", "us-east-1a"},
			},
		},
		{
			name: "multiple-values", // should not happen currently
			terms: topologyTerm{
				{"zone", "us-east-1b"},
				{"zone", "us-east-1a"},
			},
			expected: topologyTerm{
				{"zone", "us-east-1a"},
				{"zone", "us-east-1b"},
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.terms.sort()
			assert.Equal(t, tc.expected, tc.terms)
		})
	}
}

func TestTopologyTermCompare(t *testing.T) {
	testCases := []struct {
		name          string
		first, second topologyTerm
	}{
		{
			name:  "shorter",
			first: topologyTerm{},
			second: topologyTerm{
				{"zone", "us-east-1a"},
			},
		},
		{
			name: "key-smaller",
			first: topologyTerm{
				{"instance", "i-123"},
			},
			second: topologyTerm{
				{"zone", "us-east-1a"},
			},
		},
		{
			name: "value-smaller",
			first: topologyTerm{
				{"instance", "i-123"},
				{"zone", "us-east-1a"},
			},
			second: topologyTerm{
				{"instance", "i-123"},
				{"zone", "us-east-1b"},
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Less(t, tc.first.compare(tc.second), 0)
			assert.Greater(t, tc.second.compare(tc.first), 0)
			assert.Equal(t, 0, tc.first.compare(tc.first))
			assert.Equal(t, 0, tc.second.compare(tc.second))
		})
	}
}

func TestTopologyTermSubset(t *testing.T) {
	testCases := []struct {
		name         string
		terms, other topologyTerm
		subset       bool
	}{
		{
			name:   "empty",
			subset: true,
		},
		{
			name:   "shorter",
			terms:  topologyTerm{},
			other:  topologyTerm{{"zone", "us-east-1a"}},
			subset: true,
		},
		{
			name:   "longer",
			terms:  topologyTerm{{"zone", "us-east-1a"}},
			other:  topologyTerm{},
			subset: false,
		},
		{
			name:   "same",
			terms:  topologyTerm{{"zone", "us-east-1a"}},
			other:  topologyTerm{{"zone", "us-east-1a"}},
			subset: true,
		},
		{
			name:   "shorter-2",
			terms:  topologyTerm{{"instance", "i-123"}},
			other:  topologyTerm{{"instance", "i-123"}, {"zone", "us-east-1a"}},
			subset: true,
		},
		{
			name:   "longer-2",
			terms:  topologyTerm{{"instance", "i-123"}, {"zone", "us-east-1a"}},
			other:  topologyTerm{{"instance", "i-123"}},
			subset: false,
		},
		{
			name:   "shorter-3",
			terms:  topologyTerm{{"zone", "us-east-1a"}},
			other:  topologyTerm{{"instance", "i-123"}, {"zone", "us-east-1a"}},
			subset: true,
		},
		{
			name:   "longer-3",
			terms:  topologyTerm{{"instance", "i-123"}, {"zone", "us-east-1a"}},
			other:  topologyTerm{{"zone", "us-east-1a"}},
			subset: false,
		},
		{
			name:   "unrelated",
			terms:  topologyTerm{{"instance", "i-123"}},
			other:  topologyTerm{{"zone", "us-east-1a"}},
			subset: false,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.subset, tc.terms.subset(tc.other))
		})
	}
}
