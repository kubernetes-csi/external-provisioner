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
	"fmt"
	"maps"
	"sort"
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	coreinformersv1 "k8s.io/client-go/informers/core/v1"
	storageinformersv1 "k8s.io/client-go/informers/storage/v1"
	fakeclientset "k8s.io/client-go/kubernetes/fake"
	corelistersv1 "k8s.io/client-go/listers/core/v1"
	storagelistersv1 "k8s.io/client-go/listers/storage/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
)

func init() {
	klog.InitFlags(nil)
}

const (
	driverName  = "my-csi-driver"
	node1       = "node1"
	node2       = "node2"
	topologyKey = "csi.example.com/segment"
)

var (
	localStorageKey         = "nodename"
	localStorageKeys        = []string{localStorageKey}
	localStorageLabelsNode1 = map[string]string{localStorageKey: node1}
	localStorageNode1       = &Segment{
		{localStorageKey, node1},
	}
	localStorageLabelsNode2 = map[string]string{localStorageKey: node2}
	localStorageNode2       = &Segment{
		{localStorageKey, node2},
	}
	networkStorageKeys   = []string{"A", "B", "C"}
	networkStorageLabels = map[string]string{
		networkStorageKeys[0]: "US",
		networkStorageKeys[1]: "NY",
		networkStorageKeys[2]: "1",
	}
	networkStorage = &Segment{
		{networkStorageKeys[0], "US"},
		{networkStorageKeys[1], "NY"},
		{networkStorageKeys[2], "1"},
	}
	networkStorageLabels2 = map[string]string{
		networkStorageKeys[0]: "US",
		networkStorageKeys[1]: "NY",
		networkStorageKeys[2]: "2",
	}
	networkStorage2 = &Segment{
		{networkStorageKeys[0], "US"},
		{networkStorageKeys[1], "NY"},
		{networkStorageKeys[2], "2"},
	}
)

func removeNode(t *testing.T, client *fakeclientset.Clientset, nodeName string) {
	err := client.CoreV1().Nodes().Delete(context.Background(), nodeName, metav1.DeleteOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func removeCSINode(t *testing.T, client *fakeclientset.Clientset, nodeName string) {
	err := client.StorageV1().CSINodes().Delete(context.Background(), nodeName, metav1.DeleteOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestNodeTopology checks that node labels are correctly transformed
// into topology segments.
func TestNodeTopology(t *testing.T) {
	testcases := map[string]struct {
		driverName              string
		initialNodes            []testNode
		expectedSegments        []*Segment
		update                  func(t *testing.T, client *fakeclientset.Clientset)
		expectedUpdatedSegments []*Segment
	}{
		"empty": {},
		"one-node": {
			initialNodes: []testNode{
				{
					name: node1,
					driverKeys: map[string][]string{
						driverName: localStorageKeys,
					},
					labels: localStorageLabelsNode1,
				},
			},
			expectedSegments: []*Segment{localStorageNode1},
		},
		"missing-csi-node": {
			initialNodes: []testNode{
				{
					name: node1,
					driverKeys: map[string][]string{
						driverName: localStorageKeys,
					},
					labels:              localStorageLabelsNode1,
					skipCSINodeCreation: true,
				},
			},
		},
		"missing-node": {
			initialNodes: []testNode{
				{
					name: node1,
					driverKeys: map[string][]string{
						driverName: localStorageKeys,
					},
					labels:           localStorageLabelsNode1,
					skipNodeCreation: true,
				},
			},
		},
		"missing-node-labels": {
			initialNodes: []testNode{
				{
					name: node1,
					driverKeys: map[string][]string{
						driverName: localStorageKeys,
					},
				},
			},
		},
		"two-nodes": {
			initialNodes: []testNode{
				{
					name: node1,
					driverKeys: map[string][]string{
						driverName: localStorageKeys,
					},
					labels: localStorageLabelsNode1,
				},
				{
					name: node2,
					driverKeys: map[string][]string{
						driverName: localStorageKeys,
					},
					labels: localStorageLabelsNode2,
				},
			},
			expectedSegments: []*Segment{localStorageNode1, localStorageNode2},
		},
		"shared-storage": {
			initialNodes: []testNode{
				{
					name: node1,
					driverKeys: map[string][]string{
						driverName: localStorageKeys,
					},
					labels: localStorageLabelsNode1,
				},
				{
					name: node2,
					driverKeys: map[string][]string{
						driverName: localStorageKeys,
					},
					labels: localStorageLabelsNode1,
				},
			},
			expectedSegments: []*Segment{localStorageNode1},
		},
		"other-shared-storage": {
			initialNodes: []testNode{
				{
					name: node1,
					driverKeys: map[string][]string{
						driverName: localStorageKeys,
					},
					labels: localStorageLabelsNode2,
				},
				{
					name: node2,
					driverKeys: map[string][]string{
						driverName: localStorageKeys,
					},
					labels: localStorageLabelsNode2,
				},
			},
			expectedSegments: []*Segment{localStorageNode2},
		},
		"deep-topology": {
			initialNodes: []testNode{
				{
					name: node1,
					driverKeys: map[string][]string{
						driverName: networkStorageKeys,
					},
					labels: networkStorageLabels,
				},
				{
					name: node2,
					driverKeys: map[string][]string{
						driverName: networkStorageKeys,
					},
					labels: networkStorageLabels,
				},
			},
			expectedSegments: []*Segment{networkStorage},
		},
		"mixed-topology": {
			initialNodes: []testNode{
				{
					name: node1,
					driverKeys: map[string][]string{
						driverName: localStorageKeys,
					},
					labels: localStorageLabelsNode1,
				},
				{
					name: node2,
					driverKeys: map[string][]string{
						driverName: networkStorageKeys,
					},
					labels: networkStorageLabels,
				},
			},
			expectedSegments: []*Segment{localStorageNode1, networkStorage},
		},
		"partial-match": {
			initialNodes: []testNode{
				{
					name: node1,
					driverKeys: map[string][]string{
						driverName: networkStorageKeys,
					},
					labels: networkStorageLabels,
				},
				{
					name: node2,
					driverKeys: map[string][]string{
						driverName: networkStorageKeys,
					},
					labels: networkStorageLabels2,
				},
			},
			expectedSegments: []*Segment{networkStorage, networkStorage2},
		},
		"unsorted-keys": {
			initialNodes: []testNode{
				{
					name: node1,
					driverKeys: map[string][]string{
						// This node reports keys in reverse order, which must not make a difference.
						driverName: {networkStorageKeys[2], networkStorageKeys[1], networkStorageKeys[0]},
					},
					labels: networkStorageLabels,
				},
				{
					name: node2,
					driverKeys: map[string][]string{
						driverName: networkStorageKeys,
					},
					labels: networkStorageLabels,
				},
			},
			expectedSegments: []*Segment{networkStorage},
		},
		"wrong-driver": {
			driverName: "other-driver",
			initialNodes: []testNode{
				{
					name: node1,
					driverKeys: map[string][]string{
						driverName: localStorageKeys,
					},
					labels: localStorageLabelsNode1,
				},
			},
		},
		"remove-csi-node": {
			initialNodes: []testNode{
				{
					name: node1,
					driverKeys: map[string][]string{
						driverName: localStorageKeys,
					},
					labels: localStorageLabelsNode1,
				},
			},
			expectedSegments: []*Segment{localStorageNode1},
			update: func(t *testing.T, client *fakeclientset.Clientset) {
				removeCSINode(t, client, node1)
			},
		},
		"remove-node": {
			initialNodes: []testNode{
				{
					name: node1,
					driverKeys: map[string][]string{
						driverName: localStorageKeys,
					},
					labels: localStorageLabelsNode1,
				},
			},
			expectedSegments: []*Segment{localStorageNode1},
			update: func(t *testing.T, client *fakeclientset.Clientset) {
				removeNode(t, client, node1)
			},
		},
		"remove-driver": {
			initialNodes: []testNode{
				{
					name: node1,
					driverKeys: map[string][]string{
						driverName: localStorageKeys,
					},
					labels: localStorageLabelsNode1,
				},
			},
			expectedSegments: []*Segment{localStorageNode1},
			update: func(t *testing.T, client *fakeclientset.Clientset) {
				csiNode, err := client.StorageV1().CSINodes().Get(context.Background(), node1, metav1.GetOptions{})
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				csiNode.Spec.Drivers = nil
				if _, err := client.StorageV1().CSINodes().Update(context.Background(), csiNode, metav1.UpdateOptions{}); err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			},
		},
		"add-driver": {
			initialNodes: []testNode{
				{
					name: node1,
				},
			},
			expectedSegments: nil,
			update: func(t *testing.T, client *fakeclientset.Clientset) {
				csiNode, err := client.StorageV1().CSINodes().Get(context.Background(), node1, metav1.GetOptions{})
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				csiNode.Spec.Drivers = append(csiNode.Spec.Drivers, storagev1.CSINodeDriver{
					Name:         driverName,
					TopologyKeys: localStorageKeys,
				})
				if _, err := client.StorageV1().CSINodes().Update(context.Background(), csiNode, metav1.UpdateOptions{}); err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				node, err := client.CoreV1().Nodes().Get(context.Background(), node1, metav1.GetOptions{})
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if node.Labels == nil {
					node.Labels = make(map[string]string)
				}
				maps.Copy(node.Labels, localStorageLabelsNode1)
				if _, err := client.CoreV1().Nodes().Update(context.Background(), node, metav1.UpdateOptions{}); err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			},
			expectedUpdatedSegments: []*Segment{localStorageNode1},
		},
		"update-node": {
			initialNodes: []testNode{
				{
					name: node1,
				},
			},
			expectedSegments: nil,
			update: func(t *testing.T, client *fakeclientset.Clientset) {
				node, err := client.CoreV1().Nodes().Get(context.Background(), node1, metav1.GetOptions{})
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if node.Labels == nil {
					node.Labels = make(map[string]string)
				}
				maps.Copy(node.Labels, localStorageLabelsNode1)
				if _, err := client.CoreV1().Nodes().Update(context.Background(), node, metav1.UpdateOptions{}); err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			},
		},
		"update-csi-node": {
			initialNodes: []testNode{
				{
					name: node1,
				},
			},
			expectedSegments: nil,
			update: func(t *testing.T, client *fakeclientset.Clientset) {
				csiNode, err := client.StorageV1().CSINodes().Get(context.Background(), node1, metav1.GetOptions{})
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				csiNode.Spec.Drivers = append(csiNode.Spec.Drivers, storagev1.CSINodeDriver{
					Name:         driverName,
					TopologyKeys: localStorageKeys,
				})
				if _, err := client.StorageV1().CSINodes().Update(context.Background(), csiNode, metav1.UpdateOptions{}); err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			},
		},
		"add-node": {
			initialNodes: []testNode{
				{
					name: node1,
					driverKeys: map[string][]string{
						driverName: localStorageKeys,
					},
					labels:           localStorageLabelsNode1,
					skipNodeCreation: true,
				},
			},
			expectedSegments: nil,
			update: func(t *testing.T, client *fakeclientset.Clientset) {
				node := &v1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name:   node1,
						Labels: localStorageLabelsNode1,
					},
				}
				if _, err := client.CoreV1().Nodes().Create(context.Background(), node, metav1.CreateOptions{}); err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			},
			expectedUpdatedSegments: []*Segment{localStorageNode1},
		},
		"add-csi-node": {
			initialNodes: []testNode{
				{
					name: node1,
					driverKeys: map[string][]string{
						driverName: localStorageKeys,
					},
					labels:              localStorageLabelsNode1,
					skipCSINodeCreation: true,
				},
			},
			expectedSegments: nil,
			update: func(t *testing.T, client *fakeclientset.Clientset) {
				csiNode := &storagev1.CSINode{
					ObjectMeta: metav1.ObjectMeta{
						Name: node1,
					},
					Spec: storagev1.CSINodeSpec{
						Drivers: []storagev1.CSINodeDriver{
							{
								Name:         driverName,
								TopologyKeys: localStorageKeys,
							},
						},
					},
				}
				if _, err := client.StorageV1().CSINodes().Create(context.Background(), csiNode, metav1.CreateOptions{}); err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			},
			expectedUpdatedSegments: []*Segment{localStorageNode1},
		},
		"change-labels": {
			initialNodes: []testNode{
				{
					name: node1,
					driverKeys: map[string][]string{
						driverName: localStorageKeys,
					},
					labels: localStorageLabelsNode1,
				},
			},
			expectedSegments: []*Segment{localStorageNode1},
			update: func(t *testing.T, client *fakeclientset.Clientset) {
				node, err := client.CoreV1().Nodes().Get(context.Background(), node1, metav1.GetOptions{})
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				// This isn't a realistic test case because CSI drivers cannot change their topology?
				// We support it anyway.
				node.Labels[localStorageKey] = node2
				if _, err := client.CoreV1().Nodes().Update(context.Background(), node, metav1.UpdateOptions{}); err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			},
			expectedUpdatedSegments: []*Segment{localStorageNode2},
		},
	}

	for name, tc := range testcases {
		// Not run in parallel. That doesn't work well in combination with global logging.
		t.Run(name, func(t *testing.T) {
			//
			// Instead of waiting for informers to sync, we validate specific conditions
			// on the topology segments with exponential backoff, which is more resilient.
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			testDriverName := tc.driverName
			if testDriverName == "" {
				testDriverName = driverName
			}

			var objects []runtime.Object
			objects = append(objects, makeNodes(tc.initialNodes)...)
			clientSet := fakeclientset.NewSimpleClientset(objects...)
			nt := fakeNodeTopology(ctx, testDriverName, clientSet)

			// Validate initial state - this will wait for the condition with exponential backoff
			validate(ctx, t, nt, tc.expectedSegments, nil, tc.expectedSegments)

			if tc.update != nil {
				tc.update(t, clientSet)

				// Determine the expected changes based on the delta.
				var expectedAdded, expectedRemoved []*Segment
				for _, segment := range tc.expectedUpdatedSegments {
					if !containsSegment(tc.expectedSegments, segment) {
						expectedAdded = append(expectedAdded, segment)
					}
				}
				for _, segment := range tc.expectedSegments {
					if !containsSegment(tc.expectedUpdatedSegments, segment) {
						expectedRemoved = append(expectedRemoved, segment)
					}
				}

				// Validate updated state - this will wait for the condition with exponential backoff
				validate(ctx, t, nt, expectedAdded, expectedRemoved, tc.expectedUpdatedSegments)
			}
		})
	}
}

type segmentsFound map[*Segment]bool

func (sf segmentsFound) Found() []*Segment {
	var found []*Segment
	for key, value := range sf {
		if value {
			found = append(found, key)
		}
	}
	return found
}

func addTestCallback(nt *nodeTopology) (added, removed segmentsFound, called *bool) {
	added = segmentsFound{}
	removed = segmentsFound{}
	called = new(bool)
	nt.AddCallback(func(a, r []*Segment) {
		*called = true
		for _, segment := range a {
			added[segment] = true
		}
		for _, segment := range r {
			removed[segment] = true
		}
	})
	return
}

func containsSegment(segments []*Segment, segment *Segment) bool {
	for _, s := range segments {
		if s.Compare(*segment) == 0 {
			return true
		}
	}
	return false
}

func fakeNodeTopology(ctx context.Context, testDriverName string, client *fakeclientset.Clientset) *nodeTopology {
	// We don't need resyncs, they just lead to confusing log output if they get triggered while already some
	// new test is running.
	informerFactory := informers.NewSharedInformerFactory(client, 0*time.Second /* no resync */)
	nodeInformer := informerFactory.Core().V1().Nodes()
	csiNodeInformer := informerFactory.Storage().V1().CSINodes()
	rateLimiter := workqueue.NewTypedItemExponentialFailureRateLimiter[string](time.Second, 2*time.Second)
	queue := workqueue.NewTypedRateLimitingQueueWithConfig(rateLimiter, workqueue.TypedRateLimitingQueueConfig[string]{Name: "items"})

	nt := NewNodeTopology(
		testDriverName,
		client,
		nodeInformer,
		csiNodeInformer,
		queue,
	).(*nodeTopology)

	go informerFactory.Start(ctx.Done())
	informerFactory.WaitForCacheSync(ctx.Done())

	return nt
}

// waitForSegmentsCondition waits for a specific condition on segments to be met.
// This uses exponential backoff and validates the specific condition rather than
// waiting for informers to sync, which is more resilient to timing issues.
// This follows the standard Kubernetes testing pattern.
func waitForSegmentsCondition(ctx context.Context, t *testing.T, nt *nodeTopology, conditionDesc string, condition func([]*Segment) bool) error {
	t.Helper()

	// Use exponential backoff: start at 10ms, max 1s, with a 2x factor
	// This will retry up to 20 times with exponential backoff
	backoff := wait.Backoff{
		Duration: 10 * time.Millisecond,
		Factor:   2.0,
		Jitter:   0.1,
		Steps:    20, // ~10s total
		Cap:      2 * time.Second,
	}

	var lastSegments []*Segment
	err := wait.ExponentialBackoffWithContext(ctx, backoff, func(ctx context.Context) (bool, error) {
		// Give informers a brief moment to process events from the fake client.
		// The fake client updates are synchronous, but informer event handlers run asynchronously.
		time.Sleep(5 * time.Millisecond)

		// Trigger sync to process any pending events from informers
		nt.sync(ctx)

		// Get the current segments
		lastSegments = nt.List()

		// Check the condition
		return condition(lastSegments), nil
	})

	if err != nil {
		t.Logf("Condition %q not met. Last segments: %v", conditionDesc, segmentsToStrings(lastSegments))
		return fmt.Errorf("waiting for condition %q: %w", conditionDesc, err)
	}
	return nil
}

func validate(ctx context.Context, t *testing.T, nt *nodeTopology, expectedAdded, expectedRemoved, expectedAll []*Segment) {
	t.Helper()

	added, removed, called := addTestCallback(nt)

	// Wait for the expected final state with exponential backoff
	err := waitForSegmentsCondition(ctx, t, nt, "final segments match expected", func(segments []*Segment) bool {
		return segmentsMatch(segments, expectedAll)
	})
	if err != nil {
		t.Errorf("Failed to reach expected segment state: %v", err)
		validateSegments(t, "final", nt.List(), expectedAll)
		t.FailNow()
	}

	expectedChanges := len(expectedAdded) > 0 || len(expectedRemoved) > 0
	if expectedChanges && !*called {
		t.Error("change callback not invoked")
	}
	if !expectedChanges && *called {
		t.Error("change callback invoked unexpectedly")
	}
	validateSegments(t, "added", added.Found(), expectedAdded)
	validateSegments(t, "removed", removed.Found(), expectedRemoved)
	validateSegments(t, "final", nt.List(), expectedAll)

	if t.Failed() {
		t.FailNow()
	}
}

// segmentsMatch checks if two segment lists are equivalent (same segments, ignoring order)
func segmentsMatch(actual, expected []*Segment) bool {
	if len(actual) != len(expected) {
		return false
	}

	// Check that all expected segments are present
	for _, exp := range expected {
		if !containsSegment(actual, exp) {
			return false
		}
	}

	// Check that no unexpected segments are present
	for _, act := range actual {
		if !containsSegment(expected, act) {
			return false
		}
	}

	return true
}

func validateSegments(t *testing.T, what string, actual, expected []*Segment) {
	// We can just compare the string representation because that covers all
	// relevant content of the segments and is readable.
	found := map[string]bool{}
	for _, str := range segmentsToStrings(expected) {
		found[str] = false
	}
	for _, str := range segmentsToStrings(actual) {
		_, exists := found[str]
		if !exists {
			t.Errorf("unexpected %s segment: %s", what, str)
			t.Fail()
			continue
		}
		found[str] = true
	}
	for str, matched := range found {
		if !matched {
			t.Errorf("expected %s segment not found: %s", what, str)
			t.Fail()
		}
	}
}

func segmentsToStrings(segments []*Segment) []string {
	str := []string{}
	for _, segment := range segments {
		str = append(str, segment.SimpleString())
	}
	sort.Strings(str)
	return str
}

type testNode struct {
	name                                  string
	driverKeys                            map[string][]string
	labels                                map[string]string
	skipNodeCreation, skipCSINodeCreation bool
}

func makeNodes(nodes []testNode) []runtime.Object {
	var objects []runtime.Object

	for _, node := range nodes {
		if !node.skipNodeCreation {
			objects = append(objects, &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name:   node.name,
					Labels: node.labels,
				},
			})
		}
		if !node.skipCSINodeCreation {
			csiNode := &storagev1.CSINode{
				ObjectMeta: metav1.ObjectMeta{
					Name: node.name,
				},
			}
			for driver, keys := range node.driverKeys {
				csiNode.Spec.Drivers = append(csiNode.Spec.Drivers,
					storagev1.CSINodeDriver{
						Name:         driver,
						TopologyKeys: keys,
					})
			}
			objects = append(objects, csiNode)
		}
	}
	return objects
}

// BenchmarkSync checks how quickly sync can process a set of CSINode
// objects when the initial state is "no existing CSIStorageCapacity" or
// "everything processed already once, no changes". The number of nodes and
// nodes per topology segment gets varied.
func BenchmarkSync(b *testing.B) {
	// for a smooth graph: for numNodes := 0; numNodes <= 10000; numNodes += 100
	for numNodes := 1; numNodes <= 10000; numNodes *= 100 {
		b.Run(fmt.Sprintf("numNodes=%d", numNodes), func(b *testing.B) {
			for segmentSize := 1; segmentSize <= numNodes; segmentSize *= 10 {
				b.Logf("%d nodes, %d segment size -> %d segments", numNodes, segmentSize, (numNodes+segmentSize-1)/segmentSize)
				b.Run(fmt.Sprintf("segmentSize=%d", segmentSize), func(b *testing.B) {
					b.Run("initial", func(b *testing.B) {
						benchmarkSyncInitial(b, numNodes, segmentSize)
					})
					b.Run("refresh", func(b *testing.B) {
						benchmarkSyncRefresh(b, numNodes, segmentSize)
					})
				})
			}
		})
	}
}

func benchmarkSyncInitial(b *testing.B, numNodes, segmentSize int) {
	nodeInformer, csiNodeInformer := createTopology(numNodes, segmentSize)
	ctx := context.Background()
	expectedSize := (numNodes + segmentSize - 1) / segmentSize

	for b.Loop() {
		nt := nodeTopology{
			driverName:      driverName,
			nodeInformer:    nodeInformer,
			csiNodeInformer: csiNodeInformer,
		}
		nt.sync(ctx)

		// Some sanity checking...
		actualSize := len(nt.segments)
		if actualSize != expectedSize {
			b.Fatalf("expected %d segments, got %d: %+v", expectedSize, actualSize, nt.segments)
		}
	}
}

func benchmarkSyncRefresh(b *testing.B, numNodes, segmentSize int) {
	nodeInformer, csiNodeInformer := createTopology(numNodes, segmentSize)
	ctx := context.Background()
	expectedSize := (numNodes + segmentSize - 1) / segmentSize

	nt := nodeTopology{
		driverName:      driverName,
		nodeInformer:    nodeInformer,
		csiNodeInformer: csiNodeInformer,
	}
	nt.sync(ctx)

	// Some sanity checking...
	actualSize := len(nt.segments)
	if actualSize != expectedSize {
		b.Fatalf("expected %d segments, got %d: %+v", expectedSize, actualSize, nt.segments)
	}

	for b.Loop() {
		nt.sync(ctx)
	}
}

// createTopology sets up Node and CSINode instances in some in-memory informer which
// just provides enough functionality for sync to work.
func createTopology(numNodes, segmentSize int) (coreinformersv1.NodeInformer, storageinformersv1.CSINodeInformer) {
	nodeInformer := fakeNodeInformer{}
	csiNodeInformer := fakeCSINodeInformer{}

	for i := range numNodes {
		nodeName := fmt.Sprintf("node-%d", i)
		node := &v1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: nodeName,
				Labels: map[string]string{
					topologyKey: fmt.Sprintf("segment-%d", i/segmentSize),
				},
			},
		}
		nodeInformer[nodeName] = node

		csiNode := &storagev1.CSINode{
			ObjectMeta: metav1.ObjectMeta{
				Name: nodeName,
			},
			Spec: storagev1.CSINodeSpec{
				Drivers: []storagev1.CSINodeDriver{
					{
						Name:         driverName,
						TopologyKeys: []string{topologyKey},
					},
				},
			},
		}
		csiNodeInformer = append(csiNodeInformer, csiNode)
	}
	return nodeInformer, csiNodeInformer
}

type fakeNodeInformer map[string]*v1.Node

func (f fakeNodeInformer) Informer() cache.SharedIndexInformer {
	return nil
}

func (f fakeNodeInformer) Lister() corelistersv1.NodeLister {
	return f
}

func (f fakeNodeInformer) Get(name string) (*v1.Node, error) {
	if node, ok := f[name]; ok {
		return node, nil
	}
	panic(fmt.Sprintf("node %q should have been defined", name))
}

func (f fakeNodeInformer) List(selector labels.Selector) (ret []*v1.Node, err error) {
	panic("not implemented")
}

type fakeCSINodeInformer []*storagev1.CSINode

func (f fakeCSINodeInformer) Informer() cache.SharedIndexInformer {
	return nil
}

func (f fakeCSINodeInformer) Lister() storagelistersv1.CSINodeLister {
	return f
}

func (f fakeCSINodeInformer) Get(name string) (*storagev1.CSINode, error) {
	panic("not implemented")
}

func (f fakeCSINodeInformer) List(selector labels.Selector) (ret []*storagev1.CSINode, err error) {
	return []*storagev1.CSINode(f), nil
}
