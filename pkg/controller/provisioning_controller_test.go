package controller

import (
	"context"
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/informers"
	fakeclientset "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/util/workqueue"
)

func provisioningBaseClaim(pvName string) *v1.PersistentVolumeClaim {
	return fakeClaim("test", "test", "fake-claim-uid", 1000, pvName, v1.ClaimBound, &fakeSc1, "").DeepCopy()
}

func TestProvisioningFinalizerRemoval(t *testing.T) {
	testcases := map[string]struct {
		claim           runtime.Object
		expectFinalizer bool
		expectError     error
	}{
		"provisioning pvc without volumeName": {
			claim:           pvcFinalizers(provisioningBaseClaim(""), pvcProvisioningFinalizer),
			expectFinalizer: true,
		},
		"pvc without provisioning finalizer": {
			claim:           baseClaim(),
			expectFinalizer: false,
		},
		"provisioning pvc with volumeName": {
			claim:           pvcFinalizers(provisioningBaseClaim("test"), pvcProvisioningFinalizer),
			expectFinalizer: false,
		},
	}

	for k, tc := range testcases {
		tc := tc
		t.Run(k, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()

			clientSet := fakeclientset.NewSimpleClientset(tc.claim)
			provisioningProtector := fakeProvisioningProtector(clientSet, tc.claim)

			claim := tc.claim.(*v1.PersistentVolumeClaim)
			err := provisioningProtector.syncClaim(ctx, claim)

			// Get updated claim after reconcile finish
			claim, _ = clientSet.CoreV1().PersistentVolumeClaims(claim.Namespace).Get(ctx, claim.Name, metav1.GetOptions{})

			// Check finalizers removal
			if tc.expectFinalizer && !checkFinalizer(claim, pvcProvisioningFinalizer) {
				t.Errorf("Claim finalizer was expected to be found on: %s", claim.Name)
			} else if !tc.expectFinalizer && checkFinalizer(claim, pvcProvisioningFinalizer) {
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

func fakeProvisioningProtector(client *fakeclientset.Clientset, objects ...runtime.Object) *ProvisioningProtectionController {
	utilruntime.ReallyCrash = false

	informerFactory := informers.NewSharedInformerFactory(client, 1*time.Second)
	claimInformer := informerFactory.Core().V1().PersistentVolumeClaims().Informer()
	claimLister := informerFactory.Core().V1().PersistentVolumeClaims().Lister()
	rateLimiter := workqueue.NewItemExponentialFailureRateLimiter(time.Second, 2*time.Second)
	claimQueue := workqueue.NewNamedRateLimitingQueue(rateLimiter, "provisioning-protection")

	for _, claim := range objects {
		claimInformer.GetStore().Add(claim)
	}

	informerFactory.WaitForCacheSync(context.TODO().Done())
	go informerFactory.Start(context.TODO().Done())

	return NewProvisioningProtectionController(
		client,
		claimLister,
		claimInformer,
		claimQueue,
	)
}
