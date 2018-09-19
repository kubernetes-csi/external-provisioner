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
	"github.com/container-storage-interface/spec/lib/go/csi/v0"
	"k8s.io/api/core/v1"
	"sort"
	"strings"
)

// topologyTerm represents a single term where its topology key value pairs are AND'd together.
type topologyTerm map[string]string

func GenerateVolumeNodeAffinity(accessibleTopology []*csi.Topology) *v1.VolumeNodeAffinity {
	if len(accessibleTopology) == 0 {
		return nil
	}

	var terms []v1.NodeSelectorTerm
	for _, topology := range accessibleTopology {
		if len(topology.Segments) == 0 {
			continue
		}

		var expressions []v1.NodeSelectorRequirement
		for k, v := range topology.Segments {
			expressions = append(expressions, v1.NodeSelectorRequirement{
				Key:      k,
				Operator: v1.NodeSelectorOpIn,
				Values:   []string{v},
			})
		}
		terms = append(terms, v1.NodeSelectorTerm{
			MatchExpressions: expressions,
		})
	}

	return &v1.VolumeNodeAffinity{
		Required: &v1.NodeSelector{
			NodeSelectorTerms: terms,
		},
	}
}

func GenerateAccessibilityRequirements(allowedTopologies []v1.TopologySelectorTerm) (*csi.TopologyRequirement, error) {
	if len(allowedTopologies) == 0 {
		return nil, fmt.Errorf("topology aggregation not implemented")
	} else {
		topologyTerms := flatten(allowedTopologies)
		topologyTerms = deduplicate(topologyTerms)
		// TODO (verult) reduce subset duplicate terms (advanced reduction)

		var requisite []*csi.Topology
		for _, term := range topologyTerms {
			requisite = append(requisite, &csi.Topology{Segments: term})
		}
		return &csi.TopologyRequirement{Requisite: requisite}, nil
	}
}

// AllowedTopologies is an OR of TopologySelectorTerms.
// A TopologySelectorTerm contains an AND of TopologySelectorLabelRequirements.
// A TopologySelectorLabelRequirement contains a single key and an OR of topology values.
//
// The Requisite field contains an OR of Segments.
// A segment contains an AND of topology key value pairs.
//
// In order to convert AllowedTopologies to CSI Requisite, one of its OR layers must be eliminated.
// This function eliminates the OR of topology values by distributing the OR over the AND a level
// higher.
// For example, given a TopologySelectorTerm of this form:
//    {
//      "zone": { "zone1", "zone2" },
//      "rack": { "rackA", "rackB" },
//    }
// Abstractly it could be viewed as:
//    (zone1 OR zone2) AND (rackA OR rackB)
// Distributing the OR over the AND, we get:
//    (zone1 AND rackA) OR (zone2 AND rackA) OR (zone1 AND rackB) OR (zone2 AND rackB)
// which in the intermediate representation returned by this function becomes:
//    [
//      { "zone": "zone1", "rack": "rackA" },
//      { "zone": "zone2", "rack": "rackA" },
//      { "zone": "zone1", "rack": "rackB" },
//      { "zone": "zone2", "rack": "rackB" },
//    ]
//
// This flattening is then applied to all TopologySelectorTerms in AllowedTopologies, and
// the resulting terms are OR'd together.
func flatten(allowedTopologies []v1.TopologySelectorTerm) []topologyTerm {
	var finalTerms []topologyTerm
	for _, selectorTerm := range allowedTopologies { // OR

		var oldTerms []topologyTerm
		for _, selectorExpression := range selectorTerm.MatchLabelExpressions { // AND

			var newTerms []topologyTerm
			for _, v := range selectorExpression.Values { // OR
				// Distribute the OR over AND.

				if len(oldTerms) == 0 {
					// No previous terms to distribute over. Simply append the new term.
					newTerms = append(newTerms, topologyTerm{selectorExpression.Key: v})
				} else {
					for _, oldTerm := range oldTerms {
						// "Distribute" by adding an entry to the term
						newTerm := oldTerm.clone()
						newTerm[selectorExpression.Key] = v
						newTerms = append(newTerms, newTerm)
					}
				}
			}

			oldTerms = newTerms
		}

		// Concatenate all OR'd terms.
		finalTerms = append(finalTerms, oldTerms...)
	}

	return finalTerms
}

func deduplicate(terms []topologyTerm) []topologyTerm {
	termMap := make(map[string]topologyTerm)
	for _, term := range terms {
		termMap[term.hash()] = term
	}

	var dedupedTerms []topologyTerm
	for _, term := range termMap {
		dedupedTerms = append(dedupedTerms, term)
	}
	return dedupedTerms
}

func (t topologyTerm) clone() topologyTerm {
	ret := make(topologyTerm)
	for k, v := range t {
		ret[k] = v
	}
	return ret
}

// "<k1>#<v1>,<k2>#<v2>,..."
// Hash properties:
// - Two equivalent topologyTerms have the same hash
// - Ordering of hashes correspond to a natural ordering of their topologyTerms. For example:
//   - com.example.csi/zone#zone1 < com.example.csi/zone#zone2
//   - com.example.csi/rack#zz    < com.example.csi/zone#zone1
//   - com.example.csi/z#z1       < com.example.csi/zone#zone1
//   - com.example.csi/rack#rackA,com.example.csi/zone#zone2  <  com.example.csi/rackB,com.example.csi/zone#zone1
//   Note that both '#' and ',' are less than '/', '-', '_', '.', [A-Z0-9a-z]
func (t topologyTerm) hash() string {
	var segments []string
	for k, v := range t {
		segments = append(segments, k+"#"+v)
	}

	sort.Strings(segments)
	return strings.Join(segments, ",")
}
