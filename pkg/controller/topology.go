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
	"hash/fnv"
	"maps"
	"math/rand"
	"slices"
	"strconv"
	"strings"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/kubernetes-csi/csi-lib-utils/rpc"
	"github.com/kubernetes-csi/external-provisioner/v5/pkg/features"
	v1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	storagelistersv1 "k8s.io/client-go/listers/storage/v1"
	corev1helpers "k8s.io/component-helpers/scheduling/corev1"
	"k8s.io/klog/v2"
)

type topologySegment struct {
	Key, Value string
}

// topologyTerm represents a single term where its topology key value pairs are AND'd together.
//
// Be sure to sort after construction for compare() and subset() to work properly.
type topologyTerm []topologySegment

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

// VolumeIsAccessible checks whether the generated volume affinity is satisfied by
// a the node topology that a CSI driver reported in GetNodeInfoResponse.
func VolumeIsAccessible(affinity *v1.VolumeNodeAffinity, nodeTopology *csi.Topology) (bool, error) {
	if nodeTopology == nil || affinity == nil || affinity.Required == nil {
		// No topology information -> all volumes accessible.
		return true, nil
	}

	nodeLabels := labels.Set{}
	maps.Copy(nodeLabels, nodeTopology.Segments)
	node := v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Labels: nodeLabels,
		},
	}
	return corev1helpers.MatchNodeSelectorTerms(&node, affinity.Required)
}

// SupportsTopology returns whether topology is supported both for plugin and external provisioner
func SupportsTopology(pluginCapabilities rpc.PluginCapabilitySet) bool {
	return pluginCapabilities[csi.PluginCapability_Service_VOLUME_ACCESSIBILITY_CONSTRAINTS] &&
		utilfeature.DefaultFeatureGate.Enabled(features.Topology)
}

func topologyKeysLookup(
	csiNodeLister storagelistersv1.CSINodeLister,
	selectedNodeName string,
	driverName string,
	pvcNodeStore TopologyProvider,
	pvcUID types.UID) (topologyKeys []string, err error) {
	// Try to get topology keys from cache first.
	topologyKeys, err = getTopologyKeysFromCache(pvcNodeStore, pvcUID)
	if err == nil && len(topologyKeys) > 0 {
		return topologyKeys, nil
	}

	// Fallback to getting from CSINode.
	selectedCSINode, err := getSelectedCSINode(csiNodeLister, selectedNodeName)
	if err != nil {
		return nil, fmt.Errorf("failed to get selected CSINode %s: %w", selectedNodeName, err)
	}

	topologyKeys = getTopologyKeys(selectedCSINode, driverName)
	// Update in memory cache.
	if len(topologyKeys) > 0 {
		pvcNodeStore.UpdateTopologyKeys(pvcUID, topologyKeys)
	}

	return topologyKeys, nil
}

// GenerateAccessibilityRequirements returns the CSI TopologyRequirement
// to pass into the CSI CreateVolume request.
//
// This function is called if the topology feature is enabled
// in the external-provisioner and the CSI driver implements the
// CSI accessibility capability. It is disabled by default.
//
// If enabled, we require that the K8s API server is on at least
// K8s 1.17 and that the K8s Nodes are on at least K8s 1.15 in
// accordance with the 2 version skew between control plane and
// nodes.
//
// There are two main cases to consider:
//
// 1) selectedNode is not set (immediate binding):
//
//	In this case, we list all CSINode objects to find a Node that
//	the driver has registered topology keys with.
//
//	Once we get the list of CSINode objects, we find one that has
//	topology keys registered. If none are found, then we assume
//	that the driver has not started on any node yet, and we error
//	and retry.
//
//	If at least one CSINode object is found with topology keys,
//	then we continue and use that for assembling the topology
//	requirement. The available topologies will be limited to the
//	Nodes that the driver has registered with.
//
// 2) selectedNode is set (delayed binding):
//
//	We will get the topology from the CSINode object for the selectedNode
//	and error if we can't (and retry).
func GenerateAccessibilityRequirements(
	kubeClient kubernetes.Interface,
	driverName string,
	pvcUID types.UID,
	pvcName string,
	allowedTopologies []v1.TopologySelectorTerm,
	selectedNodeName string,
	strictTopology bool,
	immediateTopology bool,
	csiNodeLister storagelistersv1.CSINodeLister,
	nodeLister corelisters.NodeLister,
	pvcNodeStore TopologyProvider) (*csi.TopologyRequirement, error) {
	requirement := &csi.TopologyRequirement{}

	var (
		selectedTopology   topologyTerm
		topologyKeys       []string
		requisiteTerms     []topologyTerm
		selectedNodeLabels map[string]string
		err                error
	)

	// 1. Get topology keys for the selected node
	if len(selectedNodeName) > 0 {
		topologyKeys, err = topologyKeysLookup(csiNodeLister, selectedNodeName, driverName, pvcNodeStore, pvcUID)

		if err != nil {
			return nil, err
		}
		if len(topologyKeys) == 0 {
			// The scheduler selected a node with no topology information.
			// This can happen if:
			//
			// * the node driver is not deployed on all nodes.
			// * the node driver is being restarted and has not re-registered yet. This should be
			//   temporary and a retry should eventually succeed.
			//
			// Returning an error in provisioning will cause the scheduler to retry and potentially
			// (but not guaranteed) pick a different node.
			return nil, fmt.Errorf("no topology key found for node %s", selectedNodeName)
		}
		var isMissingKey bool
		selectedTopology, selectedNodeLabels, isMissingKey = getTopologyFromNodeName(selectedNodeName, topologyKeys, nodeLister, pvcUID, pvcNodeStore)
		if isMissingKey {
			return nil, fmt.Errorf("topology labels from selected node %v does not match topology keys from CSINode %v", selectedNodeLabels, topologyKeys)
		}

		if strictTopology {
			// Make sure that selected node topology is in allowed topologies list
			if len(allowedTopologies) != 0 {
				allowedTopologiesFlatten := flatten(allowedTopologies)
				found := false
				for _, t := range allowedTopologiesFlatten {
					if t.subset(selectedTopology) {
						found = true
						break
					}
				}
				if !found {
					return nil, fmt.Errorf("selected node '%q' topology '%v' is not in allowed topologies: %v", selectedNodeName, selectedTopology, allowedTopologiesFlatten)
				}
			}
			// Only pass topology of selected node.
			requisiteTerms = append(requisiteTerms, selectedTopology)
		}
	}

	// 2. Generate CSI Requisite Terms
	if len(requisiteTerms) == 0 {
		if len(allowedTopologies) != 0 {
			// Distribute out one of the OR layers in allowedTopologies
			requisiteTerms = flatten(allowedTopologies)
		} else {
			if len(selectedNodeName) == 0 && !immediateTopology {
				// Don't specify any topology requirements because neither the PVC nor
				// the storage class have limitations and the CSI driver is not interested
				// in being told where it runs (perhaps it already knows, for example).
				return nil, nil
			}

			// Aggregate existing topologies in nodes across the entire cluster.
			requisiteTerms, err = aggregateTopologies(driverName, selectedNodeName, topologyKeys, csiNodeLister, nodeLister, pvcUID, pvcNodeStore)
			if err != nil {
				return nil, err
			}
			if len(requisiteTerms) == 0 {
				// We may reach here if the driver has not registered on any nodes.
				// We should wait for at least one driver to start so that we can
				// provision in a supported topology.
				return nil, fmt.Errorf("no available topology found")
			}
		}
	}

	// It might be possible to reach here if allowedTopologies had empty entries.
	// We fallback to the "topology disabled" behavior.
	if len(requisiteTerms) == 0 {
		return nil, nil
	}

	slices.SortFunc(requisiteTerms, topologyTerm.compare)
	requisiteTerms = slices.CompactFunc(requisiteTerms, slices.Equal)
	// TODO (verult) reduce subset duplicate terms (advanced reduction)

	requirement.Requisite = toCSITopology(requisiteTerms)

	// 3. Generate CSI Preferred Terms
	var preferredTerms []topologyTerm
	if len(selectedNodeName) == 0 {
		// Immediate binding, we fallback to statefulset spreading hash for backwards compatibility.

		// Ensure even spreading of StatefulSet volumes by sorting
		// requisiteTerms and shifting the sorted terms based on hash of pvcName and replica index suffix
		hash, index := getPVCNameHashAndIndexOffset(pvcName)
		i := (hash + index) % uint32(len(requisiteTerms))
		preferredTerms = append(requisiteTerms[i:], requisiteTerms[:i]...)

	} else {
		// Delayed binding, use topology from that node to populate preferredTerms
		if strictTopology {
			// In case of strict topology, preferred = requisite
			preferredTerms = requisiteTerms
		} else {
			for i, t := range requisiteTerms {
				if t.subset(selectedTopology) {
					preferredTerms = append(requisiteTerms[i:], requisiteTerms[:i]...)
					break
				}
			}
			if len(preferredTerms) == 0 {
				// Topology from selected node is not in requisite. This case should never be hit:
				// - If AllowedTopologies is specified, the scheduler should choose a node satisfying the
				//   constraint.
				// - Otherwise, the aggregated topology is guaranteed to contain topology information from the
				//   selected node.
				return nil, fmt.Errorf("topology %v from selected node %q is not in requisite: %v", selectedTopology, selectedNodeName, requisiteTerms)
			}
		}
	}
	requirement.Preferred = toCSITopology(preferredTerms)
	return requirement, nil
}

// getSelectedCSINode returns the CSINode object for the given selectedNode.
func getSelectedCSINode(
	csiNodeLister storagelistersv1.CSINodeLister,
	selectedNodeName string) (*storagev1.CSINode, error) {

	selectedCSINode, err := csiNodeLister.Get(selectedNodeName)
	if err != nil {
		// We don't want to fallback and provision in the wrong topology if there's some temporary
		// error with the API server.
		return nil, err
	}
	if selectedCSINode == nil {
		return nil, fmt.Errorf("CSINode for selected node %q not found", selectedNodeName)
	}
	return selectedCSINode, nil
}

// aggregateTopologies returns all the supported topology values in the cluster that
// match the driver's topology keys.
func aggregateTopologies(
	driverName string,
	selectedNodeName string,
	selectedNodeTopologyKeys []string,
	csiNodeLister storagelistersv1.CSINodeLister,
	nodeLister corelisters.NodeLister,
	pvcKeyForStore types.UID,
	pvcNodeStore TopologyProvider) ([]topologyTerm, error) {
	// 1. Determine topologyKeys to use for aggregation
	var topologyKeys []string
	var err error
	if len(selectedNodeName) == 0 {
		// Immediate binding
		// Read from cache first to make sure retry with the same arguments
		topologyKeys, err = getTopologyKeysFromCache(pvcNodeStore, pvcKeyForStore)
		if err != nil || len(topologyKeys) == 0 {
			// Not in the in memory cache. Find from csiNodes.
			csiNodes, err := csiNodeLister.List(labels.Everything())
			if err != nil {
				// Require CSINode beta feature on K8s apiserver to be enabled.
				// We don't want to fallback and provision in the wrong topology if there's some temporary
				// error with the API server.
				return nil, fmt.Errorf("error listing CSINodes: %v", err)
			}
			rand.Shuffle(len(csiNodes), func(i, j int) {
				csiNodes[i], csiNodes[j] = csiNodes[j], csiNodes[i]
			})
			// Pick the first node with topology keys
			for _, csiNode := range csiNodes {
				keys := getTopologyKeys(csiNode, driverName)
				if len(keys) > 0 {
					topologyKeys = keys
					// Store in cache for next time.
					pvcNodeStore.UpdateTopologyKeys(pvcKeyForStore, topologyKeys)
					break
				}
			}
		}

		if len(topologyKeys) == 0 {
			// The driver supports topology but no nodes have registered any topology keys.
			// This is possible if nodes have not been upgraded to use the beta CSINode feature.
			klog.Warningf("No topology keys found on any node")
			return nil, nil
		}
	} else {
		// Delayed binding; use topology key from selected node
		topologyKeys = selectedNodeTopologyKeys
		if len(topologyKeys) == 0 {
			// The scheduler selected a node with no topology information.
			// This can happen if:
			//
			// * the node driver is not deployed on all nodes.
			// * the node driver is being restarted and has not re-registered yet. This should be
			//   temporary and a retry should eventually succeed.
			//
			// Returning an error in provisioning will cause the scheduler to retry and potentially
			// (but not guaranteed) pick a different node.
			return nil, fmt.Errorf("no topology key found on CSINode %s", selectedNodeName)
		}

		// Even though selectedNode is set, we still need to aggregate topology values across
		// all nodes in order to find additional topologies for the volume types that can span
		// multiple topology values.
		//
		// TODO (#221): allow drivers to limit the number of topology values that are returned
		// If the driver specifies 1, then we can optimize here to only return the selected node's
		// topology instead of aggregating across all Nodes.
	}

	// 2. Find all nodes with the topology keys and extract the topology values
	selector, err := buildTopologyKeySelector(topologyKeys)
	if err != nil {
		return nil, err
	}
	var terms []topologyTerm
	terms, err = getRequisiteTermsFromCache(pvcNodeStore, pvcKeyForStore)
	if err != nil || len(terms) == 0 {
		// Not in the in memory cache.
		nodes, err := nodeLister.List(selector)
		if err != nil {
			return nil, fmt.Errorf("error listing nodes: %v", err)
		}

		for _, node := range nodes {
			term, _ := getTopologyFromNode(node, topologyKeys)
			if len(term) > 0 {
				terms = append(terms, term)
			}
		}

		if len(terms) == 0 {
			// This means that a CSINode was found with topologyKeys, but we couldn't find
			// the topology labels on any nodes.
			return nil, fmt.Errorf("topologyKeys %v were not found on any nodes", topologyKeys)
		} else {
			pvcNodeStore.UpdateRequisiteTerms(pvcKeyForStore, terms)
		}
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
//
//	{
//	  "zone": { "zone1", "zone2" },
//	  "rack": { "rackA", "rackB" },
//	}
//
// Abstractly it could be viewed as:
//
//	(zone1 OR zone2) AND (rackA OR rackB)
//
// Distributing the OR over the AND, we get:
//
//	(zone1 AND rackA) OR (zone2 AND rackA) OR (zone1 AND rackB) OR (zone2 AND rackB)
//
// which in the intermediate representation returned by this function becomes:
//
//	[
//	  { "zone": "zone1", "rack": "rackA" },
//	  { "zone": "zone2", "rack": "rackA" },
//	  { "zone": "zone1", "rack": "rackB" },
//	  { "zone": "zone2", "rack": "rackB" },
//	]
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
					newTerms = append(newTerms, topologyTerm{{selectorExpression.Key, v}})
				} else {
					for _, oldTerm := range oldTerms {
						// "Distribute" by adding an entry to the term
						newTerm := slices.Clone(oldTerm)
						newTerm = append(newTerm, topologySegment{selectorExpression.Key, v})
						newTerms = append(newTerms, newTerm)
					}
				}
			}

			oldTerms = newTerms
		}

		// Concatenate all OR'd terms.
		finalTerms = append(finalTerms, oldTerms...)
	}

	for _, term := range finalTerms {
		term.sort()
	}
	return finalTerms
}

func getTopologyKeysFromCache(pvcNodeStore TopologyProvider, pvcKey types.UID) ([]string, error) {
	cacheInfo, err := pvcNodeStore.GetByPvcUID(pvcKey)
	if err != nil {
		return nil, err
	}
	if len(cacheInfo.TopologyKeys) == 0 {
		return nil, nil
	}
	return cacheInfo.TopologyKeys, nil
}

func getRequisiteTermsFromCache(pvcNodeStore TopologyProvider, pvcKey types.UID) ([]topologyTerm, error) {
	cacheInfo, err := pvcNodeStore.GetByPvcUID(pvcKey)
	if err != nil {
		return nil, err
	}
	if len(cacheInfo.RequisiteTerms) == 0 {
		return nil, nil
	}
	return cacheInfo.RequisiteTerms, nil
}

func getNodeLabelsFromCache(pvcNodeStore TopologyProvider, pvcKey types.UID) (map[string]string, error) {
	cacheInfo, err := pvcNodeStore.GetByPvcUID(pvcKey)
	if err != nil {
		return nil, err
	}
	if len(cacheInfo.NodeLabels) == 0 {
		return nil, nil
	}
	return cacheInfo.NodeLabels, nil
}

func getTopologyKeys(csiNode *storagev1.CSINode, driverName string) []string {
	for _, driver := range csiNode.Spec.Drivers {
		if driver.Name == driverName {
			return driver.TopologyKeys
		}
	}
	return nil
}

func extractTopologyTerm(labels map[string]string, topologyKeys []string) (topologyTerm, map[string]string, bool) {
	term := topologyTerm{}
	for _, key := range topologyKeys {
		v, ok := labels[key]
		if !ok {
			return nil, nil, true // isMissingKey = true
		}
		term = append(term, topologySegment{key, v})
	}
	term.sort()
	return term, labels, false // isMissingKey = false
}

func getTopologyFromNodeName(nodeName string, topologyKeys []string, nodeLister corelisters.NodeLister, pvcKeyForStore types.UID, pvcNodeStore TopologyProvider) (term topologyTerm, selectedNodeLabels map[string]string, isMissingKey bool) {
	// Read from the cache first.
	nodeLabels, err := getNodeLabelsFromCache(pvcNodeStore, pvcKeyForStore)
	if err == nil && len(nodeLabels) > 0 {
		return extractTopologyTerm(nodeLabels, topologyKeys)
	}

	// Get Node from API server.
	if nodeLister != nil {
		node, err := nodeLister.Get(nodeName)
		if err != nil {
			// Any error, including NotFound, results in us not being
			// able to determine topology. The cache was already checked.
			return nil, nil, true
		}

		// Add or update cache for the node.
		if len(node.Labels) > 0 {
			pvcNodeStore.UpdateNodeLabels(pvcKeyForStore, node.Labels)
		}
		return extractTopologyTerm(node.Labels, topologyKeys)
	}

	// nodeLister cannot be nil if the plugin supports topology
	klog.Errorf("nodeLister is nil but the plugin supports topology")
	return nil, nil, true
}

func getTopologyFromNode(node *v1.Node, topologyKeys []string) (term topologyTerm, isMissingKey bool) {
	term = topologyTerm{}
	// Get Node Here
	for _, key := range topologyKeys {
		v, ok := node.Labels[key]
		if !ok {
			return nil, true
		}
		term = append(term, topologySegment{key, v})
	}
	term.sort()
	return term, false
}

func buildTopologyKeySelector(topologyKeys []string) (labels.Selector, error) {
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
		return nil, fmt.Errorf("error parsing topology keys selector: %v", err)
	}

	return selector, nil
}

func (t topologyTerm) sort() {
	slices.SortFunc(t, func(a, b topologySegment) int {
		r := strings.Compare(a.Key, b.Key)
		if r != 0 {
			return r
		}
		// Should not happen currently. We may support multi-value in the future?
		return strings.Compare(a.Value, b.Value)
	})
}

func (t topologyTerm) compare(other topologyTerm) int {
	if len(t) != len(other) {
		return len(t) - len(other)
	}
	for i, k1 := range t {
		k2 := other[i]
		r := strings.Compare(k1.Key, k2.Key)
		if r != 0 {
			return r
		}
		r = strings.Compare(k1.Value, k2.Value)
		if r != 0 {
			return r
		}
	}
	return 0
}

func (t topologyTerm) subset(other topologyTerm) bool {
	if len(t) == 0 {
		return true
	}
	j := 0
	for _, k2 := range other {
		k1 := t[j]
		if k1.Key != k2.Key {
			continue
		}
		if k1.Value != k2.Value {
			return false
		}
		j++
		if j == len(t) {
			// All segments in t have been checked and is present in other.
			return true
		}
	}
	return false
}

func toCSITopology(terms []topologyTerm) []*csi.Topology {
	out := []*csi.Topology{}
	for _, term := range terms {
		if len(term) == 0 {
			continue
		}
		segs := make(map[string]string, len(term))
		for _, k := range term {
			segs[k.Key] = k.Value
		}
		out = append(out, &csi.Topology{Segments: segs})
	}
	return out
}

// identical to logic in getPVCNameHashAndIndexOffset in pkg/volume/util/util.go in-tree
// [https://github.com/kubernetes/kubernetes/blob/master/pkg/volume/util/util.go]
func getPVCNameHashAndIndexOffset(pvcName string) (hash uint32, index uint32) {
	if pvcName == "" {
		// We should always be called with a name; this shouldn't happen
		hash = rand.Uint32()
	} else {
		hashString := pvcName

		// Heuristic to make sure that volumes in a StatefulSet are spread across zones
		// StatefulSet PVCs are (currently) named ClaimName-StatefulSetName-Id,
		// where Id is an integer index.
		// Note though that if a StatefulSet pod has multiple claims, we need them to be
		// in the same zone, because otherwise the pod will be unable to mount both volumes,
		// and will be unschedulable.  So we hash _only_ the "StatefulSetName" portion when
		// it looks like `ClaimName-StatefulSetName-Id`.
		// We continue to round-robin volume names that look like `Name-Id` also; this is a useful
		// feature for users that are creating statefulset-like functionality without using statefulsets.
		lastDash := strings.LastIndexByte(pvcName, '-')
		if lastDash != -1 {
			statefulsetIDString := pvcName[lastDash+1:]
			statefulsetID, err := strconv.ParseUint(statefulsetIDString, 10, 32)
			if err == nil {
				// Offset by the statefulsetID, so we round-robin across zones
				index = uint32(statefulsetID)
				// We still hash the volume name, but only the prefix
				hashString = pvcName[:lastDash]

				// In the special case where it looks like `ClaimName-StatefulSetName-Id`,
				// hash only the StatefulSetName, so that different claims on the same StatefulSet
				// member end up in the same zone.
				// Note that StatefulSetName (and ClaimName) might themselves both have dashes.
				// We actually just take the portion after the final - of ClaimName-StatefulSetName.
				// For our purposes it doesn't much matter (just suboptimal spreading).
				lastDash := strings.LastIndexByte(hashString, '-')
				if lastDash != -1 {
					hashString = hashString[lastDash+1:]
				}
			}
		}

		// We hash the (base) volume name, so we don't bias towards the first N zones
		h := fnv.New32()
		h.Write([]byte(hashString))
		hash = h.Sum32()
	}

	return hash, index
}
