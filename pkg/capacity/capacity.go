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

// Package capacity contains the code which controls the CSIStorageCapacity
// objects owned by the external-provisioner.
package capacity

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"sync"
	"time"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/kubernetes-csi/external-provisioner/pkg/capacity/topology"
	"google.golang.org/grpc"
	v1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	storageinformersv1 "k8s.io/client-go/informers/storage/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/component-base/metrics"
	"k8s.io/klog/v2"
)

const (
	DriverNameLabel = "csi.storage.k8s.io/drivername"
	ManagedByLabel  = "csi.storage.k8s.io/managed-by"
)

// Controller creates and updates CSIStorageCapacity objects.  It
// deletes those which are no longer needed because their storage
// class or topology segment are gone. The controller only manages
// those CSIStorageCapacity objects that are owned by a certain
// entity.
//
// The controller maintains a set of topology segments (= NodeSelector
// pointers). Work items are a combination of such a pointer and a
// pointer to a storage class. These keys are mapped to the
// corresponding CSIStorageCapacity object, if one exists.
//
// When processing a work item, the controller first checks whether
// the topology segment and storage class still exist. If not,
// the CSIStorageCapacity object gets deleted. Otherwise, it gets updated
// or created.
//
// New work items are queued for processing when the reconiliation loop
// finds differences, periodically (to refresh existing items) and when
// capacity is expected to have changed.
//
// The work queue is also used to delete duplicate CSIStorageCapacity objects,
// i.e. those that for some reason have the same topology segment
// and storage class name as some other object. That should never happen,
// but the controller is prepared to clean that up, just in case.
type Controller struct {
	metrics.BaseStableCollector

	logger           klog.Logger
	csiController    CSICapacityClient
	driverName       string
	clientFactory    CSIStorageCapacityFactory
	queue            workqueue.RateLimitingInterface
	owner            *metav1.OwnerReference
	managedByID      string
	ownerNamespace   string
	topologyInformer topology.Informer
	scInformer       storageinformersv1.StorageClassInformer
	cInformer        storageinformersv1.CSIStorageCapacityInformer
	pollPeriod       time.Duration
	immediateBinding bool
	timeout          time.Duration

	// capacities contains one entry for each object that is
	// supposed to exist. Entries that exist on the API server
	// have a non-nil pointer. Those get added and updated
	// exclusively through the informer event handler to avoid
	// races.
	capacities     map[workItem]*storagev1.CSIStorageCapacity
	capacitiesLock sync.Mutex
}

type workItem struct {
	segment          *topology.Segment
	storageClassName string
}

func (w workItem) equals(capacity *storagev1.CSIStorageCapacity) bool {
	return w.storageClassName == capacity.StorageClassName &&
		reflect.DeepEqual(w.segment.GetLabelSelector(), capacity.NodeTopology)
}

var (
	// Defines parameters for ExponentialBackoff used while starting up
	// and listing CSIStorageCapacity objects.
	listCSIStorageCapacityBackoff = wait.Backoff{
		Duration: time.Second * 5,
		Factor:   1.1,
		Steps:    10,
	}

	objectsGoalDesc = metrics.NewDesc(
		"csistoragecapacities_desired_goal",
		"Number of CSIStorageCapacity objects that are supposed to be managed automatically.",
		nil, nil,
		metrics.ALPHA,
		"",
	)
	objectsCurrentDesc = metrics.NewDesc(
		"csistoragecapacities_desired_current",
		"Number of CSIStorageCapacity objects that exist and are supposed to be managed automatically.",
		nil, nil,
		metrics.ALPHA,
		"",
	)
	objectsObsoleteDesc = metrics.NewDesc(
		"csistoragecapacities_obsolete",
		"Number of CSIStorageCapacity objects that exist and will be deleted automatically. Objects that exist and may need an update are not considered obsolete and therefore not included in this value.",
		nil, nil,
		metrics.ALPHA,
		"",
	)
)

// CSICapacityClient is the relevant subset of csi.ControllerClient.
type CSICapacityClient interface {
	GetCapacity(ctx context.Context, in *csi.GetCapacityRequest, opts ...grpc.CallOption) (*csi.GetCapacityResponse, error)
}

// CSIStorageCapacityInterface is a subset of the client-go interface for
// v1.CSIStorageCapacity.
type CSIStorageCapacityInterface interface {
	Create(ctx context.Context, cSIStorageCapacity *storagev1.CSIStorageCapacity, opts metav1.CreateOptions) (*storagev1.CSIStorageCapacity, error)
	Update(ctx context.Context, cSIStorageCapacity *storagev1.CSIStorageCapacity, opts metav1.UpdateOptions) (*storagev1.CSIStorageCapacity, error)
	Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error
}

// CSIStorageCapacityFactory corresponds to StorageV1().CSIStorageCapacities but returns
// just what we need.
type CSIStorageCapacityFactory func(namespace string) CSIStorageCapacityInterface

// NewController creates a new controller for CSIStorageCapacity objects.
// It implements metrics.StableCollector and thus can be registered in
// a registry.
func NewCentralCapacityController(
	logger klog.Logger,
	csiController CSICapacityClient,
	driverName string,
	clientFactory CSIStorageCapacityFactory,
	queue workqueue.RateLimitingInterface,
	owner *metav1.OwnerReference,
	managedByID string,
	ownerNamespace string,
	topologyInformer topology.Informer,
	scInformer storageinformersv1.StorageClassInformer,
	cInformer storageinformersv1.CSIStorageCapacityInformer,
	pollPeriod time.Duration,
	immediateBinding bool,
	timeout time.Duration,
) *Controller {
	c := &Controller{
		logger:           logger,
		csiController:    csiController,
		driverName:       driverName,
		clientFactory:    clientFactory,
		queue:            queue,
		owner:            owner,
		managedByID:      managedByID,
		ownerNamespace:   ownerNamespace,
		topologyInformer: topologyInformer,
		scInformer:       scInformer,
		cInformer:        cInformer,
		pollPeriod:       pollPeriod,
		immediateBinding: immediateBinding,
		timeout:          timeout,
		capacities:       map[workItem]*storagev1.CSIStorageCapacity{},
	}

	// Now register for changes. Depending on the implementation of the informers,
	// this may already invoke callbacks.
	handler := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			sc, ok := obj.(*storagev1.StorageClass)
			if !ok {
				logger.Error(fmt.Errorf("expected StorageClass, got %T -> ignoring it", obj), "added object")
				return
			}
			c.onSCAddOrUpdate(sc)
		},
		UpdateFunc: func(_ interface{}, newObj interface{}) {
			sc, ok := newObj.(*storagev1.StorageClass)
			if !ok {
				logger.Error(fmt.Errorf("expected StorageClass, got %T -> ignoring it", newObj), "updated object")
				return
			}
			c.onSCAddOrUpdate(sc)
		},
		DeleteFunc: func(obj interface{}) {
			// Beware of "xxx deleted" events
			if unknown, ok := obj.(cache.DeletedFinalStateUnknown); ok && unknown.Obj != nil {
				obj = unknown.Obj
			}
			sc, ok := obj.(*storagev1.StorageClass)
			if !ok {
				logger.Error(fmt.Errorf("expected StorageClass, got %T -> ignoring it", obj), "deleted object")
				return
			}
			c.onSCDelete(sc)
		},
	}
	c.scInformer.Informer().AddEventHandler(handler)
	c.topologyInformer.AddCallback(c.onTopologyChanges)

	// We don't want the callbacks yet, but need to ensure that
	// the informer controller is instantiated before the caller
	// starts the factory.
	cInformer.Informer()

	return c
}

var _ metrics.StableCollector = &Controller{}

// Run is a main Controller handler
func (c *Controller) Run(ctx context.Context, threadiness int) {
	logger := c.logger

	logger.Info("starting controller")
	defer c.queue.ShutDown()

	c.prepare(ctx)
	for i := 0; i < threadiness; i++ {
		go wait.UntilWithContext(ctx, func(ctx context.Context) {
			c.runWorker(ctx)
		}, time.Second)
	}

	go wait.UntilWithContext(ctx, func(ctx context.Context) { c.pollCapacities(logger) }, c.pollPeriod)

	logger.Info("started controller")
	<-ctx.Done()
	logger.Info("shutting down controller")
}

func (c *Controller) prepare(ctx context.Context) {
	logger := klog.FromContext(ctx).WithName("prepare")

	// Wait for topology and storage class informer sync. Once we have that,
	// we know which CSIStorageCapacity objects we need.
	if !cache.WaitForCacheSync(ctx.Done(), c.topologyInformer.HasSynced, c.scInformer.Informer().HasSynced, c.cInformer.Informer().HasSynced) {
		return
	}

	// The caches are fully populated now, but the event handlers
	// may or may not have been invoked yet. To be sure that we
	// have all data, we need to list all resources. Here we list
	// topology segments, onTopologyChanges lists the classes.
	c.onTopologyChanges(c.topologyInformer.List(), nil)

	logger3 := logger.V(3)
	if logger3.Enabled() {
		scs, err := c.scInformer.Lister().List(labels.Everything())
		if err != nil {
			// Shouldn't happen.
			utilruntime.HandleError(err)
		}
		logger3.Info("initial state",
			"topology segments", len(c.topologyInformer.List()),
			"storage classes", len(scs),
			"potential CSIStorageCapacity objects", len(c.capacities),
		)
	}

	// Now that we know what we need, we can check what we have.
	// We do that both via callbacks *and* by iterating over all
	// objects: callbacks handle future updates and iterating
	// avoids the assumumption that the callback will be invoked
	// for all objects immediately when adding it.
	logger3.Info("checking for existing CSIStorageCapacity objects")
	handler := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			csc, ok := obj.(*storagev1.CSIStorageCapacity)
			if !ok {
				logger.Error(fmt.Errorf("expected CSIStorageCapacity, got %T -> ignoring it", obj), "added object")
				return
			}
			c.onCAddOrUpdate(ctx, csc)
		},
		UpdateFunc: func(_ interface{}, newObj interface{}) {
			csc, ok := newObj.(*storagev1.CSIStorageCapacity)
			if !ok {
				logger.Error(fmt.Errorf("expected CSIStorageCapacity, got %T -> ignoring it", newObj), "updated object")
				return
			}
			c.onCAddOrUpdate(ctx, csc)
		},
		DeleteFunc: func(obj interface{}) {
			// Beware of "xxx deleted" events
			if unknown, ok := obj.(cache.DeletedFinalStateUnknown); ok && unknown.Obj != nil {
				obj = unknown.Obj
			}
			csc, ok := obj.(*storagev1.CSIStorageCapacity)
			if !ok {
				logger.Error(fmt.Errorf("expected CSIStorageCapacity, got %T -> ignoring it", obj), "deleted object")
				return
			}
			c.onCDelete(ctx, csc)
		},
	}
	c.cInformer.Informer().AddEventHandler(handler)
	capacities, err := c.cInformer.Lister().List(labels.Everything())
	if err != nil {
		// This shouldn't happen.
		utilruntime.HandleError(err)
		return
	}
	for _, capacity := range capacities {
		c.onCAddOrUpdate(ctx, capacity)
	}

	// Now that we have seen all existing objects, we are done
	// with the preparation and can let our caller start
	// processing work items.
}

// onTopologyChanges is called by the topology informer.
func (c *Controller) onTopologyChanges(added []*topology.Segment, removed []*topology.Segment) {
	logger := c.logger.WithName("onTopologyChanges")
	logger.V(3).Info("topology changed", "added", added, "removed", removed)

	storageclasses, err := c.scInformer.Lister().List(labels.Everything())
	if err != nil {
		utilruntime.HandleError(err)
		return
	}

	c.capacitiesLock.Lock()
	defer c.capacitiesLock.Unlock()

	for _, sc := range storageclasses {
		if sc.Provisioner != c.driverName {
			continue
		}
		if !c.immediateBinding && sc.VolumeBindingMode != nil && *sc.VolumeBindingMode == storagev1.VolumeBindingImmediate {
			continue
		}
		for _, segment := range added {
			c.addWorkItem(logger, segment, sc)
		}
		for _, segment := range removed {
			c.removeWorkItem(logger, segment, sc)
		}
	}
}

// onSCAddOrUpdate is called for add or update events by the storage
// class listener.
func (c *Controller) onSCAddOrUpdate(sc *storagev1.StorageClass) {
	if sc.Provisioner != c.driverName {
		return
	}
	logger := c.logger.WithName("onSCAddOrUpdate").WithValues("storageclass", klog.KObj(sc))

	logger.V(3).Info("updated or added")
	if !c.immediateBinding && sc.VolumeBindingMode != nil && *sc.VolumeBindingMode == storagev1.VolumeBindingImmediate {
		logger.V(3).Info("ignoring storage class because it uses immediate binding")
		return
	}
	segments := c.topologyInformer.List()

	c.capacitiesLock.Lock()
	defer c.capacitiesLock.Unlock()
	for _, segment := range segments {
		c.addWorkItem(logger, segment, sc)
	}
}

// onSCDelete is called for delete events by the storage class listener.
func (c *Controller) onSCDelete(sc *storagev1.StorageClass) {
	if sc.Provisioner != c.driverName {
		return
	}
	logger := c.logger.WithName("onSCDelete").WithValues("storageclass", klog.KObj(sc))

	logger.V(3).Info("removed")
	segments := c.topologyInformer.List()

	c.capacitiesLock.Lock()
	defer c.capacitiesLock.Unlock()
	for _, segment := range segments {
		c.removeWorkItem(logger, segment, sc)
	}
}

// refreshTopology identifies all work items matching the topology and schedules
// a refresh. The node affinity is expected to come from controller.GenerateVolumeNodeAffinity,
// i.e. only use NodeSelectorTerms and each of those must be based on the CSI driver's
// topology key/value pairs of a topology segment.
func (c *Controller) refreshTopology(logger klog.Logger, nodeAffinity v1.VolumeNodeAffinity) {
	if nodeAffinity.Required == nil || nodeAffinity.Required.NodeSelectorTerms == nil {
		logger.Error(fmt.Errorf("unexpected VolumeNodeAffinity, missing NodeSelectorTerms: %v", nodeAffinity), "skipping refresh")
		return
	}

	c.capacitiesLock.Lock()
	defer c.capacitiesLock.Unlock()

	for _, term := range nodeAffinity.Required.NodeSelectorTerms {
		segment, err := termToSegment(term)
		if err != nil {
			logger.Error(fmt.Errorf("unexpected node selector term %+v: %v", term, err), "skipping refresh")
			continue
		}
		for item := range c.capacities {
			if item.segment.Compare(segment) == 0 {
				logger.V(5).Info("enqueuing because of the topology", "workitem", item)
				c.queue.Add(item)
			}
		}
	}
}

func termToSegment(term v1.NodeSelectorTerm) (segment topology.Segment, err error) {
	if len(term.MatchFields) > 0 {
		err = fmt.Errorf("MatchFields not empty: %+v", term.MatchFields)
		return
	}
	for _, match := range term.MatchExpressions {
		if match.Operator != v1.NodeSelectorOpIn {
			err = fmt.Errorf("unexpected operator: %v", match.Operator)
			return
		}
		if len(match.Values) != 1 {
			err = fmt.Errorf("need exactly one label value, got: %v", match.Values)
			return
		}
		segment = append(segment, topology.SegmentEntry{Key: match.Key, Value: match.Values[0]})
	}
	sort.Sort(&segment)
	return
}

// refreshSC identifies all work items matching the storage class and schedules
// a refresh.
func (c *Controller) refreshSC(logger klog.Logger, storageClassName string) {
	c.capacitiesLock.Lock()
	defer c.capacitiesLock.Unlock()

	for item := range c.capacities {
		if item.storageClassName == storageClassName {
			logger.V(5).Info("enqueuing because of the storage class", "workitem", item)
			c.queue.Add(item)
		}
	}
}

// addWorkItem ensures that there is an item in c.capacities. It
// must be called while holding c.capacitiesLock!
func (c *Controller) addWorkItem(logger klog.Logger, segment *topology.Segment, sc *storagev1.StorageClass) {
	item := workItem{
		segment:          segment,
		storageClassName: sc.Name,
	}
	// Ensure that we have an entry for it...
	_, found := c.capacities[item]
	if !found {
		c.capacities[item] = nil
	}

	// ... and then tell our workers to update
	// or create that capacity object.
	logger.V(5).Info("enqueuing", "workitem", item)
	c.queue.Add(item)
}

// removeWorkItem ensures that the item gets removed from c.capacities. It
// must be called while holding c.capacitiesLock!
func (c *Controller) removeWorkItem(logger klog.Logger, segment *topology.Segment, sc *storagev1.StorageClass) {
	logger = logger.WithName("removeWorkItem")

	item := workItem{
		segment:          segment,
		storageClassName: sc.Name,
	}
	capacity, found := c.capacities[item]
	if !found {
		// Already gone or in the queue to be removed.
		logger.V(5).Info("already removed", "workitem", item)
		return
	}
	// Deleting the item will prevent further updates to
	// it, in case that it is already in the queue.
	delete(c.capacities, item)

	if capacity == nil {
		// No object to remove.
		logger.V(5).Info("removed, no CSIStorageCapacity", "workitem", item)
		return
	}

	// Any capacity object in the queue will be deleted.
	logger.V(5).Info("enqueuing for removal", "CSIStorageCapacity", klog.KObj(capacity))
	c.queue.Add(capacity)
}

// pollCapacities must be called periodically to detect when the underlying storage capacity has changed.
func (c *Controller) pollCapacities(logger klog.Logger) {
	logger = logger.WithName("pollCapacities")

	c.capacitiesLock.Lock()
	defer c.capacitiesLock.Unlock()

	for item := range c.capacities {
		logger.V(5).Info("enqueuing for periodic update", "workitem", item)
		c.queue.Add(item)
	}
}

func (c *Controller) runWorker(ctx context.Context) {
	for c.processNextWorkItem(ctx) {
	}
}

// processNextWorkItem processes items from queue.
func (c *Controller) processNextWorkItem(ctx context.Context) bool {
	obj, shutdown := c.queue.Get()
	if shutdown {
		return false
	}

	err := func() error {
		defer c.queue.Done(obj)

		switch obj := obj.(type) {
		case workItem:
			return c.syncCapacity(ctx, obj)
		case *storagev1.CSIStorageCapacity:
			return c.deleteCapacity(ctx, obj)
		default:
			klog.FromContext(ctx).Info("unexpected work item type", "work item", obj)
		}

		return nil
	}()

	if err != nil {
		utilruntime.HandleError(err) // TODO: remove?
		klog.FromContext(ctx).Info("retrying", "work item", obj, "num-failures", c.queue.NumRequeues(obj))
		c.queue.AddRateLimited(obj)
	} else {
		c.queue.Forget(obj)
	}

	return true
}

// syncCapacity gets the capacity and then updates or creates the object.
func (c *Controller) syncCapacity(ctx context.Context, item workItem) error {
	// We lock only while accessing c.capacities, but not during
	// the potentially long-running operations. That is okay
	// because there is only a single worker per item. In the
	// unlikely case that the desired state of the item changes
	// while we work on it, we will add or update an obsolete
	// object which we then don't store and instead queue for
	// removal.
	c.capacitiesLock.Lock()
	capacity, found := c.capacities[item]
	c.capacitiesLock.Unlock()

	logger := klog.FromContext(ctx).WithValues("workitem", item)
	logger.V(5).Info("refreshing")
	if !found {
		// The item was removed in the meantime. This can happen when the storage class
		// or the topology segment are gone.
		logger.V(5).Info("became obsolete")
		return nil
	}

	sc, err := c.scInformer.Lister().Get(item.storageClassName)
	if err != nil {
		if apierrs.IsNotFound(err) {
			// Another indication that the value is no
			// longer needed.
			return nil
		}
		return fmt.Errorf("retrieve storage class for %+v: %v", item, err)
	}

	req := &csi.GetCapacityRequest{
		Parameters: sc.Parameters,
		// The assumption is that the capacity is independent of the
		// capabilities. The standard makes it mandatory to pass something,
		// therefore we pick something rather arbitrarily.
		VolumeCapabilities: []*csi.VolumeCapability{
			{
				AccessType: &csi.VolumeCapability_Mount{},
				AccessMode: &csi.VolumeCapability_AccessMode{
					Mode: csi.VolumeCapability_AccessMode_UNKNOWN,
				},
			},
		},
	}
	if item.segment != nil {
		req.AccessibleTopology = &csi.Topology{
			Segments: item.segment.GetLabelMap(),
		}
	}
	syncCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	resp, err := c.csiController.GetCapacity(syncCtx, req)
	if err != nil {
		return fmt.Errorf("CSI GetCapacity for %+v: %v", item, err)
	}

	quantity := resource.NewQuantity(resp.AvailableCapacity, resource.BinarySI)
	var maximumVolumeSize *resource.Quantity
	if resp.MaximumVolumeSize != nil {
		maximumVolumeSize = resource.NewQuantity(resp.MaximumVolumeSize.Value, resource.BinarySI)
	}

	logger = logger.WithValues("quantity", quantity)
	if capacity == nil {
		// Create new object.
		capacity = &storagev1.CSIStorageCapacity{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "csisc-",
				Labels: map[string]string{
					DriverNameLabel: c.driverName,
					ManagedByLabel:  c.managedByID,
				},
			},
			StorageClassName:  item.storageClassName,
			NodeTopology:      item.segment.GetLabelSelector(),
			Capacity:          quantity,
			MaximumVolumeSize: maximumVolumeSize,
		}
		if c.owner != nil {
			capacity.OwnerReferences = []metav1.OwnerReference{*c.owner}
		}
		var err error
		klog.V(5).Infof("Capacity Controller: creating new object for %+v, new capacity %v", item, quantity)
		capacity, err = c.clientFactory(c.ownerNamespace).Create(ctx, capacity, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("create CSIStorageCapacity for %+v: %v", item, err)
		}
		logger.V(5).Info("created object", "capacity", klog.KObj(capacity), "resourceVersion", capacity.ResourceVersion)
		// We intentionally avoid storing the created object in c.capacities because that
		// would race with receiving that object through the event handler. In the unlikely
		// scenario that we end up creating two objects for the same work item, the second
		// one will be recognized as duplicate and get deleted again once we receive it.
	} else if capacity.Capacity.Value() == quantity.Value() &&
		sizesAreEqual(capacity.MaximumVolumeSize, maximumVolumeSize) &&
		(c.owner == nil || c.isOwnedByUs(capacity)) {
		klog.V(5).Infof("Capacity Controller: no need to update %s for %+v, same capacity %v, same maximumVolumeSize %v and correct owner", capacity.Name, item, quantity, maximumVolumeSize)
		return nil
	} else {
		// Update existing object. Must not modify object in the informer cache.
		capacity := capacity.DeepCopy()
		capacity.Capacity = quantity
		capacity.MaximumVolumeSize = maximumVolumeSize
		if c.owner != nil && !c.isOwnedByUs(capacity) {
			capacity.OwnerReferences = append(capacity.OwnerReferences, *c.owner)
		}
		var err error
		klog.V(5).Infof("Capacity Controller: updating %s for %+v, new capacity %v, new maximumVolumeSize %v", capacity.Name, item, quantity, maximumVolumeSize)
		capacity, err = c.clientFactory(capacity.Namespace).Update(ctx, capacity, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("update CSIStorageCapacity for %+v: %v", item, err)
		}
		logger.V(5).Info("updated", "capacity", klog.KObj(capacity), "resourceVersion", capacity.ResourceVersion)
		// As for Create above, c.capacities intentionally doesn't get updated with the modified
		// object to avoid races.
	}

	return nil
}

// deleteCapacity ensures that the object is gone when done.
func (c *Controller) deleteCapacity(ctx context.Context, capacity *storagev1.CSIStorageCapacity) error {
	klog.V(5).Infof("Capacity Controller: removing CSIStorageCapacity %s", capacity.Name)
	err := c.clientFactory(capacity.Namespace).Delete(ctx, capacity.Name, metav1.DeleteOptions{})
	if err != nil && apierrs.IsNotFound(err) {
		return nil
	}
	return err
}

// onCAddOrUpdate takes a read-only CSIStorageCapacity object
// and either remembers the pointer to it for future updates or
// ensures that it gets deleted if no longer needed. Foreign objects
// are ignored.
func (c *Controller) onCAddOrUpdate(ctx context.Context, capacity *storagev1.CSIStorageCapacity) {
	logger := c.logger.WithName("onCAddOrUpdate").WithValues("capacity", klog.KObj(capacity), "resourceVersion", capacity.ResourceVersion)

	if !c.isManaged(capacity) {
		// Not ours (anymore?). For the unlikely case that someone removed our owner reference,
		// we also must remove our reference to the object.
		c.capacitiesLock.Lock()
		defer c.capacitiesLock.Unlock()
		for item, capacity2 := range c.capacities {
			if capacity2 != nil && capacity2.UID == capacity.UID {
				c.capacities[item] = nil
				logger.V(5).Info("CSIStorageCapacity owner was modified by someone, enqueueing for re-creation", "workitem", item)
				c.queue.Add(item)
			}
		}
		return
	}

	c.capacitiesLock.Lock()
	defer c.capacitiesLock.Unlock()
	for item, capacity2 := range c.capacities {
		if capacity2 != nil && capacity2.UID == capacity.UID {
			// We already have matched the object.
			logger.V(5).Info("CSIStorageCapacity already known to match", "workitem", item)
			// Either way, remember the new object revision to avoid the "conflict" error
			// when we try to update the old object.
			c.capacities[item] = capacity
			return
		}
		if capacity2 == nil &&
			item.equals(capacity) {
			// This is the capacity object for this particular combination
			// of parameters. Reuse it.
			logger.V(5).Info("found match", "workitem", item)
			c.capacities[item] = capacity
			return
		}
	}
	// The CSIStorageCapacity object is obsolete, delete it.
	logger.V(5).Info("CSIStorageCapacity obsolete, enqueue for removal")
	c.queue.Add(capacity)
}

func (c *Controller) onCDelete(ctx context.Context, capacity *storagev1.CSIStorageCapacity) {
	logger := c.logger.WithName("onCDelete").WithValues("capacity", klog.KObj(capacity), "resourceVersion", capacity.ResourceVersion)

	c.capacitiesLock.Lock()
	defer c.capacitiesLock.Unlock()
	for item, capacity2 := range c.capacities {
		if capacity2 != nil && capacity2.UID == capacity.UID {
			// The object is still needed. Someone else must have removed it.
			// Re-create it...
			logger.V(5).Info("CSIStorageCapacity removed by someone, enqueue for re-creation", "workitem", item)
			c.capacities[item] = nil
			c.queue.Add(item)
			return
		}
	}
}

// DescribeWithStability implements the metrics.StableCollector interface.
func (c *Controller) DescribeWithStability(ch chan<- *metrics.Desc) {
	ch <- objectsGoalDesc
	ch <- objectsCurrentDesc
	ch <- objectsObsoleteDesc
}

// CollectWithStability implements the metrics.StableCollector interface.
func (c *Controller) CollectWithStability(ch chan<- metrics.Metric) {
	c.capacitiesLock.Lock()
	defer c.capacitiesLock.Unlock()

	ch <- metrics.NewLazyConstMetric(objectsGoalDesc,
		metrics.GaugeValue,
		float64(c.getObjectsGoal()),
	)
	ch <- metrics.NewLazyConstMetric(objectsCurrentDesc,
		metrics.GaugeValue,
		float64(c.getObjectsCurrent()),
	)
	ch <- metrics.NewLazyConstMetric(objectsObsoleteDesc,
		metrics.GaugeValue,
		float64(c.getObjectsObsolete()),
	)
}

// getObjectsGoal is called during metrics gathering and calculates the number
// of CSIStorageCapacity objects which are are meant to
// to exist.
func (c *Controller) getObjectsGoal() int64 {
	return int64(len(c.capacities))
}

// getObjectsCurrent is called during metrics gathering and calculates the number
// of CSIStorageCapacity objects which are currently exist and are meant to
// continue to exist.
func (c *Controller) getObjectsCurrent() int64 {
	current := int64(0)
	for _, capacity := range c.capacities {
		if capacity != nil {
			current++
		}
	}
	return current
}

// getObsoleteObjects is called during metrics gathering and calculates the number
// of CSIStorageCapacity objects which currently exist (according to our informer)
// and which are no longer needed.
func (c *Controller) getObjectsObsolete() int64 {
	obsolete := int64(0)
	capacities, _ := c.cInformer.Lister().List(labels.Everything())
	if capacities == nil {
		// Shouldn't happen, local operation.
		return 0
	}
	for _, capacity := range capacities {
		if !c.isManaged(capacity) {
			continue
		}
		if c.isObsolete(capacity) {
			obsolete++
		}
	}
	return obsolete
}

func (c *Controller) isObsolete(capacity *storagev1.CSIStorageCapacity) bool {
	for item, _ := range c.capacities {
		if item.equals(capacity) {
			return false
		}
	}
	return true
}

// isOwnedByUs implements the same logic as https://pkg.go.dev/k8s.io/apimachinery/pkg/apis/meta/v1?tab=doc#IsControlledBy,
// just with the expected owner identified directly with the UID.
func (c *Controller) isOwnedByUs(capacity *storagev1.CSIStorageCapacity) bool {
	for _, owner := range capacity.OwnerReferences {
		if owner.Controller != nil && *owner.Controller && owner.UID == c.owner.UID {
			return true
		}
	}
	return false
}

// isManaged checks the labels to determine whether this capacity object is managed by
// the controller instance. With server-side filtering via the informer, this
// function becomes a simple safe-guard and should always return true.
func (c *Controller) isManaged(capacity *storagev1.CSIStorageCapacity) bool {
	return capacity.Labels[DriverNameLabel] == c.driverName &&
		capacity.Labels[ManagedByLabel] == c.managedByID
}

func sizesAreEqual(expected, actual *resource.Quantity) bool {
	if expected == actual {
		// Both nil or pointer to same value.
		return true
	}
	if expected == nil || actual == nil {
		// can not compare nil with non-nil.
		return false
	}
	// Both not nil, compare values.
	return expected.Value() == actual.Value()
}
