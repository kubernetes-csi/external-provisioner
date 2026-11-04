package controller

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
	storagelistersv1 "k8s.io/client-go/listers/storage/v1"
	k8stesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
)

var testKey = types.NamespacedName{Namespace: "ns", Name: "secret"}

func newTestSecretCache(maxEntries int) (*secretCache, *time.Time) {
	now := time.Unix(1000, 0)
	c := newSecretCache(10*time.Second, maxEntries)
	c.now = func() time.Time { return now }
	return c, &now
}

func TestSecretCacheExpiry(t *testing.T) {
	c, now := newTestSecretCache(8)
	value, fetchErr := "v1", error(nil)
	fetches := 0
	fetch := func() (map[string]string, error) {
		fetches++
		return map[string]string{"key": value}, fetchErr
	}

	got, err := c.get(testKey, fetch)
	require.NoError(t, err)
	assert.Equal(t, "v1", got["key"])

	// A rotation is not seen until the entry expires, which is counted from the fetch.
	value = "v2"
	*now = now.Add(9 * time.Second)
	got, _ = c.get(testKey, fetch)
	assert.Equal(t, "v1", got["key"])
	*now = now.Add(time.Second)
	got, _ = c.get(testKey, fetch)
	assert.Equal(t, "v2", got["key"])
	assert.Equal(t, 2, fetches)

	// Errors are not cached and an expired entry is not served instead of an error.
	*now = now.Add(10 * time.Second)
	fetchErr = errors.New("not found")
	for i := 0; i < 2; i++ {
		got, err = c.get(testKey, fetch)
		assert.Error(t, err)
		assert.Nil(t, got)
	}
	assert.Equal(t, 4, fetches)
}

func TestSecretCacheKeysAndCopies(t *testing.T) {
	c, _ := newTestSecretCache(8)
	fetches := 0
	fetch := func() (map[string]string, error) {
		fetches++
		return map[string]string{"key": "v"}, nil
	}
	for _, key := range []types.NamespacedName{{Namespace: "a", Name: "s"}, {Namespace: "b", Name: "s"}, {Namespace: "a", Name: "t"}} {
		first, _ := c.get(key, fetch)
		first["key"] = "changed by caller"
		second, _ := c.get(key, fetch)
		assert.Equal(t, "v", second["key"])
	}
	assert.Equal(t, 3, fetches, "one fetch per Secret")
}

func TestSecretCacheMaxEntries(t *testing.T) {
	c, now := newTestSecretCache(2)
	fetches := 0
	fetch := func() (map[string]string, error) {
		fetches++
		return map[string]string{}, nil
	}
	key := func(name string) types.NamespacedName { return types.NamespacedName{Namespace: "ns", Name: name} }

	for _, name := range []string{"a", "b", "c", "c"} {
		_, err := c.get(key(name), fetch)
		require.NoError(t, err)
	}
	assert.Len(t, c.entries, 2)
	assert.Equal(t, 4, fetches, "c does not fit and is fetched every time")

	*now = now.Add(10 * time.Second)
	for i := 0; i < 2; i++ {
		_, err := c.get(key("c"), fetch)
		require.NoError(t, err)
	}
	assert.Len(t, c.entries, 1, "expired entries make room")
	assert.Equal(t, 5, fetches)
}

func TestSecretCacheConcurrentGets(t *testing.T) {
	c := newSecretCache(time.Minute, 8)
	release := make(chan struct{})
	var fetches atomic.Int32
	fetch := func() (map[string]string, error) {
		fetches.Add(1)
		<-release
		return map[string]string{"key": "v"}, nil
	}

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, err := c.get(testKey, fetch)
			assert.NoError(t, err)
			assert.Equal(t, "v", got["key"])
			got["key"] = "changed by caller"
		}()
	}
	close(release)
	wg.Wait()
	assert.EqualValues(t, 1, fetches.Load())
}

func newSecretCountingClient(secrets ...runtime.Object) (*fake.Clientset, *atomic.Int32) {
	client := fake.NewSimpleClientset(secrets...)
	var gets atomic.Int32
	client.PrependReactor("get", "secrets", func(k8stesting.Action) (bool, runtime.Object, error) {
		gets.Add(1)
		return false, nil, nil
	})
	return client, &gets
}

func testSecret(value string) *v1.Secret {
	return &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "secret"},
		Data:       map[string][]byte{"key": []byte(value)},
	}
}

func TestGetCredentialsCache(t *testing.T) {
	ref := &v1.SecretReference{Namespace: "ns", Name: "secret"}
	for name, secretCache := range map[string]*secretCache{"disabled": nil, "enabled": newSecretCache(time.Minute, 8)} {
		t.Run(name, func(t *testing.T) {
			client, gets := newSecretCountingClient(testSecret("v1"))
			p := &csiProvisioner{client: client, secretCache: secretCache}
			for i := 0; i < 3; i++ {
				got, err := p.getCredentials(context.Background(), ref)
				require.NoError(t, err)
				assert.Equal(t, map[string]string{"key": "v1"}, got)
			}
			if secretCache == nil {
				assert.EqualValues(t, 3, gets.Load())
			} else {
				assert.EqualValues(t, 1, gets.Load())
			}
			got, err := p.getCredentials(context.Background(), nil)
			assert.NoError(t, err)
			assert.Nil(t, got)
		})
	}
}

func TestHandleSecretsForDeletionCache(t *testing.T) {
	sc := &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{Name: "sc"},
		Parameters: map[string]string{
			prefixedProvisionerSecretNameKey:      "secret",
			prefixedProvisionerSecretNamespaceKey: "ns",
		},
	}
	for name, annotations := range map[string]map[string]string{
		"from annotations": {
			annDeletionProvisionerSecretRefName:      "secret",
			annDeletionProvisionerSecretRefNamespace: "ns",
		},
		"from storage class": nil,
	} {
		t.Run(name, func(t *testing.T) {
			client, gets := newSecretCountingClient(testSecret("v1"))
			indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
			require.NoError(t, indexer.Add(sc))
			p := &csiProvisioner{
				client:      client,
				scLister:    storagelistersv1.NewStorageClassLister(indexer),
				secretCache: newSecretCache(time.Minute, 8),
			}
			pv := &v1.PersistentVolume{
				ObjectMeta: metav1.ObjectMeta{Name: "pv", Annotations: annotations},
				Spec: v1.PersistentVolumeSpec{
					StorageClassName: "sc",
					ClaimRef:         &v1.ObjectReference{Namespace: "ns", Name: "claim"},
				},
			}
			for i := 0; i < 3; i++ {
				req := &csi.DeleteVolumeRequest{}
				require.NoError(t, p.handleSecretsForDeletion(context.Background(), pv, req, false))
				assert.Equal(t, map[string]string{"key": "v1"}, req.Secrets)
			}
			assert.EqualValues(t, 1, gets.Load())
		})
	}
}
