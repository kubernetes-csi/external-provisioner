/*
Copyright 2026 The Kubernetes Authors.

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
	"testing"

	"github.com/google/go-cmp/cmp"
	v1 "k8s.io/api/core/v1"
)

const (
	snapRegionKey = "topology.kubernetes.io/region"
	snapZoneKey   = "topology.kubernetes.io/zone"
)

func zoneTerm(values ...string) v1.TopologySelectorTerm {
	return v1.TopologySelectorTerm{
		MatchLabelExpressions: []v1.TopologySelectorLabelRequirement{
			{Key: snapZoneKey, Values: values},
		},
	}
}

// singleZoneTerm builds a term with one single-value zone expression, matching
// the flattened shape intersectSnapshotTopology returns.
func singleZoneTerm(value string) v1.TopologySelectorTerm {
	return zoneTerm(value)
}

func regionTerm(values ...string) v1.TopologySelectorTerm {
	return v1.TopologySelectorTerm{
		MatchLabelExpressions: []v1.TopologySelectorLabelRequirement{
			{Key: snapRegionKey, Values: values},
		},
	}
}

// regionZoneTerm builds a flattened region+zone term (segments sorted by key).
func regionZoneTerm(region, zone string) v1.TopologySelectorTerm {
	return v1.TopologySelectorTerm{
		MatchLabelExpressions: []v1.TopologySelectorLabelRequirement{
			{Key: snapRegionKey, Values: []string{region}},
			{Key: snapZoneKey, Values: []string{zone}},
		},
	}
}

// TestIntersectSnapshotTopology covers the Immediate-binding intersection of
// StorageClass.AllowedTopologies with a snapshot's NodeAffinity (KEP-5943).
// The result is a flattened []v1.TopologySelectorTerm (one single-value
// expression per key) suitable to pass to GenerateAccessibilityRequirements.
func TestIntersectSnapshotTopology(t *testing.T) {
	testcases := map[string]struct {
		scTopology   []v1.TopologySelectorTerm
		snapTopology []v1.TopologySelectorTerm
		expected     []v1.TopologySelectorTerm
	}{
		"no snapshot topology falls back to StorageClass terms": {
			scTopology:   []v1.TopologySelectorTerm{zoneTerm("us-west-2a", "us-west-2b")},
			snapTopology: nil,
			expected: []v1.TopologySelectorTerm{
				singleZoneTerm("us-west-2a"),
				singleZoneTerm("us-west-2b"),
			},
		},
		"no StorageClass topology falls back to snapshot terms": {
			scTopology:   nil,
			snapTopology: []v1.TopologySelectorTerm{zoneTerm("us-west-2c")},
			expected: []v1.TopologySelectorTerm{
				singleZoneTerm("us-west-2c"),
			},
		},
		"overlapping zones intersect to the common subset": {
			scTopology:   []v1.TopologySelectorTerm{zoneTerm("us-west-2a", "us-west-2b")},
			snapTopology: []v1.TopologySelectorTerm{zoneTerm("us-west-2b", "us-west-2c")},
			expected: []v1.TopologySelectorTerm{
				singleZoneTerm("us-west-2b"),
			},
		},
		"identical single zone": {
			scTopology:   []v1.TopologySelectorTerm{zoneTerm("us-west-2a")},
			snapTopology: []v1.TopologySelectorTerm{zoneTerm("us-west-2a")},
			expected: []v1.TopologySelectorTerm{
				singleZoneTerm("us-west-2a"),
			},
		},
		"disjoint zones (same key) yield empty intersection": {
			scTopology:   []v1.TopologySelectorTerm{zoneTerm("us-west-2d")},
			snapTopology: []v1.TopologySelectorTerm{zoneTerm("us-west-2a", "us-west-2b", "us-west-2c")},
			expected:     nil,
		},
		"disjoint keys merge into a combined region+zone term": {
			// SC pins a zone, snapshot is regional: the node must satisfy both.
			scTopology:   []v1.TopologySelectorTerm{zoneTerm("us-east-2a")},
			snapTopology: []v1.TopologySelectorTerm{regionTerm("us-east-2")},
			expected:     []v1.TopologySelectorTerm{regionZoneTerm("us-east-2", "us-east-2a")},
		},
		"disjoint keys merge even when physically impossible (left to the driver)": {
			// Zone in a different region: the provisioner has no hierarchy
			// knowledge, so it still merges and the driver rejects the impossible
			// combination.
			scTopology:   []v1.TopologySelectorTerm{zoneTerm("us-west-1c")},
			snapTopology: []v1.TopologySelectorTerm{regionTerm("us-east-2")},
			expected:     []v1.TopologySelectorTerm{regionZoneTerm("us-east-2", "us-west-1c")},
		},
		"StorageClass with only an empty term matches nothing": {
			scTopology:   []v1.TopologySelectorTerm{{}},
			snapTopology: []v1.TopologySelectorTerm{zoneTerm("us-west-2a")},
			expected:     nil,
		},
		"snapshot term with an empty-valued expression is unsatisfiable": {
			// zone=[] AND region=[us-west-2]: the empty zone expression matches
			// nothing, so the whole term is unsatisfiable and must not collapse to
			// region=us-west-2.
			scTopology: []v1.TopologySelectorTerm{zoneTerm("us-west-2a")},
			snapTopology: []v1.TopologySelectorTerm{{
				MatchLabelExpressions: []v1.TopologySelectorLabelRequirement{
					{Key: snapZoneKey, Values: []string{}},
					{Key: snapRegionKey, Values: []string{"us-west-2"}},
				},
			}},
			expected: nil,
		},
		"snapshot zone within StorageClass region keeps the more specific snapshot term": {
			scTopology: []v1.TopologySelectorTerm{{
				MatchLabelExpressions: []v1.TopologySelectorLabelRequirement{
					{Key: snapRegionKey, Values: []string{"us-west-2"}},
				},
			}},
			snapTopology: []v1.TopologySelectorTerm{{
				MatchLabelExpressions: []v1.TopologySelectorLabelRequirement{
					{Key: snapRegionKey, Values: []string{"us-west-2"}},
					{Key: snapZoneKey, Values: []string{"us-west-2a"}},
				},
			}},
			// Segments are sorted by key within a flattened term: region, then zone.
			expected: []v1.TopologySelectorTerm{{
				MatchLabelExpressions: []v1.TopologySelectorLabelRequirement{
					{Key: snapRegionKey, Values: []string{"us-west-2"}},
					{Key: snapZoneKey, Values: []string{"us-west-2a"}},
				},
			}},
		},
	}

	for name, tc := range testcases {
		t.Run(name, func(t *testing.T) {
			got := intersectSnapshotTopology(tc.scTopology, tc.snapTopology)
			if !cmp.Equal(got, tc.expected) {
				t.Errorf("intersectSnapshotTopology() mismatch (-got +want):\n%s",
					cmp.Diff(got, tc.expected))
			}
		})
	}
}

// TestIntersectSnapshotTopologyEmptyIsFatalSignal documents that an empty result
// is the signal callers use to fail provisioning fast when the snapshot and
// StorageClass topologies are incompatible.
func TestIntersectSnapshotTopologyEmptyIsFatalSignal(t *testing.T) {
	got := intersectSnapshotTopology(
		[]v1.TopologySelectorTerm{zoneTerm("us-west-2d")},
		[]v1.TopologySelectorTerm{zoneTerm("us-west-2a")},
	)
	if len(got) != 0 {
		t.Fatalf("expected empty intersection for disjoint topologies, got %v", got)
	}
}
