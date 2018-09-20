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
	"github.com/golang/glog"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	csiv1alpha1 "k8s.io/csi-api/pkg/apis/csi/v1alpha1"
	csiclientset "k8s.io/csi-api/pkg/client/clientset/versioned"
	"math/rand"
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

func GenerateAccessibilityRequirements(
	kubeClient kubernetes.Interface,
	csiAPIClient csiclientset.Interface,
	driverName string,
	allowedTopologies []v1.TopologySelectorTerm,
	selectedNode *v1.Node) (*csi.TopologyRequirement, error) {
	requirement := &csi.TopologyRequirement{}

	var topologyTerms []topologyTerm
	if len(allowedTopologies) == 0 {
		// Aggregate existing topologies in nodes across the entire cluster.
		var err error
		topologyTerms, err = aggregateTopologies(kubeClient, csiAPIClient, driverName, selectedNode)
		if err != nil {
			return nil, err
		}
	} else {
		topologyTerms = flatten(allowedTopologies)
	}

	if len(topologyTerms) == 0 {
		return nil, nil
	}

	topologyTerms = deduplicate(topologyTerms)
	// TODO (verult) reduce subset duplicate terms (advanced reduction)

	var requisite []*csi.Topology
	for _, term := range topologyTerms {
		requisite = append(requisite, &csi.Topology{Segments: term})
	}
	requirement.Requisite = requisite

	return requirement, nil
}

func aggregateTopologies(
	kubeClient kubernetes.Interface,
	csiAPIClient csiclientset.Interface,
	driverName string,
	selectedNode *v1.Node) ([]topologyTerm, error) {

	var topologyKeys []string
	if selectedNode == nil {
		// TODO (verult) retry
		nodeInfos, err := csiAPIClient.CsiV1alpha1().CSINodeInfos().List(metav1.ListOptions{})
		if err != nil {
			// We must support provisioning if CSINodeInfo is missing, for backward compatibility.
			glog.Warningf("error listing CSINodeInfos: %v; proceeding to provision without topology information", err)
			return nil, nil
		}

		rand.Shuffle(len(nodeInfos.Items), func(i, j int) {
			nodeInfos.Items[i], nodeInfos.Items[j] = nodeInfos.Items[j], nodeInfos.Items[i]
		})

		// Pick the first node with topology keys
		for _, nodeInfo := range nodeInfos.Items {
			topologyKeys = getTopologyKeysFromNodeInfo(&nodeInfo, driverName)
			if topologyKeys != nil {
				break
			}
		}
	} else {
		// TODO (verult) retry
		selectedNodeInfo, err := csiAPIClient.CsiV1alpha1().CSINodeInfos().Get(selectedNode.Name, metav1.GetOptions{})
		if err != nil {
			// We must support provisioning if CSINodeInfo is missing, for backward compatibility.
			glog.Warningf("error getting CSINodeInfo for selected node %q: %v; proceeding to provision without topology information", selectedNode.Name, err)
			return nil, nil
		}
		topologyKeys = getTopologyKeysFromNodeInfo(selectedNodeInfo, driverName)
	}

	if len(topologyKeys) == 0 {
		// Assuming the external provisioner is never running during node driver upgrades.
		// If selectedNode != nil, the scheduler selected a node with no topology information.
		// If selectedNode == nil, all nodes in the cluster are missing topology information.
		// In either case, provisioning needs to be allowed to proceed.
		return nil, nil
	}

	selector, err := buildTopologyKeySelector(topologyKeys)
	if err != nil {
		return nil, err
	}
	nodes, err := kubeClient.CoreV1().Nodes().List(metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return nil, fmt.Errorf("error listing nodes: %v", err)
	}

	return extractTopologyFromNodes(nodes, topologyKeys), nil
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

func getTopologyKeysFromNodeInfo(nodeInfo *csiv1alpha1.CSINodeInfo, driverName string) []string {
	for _, driver := range nodeInfo.CSIDrivers {
		if driver.Driver == driverName {
			return driver.TopologyKeys
		}
	}
	return nil
}

func extractTopologyFromNodes(nodes *v1.NodeList, topologyKeys []string) []topologyTerm {
	var terms []topologyTerm
	for _, node := range nodes.Items {
		segments := make(map[string]string)
		for _, key := range topologyKeys {
			// Key always exists because nodes were selected by these keys.
			segments[key] = node.Labels[key]
		}
		terms = append(terms, segments)
	}
	return terms
}

func buildTopologyKeySelector(topologyKeys []string) (string, error) {
	var expr []metav1.LabelSelectorRequirement
	for _, key := range topologyKeys {
		expr = append(expr, metav1.LabelSelectorRequirement{
			Key:      key,
			Operator: metav1.LabelSelectorOpExists,
		})
	}

	labelSelector := metav1.LabelSelector{
		MatchExpressions: expr,
	}

	selector, err := metav1.LabelSelectorAsSelector(&labelSelector)
	if err != nil {
		return "", fmt.Errorf("error parsing topology keys selector: %v", err)
	}

	return selector.String(), nil
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
