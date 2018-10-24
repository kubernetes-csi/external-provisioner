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
	"k8s.io/apimachinery/pkg/util/sets"
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
				Key:      csiTopologyKeyToKubernetesLabelKey(k),
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
	topologyKeys []string,
	kubeClient kubernetes.Interface,
	driverName string,
	allowedTopologies []v1.TopologySelectorTerm,
	selectedNode *v1.Node) (*csi.TopologyRequirement, error) {
	requirement := &csi.TopologyRequirement{}

	/* Requisite */
	var requisiteTerms []topologyTerm
	if len(allowedTopologies) == 0 {
		// Aggregate existing topologies in nodes across the entire cluster.
		var err error
		requisiteTerms, err = aggregateTopologies(topologyKeys, kubeClient)
		if err != nil {
			return nil, err
		}
	} else {
		// Distribute out one of the OR layers in allowedTopologies
		var err error
		requisiteTerms, err = flatten(topologyKeys, allowedTopologies)
		if err != nil {
			return nil, err
		}
	}

	if len(requisiteTerms) == 0 {
		return nil, nil
	}

	requisiteTerms = deduplicate(requisiteTerms)
	// TODO (verult) reduce subset duplicate terms (advanced reduction)

	requirement.Requisite = toCSITopology(requisiteTerms)

	/* Preferred */
	if selectedNode != nil {
		selectedTopology, isMissingKey := getTopologyFromNode(selectedNode, topologyKeys)
		if isMissingKey {
			return nil, fmt.Errorf("topology labels from selected node %v does not match topology keys from CSINodeInfo %v", selectedNode.Labels, topologyKeys)
		}

		preferredTerms := sortAndShift(requisiteTerms, selectedTopology)
		if preferredTerms == nil {
			// Topology from selected node is not in requisite. This case should never be hit:
			// - If AllowedTopologies is specified, the scheduler should choose a node satisfying the
			//   constraint.
			// - Otherwise, the aggregated topology is guaranteed to contain topology information from the
			//   selected node.
			return nil, fmt.Errorf("topology %v from selected node %q is not in requisite", selectedTopology, selectedNode.Name)
		}

		requirement.Preferred = toCSITopology(preferredTerms)
	}

	return requirement, nil
}

func lookupTopologyKeys(
	kubeClient kubernetes.Interface,
	csiAPIClient csiclientset.Interface,
	driverName string,
	selectedNode *v1.Node) []string {

	var topologyKeys []string
	if selectedNode == nil {
		// TODO (verult) retry
		nodeInfos, err := csiAPIClient.CsiV1alpha1().CSINodeInfos().List(metav1.ListOptions{})
		if err != nil {
			// We must support provisioning if CSINodeInfo is missing, for backward compatibility.
			glog.Warningf("error listing CSINodeInfos: %v; proceeding to provision without topology information", err)
			return nil
		}

		rand.Shuffle(len(nodeInfos.Items), func(i, j int) {
			nodeInfos.Items[i], nodeInfos.Items[j] = nodeInfos.Items[j], nodeInfos.Items[i]
		})

		// Pick the first node with topology keys
		for _, nodeInfo := range nodeInfos.Items {
			topologyKeys = getTopologyKeys(&nodeInfo, driverName)
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
			return nil
		}
		topologyKeys = getTopologyKeys(selectedNodeInfo, driverName)
	}

	return topologyKeys
}

func aggregateTopologies(topologyKeys []string, kubeClient kubernetes.Interface) ([]topologyTerm, error) {
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

	var terms []topologyTerm
	for _, node := range nodes.Items {
		// missingKey bool can be ignored because nodes were selected by these keys.
		term, _ := getTopologyFromNode(&node, topologyKeys)
		terms = append(terms, term)
	}
	return terms, nil
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
func flatten(topologyKeys []string, allowedTopologies []v1.TopologySelectorTerm) ([]topologyTerm, error) {
	labelToTopologyKey := map[string]string{}
	for _, topologyKey := range topologyKeys {
		labelToTopologyKey[csiTopologyKeyToKubernetesLabelKey(topologyKey)] = topologyKey
	}

	var finalTerms []topologyTerm
	for _, selectorTerm := range allowedTopologies { // OR

		var oldTerms []topologyTerm
		for _, selectorExpression := range selectorTerm.MatchLabelExpressions { // AND

			var newTerms []topologyTerm
			for _, v := range selectorExpression.Values { // OR
				// Distribute the OR over AND.

				topologyKey := labelToTopologyKey[selectorExpression.Key]
				if len(topologyKey) == 0 {
					// this means the storage class topology selector is referencing a label the CSI driver doesn't consider a topology key.
					knownLabels := sets.StringKeySet(labelToTopologyKey)
					return nil, fmt.Errorf("unknown required topology label %q; known topology labels are %q", selectorExpression.Key, knownLabels.List())
				}

				if len(oldTerms) == 0 {
					// No previous terms to distribute over. Simply append the new term.
					newTerms = append(newTerms, topologyTerm{topologyKey: v})
				} else {
					for _, oldTerm := range oldTerms {
						// "Distribute" by adding an entry to the term
						newTerm := oldTerm.clone()
						newTerm[topologyKey] = v
						newTerms = append(newTerms, newTerm)
					}
				}
			}

			oldTerms = newTerms
		}

		// Concatenate all OR'd terms.
		finalTerms = append(finalTerms, oldTerms...)
	}

	return finalTerms, nil
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

// Sort the given terms in place,
// then return a new list of terms equivalent to the sorted terms, but shifted so that the primary
// term is the first in the list.
func sortAndShift(terms []topologyTerm, primary topologyTerm) []topologyTerm {
	var preferredTerms []topologyTerm
	sort.Slice(terms, func(i, j int) bool {
		return terms[i].less(terms[j])
	})
	for i, t := range terms {
		if t.equal(primary) {
			preferredTerms = append(terms[i:], terms[:i]...)
			break
		}
	}
	return preferredTerms
}

func getTopologyKeys(nodeInfo *csiv1alpha1.CSINodeInfo, driverName string) []string {
	for _, driver := range nodeInfo.CSIDrivers {
		if driver.Driver == driverName {
			return driver.TopologyKeys
		}
	}
	return nil
}

func getTopologyFromNode(node *v1.Node, topologyKeys []string) (term topologyTerm, isMissingKey bool) {
	term = make(topologyTerm)
	for _, key := range topologyKeys {
		v, ok := node.Labels[csiTopologyKeyToKubernetesLabelKey(key)]
		if !ok {
			return nil, true
		}
		term[key] = v
	}
	return term, false
}

func buildTopologyKeySelector(topologyKeys []string) (string, error) {
	var expr []metav1.LabelSelectorRequirement
	for _, key := range topologyKeys {
		expr = append(expr, metav1.LabelSelectorRequirement{
			Key:      csiTopologyKeyToKubernetesLabelKey(key),
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

func (t topologyTerm) less(other topologyTerm) bool {
	return t.hash() < other.hash()
}

func (t topologyTerm) equal(other topologyTerm) bool {
	return t.hash() == other.hash()
}

func toCSITopology(terms []topologyTerm) []*csi.Topology {
	var out []*csi.Topology
	for _, term := range terms {
		out = append(out, &csi.Topology{Segments: term})
	}
	return out
}

const topologyNamespace = "topology.kubernetes.io"

func csiTopologyKeyToKubernetesLabelKey(topologyKey string) string {
	items := strings.SplitN(topologyKey, "/", 2)
	if len(items) == 1 {
		// "zone" -> "topology.kubernetes.io/zone"
		return topologyNamespace + "/" + items[0]
	}
	namespace, name := items[0], items[1]
	if namespace == topologyNamespace || strings.HasSuffix(namespace, "."+topologyNamespace) {
		// "topology.kubernetes.io/zone" -> "topology.kubernetes.io/zone"
		// "acme.com.topology.kubernetes.io/zone" -> "acme.com.topology.kubernetes.io/zone"
		return topologyKey
	}

	// "acme.com/zone" -> "acme.com.topology.kubernetes.io/zone"
	return namespace + "." + topologyNamespace + "/" + name
}
