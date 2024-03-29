/*
Copyright 2024 The Kubernetes Authors.

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
	"context"
	"fmt"
	"time"

	v1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	"sigs.k8s.io/sig-storage-lib-external-provisioner/v9/controller"
)

type ProvisioningProtectionController struct {
	client        kubernetes.Interface
	claimLister   corelisters.PersistentVolumeClaimLister
	claimInformer cache.SharedInformer
	claimQueue    workqueue.RateLimitingInterface
}

// NewProvisioningProtectionController creates new controller for additional CSI claim protection capabilities
func NewProvisioningProtectionController(
	client kubernetes.Interface,
	claimLister corelisters.PersistentVolumeClaimLister,
	claimInformer cache.SharedInformer,
	claimQueue workqueue.RateLimitingInterface,
) *ProvisioningProtectionController {
	controller := &ProvisioningProtectionController{
		client:        client,
		claimLister:   claimLister,
		claimInformer: claimInformer,
		claimQueue:    claimQueue,
	}
	return controller
}

// Run is a main ProvisioningProtectionController handler
func (p *ProvisioningProtectionController) Run(ctx context.Context, threadiness int) {
	klog.Info("Starting ProvisioningProtection controller")
	defer utilruntime.HandleCrash()
	defer p.claimQueue.ShutDown()

	claimHandler := cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { p.enqueueClaimUpdate(obj) },
		UpdateFunc: func(_ interface{}, newObj interface{}) { p.enqueueClaimUpdate(newObj) },
	}
	p.claimInformer.AddEventHandlerWithResyncPeriod(claimHandler, controller.DefaultResyncPeriod)

	for i := 0; i < threadiness; i++ {
		go wait.Until(func() {
			p.runClaimWorker(ctx)
		}, time.Second, ctx.Done())
	}

	klog.Infof("Started ProvisioningProtection controller")
	<-ctx.Done()
	klog.Info("Shutting down ProvisioningProtection controller")
}

func (p *ProvisioningProtectionController) runClaimWorker(ctx context.Context) {
	for p.processNextClaimWorkItem(ctx) {
	}
}

// processNextClaimWorkItem processes items from claimQueue
func (p *ProvisioningProtectionController) processNextClaimWorkItem(ctx context.Context) bool {
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
func (p *ProvisioningProtectionController) enqueueClaimUpdate(obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		utilruntime.HandleError(err)
		return
	}

	p.claimQueue.Add(key)
}

// syncClaimHandler gets the claim from informer's cache then calls syncClaim
func (p *ProvisioningProtectionController) syncClaimHandler(ctx context.Context, key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	claim, err := p.claimLister.PersistentVolumeClaims(namespace).Get(name)
	if err != nil {
		if apierrs.IsNotFound(err) {
			utilruntime.HandleError(fmt.Errorf("item '%s' in work queue no longer exists", key))
			return nil
		}

		return err
	}

	return p.syncClaim(ctx, claim)
}

// syncClaim removes finalizers from a PVC, when provision is finished
func (p *ProvisioningProtectionController) syncClaim(ctx context.Context, claim *v1.PersistentVolumeClaim) error {
	if !checkFinalizer(claim, pvcProvisioningFinalizer) || claim.Spec.VolumeName == "" {
		return nil
	}

	// Remove provision finalizer
	finalizers := make([]string, 0)
	for _, finalizer := range claim.ObjectMeta.Finalizers {
		if finalizer != pvcProvisioningFinalizer {
			finalizers = append(finalizers, finalizer)
		}
	}

	clone := claim.DeepCopy()
	clone.Finalizers = finalizers
	if _, err := p.client.CoreV1().PersistentVolumeClaims(clone.Namespace).Update(ctx, clone, metav1.UpdateOptions{}); err != nil {
		return err
	}
	return nil
}
