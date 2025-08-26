/*
Copyright 2022 The Kubernetes Authors.

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

// Package capacity contains the code which controls the CSIStorageCapacity
// objects owned by the external-provisioner.
package capacity

import (
	"context"

	storagev1 "k8s.io/api/storage/v1"
	storagev1beta1 "k8s.io/api/storage/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	storageinformersv1 "k8s.io/client-go/informers/storage/v1"
	storageinformersv1beta1 "k8s.io/client-go/informers/storage/v1beta1"
	"k8s.io/client-go/kubernetes"
	storagev1beta1typed "k8s.io/client-go/kubernetes/typed/storage/v1beta1"
	storagelistersv1 "k8s.io/client-go/listers/storage/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

func NewV1ClientFactory(clientSet kubernetes.Interface) CSIStorageCapacityFactory {
	return func(namespace string) CSIStorageCapacityInterface {
		return clientSet.StorageV1().CSIStorageCapacities(namespace)
	}
}

func NewV1beta1ClientFactory(clientSet kubernetes.Interface) CSIStorageCapacityFactory {
	return func(namespace string) CSIStorageCapacityInterface {
		return csiStorageCapacityBridge{CSIStorageCapacityInterface: clientSet.StorageV1beta1().CSIStorageCapacities(namespace)}
	}
}

// csiStorageCapacityBridge converts between v1 and v1beta. Delete is the same
// in both API versions, we can inherit its implementation. Create and Update
// must be wrapped.
type csiStorageCapacityBridge struct {
	storagev1beta1typed.CSIStorageCapacityInterface
}

var _ CSIStorageCapacityInterface = csiStorageCapacityBridge{}

func (c csiStorageCapacityBridge) Create(ctx context.Context, cscv1 *storagev1.CSIStorageCapacity, opts metav1.CreateOptions) (*storagev1.CSIStorageCapacity, error) {
	cscv1beta1, err := c.CSIStorageCapacityInterface.Create(ctx, v1Tov1beta1(cscv1), opts)
	if err != nil {
		return nil, err
	}
	return v1beta1Tov1(cscv1beta1), nil
}

func (c csiStorageCapacityBridge) Update(ctx context.Context, cscv1 *storagev1.CSIStorageCapacity, opts metav1.UpdateOptions) (*storagev1.CSIStorageCapacity, error) {
	cscv1beta1, err := c.CSIStorageCapacityInterface.Update(ctx, v1Tov1beta1(cscv1), opts)
	if err != nil {
		return nil, err
	}
	return v1beta1Tov1(cscv1beta1), nil
}

func NewV1beta1InformerBridge(informer storageinformersv1beta1.CSIStorageCapacityInformer) storageinformersv1.CSIStorageCapacityInformer {
	return csiStorageCapacityInformerBridge{informer: informer}
}

type csiStorageCapacityInformerBridge struct {
	informer storageinformersv1beta1.CSIStorageCapacityInformer
}

var _ storageinformersv1.CSIStorageCapacityInformer = csiStorageCapacityInformerBridge{}

func (ib csiStorageCapacityInformerBridge) Lister() storagelistersv1.CSIStorageCapacityLister {
	return ib
}

func (ib csiStorageCapacityInformerBridge) List(selector labels.Selector) ([]*storagev1.CSIStorageCapacity, error) {
	cscsv1beta1, err := ib.informer.Lister().List(selector)
	if err != nil {
		return nil, err
	}
	cscsv1 := make([]*storagev1.CSIStorageCapacity, 0, len(cscsv1beta1))
	for _, cscv1beta1 := range cscsv1beta1 {
		cscsv1 = append(cscsv1, v1beta1Tov1(cscv1beta1))
	}
	return cscsv1, nil
}

func (ib csiStorageCapacityInformerBridge) Informer() cache.SharedIndexInformer {
	return csiStorageCapacityIndexInformerBridge{SharedIndexInformer: ib.informer.Informer()}
}

func (ib csiStorageCapacityInformerBridge) CSIStorageCapacities(namespace string) storagelistersv1.CSIStorageCapacityNamespaceLister {
	// Not implemented, not needed.
	return nil
}

// csiStorageCapacityIndexInformerBridge wraps a SharedIndexInformer for
// v1beta1 CSIStorageCapacity. It makes sure that handlers only ever see v1
// CSIStorageCapacity.
type csiStorageCapacityIndexInformerBridge struct {
	cache.SharedIndexInformer
}

var _ cache.SharedIndexInformer = csiStorageCapacityIndexInformerBridge{}

func (iib csiStorageCapacityIndexInformerBridge) AddEventHandler(handlerv1 cache.ResourceEventHandler) (cache.ResourceEventHandlerRegistration, error) {
	handlerv1beta1 := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj any) {
			csc, ok := obj.(*storagev1beta1.CSIStorageCapacity)
			if !ok {
				klog.Errorf("added object: expected v1beta1.CSIStorageCapacity, got %T -> ignoring it", obj)
				return
			}
			handlerv1.OnAdd(v1beta1Tov1(csc), iib.HasSynced())
		},
		UpdateFunc: func(oldObj any, newObj any) {
			oldCsc, ok := oldObj.(*storagev1beta1.CSIStorageCapacity)
			if !ok {
				klog.Errorf("updated object: expected v1beta1.CSIStorageCapacity, got %T -> ignoring it", oldObj)
				return
			}
			newCsc, ok := newObj.(*storagev1beta1.CSIStorageCapacity)
			if !ok {
				klog.Errorf("updated object: expected v1beta1.CSIStorageCapacity, got %T -> ignoring it", newObj)
				return
			}
			handlerv1.OnUpdate(v1beta1Tov1(oldCsc), v1beta1Tov1(newCsc))
		},
		DeleteFunc: func(obj any) {
			// Beware of "xxx deleted" events
			if unknown, ok := obj.(cache.DeletedFinalStateUnknown); ok && unknown.Obj != nil {
				obj = unknown.Obj
			}
			csc, ok := obj.(*storagev1beta1.CSIStorageCapacity)
			if !ok {
				klog.Errorf("deleted object: expected v1beta1.CSIStorageCapacity, got %T -> ignoring it", obj)
				return
			}
			handlerv1.OnDelete(v1beta1Tov1(csc))
		},
	}
	return iib.SharedIndexInformer.AddEventHandler(handlerv1beta1)
}

// Shallow copies are good enough for our purpose.

func v1beta1Tov1(csc *storagev1beta1.CSIStorageCapacity) *storagev1.CSIStorageCapacity {
	return &storagev1.CSIStorageCapacity{
		ObjectMeta:        csc.ObjectMeta,
		NodeTopology:      csc.NodeTopology,
		StorageClassName:  csc.StorageClassName,
		Capacity:          csc.Capacity,
		MaximumVolumeSize: csc.MaximumVolumeSize,
	}
}

func v1Tov1beta1(csc *storagev1.CSIStorageCapacity) *storagev1beta1.CSIStorageCapacity {
	return &storagev1beta1.CSIStorageCapacity{
		ObjectMeta:        csc.ObjectMeta,
		NodeTopology:      csc.NodeTopology,
		StorageClassName:  csc.StorageClassName,
		Capacity:          csc.Capacity,
		MaximumVolumeSize: csc.MaximumVolumeSize,
	}
}
