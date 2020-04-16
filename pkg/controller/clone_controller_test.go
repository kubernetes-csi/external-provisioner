package controller

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/kubernetes-csi/csi-lib-utils/rpc"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/informers"
	fakeclientset "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/util/workqueue"
)

var requestedBytes int64 = 1000
var fakeSc1 = "fake-sc-1"

const (
	srcName      = "clone-source-pvc"
	dstName      = "destination-pvc"
	srcNamespace = "fake-pvc-namespace"
	pvName       = "test-testi"
)

func pvcFinalizers(pvc *v1.PersistentVolumeClaim, finalizers ...string) *v1.PersistentVolumeClaim {
	pvc.Finalizers = append(pvc.Finalizers, finalizers...)
	return pvc
}

func pvcDataSourceClone(srcName string, pvc *v1.PersistentVolumeClaim) *v1.PersistentVolumeClaim {
	apiGr := ""
	pvc.Spec.DataSource = &v1.TypedLocalObjectReference{
		APIGroup: &apiGr,
		Kind:     pvcKind,
		Name:     srcName,
	}
	return pvc
}

func pvcNamed(name string, pvc *v1.PersistentVolumeClaim) *v1.PersistentVolumeClaim {
	pvc.Name = name
	return pvc
}

func pvcNamespaced(namespace string, pvc *v1.PersistentVolumeClaim) *v1.PersistentVolumeClaim {
	pvc.Namespace = namespace
	return pvc
}

func pvcPhase(phase v1.PersistentVolumeClaimPhase, pvc *v1.PersistentVolumeClaim) *v1.PersistentVolumeClaim {
	pvc.Status.Phase = phase
	return pvc
}

func baseClaim() *v1.PersistentVolumeClaim {
	return fakeClaim(srcName, srcNamespace, "fake-claim-uid", requestedBytes, pvName, v1.ClaimBound, &fakeSc1, "").DeepCopy()
}

func pvcDeletionMarked(pvc *v1.PersistentVolumeClaim) *v1.PersistentVolumeClaim {
	timeX := metav1.NewTime(time.Now())
	pvc.DeletionTimestamp = &timeX
	return pvc
}

// TestCloneFinalizerRemoval tests create volume clone
func TestCloneFinalizerRemoval(t *testing.T) {
	testcases := map[string]struct {
		initialClaims   []runtime.Object
		cloneSource     runtime.Object
		expectFinalizer bool
		expectError     error
	}{
		"delete source pvc with no cloning in progress": {
			cloneSource:   pvcFinalizers(baseClaim(), pvcCloneFinalizer),
			initialClaims: []runtime.Object{pvcDataSourceClone(srcName, pvcNamed(dstName, baseClaim()))},
		},
		"delete source pvc when destination pvc status is claim pending": {
			cloneSource:     pvcFinalizers(baseClaim(), pvcCloneFinalizer),
			initialClaims:   []runtime.Object{pvcPhase(v1.ClaimPending, pvcDataSourceClone(srcName, pvcNamed(dstName, baseClaim())))},
			expectFinalizer: true,
			expectError:     fmt.Errorf("PVC '%s' is in 'Pending' state, cloning in progress", dstName),
		},
		"delete source pvc when at least one destination pvc status is claim pending": {
			cloneSource: pvcFinalizers(baseClaim(), pvcCloneFinalizer),
			initialClaims: []runtime.Object{
				pvcDataSourceClone(srcName, pvcNamed(dstName, baseClaim())),
				pvcPhase(v1.ClaimPending, pvcDataSourceClone(srcName, pvcNamed(dstName+"1", baseClaim()))),
			},
			expectFinalizer: true,
			expectError:     fmt.Errorf("PVC '%s' is in 'Pending' state, cloning in progress", dstName+"1"),
		},
		"delete source pvc located in another namespace should not block": {
			cloneSource:   pvcNamespaced(srcNamespace+"1", pvcFinalizers(baseClaim(), pvcCloneFinalizer)),
			initialClaims: []runtime.Object{pvcPhase(v1.ClaimPending, pvcDataSourceClone(srcName, pvcNamed(dstName+"1", baseClaim())))},
		},
		"delete source pvc which is not cloned by any other pvc": {
			cloneSource: pvcFinalizers(baseClaim(), pvcCloneFinalizer),
		},
		"delete source pvc without finalizer": {
			cloneSource: baseClaim(),
		},
	}

	for k, tc := range testcases {
		tc := tc
		t.Run(k, func(t *testing.T) {
			t.Parallel()

			objects := append(tc.initialClaims, tc.cloneSource)
			clientSet := fakeclientset.NewSimpleClientset(objects...)
			cloningProtector := fakeCloningProtector(clientSet, objects...)

			// Simulate Delete behavior
			claim := pvcDeletionMarked(tc.cloneSource.(*v1.PersistentVolumeClaim))
			err := cloningProtector.syncClaim(claim)

			// Get updated claim after reconcile finish
			claim, _ = clientSet.CoreV1().PersistentVolumeClaims(claim.Namespace).Get(claim.Name, metav1.GetOptions{})

			// Check finalizers removal
			if tc.expectFinalizer && !checkFinalizer(claim, pvcCloneFinalizer) {
				t.Errorf("Claim finalizer was expected to be found on: %s", claim.Name)
			} else if !tc.expectFinalizer && checkFinalizer(claim, pvcCloneFinalizer) {
				t.Errorf("Claim finalizer was not expected to be found on: %s", claim.Name)
			}

			if tc.expectError == nil && err != nil {
				t.Errorf("Caught an unexpected error during 'syncClaim' run: %s", err)
			} else if tc.expectError != nil && err == nil {
				t.Errorf("Expected error during 'syncClaim' run, got nil: %s", tc.expectError)
			} else if tc.expectError != nil && err != nil && tc.expectError.Error() != err.Error() {
				t.Errorf("Unexpected error during 'syncClaim' run:\n\t%s\nExpected:\n\t%s", err, tc.expectError)
			}
		})
	}

}

// TestEnqueueClaimUpadate ensure that PVCs will be processed for finalizer removal only on deletionTimestamp being set on the resource
func TestEnqueueClaimUpadate(t *testing.T) {
	testcases := map[string]struct {
		claim    *v1.PersistentVolumeClaim
		queueLen int
	}{
		"enqueue claim with deletionTimestamp": {
			claim:    pvcDeletionMarked(baseClaim()),
			queueLen: 1,
		},
		"enqueue claim without deletionTimestamp": {
			claim:    baseClaim(),
			queueLen: 0,
		},
	}

	for k, tc := range testcases {
		tc := tc
		t.Run(k, func(t *testing.T) {
			t.Parallel()

			objects := []runtime.Object{}
			clientSet := fakeclientset.NewSimpleClientset(objects...)
			cloningProtector := fakeCloningProtector(clientSet, objects...)

			// Simulate queue behavior
			cloningProtector.enqueueClaimUpadate(tc.claim)

			if cloningProtector.claimQueue.Len() != tc.queueLen {
				t.Errorf("claimQueue should contain %d items, got: %d", tc.queueLen, cloningProtector.claimQueue.Len())
			}
		})
	}
}

func fakeCloningProtector(client *fakeclientset.Clientset, objects ...runtime.Object) *CloningProtectionController {
	utilruntime.ReallyCrash = false
	controllerCapabilities := rpc.ControllerCapabilitySet{
		csi.ControllerServiceCapability_RPC_CLONE_VOLUME: true,
	}

	informerFactory := informers.NewSharedInformerFactory(client, 1*time.Second)
	claimInformer := informerFactory.Core().V1().PersistentVolumeClaims().Informer()
	claimLister := informerFactory.Core().V1().PersistentVolumeClaims().Lister()
	rateLimiter := workqueue.NewItemExponentialFailureRateLimiter(time.Second, 2*time.Second)
	claimQueue := workqueue.NewNamedRateLimitingQueue(rateLimiter, "claims")

	for _, claim := range objects {
		claimInformer.GetStore().Add(claim)
	}

	informerFactory.WaitForCacheSync(context.TODO().Done())
	go informerFactory.Start(context.TODO().Done())

	return NewCloningProtectionController(
		client,
		claimLister,
		claimInformer,
		claimQueue,
		controllerCapabilities,
	)
}
