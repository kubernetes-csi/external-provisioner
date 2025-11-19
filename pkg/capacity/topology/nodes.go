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

package topology

import (
	"context"
	"reflect"
	"sort"
	"sync"
	"sync/atomic"

	v1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	coreinformersv1 "k8s.io/client-go/informers/core/v1"
	storageinformersv1 "k8s.io/client-go/informers/storage/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
)

// NewNodeTopology returns an informer that synthesizes storage
// topology segments based on the accessible topology that each CSI
// driver node instance reports.  See
// https://github.com/kubernetes/enhancements/tree/master/keps/sig-storage/1472-storage-capacity-tracking#with-central-controller
// for details.
func NewNodeTopology(
	driverName string,
	client kubernetes.Interface,
	nodeInformer coreinformersv1.NodeInformer,
	csiNodeInformer storageinformersv1.CSINodeInformer,
	queue workqueue.TypedRateLimitingInterface[string],
) Informer {
	nt := &nodeTopology{
		driverName:      driverName,
		client:          client,
		nodeInformer:    nodeInformer,
		csiNodeInformer: csiNodeInformer,
		queue:           queue,
	}

	// Whenever Node or CSINode objects change, we need to
	// recalculate the new topology segments. We could do that
	// immediately, but it is better to let the input data settle
	// a bit and just remember that there is work to be done.
	nodeHandler := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj any) {
			node, ok := obj.(*v1.Node)
			if !ok {
				klog.Errorf("added object: expected Node, got %T -> ignoring it", obj)
				return
			}
			klog.V(5).Infof("capacity topology: new node: %s", node.Name)
			queue.Add("")
		},
		UpdateFunc: func(oldObj any, newObj any) {
			oldNode, ok := oldObj.(*v1.Node)
			if !ok {
				klog.Errorf("original object: expected Node, got %T -> ignoring it", oldObj)
				return
			}
			newNode, ok := newObj.(*v1.Node)
			if !ok {
				klog.Errorf("updated object: expected Node, got %T -> ignoring it", newObj)
				return
			}
			if reflect.DeepEqual(oldNode.Labels, newNode.Labels) {
				// Shortcut: labels haven't changed, no need to sync.
				return
			}
			klog.V(5).Infof("capacity topology: updated node: %s", newNode.Name)
			queue.Add("")
		},
		DeleteFunc: func(obj any) {
			// Beware of "xxx deleted" events
			if unknown, ok := obj.(cache.DeletedFinalStateUnknown); ok && unknown.Obj != nil {
				obj = unknown.Obj
			}
			node, ok := obj.(*v1.Node)
			if !ok {
				klog.Errorf("deleted object: expected Node, got %T -> ignoring it", obj)
				return
			}
			klog.V(5).Infof("capacity topology: removed node: %s", node.Name)
			queue.Add("")
		},
	}
	nodeInformer.Informer().AddEventHandler(nodeHandler)
	csiNodeHandler := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj any) {
			csiNode, ok := obj.(*storagev1.CSINode)
			if !ok {
				klog.Errorf("added object: expected CSINode, got %T -> ignoring it", obj)
				return
			}
			klog.V(5).Infof("capacity topology: new CSINode: %s", csiNode.Name)
			queue.Add("")
		},
		UpdateFunc: func(oldObj any, newObj any) {
			oldCSINode, ok := oldObj.(*storagev1.CSINode)
			if !ok {
				klog.Errorf("original object: expected CSINode, got %T -> ignoring it", oldObj)
				return
			}
			newCSINode, ok := newObj.(*storagev1.CSINode)
			if !ok {
				klog.Errorf("updated object: expected CSINode, got %T -> ignoring it", newObj)
				return
			}
			oldKeys := nt.driverTopologyKeys(oldCSINode)
			newKeys := nt.driverTopologyKeys(newCSINode)
			if reflect.DeepEqual(oldKeys, newKeys) {
				// Shortcut: keys haven't changed, no need to sync.
				return
			}
			klog.V(5).Infof("capacity topology: updated CSINode: %s", newCSINode.Name)
			queue.Add("")
		},
		DeleteFunc: func(obj any) {
			// Beware of "xxx deleted" events
			if unknown, ok := obj.(cache.DeletedFinalStateUnknown); ok && unknown.Obj != nil {
				obj = unknown.Obj
			}
			csiNode, ok := obj.(*storagev1.CSINode)
			if !ok {
				klog.Errorf("deleted object: expected CSINode, got %T -> ignoring it", obj)
				return
			}
			klog.V(5).Infof("capacity topology: removed CSINode: %s", csiNode.Name)
			queue.Add("")
		},
	}
	csiNodeInformer.Informer().AddEventHandler(csiNodeHandler)

	return nt
}

var _ Informer = &nodeTopology{}

type nodeTopology struct {
	driverName      string
	client          kubernetes.Interface
	nodeInformer    coreinformersv1.NodeInformer
	csiNodeInformer storageinformersv1.CSINodeInformer
	queue           workqueue.TypedRateLimitingInterface[string]
	hasSynced       atomic.Bool

	mutex sync.Mutex
	// segments hold a list of all currently known topology segments.
	segments []*Segment
	// callbacks contains all callbacks that need to be invoked
	// after making changes to the list of known segments.
	callbacks []Callback
}

// driverTopologyKeys returns nil if the driver is not running
// on the node, otherwise at least an empty slice of topology keys.
func (nt *nodeTopology) driverTopologyKeys(csiNode *storagev1.CSINode) []string {
	for _, csiNodeDriver := range csiNode.Spec.Drivers {
		if csiNodeDriver.Name == nt.driverName {
			if csiNodeDriver.TopologyKeys == nil {
				return []string{}
			}
			return csiNodeDriver.TopologyKeys
		}
	}
	return nil
}

func (nt *nodeTopology) AddCallback(cb Callback) {
	nt.mutex.Lock()
	defer nt.mutex.Unlock()

	nt.callbacks = append(nt.callbacks, cb)
}

func (nt *nodeTopology) List() []*Segment {
	nt.mutex.Lock()
	defer nt.mutex.Unlock()

	// We need to return a new slice to protect against future
	// changes in nt.segments. The segments themselves are
	// immutable and shared.
	segments := make([]*Segment, len(nt.segments))
	copy(segments, nt.segments)
	return segments
}

func (nt *nodeTopology) RunWorker(ctx context.Context) {
	klog.Info("Started node topology worker")
	defer klog.Info("Shutting node topology worker")

	for nt.processNextWorkItem(ctx) {
	}
}

func (nt *nodeTopology) upstreamSynced() bool {
	return nt.nodeInformer.Informer().HasSynced() &&
		nt.csiNodeInformer.Informer().HasSynced()
}

func (nt *nodeTopology) HasSynced() bool {
	if nt.hasSynced.Load() {
		return true
	}
	if nt.upstreamSynced() {
		// Now that both informers are up-to-date,
		// trigger a sync to update the list of topology segments.
		nt.queue.Add("")
	}
	return false
}

func (nt *nodeTopology) processNextWorkItem(ctx context.Context) bool {
	obj, shutdown := nt.queue.Get()
	if shutdown {
		return false
	}
	defer nt.queue.Done(obj)
	nt.sync(ctx)
	return true
}

func (nt *nodeTopology) sync(_ context.Context) {
	if !nt.hasSynced.Load() && nt.upstreamSynced() {
		// We are not yet synced, but the upstream informers are.
		// we will become synced when this function returns
		defer nt.hasSynced.Store(true)
	}
	// For all nodes on which the driver is registered, collect the topology key/value pairs
	// and sort them by key name to make the result deterministic. Skip all segments that have
	// been seen before.
	segments := nt.List()
	removalCandidates := map[*Segment]bool{}
	var addedSegments, removedSegments []*Segment
	for _, segment := range segments {
		// Assume that the segment is removed. Will be set to
		// false if we find out otherwise.
		removalCandidates[segment] = true
	}

	csiNodes, err := nt.csiNodeInformer.Lister().List(labels.Everything())
	if err != nil {
		utilruntime.HandleError(err)
		return
	}
	existingSegments := make([]*Segment, 0, len(segments))
node:
	for _, csiNode := range csiNodes {
		topologyKeys := nt.driverTopologyKeys(csiNode)
		if topologyKeys == nil {
			// Driver not running on node, ignore it.
			continue
		}
		node, err := nt.nodeInformer.Lister().Get(csiNode.Name)
		if err != nil {
			if apierrs.IsNotFound(err) {
				// Obsolete CSINode object? Ignore it.
				continue
			}
			// This shouldn't happen. If it does,
			// something is very wrong and we give up.
			utilruntime.HandleError(err)
			return
		}

		newSegment := Segment{}
		sort.Strings(topologyKeys)
		for _, key := range topologyKeys {
			value, ok := node.Labels[key]
			if !ok {
				// The driver announced some topology key and kubelet recorded
				// it in CSINode, but we haven't seen the corresponding
				// node update yet as the label is not set. Ignore the node
				// for now, we'll sync up when we get the node update.
				continue node
			}
			newSegment = append(newSegment, SegmentEntry{key, value})
		}

		// Add it only if new, otherwise look at the next node.
		for _, segment := range segments {
			if newSegment.Compare(*segment) == 0 {
				// Reuse a segment instead of using the new one. This keeps pointers stable.
				removalCandidates[segment] = false
				existingSegments = append(existingSegments, segment)
				continue node
			}
		}
		for _, segment := range addedSegments {
			if newSegment.Compare(*segment) == 0 {
				// We already discovered this new segment.
				continue node
			}
		}

		// A completely new segment.
		addedSegments = append(addedSegments, &newSegment)
		existingSegments = append(existingSegments, &newSegment)
	}

	// Lock while making changes, but unlock before actually invoking callbacks.
	nt.mutex.Lock()
	nt.segments = existingSegments

	// Theoretically callbacks could change while we don't have
	// the lock, so make a copy.
	callbacks := make([]Callback, len(nt.callbacks))
	copy(callbacks, nt.callbacks)
	nt.mutex.Unlock()

	for segment, wasRemoved := range removalCandidates {
		if wasRemoved {
			removedSegments = append(removedSegments, segment)
		}
	}
	if len(addedSegments) > 0 || len(removedSegments) > 0 {
		klog.V(5).Infof("topology changed: added %v, removed %v", addedSegments, removedSegments)
		for _, cb := range callbacks {
			cb(addedSegments, removedSegments)
		}
	} else {
		klog.V(5).Infof("topology unchanged")
	}
}
