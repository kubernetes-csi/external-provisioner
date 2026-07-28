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
		"disjoint zones yield empty intersection": {
			scTopology:   []v1.TopologySelectorTerm{zoneTerm("us-west-2d")},
			snapTopology: []v1.TopologySelectorTerm{zoneTerm("us-west-2a", "us-west-2b", "us-west-2c")},
			expected:     nil,
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

// TestIntersectSnapshotTopologyEmptyIsFatalSignal documents that an empty (but
// non-nil) result is the signal callers use to fail provisioning fast when the
// snapshot and StorageClass topologies are incompatible.
func TestIntersectSnapshotTopologyEmptyIsFatalSignal(t *testing.T) {
	got := intersectSnapshotTopology(
		[]v1.TopologySelectorTerm{zoneTerm("us-west-2d")},
		[]v1.TopologySelectorTerm{zoneTerm("us-west-2a")},
	)
	if len(got) != 0 {
		t.Fatalf("expected empty intersection for disjoint topologies, got %v", got)
	}
}
