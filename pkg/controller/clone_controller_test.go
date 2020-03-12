package controller

import (
	"context"
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/informers"
	fakeclientset "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/util/workqueue"
)

var requestedBytes int64 = 1000
var fakeSc1 string = "fake-sc-1"

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

func pvcDataSourceClone(pvc *v1.PersistentVolumeClaim, srcName string) *v1.PersistentVolumeClaim {
	apiGr := ""
	pvc.Spec.DataSource = &v1.TypedLocalObjectReference{
		APIGroup: &apiGr,
		Kind:     pvcKind,
		Name:     srcName,
	}
	return pvc
}

func pvcNamed(pvc *v1.PersistentVolumeClaim, name string) *v1.PersistentVolumeClaim {
	pvc.Name = name
	return pvc
}

func pvcNamespaced(pvc *v1.PersistentVolumeClaim, namespace string) *v1.PersistentVolumeClaim {
	pvc.Namespace = namespace
	return pvc
}

func pvcPhase(pvc *v1.PersistentVolumeClaim, phase v1.PersistentVolumeClaimPhase) *v1.PersistentVolumeClaim {
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
		initialClaims     []runtime.Object
		expectClaim       bool
		dstPVCStatusPhase v1.PersistentVolumeClaimPhase
	}{
		"delete source pvc with no cloning in progress": {
			initialClaims: []runtime.Object{
				pvcFinalizers(baseClaim(), pvcCloneFinalizer),
				pvcDataSourceClone(
					pvcNamed(baseClaim(), dstName),
					srcName,
				),
			},
		},
		"delete source pvc when destination pvc status is claim pending": {
			initialClaims: []runtime.Object{
				pvcFinalizers(baseClaim(), pvcCloneFinalizer),
				pvcPhase(
					pvcDataSourceClone(
						pvcNamed(baseClaim(), dstName),
						srcName,
					),
					v1.ClaimPending)},
			expectClaim: true,
		},
		"delete source pvc when at least one destination pvc status is claim pending": {
			initialClaims: []runtime.Object{
				pvcFinalizers(baseClaim(), pvcCloneFinalizer),
				pvcDataSourceClone(
					pvcNamed(baseClaim(), dstName),
					srcName,
				),
				pvcPhase(
					pvcDataSourceClone(
						pvcNamed(baseClaim(), dstName+"1"),
						srcName,
					),
					v1.ClaimPending)},
			expectClaim: true,
		},
		"delete source pvc located in another namespace should not block": {
			initialClaims: []runtime.Object{
				pvcNamespaced(
					pvcFinalizers(
						baseClaim(), pvcCloneFinalizer,
					),
					srcNamespace+"1"),
				pvcPhase(
					pvcDataSourceClone(
						pvcNamed(baseClaim(), dstName+"1"),
						srcName,
					),
					v1.ClaimPending)},
		},
		"delete source pvc which is not cloned by any other pvc": {
			initialClaims: []runtime.Object{pvcFinalizers(baseClaim(), pvcCloneFinalizer)},
		},
		"delete source pvc without finalizer": {
			initialClaims: []runtime.Object{baseClaim()},
		},
	}

	for k, tc := range testcases {
		tc := tc
		t.Run(k, func(t *testing.T) {
			t.Parallel()
			var clientSet *fakeclientset.Clientset

			utilruntime.ReallyCrash = false

			clientSet = fakeclientset.NewSimpleClientset(tc.initialClaims...)
			informerFactory := informers.NewSharedInformerFactory(clientSet, 1*time.Second)
			claimInformer := informerFactory.Core().V1().PersistentVolumeClaims().Informer()
			claimLister := informerFactory.Core().V1().PersistentVolumeClaims().Lister()
			rateLimiter := workqueue.NewItemExponentialFailureRateLimiter(time.Second, 2*time.Second)
			claimQueue := workqueue.NewNamedRateLimitingQueue(rateLimiter, "claims")

			for _, claim := range tc.initialClaims {
				claimInformer.GetStore().Add(claim)
			}

			informerFactory.WaitForCacheSync(context.TODO().Done())
			go informerFactory.Start(context.TODO().Done())

			cloningProtector := NewCloningProtectionController(
				clientSet,
				claimLister,
				claimInformer,
				claimQueue,
			)

			go cloningProtector.Run(1, context.TODO().Done())

			// Get PVC as accepted by fake client
			var claim *v1.PersistentVolumeClaim
			claims, _ := claimLister.List(labels.Everything())
			for _, c := range claims {
				if c.Name == srcName {
					claim = c
				}
			}

			// Simulate Delete behavior
			clientSet.CoreV1().PersistentVolumeClaims(claim.Namespace).Update(pvcDeletionMarked(claim))

			// Wait for couple reconciles for controller to adjust finalizers
			time.Sleep(2 * time.Second)

			claim, _ = clientSet.CoreV1().PersistentVolumeClaims(claim.Namespace).Get(claim.Name, metav1.GetOptions{})
			if !checkFinalizer(claim, pvcCloneFinalizer) {
				clientSet.CoreV1().PersistentVolumeClaims(claim.Namespace).Delete(claim.Name, &metav1.DeleteOptions{})
				// Wait for deletion and cache update
				time.Sleep(2 * time.Second)
			}

			// Check finalizers removal
			if tc.expectClaim {
				_, err := claimLister.PersistentVolumeClaims(claim.Namespace).Get(claim.Name)
				if err != nil {
					t.Errorf("Source claim does not exist: %s", err)
				}
			} else {
				claims, err := claimLister.List(labels.Everything())
				if err != nil {
					t.Errorf("error listing claims: %s", err)
				}
				if len(claims) == len(tc.initialClaims) {
					t.Error("Claim was not removed")
				}
			}
		})
	}

}
