/*
Copyright 2021 The Kubernetes Authors.

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

package data_source_validator

import (
	"fmt"
	"time"

	clientset "github.com/kubernetes-csi/external-provisioner/client/clientset/versioned"
	popinformers "github.com/kubernetes-csi/external-provisioner/client/informers/externalversions/volumepopulator/v1beta1"
	poplisters "github.com/kubernetes-csi/external-provisioner/client/listers/volumepopulator/v1beta1"
	volumesnapshotv1beta1 "github.com/kubernetes-csi/external-snapshotter/client/v3/apis/volumesnapshot/v1beta1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
)

type populatorController struct {
	clientset     clientset.Interface
	client        kubernetes.Interface
	eventRecorder record.EventRecorder
	queue         workqueue.RateLimitingInterface

	popLister       poplisters.VolumePopulatorLister
	popListerSynced cache.InformerSynced
	pvcLister       corelisters.PersistentVolumeClaimLister
	pvcListerSynced cache.InformerSynced

	resyncPeriod time.Duration
}

var (
	pvcGK            = metav1.GroupKind{Group: v1.GroupName, Kind: "PersistentVolumeClaim"}
	volumeSnapshotGK = metav1.GroupKind{Group: volumesnapshotv1beta1.GroupName, Kind: "VolumeSnapshot"}
)

func NewDataSourceValidator(
	clientset clientset.Interface,
	client kubernetes.Interface,
	volumePopulatorInformer popinformers.VolumePopulatorInformer,
	pvcInformer coreinformers.PersistentVolumeClaimInformer,
	resyncPeriod time.Duration,
) *populatorController {
	broadcaster := record.NewBroadcaster()
	broadcaster.StartLogging(klog.Infof)
	broadcaster.StartRecordingToSink(&corev1.EventSinkImpl{Interface: client.CoreV1().Events(v1.NamespaceAll)})
	var eventRecorder record.EventRecorder
	eventRecorder = broadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: fmt.Sprintf("data-source-validator")})

	ctrl := &populatorController{
		clientset:     clientset,
		client:        client,
		eventRecorder: eventRecorder,
		resyncPeriod:  resyncPeriod,
		queue:         workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "pvc"),
	}

	pvcInformer.Informer().AddEventHandlerWithResyncPeriod(
		cache.ResourceEventHandlerFuncs{
			AddFunc:    func(obj interface{}) { ctrl.enqueueWork(obj) },
			UpdateFunc: func(oldObj, newObj interface{}) { ctrl.enqueueWork(newObj) },
			DeleteFunc: func(obj interface{}) { ctrl.enqueueWork(obj) },
		},
		ctrl.resyncPeriod,
	)
	ctrl.pvcLister = pvcInformer.Lister()
	ctrl.pvcListerSynced = pvcInformer.Informer().HasSynced

	ctrl.popLister = volumePopulatorInformer.Lister()
	ctrl.popListerSynced = volumePopulatorInformer.Informer().HasSynced

	return ctrl
}

func (ctrl *populatorController) Run(workers int, stopCh <-chan struct{}) {
	defer ctrl.queue.ShutDown()

	klog.Infof("Starting data-source-validator controller")
	defer klog.Infof("Shutting down data-source-validator controller")

	if !cache.WaitForCacheSync(stopCh, ctrl.popListerSynced, ctrl.pvcListerSynced) {
		klog.Errorf("Cannot sync caches")
		return
	}

	for i := 0; i < workers; i++ {
		go wait.Until(ctrl.worker, 0, stopCh)
	}

	<-stopCh
}

// enqueueWork adds PVC to given work queue.
func (ctrl *populatorController) enqueueWork(obj interface{}) {
	// Beware of "xxx deleted" events
	if unknown, ok := obj.(cache.DeletedFinalStateUnknown); ok && unknown.Obj != nil {
		obj = unknown.Obj
	}
	if pvc, ok := obj.(*v1.PersistentVolumeClaim); ok {
		objName, err := cache.DeletionHandlingMetaNamespaceKeyFunc(pvc)
		if err != nil {
			klog.Errorf("failed to get key from object: %v, %v", err, pvc)
			return
		}
		klog.V(5).Infof("enqueued %q for sync", objName)
		ctrl.queue.Add(objName)
	}
}

// worker is the main worker for PVCs.
func (ctrl *populatorController) worker() {
	keyObj, quit := ctrl.queue.Get()
	if quit {
		return
	}
	defer ctrl.queue.Done(keyObj)

	if err := ctrl.syncPvcByKey(keyObj.(string)); err != nil {
		// Rather than wait for a full resync, re-add the key to the
		// queue to be processed.
		ctrl.queue.AddRateLimited(keyObj)
		klog.V(4).Infof("Failed to sync pvc %q, will retry again: %v", keyObj.(string), err)
	} else {
		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		ctrl.queue.Forget(keyObj)
	}
}

// syncPvcByKey processes a PVC request.
func (ctrl *populatorController) syncPvcByKey(key string) error {
	klog.V(5).Infof("syncPvcByKey[%s]", key)

	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	klog.V(5).Infof("worker: pvc namespace [%s] name [%s]", namespace, name)
	if err != nil {
		klog.Errorf("error getting namespace & name of pvc %q to get pvc from informer: %v", key, err)
		return nil
	}
	pvc, err := ctrl.pvcLister.PersistentVolumeClaims(namespace).Get(name)
	if err != nil && !errors.IsNotFound(err) {
		klog.V(2).Infof("error getting pvc %q from informer: %v", key, err)
		return err
	}

	dataSource := pvc.Spec.DataSource
	if nil == dataSource {
		// No data source
		return nil
	}
	if nil == dataSource.APIGroup {
		// Data source is a core object
		return nil
	}

	klog.V(3).Infof("PVC %s datasource is %s.%s", pvc.Name, *dataSource.APIGroup, dataSource.Kind)
	gk := metav1.GroupKind{
		Group: *dataSource.APIGroup,
		Kind:  dataSource.Kind,
	}

	valid, err := ctrl.validateGroupKind(gk)
	if nil != err {
		return err
	}

	if !valid {
		ctrl.eventRecorder.Event(pvc, v1.EventTypeWarning, "UnrecognizedDataSourceKind",
			"The data source for this PVC does not match any registered VolumePopulator")
	}

	return nil
}

func (ctrl *populatorController) validateGroupKind(gk metav1.GroupKind) (bool, error) {
	switch gk {
	case pvcGK, volumeSnapshotGK:
		// Cloning PVCs and Volume Snapshots are special cases, allowed by the
		// core, so don't reject these.
		klog.V(4).Infof("Allowing %s as a special case", gk.String())
		return true, nil
	}
	populators, err := ctrl.popLister.List(labels.Everything())
	if nil != err {
		klog.Errorf("Failed to list populators: %v", err)
		return false, err
	}
	for _, populator := range populators {
		if populator.SourceKind == gk {
			klog.V(4).Infof("Allowing %s due to %s populator", gk.String(), populator.Name)
			return true, nil
		}
	}
	klog.Warningf("No populator matches %s", gk.String())
	return false, nil
}
