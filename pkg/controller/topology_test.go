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

	"github.com/container-storage-interface/spec/lib/go/csi"
	v1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	for i := 0; i < len(nodeLabels); i++ {
		topologyKeys = append(topologyKeys, keys)
	}
	// Ordering of segments in preferred array is sensitive to statefulset name portion of pvcName.
	// In the tests below, name of the statefulset: testset is the portion whose hash determines ordering.
	// If statefulset name is changed, make sure expectedPreferred is kept in sync.
	// pvc prefix in pvcName does not have any effect on segment ordering
	testcases := map[string]struct {
		pvcName           string
		allowedTopologies []v1.TopologySelectorTerm
		expectedPreferred []*csi.Topology
	}{
		"select index 0 among nodes for pvc with statefulset name:testset and id:1; ignore claimname:testpvcA": {
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
		"select index 1 among allowedTopologies with multiple terms/multiple requirments for pvc with statefulset name:testset and id:2; ignore claimname:testpvcB": {
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
					for immediateTopology, withOrWithout := range withWithout {
						t.Run(withOrWithout+" immediate topology", func(t *testing.T) {
							requirements, err := GenerateAccessibilityRequirements(
								kubeClient,
								testDriverName,
								tc.pvcName,
								tc.allowedTopologies,
								nil,
								strictTopology,
								immediateTopology,
								csiNodeLister,
								nodeLister,
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
							if expected != nil && !equality.Semantic.DeepEqual(requirements.Preferred, expected) {
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
			for strictTopology, withOrWithout := range withWithout {
				t.Run(withOrWithout+" strict topology", func(t *testing.T) {
					for immediateTopology, withOrWithout := range withWithout {
						t.Run(withOrWithout+" immediate topology", func(t *testing.T) {
							requirements, err := GenerateAccessibilityRequirements(
								nil,           /* kubeClient */
								"test-driver", /* driverName */
								"testpvc",
								tc.allowedTopologies,
								nil, /* selectedNode */
								strictTopology,
								immediateTopology,
								nil,
								nil,
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

							kubeClient := fakeclientset.NewSimpleClientset(nodes, csiNodes)

							_, csiNodeLister, nodeLister, _, _, stopChan := listers(kubeClient)
							defer close(stopChan)

							var selectedNode *v1.Node
							if tc.hasSelectedNode {
								selectedNode = &nodes.Items[0]
							}
							requirements, err := GenerateAccessibilityRequirements(
								kubeClient,
								testDriverName,
								"testpvc",
								nil, /* allowedTopologies */
								selectedNode,
								strictTopology,
								immediateTopology,
								csiNodeLister,
								nodeLister,
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

							kubeClient := fakeclientset.NewSimpleClientset(nodes, csiNodes)
							selectedNode := &nodes.Items[0]

							_, csiNodeLister, nodeLister, _, _, stopChan := listers(kubeClient)
							defer close(stopChan)

							requirements, err := GenerateAccessibilityRequirements(
								kubeClient,
								testDriverName,
								"testpvc",
								tc.allowedTopologies,
								selectedNode,
								strictTopology,
								immediateTopology,
								csiNodeLister,
								nodeLister,
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
								if !equality.Semantic.DeepEqual(requirements.Preferred, expectedPreferred) {
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
	if vals1.Equal(vals2) {
		return true
	}
	return false
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
			if equality.Semantic.DeepEqual(t1[i], topology) {
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
