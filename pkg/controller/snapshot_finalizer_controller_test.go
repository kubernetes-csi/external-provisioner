package controller

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/kubernetes-csi/csi-lib-utils/rpc"
	v1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	discoveryfake "k8s.io/client-go/discovery/fake"
	corelisters "k8s.io/client-go/listers/core/v1"
	k8stesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"

	crdv1 "github.com/kubernetes-csi/external-snapshotter/client/v8/apis/volumesnapshot/v1"
	snapshotfake "github.com/kubernetes-csi/external-snapshotter/client/v8/clientset/versioned/fake"
)

func newFakeClaimLister(pvcs ...*v1.PersistentVolumeClaim) corelisters.PersistentVolumeClaimLister {
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	for _, pvc := range pvcs {
		indexer.Add(pvc)
	}
	return corelisters.NewPersistentVolumeClaimLister(indexer)
}

// fakeSnapshotDiscovery fails the first `failures` discovery calls with HTTP 429,
// then reports whether snapshot.storage.k8s.io/v1 VolumeSnapshot is served.
type fakeSnapshotDiscovery struct {
	*discoveryfake.FakeDiscovery
	calls int
}

func newFakeSnapshotDiscovery(served bool, failures int) *fakeSnapshotDiscovery {
	d := &fakeSnapshotDiscovery{FakeDiscovery: &discoveryfake.FakeDiscovery{Fake: &k8stesting.Fake{}}}
	if served {
		d.Resources = []*metav1.APIResourceList{{
			GroupVersion: "snapshot.storage.k8s.io/v1",
			APIResources: []metav1.APIResource{{Kind: "VolumeSnapshot"}},
		}}
	}
	d.PrependReactor("get", "resource", func(k8stesting.Action) (bool, runtime.Object, error) {
		d.calls++
		if d.calls <= failures {
			return true, nil, apierrs.NewTooManyRequests("too many requests", 1)
		}
		return false, nil, nil
	})
	return d
}

func fakeSnapshotFinalizerController(snapClient *snapshotfake.Clientset, claimLister corelisters.PersistentVolumeClaimLister, snapshots ...*crdv1.VolumeSnapshot) *SnapshotFinalizerController {
	ctrl := NewSnapshotFinalizerController(
		snapClient,
		newFakeSnapshotDiscovery(true, 0),
		claimLister,
		snapshotCapabilities(),
		5*time.Minute,
	)

	// Replace the informer with one pre-populated with test data.
	snapshotInformer := cache.NewSharedInformer(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				return &crdv1.VolumeSnapshotList{}, nil
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				return watch.NewFake(), nil
			},
		},
		&crdv1.VolumeSnapshot{},
		0,
	)
	for _, s := range snapshots {
		snapshotInformer.GetStore().Add(s)
	}
	ctrl.snapshotInformer = snapshotInformer
	return ctrl
}

func TestSyncSnapshot_RemoveFinalizerWhenNoPendingPVC(t *testing.T) {
	snapshotName := "test-snapshot"
	namespace := "default"

	snapshot := &crdv1.VolumeSnapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name:      snapshotName,
			Namespace: namespace,
			Finalizers: []string{
				"snapshot.storage.kubernetes.io/volumesnapshot-bound-protection",
				snapshotSourceProtectionFinalizer,
			},
		},
	}

	boundPVC := &v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "restored-pvc",
			Namespace: namespace,
			UID:       types.UID("pvc-uid-1"),
		},
		Spec: v1.PersistentVolumeClaimSpec{
			DataSource: &v1.TypedLocalObjectReference{
				Kind: snapshotKind,
				Name: snapshotName,
			},
		},
		Status: v1.PersistentVolumeClaimStatus{
			Phase: v1.ClaimBound,
		},
	}

	snapClient := snapshotfake.NewSimpleClientset(snapshot)
	claimLister := newFakeClaimLister(boundPVC)
	ctrl := fakeSnapshotFinalizerController(snapClient, claimLister, snapshot)

	ctrl.sweepOrphanedFinalizers(context.Background())

	updated, err := snapClient.SnapshotV1().VolumeSnapshots(namespace).Get(context.Background(), snapshotName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to get snapshot: %v", err)
	}
	if checkFinalizer(updated, snapshotSourceProtectionFinalizer) {
		t.Error("expected snapshot source-protection finalizer to be removed")
	}
	if !checkFinalizer(updated, "snapshot.storage.kubernetes.io/volumesnapshot-bound-protection") {
		t.Error("expected other finalizers to be preserved")
	}
}

func TestSyncSnapshot_KeepFinalizerWhenPVCPending(t *testing.T) {
	snapshotName := "test-snapshot"
	namespace := "default"

	snapshot := &crdv1.VolumeSnapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name:      snapshotName,
			Namespace: namespace,
			Finalizers: []string{
				snapshotSourceProtectionFinalizer,
			},
		},
	}

	pendingPVC := &v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pending-pvc",
			Namespace: namespace,
			UID:       types.UID("pvc-uid-2"),
		},
		Spec: v1.PersistentVolumeClaimSpec{
			DataSource: &v1.TypedLocalObjectReference{
				Kind: snapshotKind,
				Name: snapshotName,
			},
		},
		Status: v1.PersistentVolumeClaimStatus{
			Phase: v1.ClaimPending,
		},
	}

	snapClient := snapshotfake.NewSimpleClientset(snapshot)
	claimLister := newFakeClaimLister(pendingPVC)
	ctrl := fakeSnapshotFinalizerController(snapClient, claimLister, snapshot)

	ctrl.sweepOrphanedFinalizers(context.Background())

	updated, err := snapClient.SnapshotV1().VolumeSnapshots(namespace).Get(context.Background(), snapshotName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to get snapshot: %v", err)
	}
	if !checkFinalizer(updated, snapshotSourceProtectionFinalizer) {
		t.Error("expected finalizer to still be present")
	}
}

func TestSyncSnapshot_RemoveFinalizerWhenNoPVCExists(t *testing.T) {
	snapshotName := "test-snapshot"
	namespace := "default"

	snapshot := &crdv1.VolumeSnapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name:      snapshotName,
			Namespace: namespace,
			Finalizers: []string{
				snapshotSourceProtectionFinalizer,
			},
		},
	}

	snapClient := snapshotfake.NewSimpleClientset(snapshot)
	claimLister := newFakeClaimLister()
	ctrl := fakeSnapshotFinalizerController(snapClient, claimLister, snapshot)

	ctrl.sweepOrphanedFinalizers(context.Background())

	updated, err := snapClient.SnapshotV1().VolumeSnapshots(namespace).Get(context.Background(), snapshotName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to get snapshot: %v", err)
	}
	if checkFinalizer(updated, snapshotSourceProtectionFinalizer) {
		t.Error("expected finalizer to be removed when no PVCs reference the snapshot")
	}
}

func TestSyncSnapshot_MultiplePVCs(t *testing.T) {
	snapshotName := "test-snapshot"
	namespace := "default"

	snapshot := &crdv1.VolumeSnapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name:      snapshotName,
			Namespace: namespace,
			Finalizers: []string{
				snapshotSourceProtectionFinalizer,
			},
		},
	}

	boundPVC := &v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: "bound-pvc", Namespace: namespace},
		Spec: v1.PersistentVolumeClaimSpec{
			DataSource: &v1.TypedLocalObjectReference{Kind: snapshotKind, Name: snapshotName},
		},
		Status: v1.PersistentVolumeClaimStatus{Phase: v1.ClaimBound},
	}
	pendingPVC := &v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: "pending-pvc", Namespace: namespace},
		Spec: v1.PersistentVolumeClaimSpec{
			DataSource: &v1.TypedLocalObjectReference{Kind: snapshotKind, Name: snapshotName},
		},
		Status: v1.PersistentVolumeClaimStatus{Phase: v1.ClaimPending},
	}

	snapClient := snapshotfake.NewSimpleClientset(snapshot)
	claimLister := newFakeClaimLister(boundPVC, pendingPVC)
	ctrl := fakeSnapshotFinalizerController(snapClient, claimLister, snapshot)

	ctrl.sweepOrphanedFinalizers(context.Background())

	updated, err := snapClient.SnapshotV1().VolumeSnapshots(namespace).Get(context.Background(), snapshotName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to get snapshot: %v", err)
	}
	if !checkFinalizer(updated, snapshotSourceProtectionFinalizer) {
		t.Error("expected finalizer to still be present while provisioning is active")
	}
}

func TestNewSnapshotFinalizerController_NilSnapshotClient(t *testing.T) {
	ctrl := NewSnapshotFinalizerController(nil, newFakeSnapshotDiscovery(true, 0), newFakeClaimLister(), nil, 5*time.Minute)
	if ctrl != nil {
		t.Error("expected nil controller when snapshotClient is nil")
	}
}

func TestNewSnapshotFinalizerController_NoSnapshotCapability(t *testing.T) {
	capabilities := rpc.ControllerCapabilitySet{
		csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME: true,
	}
	ctrl := NewSnapshotFinalizerController(
		snapshotfake.NewSimpleClientset(),
		newFakeSnapshotDiscovery(true, 0),
		newFakeClaimLister(),
		capabilities,
		5*time.Minute,
	)
	if ctrl != nil {
		t.Error("expected nil controller when driver lacks snapshot capability")
	}
}

func TestSweepOrphanedFinalizers(t *testing.T) {
	namespace := "default"

	orphanSnapshot := &crdv1.VolumeSnapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "orphan-snap",
			Namespace:  namespace,
			Finalizers: []string{snapshotSourceProtectionFinalizer},
		},
	}
	cleanSnapshot := &crdv1.VolumeSnapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "clean-snap",
			Namespace:  namespace,
			Finalizers: []string{"other-finalizer"},
		},
	}

	snapClient := snapshotfake.NewSimpleClientset(orphanSnapshot, cleanSnapshot)
	claimLister := newFakeClaimLister()
	ctrl := fakeSnapshotFinalizerController(snapClient, claimLister, orphanSnapshot, cleanSnapshot)

	ctrl.sweepOrphanedFinalizers(context.Background())

	updated, err := snapClient.SnapshotV1().VolumeSnapshots(namespace).Get(context.Background(), "orphan-snap", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to get snapshot: %v", err)
	}
	if checkFinalizer(updated, snapshotSourceProtectionFinalizer) {
		t.Error("expected sweep to remove orphaned finalizer")
	}

	clean, err := snapClient.SnapshotV1().VolumeSnapshots(namespace).Get(context.Background(), "clean-snap", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to get snapshot: %v", err)
	}
	if !checkFinalizer(clean, "other-finalizer") {
		t.Error("expected clean snapshot's finalizer to be untouched")
	}
}

func TestSweepOrphanedFinalizers_RetainsPendingPVC(t *testing.T) {
	namespace := "default"
	snapshotName := "active-snap"

	snapshot := &crdv1.VolumeSnapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name:       snapshotName,
			Namespace:  namespace,
			Finalizers: []string{snapshotSourceProtectionFinalizer},
		},
	}

	pendingPVC := &v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: "still-provisioning", Namespace: namespace},
		Spec: v1.PersistentVolumeClaimSpec{
			DataSource: &v1.TypedLocalObjectReference{Kind: snapshotKind, Name: snapshotName},
		},
		Status: v1.PersistentVolumeClaimStatus{Phase: v1.ClaimPending},
	}

	snapClient := snapshotfake.NewSimpleClientset(snapshot)
	claimLister := newFakeClaimLister(pendingPVC)
	ctrl := fakeSnapshotFinalizerController(snapClient, claimLister, snapshot)

	ctrl.sweepOrphanedFinalizers(context.Background())

	updated, err := snapClient.SnapshotV1().VolumeSnapshots(namespace).Get(context.Background(), snapshotName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to get snapshot: %v", err)
	}
	if !checkFinalizer(updated, snapshotSourceProtectionFinalizer) {
		t.Error("expected sweep to retain finalizer while PVC is still Pending")
	}
}

func TestIsSnapshotSourcePVC(t *testing.T) {
	snapshotAPIGroup := "snapshot.storage.k8s.io"
	tests := []struct {
		name   string
		pvc    *v1.PersistentVolumeClaim
		expect bool
	}{
		{
			name: "DataSource with snapshot kind",
			pvc: &v1.PersistentVolumeClaim{
				Spec: v1.PersistentVolumeClaimSpec{
					DataSource: &v1.TypedLocalObjectReference{Kind: snapshotKind, Name: "snap"},
				},
			},
			expect: true,
		},
		{
			name: "DataSourceRef with snapshot kind",
			pvc: &v1.PersistentVolumeClaim{
				Spec: v1.PersistentVolumeClaimSpec{
					DataSourceRef: &v1.TypedObjectReference{APIGroup: &snapshotAPIGroup, Kind: snapshotKind, Name: "snap"},
				},
			},
			expect: true,
		},
		{
			name: "DataSource with PVC kind",
			pvc: &v1.PersistentVolumeClaim{
				Spec: v1.PersistentVolumeClaimSpec{
					DataSource: &v1.TypedLocalObjectReference{Kind: pvcKind, Name: "pvc"},
				},
			},
			expect: false,
		},
		{
			name: "No data source",
			pvc: &v1.PersistentVolumeClaim{
				Spec: v1.PersistentVolumeClaimSpec{},
			},
			expect: false,
		},
		{
			name: "DataSource with wrong APIGroup is not a snapshot",
			pvc: &v1.PersistentVolumeClaim{
				Spec: v1.PersistentVolumeClaimSpec{
					DataSource: &v1.TypedLocalObjectReference{
						APIGroup: func() *string { s := "example.com"; return &s }(),
						Kind:     snapshotKind,
						Name:     "snap",
					},
				},
			},
			expect: false,
		},
		{
			name: "DataSourceRef with wrong APIGroup is not a snapshot",
			pvc: &v1.PersistentVolumeClaim{
				Spec: v1.PersistentVolumeClaimSpec{
					DataSourceRef: &v1.TypedObjectReference{
						APIGroup: func() *string { s := "example.com"; return &s }(),
						Kind:     snapshotKind,
						Name:     "snap",
					},
				},
			},
			expect: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isSnapshotSourcePVC(tt.pvc); got != tt.expect {
				t.Errorf("isSnapshotSourcePVC() = %v, want %v", got, tt.expect)
			}
		})
	}
}

func TestSnapshotNameFromPVC_CrossNamespace(t *testing.T) {
	snapshotAPIGroup := "snapshot.storage.k8s.io"
	otherNs := "other-namespace"

	pvc := &v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: "pvc1", Namespace: "local-ns"},
		Spec: v1.PersistentVolumeClaimSpec{
			DataSourceRef: &v1.TypedObjectReference{
				APIGroup:  &snapshotAPIGroup,
				Kind:      snapshotKind,
				Name:      "remote-snapshot",
				Namespace: &otherNs,
			},
		},
	}

	ns, name := snapshotNameFromPVC(pvc)
	if ns != "other-namespace" {
		t.Errorf("expected namespace %q, got %q", "other-namespace", ns)
	}
	if name != "remote-snapshot" {
		t.Errorf("expected name %q, got %q", "remote-snapshot", name)
	}
}

func TestSyncSnapshot_DataSourceRefPending(t *testing.T) {
	snapshotName := "test-snapshot"
	namespace := "default"
	snapshotAPIGroup := "snapshot.storage.k8s.io"

	snapshot := &crdv1.VolumeSnapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name:       snapshotName,
			Namespace:  namespace,
			Finalizers: []string{snapshotSourceProtectionFinalizer},
		},
	}

	pendingPVC := &v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: "pending-pvc", Namespace: namespace},
		Spec: v1.PersistentVolumeClaimSpec{
			DataSourceRef: &v1.TypedObjectReference{APIGroup: &snapshotAPIGroup, Kind: snapshotKind, Name: snapshotName},
		},
		Status: v1.PersistentVolumeClaimStatus{Phase: v1.ClaimPending},
	}

	snapClient := snapshotfake.NewSimpleClientset(snapshot)
	claimLister := newFakeClaimLister(pendingPVC)
	ctrl := fakeSnapshotFinalizerController(snapClient, claimLister, snapshot)

	ctrl.sweepOrphanedFinalizers(context.Background())

	updated, err := snapClient.SnapshotV1().VolumeSnapshots(namespace).Get(context.Background(), snapshotName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to get snapshot: %v", err)
	}
	if !checkFinalizer(updated, snapshotSourceProtectionFinalizer) {
		t.Error("expected finalizer to still be present when DataSourceRef PVC is Pending")
	}
}

func TestWaitForSnapshotAPI(t *testing.T) {
	tests := []struct {
		name      string
		served    bool
		failures  int
		want      bool
		wantCalls int
	}{
		{name: "served", served: true, want: true, wantCalls: 1},
		{name: "not served", served: false, want: false, wantCalls: 1},
		{name: "transient errors then served", served: true, failures: 3, want: true, wantCalls: 4},
		{name: "persistent errors give up", served: true, failures: 100, want: false, wantCalls: 5},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			disc := newFakeSnapshotDiscovery(tc.served, tc.failures)
			c := NewSnapshotFinalizerController(snapshotfake.NewSimpleClientset(), disc, newFakeClaimLister(), snapshotCapabilities(), time.Minute)
			c.apiCheckBackoff = wait.Backoff{Duration: time.Millisecond, Factor: 1, Steps: 5}

			if got := c.waitForSnapshotAPI(context.Background()); got != tc.want {
				t.Errorf("waitForSnapshotAPI() = %v, want %v", got, tc.want)
			}
			if disc.calls != tc.wantCalls {
				t.Errorf("discovery calls = %d, want %d", disc.calls, tc.wantCalls)
			}
		})
	}
}

func TestRun_SnapshotAPINotServed(t *testing.T) {
	snapClient := snapshotfake.NewSimpleClientset()
	c := NewSnapshotFinalizerController(snapClient, newFakeSnapshotDiscovery(false, 0), newFakeClaimLister(), snapshotCapabilities(), time.Minute)

	done := make(chan struct{})
	go func() {
		c.Run(context.Background(), &sync.WaitGroup{})
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return when the VolumeSnapshot v1 API is not served")
	}
	if actions := snapClient.Actions(); len(actions) != 0 {
		t.Errorf("expected no VolumeSnapshot API calls, got %v", actions)
	}
}

func snapshotCapabilities() rpc.ControllerCapabilitySet {
	return rpc.ControllerCapabilitySet{csi.ControllerServiceCapability_RPC_CREATE_DELETE_SNAPSHOT: true}
}
