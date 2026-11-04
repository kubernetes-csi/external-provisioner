package controller

import (
	"maps"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
	"k8s.io/apimachinery/pkg/types"
)

// secretCache caches the data of the Secrets used as provisioner credentials.
// Entries expire a fixed time after they were fetched, concurrent fetches of
// one Secret are collapsed into one and errors are never cached.
type secretCache struct {
	ttl        time.Duration
	maxEntries int
	now        func() time.Time

	mutex   sync.Mutex
	entries map[types.NamespacedName]secretCacheEntry
	group   singleflight.Group
}

type secretCacheEntry struct {
	data    map[string]string
	expires time.Time
}

func newSecretCache(ttl time.Duration, maxEntries int) *secretCache {
	return &secretCache{
		ttl:        ttl,
		maxEntries: maxEntries,
		now:        time.Now,
		entries:    make(map[types.NamespacedName]secretCacheEntry),
	}
}

// get returns a copy of the cached data for key or calls fetch to get it.
func (c *secretCache) get(key types.NamespacedName, fetch func() (map[string]string, error)) (map[string]string, error) {
	if data, ok := c.lookup(key); ok {
		return data, nil
	}
	data, err, _ := c.group.Do(key.String(), func() (any, error) {
		if data, ok := c.lookup(key); ok {
			return data, nil
		}
		data, err := fetch()
		if err == nil {
			c.store(key, data)
		}
		return data, err
	})
	if err != nil {
		return nil, err
	}
	return maps.Clone(data.(map[string]string)), nil
}

func (c *secretCache) lookup(key types.NamespacedName) (map[string]string, bool) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	entry, ok := c.entries[key]
	if !ok || !c.now().Before(entry.expires) {
		delete(c.entries, key)
		return nil, false
	}
	return maps.Clone(entry.data), true
}

// store does nothing if the cache is full of unexpired entries.
func (c *secretCache) store(key types.NamespacedName, data map[string]string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	now := c.now()
	if _, ok := c.entries[key]; !ok && len(c.entries) >= c.maxEntries {
		for k, e := range c.entries {
			if !now.Before(e.expires) {
				delete(c.entries, k)
			}
		}
		if len(c.entries) >= c.maxEntries {
			return
		}
	}
	c.entries[key] = secretCacheEntry{data: maps.Clone(data), expires: now.Add(c.ttl)}
}
