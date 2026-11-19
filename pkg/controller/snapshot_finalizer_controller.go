package controller

import (
	"context"
	"sync"
	"time"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/kubernetes-csi/csi-lib-utils/rpc"
	"github.com/kubernetes-csi/external-provisioner/v6/pkg/features"
	v1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/client-go/discovery"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	crdv1 "github.com/kubernetes-csi/external-snapshotter/client/v8/apis/volumesnapshot/v1"
	snapclientset "github.com/kubernetes-csi/external-snapshotter/client/v8/clientset/versioned"
)

// SnapshotFinalizerController periodically sweeps all VolumeSnapshots (via a
// local informer cache) looking for the provisioner's source-protection
// finalizer. When it finds one, it checks whether any Pending PVC still
// references that snapshot. If not, the finalizer is orphaned and is removed.
//
// This handles the case where the finalizer becomes orphaned due to:
//   - Provisioner crash between setting and removing the finalizer
//   - Duplicate provisioning race where the second attempt fails permanently
//   - PVC deletion while provisioning is still in progress
type SnapshotFinalizerController struct {
	snapshotClient   snapclientset.Interface
	discoveryClient  discovery.DiscoveryInterface
	snapshotInformer cache.SharedInformer
	claimLister      corelisters.PersistentVolumeClaimLister

	resyncInterval time.Duration
	// apiCheckBackoff paces retries of the VolumeSnapshot v1 API availability
	// check when the API server returns errors.
	apiCheckBackoff wait.Backoff
}

// NewSnapshotFinalizerController creates a new controller for removing orphaned
// snapshot source-protection finalizers after provisioning completes or fails.
// Returns nil if the snapshot client is nil or the driver lacks snapshot capability.
// Whether the cluster serves the VolumeSnapshot v1 API is checked in Run.
func NewSnapshotFinalizerController(
	snapshotClient snapclientset.Interface,
	discoveryClient discovery.DiscoveryInterface,
	claimLister corelisters.PersistentVolumeClaimLister,
	controllerCapabilities rpc.ControllerCapabilitySet,
	sweepInterval time.Duration,
) *SnapshotFinalizerController {
	if snapshotClient == nil {
		return nil
	}
	if !controllerCapabilities[csi.ControllerServiceCapability_RPC_CREATE_DELETE_SNAPSHOT] {
		return nil
	}

	snapshotInformer := cache.NewSharedInformer(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				return snapshotClient.SnapshotV1().VolumeSnapshots("").List(context.Background(), options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				return snapshotClient.SnapshotV1().VolumeSnapshots("").Watch(context.Background(), options)
			},
		},
		&crdv1.VolumeSnapshot{},
		10*time.Minute,
	)

	return &SnapshotFinalizerController{
		snapshotClient:   snapshotClient,
		discoveryClient:  discoveryClient,
		snapshotInformer: snapshotInformer,
		claimLister:      claimLister,
		resyncInterval:   sweepInterval,
		// 11 attempts over about 17 minutes before giving up.
		apiCheckBackoff: wait.Backoff{
			Duration: time.Second,
			Factor:   2,
			Jitter:   0.1,
			Steps:    11,
		},
	}
}

// waitForSnapshotAPI reports whether the VolumeSnapshot v1 API is served,
// retrying discovery errors with backoff. It returns false when the API is not
// served, the retries are exhausted, or ctx is cancelled.
func (c *SnapshotFinalizerController) waitForSnapshotAPI(ctx context.Context) bool {
	var available bool
	var lastErr error
	err := wait.ExponentialBackoffWithContext(ctx, c.apiCheckBackoff, func(ctx context.Context) (bool, error) {
		available, lastErr = features.IsVolumeSnapshotV1Available(c.discoveryClient)
		if lastErr != nil {
			klog.Warningf("SnapshotFinalizerProtection: failed to check VolumeSnapshot v1 API availability: %v", lastErr)
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		if ctx.Err() == nil {
			klog.Errorf("SnapshotFinalizerProtection: disabled, giving up checking VolumeSnapshot v1 API availability: %v", lastErr)
		}
		return false
	}
	if !available {
		klog.Info("SnapshotFinalizerProtection: disabled, VolumeSnapshot v1 API is not served by the cluster")
	}
	return available
}

// Run starts the controller's periodic sweep loop.
func (c *SnapshotFinalizerController) Run(ctx context.Context, wg *sync.WaitGroup) {
	klog.Info("Starting SnapshotFinalizerProtection controller")
	defer utilruntime.HandleCrash()

	if !c.waitForSnapshotAPI(ctx) {
		return
	}

	// Preflight: verify we have list/watch permissions on VolumeSnapshots.
	// These are optional RBAC verbs, so exit gracefully if forbidden.
	if _, err := c.snapshotClient.SnapshotV1().VolumeSnapshots("").List(ctx, metav1.ListOptions{Limit: 1}); err != nil {
		if apierrs.IsForbidden(err) {
			klog.V(3).Infof("SnapshotFinalizerProtection: disabled, missing list permission on volumesnapshots: %v", err)
			return
		}
	}

	go c.snapshotInformer.Run(ctx.Done())

	if !cache.WaitForCacheSync(ctx.Done(), c.snapshotInformer.HasSynced) {
		klog.Error("SnapshotFinalizerProtection: failed to sync informer caches")
		return
	}

	if utilfeature.DefaultFeatureGate.Enabled(features.ReleaseLeaderElectionOnExit) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			wait.Until(func() { c.sweepOrphanedFinalizers(ctx) }, c.resyncInterval, ctx.Done())
		}()
	} else {
		go wait.Until(func() { c.sweepOrphanedFinalizers(ctx) }, c.resyncInterval, ctx.Done())
	}

	klog.Info("Started SnapshotFinalizerProtection controller")
	<-ctx.Done()
	klog.Info("Shutting down SnapshotFinalizerProtection controller")
}

// sweepOrphanedFinalizers iterates VolumeSnapshots from the local informer cache
// and checks whether any still carry the provisioner's finalizer without a
// corresponding Pending PVC. This catches cases where PVC deletion events were
// missed (e.g., controller was not running when the PVC was deleted).
func (c *SnapshotFinalizerController) sweepOrphanedFinalizers(ctx context.Context) {
	snapshots := c.snapshotInformer.GetStore().List()

	// Build an index of snapshots that have at least one Pending PVC referencing
	// them. This makes the per-snapshot check O(1) instead of scanning all PVCs.
	pendingSnapshots := c.buildPendingSnapshotIndex()

	for _, obj := range snapshots {
		if ctx.Err() != nil {
			return
		}
		snapshot, ok := obj.(*crdv1.VolumeSnapshot)
		if !ok {
			continue
		}
		if !checkFinalizer(snapshot, snapshotSourceProtectionFinalizer) {
			continue
		}
		key := snapshot.Namespace + "/" + snapshot.Name
		if pendingSnapshots.Has(key) {
			continue
		}
		c.removeSnapshotFinalizer(ctx, snapshot)
	}
}

// buildPendingSnapshotIndex returns a set of "namespace/name" keys for snapshots
// that are currently referenced by at least one PVC whose provisioning has not
// yet completed (Phase is Pending or empty).
func (c *SnapshotFinalizerController) buildPendingSnapshotIndex() sets.Set[string] {
	allPVCs, err := c.claimLister.List(labels.Everything())
	if err != nil {
		klog.V(3).Infof("SnapshotFinalizerProtection: failed to list PVCs: %v", err)
		return nil
	}

	pending := sets.New[string]()
	for _, pvc := range allPVCs {
		if pvc.Status.Phase != v1.ClaimPending && pvc.Status.Phase != "" {
			continue
		}
		if !isSnapshotSourcePVC(pvc) {
			continue
		}
		snapNs, snapName := snapshotNameFromPVC(pvc)
		pending.Insert(snapNs + "/" + snapName)
	}
	return pending
}

// removeSnapshotFinalizer removes the provisioner's source-protection finalizer
// from the given snapshot.
func (c *SnapshotFinalizerController) removeSnapshotFinalizer(ctx context.Context, snapshot *crdv1.VolumeSnapshot) {
	namespace := snapshot.Namespace
	snapshotName := snapshot.Name

	// No Pending PVC references this snapshot. Remove the finalizer.
	finalizers := make([]string, 0, len(snapshot.Finalizers))
	for _, f := range snapshot.Finalizers {
		if f != snapshotSourceProtectionFinalizer {
			finalizers = append(finalizers, f)
		}
	}

	clone := snapshot.DeepCopy()
	clone.Finalizers = finalizers
	if _, err := c.snapshotClient.SnapshotV1().VolumeSnapshots(namespace).Update(ctx, clone, metav1.UpdateOptions{}); err != nil {
		if apierrs.IsNotFound(err) || apierrs.IsConflict(err) {
			klog.V(4).Infof("SnapshotFinalizerProtection: transient error removing finalizer from snapshot %s/%s, will retry: %v", namespace, snapshotName, err)
			return
		}
		if apierrs.IsForbidden(err) {
			klog.V(3).Infof("SnapshotFinalizerProtection: unable to remove finalizer from snapshot %s/%s due to missing RBAC permissions: %v", namespace, snapshotName, err)
			return
		}
		klog.Warningf("SnapshotFinalizerProtection: failed to remove finalizer from snapshot %s/%s: %v", namespace, snapshotName, err)
		return
	}

	klog.Infof("SnapshotFinalizerProtection: removed orphaned finalizer from snapshot %s/%s", namespace, snapshotName)
}

func isSnapshotSourcePVC(pvc *v1.PersistentVolumeClaim) bool {
	if pvc.Spec.DataSource != nil && pvc.Spec.DataSource.Kind == snapshotKind &&
		(pvc.Spec.DataSource.APIGroup == nil || *pvc.Spec.DataSource.APIGroup == snapshotAPIGroup) {
		return true
	}
	if pvc.Spec.DataSourceRef != nil && pvc.Spec.DataSourceRef.Kind == snapshotKind &&
		(pvc.Spec.DataSourceRef.APIGroup == nil || *pvc.Spec.DataSourceRef.APIGroup == snapshotAPIGroup) {
		return true
	}
	return false
}

// snapshotNameFromPVC returns the snapshot name and namespace referenced by a PVC.
// It checks both DataSource and DataSourceRef fields. For cross-namespace references
// via DataSourceRef, the snapshot namespace may differ from the PVC namespace.
func snapshotNameFromPVC(pvc *v1.PersistentVolumeClaim) (namespace, name string) {
	if pvc.Spec.DataSource != nil && pvc.Spec.DataSource.Kind == snapshotKind &&
		(pvc.Spec.DataSource.APIGroup == nil || *pvc.Spec.DataSource.APIGroup == snapshotAPIGroup) {
		return pvc.Namespace, pvc.Spec.DataSource.Name
	}
	if pvc.Spec.DataSourceRef != nil && pvc.Spec.DataSourceRef.Kind == snapshotKind &&
		(pvc.Spec.DataSourceRef.APIGroup == nil || *pvc.Spec.DataSourceRef.APIGroup == snapshotAPIGroup) {
		ns := pvc.Namespace
		if pvc.Spec.DataSourceRef.Namespace != nil && len(*pvc.Spec.DataSourceRef.Namespace) > 0 {
			ns = *pvc.Spec.DataSourceRef.Namespace
		}
		return ns, pvc.Spec.DataSourceRef.Name
	}
	return "", ""
}
