package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/kubernetes-csi/csi-lib-utils/rpc"
	v1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	"sigs.k8s.io/sig-storage-lib-external-provisioner/v9/controller"
)

//
// This package introduces a way to handle finalizers, related to in-progress PVC cloning. This is a two-step approach:
//
// 1) PVC referenced as a data source is now updated with a finalizer `provisioner.storage.kubernetes.io/cloning-protection` during a ProvisionExt method call.
// The detection of cloning in-progress is based on the assumption that a PVC with `spec.DataSource` pointing on a another PVC will go into `Pending` state.
// The downside of this, is that fact that any other reason causing PVC to stay in the `Pending` state also blocks resource from deletion it from deletion
//
// 2) When cloning is finished for each PVC referencing the one as a data source,
// this PVC will go from `Pending` to `Bound` state. That allows remove the finalizer.
//

// CloningProtectionController is storing all related interfaces
// to handle cloning protection finalizer removal after CSI cloning is finished
type CloningProtectionController struct {
	client        kubernetes.Interface
	claimLister   corelisters.PersistentVolumeClaimLister
	claimInformer cache.SharedInformer
	claimQueue    workqueue.RateLimitingInterface
}

// NewCloningProtectionController creates new controller for additional CSI claim protection capabilities
func NewCloningProtectionController(
	client kubernetes.Interface,
	claimLister corelisters.PersistentVolumeClaimLister,
	claimInformer cache.SharedInformer,
	claimQueue workqueue.RateLimitingInterface,
	controllerCapabilities rpc.ControllerCapabilitySet,
) *CloningProtectionController {
	if !controllerCapabilities[csi.ControllerServiceCapability_RPC_CLONE_VOLUME] {
		return nil
	}
	controller := &CloningProtectionController{
		client:        client,
		claimLister:   claimLister,
		claimInformer: claimInformer,
		claimQueue:    claimQueue,
	}
	return controller
}

// Run is a main CloningProtectionController handler
func (p *CloningProtectionController) Run(ctx context.Context, threadiness int) {
	klog.Info("Starting CloningProtection controller")
	defer utilruntime.HandleCrash()
	defer p.claimQueue.ShutDown()

	claimHandler := cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { p.enqueueClaimUpdate(ctx, obj) },
		UpdateFunc: func(_ interface{}, newObj interface{}) { p.enqueueClaimUpdate(ctx, newObj) },
	}
	p.claimInformer.AddEventHandlerWithResyncPeriod(claimHandler, controller.DefaultResyncPeriod)

	for i := 0; i < threadiness; i++ {
		go wait.Until(func() {
			p.runClaimWorker(ctx)
		}, time.Second, ctx.Done())
	}

	klog.Infof("Started CloningProtection controller")
	<-ctx.Done()
	klog.Info("Shutting down CloningProtection controller")
}

func (p *CloningProtectionController) runClaimWorker(ctx context.Context) {
	for p.processNextClaimWorkItem(ctx) {
	}
}

// processNextClaimWorkItem processes items from claimQueue
func (p *CloningProtectionController) processNextClaimWorkItem(ctx context.Context) bool {
	obj, shutdown := p.claimQueue.Get()
	if shutdown {
		return false
	}

	err := func(obj interface{}) error {
		defer p.claimQueue.Done(obj)
		var key string
		var ok bool
		if key, ok = obj.(string); !ok {
			p.claimQueue.Forget(obj)
			return fmt.Errorf("expected string in workqueue but got %#v", obj)
		}

		if err := p.syncClaimHandler(ctx, key); err != nil {
			klog.Warningf("Retrying syncing claim %q after %v failures", key, p.claimQueue.NumRequeues(obj))
			p.claimQueue.AddRateLimited(obj)
		} else {
			p.claimQueue.Forget(obj)
		}

		return nil
	}(obj)

	if err != nil {
		utilruntime.HandleError(err)
		return true
	}

	return true
}

// enqueueClaimUpdate takes a PVC obj and stores it into the claim work queue.
func (p *CloningProtectionController) enqueueClaimUpdate(ctx context.Context, obj interface{}) {
	new, ok := obj.(*v1.PersistentVolumeClaim)
	if !ok {
		utilruntime.HandleError(fmt.Errorf("expected claim but got %+v", new))
		return
	}

	// Timestamp didn't appear
	if new.DeletionTimestamp == nil {
		return
	}

	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		utilruntime.HandleError(err)
		return
	}

	p.claimQueue.Add(key)
}

// syncClaimHandler gets the claim from informer's cache then calls syncClaim
func (p *CloningProtectionController) syncClaimHandler(ctx context.Context, key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	claim, err := p.claimLister.PersistentVolumeClaims(namespace).Get(name)
	if err != nil {
		if apierrs.IsNotFound(err) {
			utilruntime.HandleError(fmt.Errorf("Item '%s' in work queue no longer exists", key))
			return nil
		}

		return err
	}

	return p.syncClaim(ctx, claim)
}

// syncClaim removes finalizers from a PVC, when cloning is finished
func (p *CloningProtectionController) syncClaim(ctx context.Context, claim *v1.PersistentVolumeClaim) error {
	if !checkFinalizer(claim, pvcCloneFinalizer) {
		return nil
	}

	// Checking for PVCs in the same namespace to have other states aside from Pending, which means that cloning is still in progress
	pvcList, err := p.claimLister.PersistentVolumeClaims(claim.Namespace).List(labels.Everything())
	if err != nil {
		return err
	}

	// Check for pvc state with DataSource pointing to claim
	for _, pvc := range pvcList {
		if pvc.Spec.DataSource == nil {
			continue
		}

		// Requeue when at least one PVC is still works on cloning
		if pvc.Spec.DataSource.Kind == pvcKind &&
			pvc.Spec.DataSource.Name == claim.Name &&
			pvc.Status.Phase == v1.ClaimPending {
			return fmt.Errorf("PVC '%s' is in 'Pending' state, cloning in progress", pvc.Name)
		}
	}

	// Remove clone finalizer
	finalizers := make([]string, 0)
	for _, finalizer := range claim.ObjectMeta.Finalizers {
		if finalizer != pvcCloneFinalizer {
			finalizers = append(finalizers, finalizer)
		}
	}
	claim.ObjectMeta.Finalizers = finalizers

	if _, err = p.client.CoreV1().PersistentVolumeClaims(claim.Namespace).Update(ctx, claim, metav1.UpdateOptions{}); err != nil {
		if !apierrs.IsNotFound(err) {
			// Couldn't remove finalizer and the object still exists, the controller may
			// try to remove the finalizer again on the next update
			klog.Infof("failed to remove clone finalizer from PVC %v", claim.Name)
			return err
		}
	}

	return nil
}
