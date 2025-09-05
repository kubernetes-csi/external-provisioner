package controller

import (
	"fmt"
	"reflect"
	"sync"
	"testing"
)

// TestNewInMemoryStore tests the NewInMemoryStore function.
func TestNewInMemoryStore(t *testing.T) {
	store := NewInMemoryStore()
	if store == nil {
		t.Error("Expected a new store, but got nil")
	}
	if store.data == nil {
		t.Error("Expected store data to be initialized, but it was nil")
	}
}

// TestAddAndGet tests the Add and GetByName methods.
func TestAddAndGetInInMemoryStore(t *testing.T) {
	store := NewInMemoryStore()
	info := &TopologyInfo{NodeLabels: map[string]string{"foo": "bar"}}
	store.Add("test", info)

	retrieved, err := store.GetByName("test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(info, retrieved) {
		t.Errorf("Expected %+v, got %+v", info, retrieved)
	}

	_, err = store.GetByName("nonexistent")
	if err == nil {
		t.Error("Expected an error for a nonexistent entry, but got nil")
	}
}

// TestDelete tests the Delete method.
func TestDeleteInInMemoryStore(t *testing.T) {
	store := NewInMemoryStore()
	info := &TopologyInfo{NodeLabels: map[string]string{"foo": "bar"}}
	store.Add("test", info)

	err := store.Delete("test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = store.GetByName("test")
	if err == nil {
		t.Error("Expected an error after deleting the entry, but got nil")
	}

	err = store.Delete("nonexistent")
	if err == nil {
		t.Error("Expected an error for deleting a nonexistent entry, but got nil")
	}
}

// TestUpdate tests the update methods.
func TestUpdateInInMemoryStore(t *testing.T) {
	store := NewInMemoryStore()

	// Test updating a nonexistent entry
	store.UpdateNodeLabels("test", map[string]string{"foo": "bar"})
	retrieved, _ := store.GetByName("test")
	if retrieved.NodeLabels["foo"] != "bar" {
		t.Errorf("Expected NodeLabels to be updated")
	}

	// Test updating an existing entry
	store.UpdateNodeLabels("test", map[string]string{"foo": "baz"})
	retrieved, _ = store.GetByName("test")
	if retrieved.NodeLabels["foo"] != "baz" {
		t.Errorf("Expected NodeLabels to be updated")
	}

	store.UpdateTopologyKeys("test", []string{"key1"})
	retrieved, _ = store.GetByName("test")
	if retrieved.TopologyKeys[0] != "key1" {
		t.Errorf("Expected TopologyKeys to be updated")
	}

	store.UpdateSelectedNodeName("test", "node1")
	retrieved, _ = store.GetByName("test")
	if retrieved.SelectedNodeName != "node1" {
		t.Errorf("Expected SelectedNodeName to be updated")
	}
}

// TestConcurrentAccess tests thread-safety of the InMemoryStore.
func TestConcurrentAccessInInMemoryStore(t *testing.T) {
	store := NewInMemoryStore()
	var wg sync.WaitGroup

	// Number of concurrent goroutines
	concurrency := 100

	wg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		go func(i int) {
			defer wg.Done()
			name := fmt.Sprintf("item-%d", i)
			info := &TopologyInfo{SelectedNodeName: fmt.Sprintf("node-%d", i)}

			store.Add(name, info)

			retrieved, err := store.GetByName(name)
			if err != nil {
				t.Errorf("goroutine %d: unexpected error getting item: %v", i, err)
			}
			if retrieved.SelectedNodeName != info.SelectedNodeName {
				t.Errorf("goroutine %d: retrieved wrong data", i)
			}

			store.UpdateSelectedNodeName(name, fmt.Sprintf("new-node-%d", i))

			err = store.Delete(name)
			if err != nil {
				t.Errorf("goroutine %d: unexpected error deleting item: %v", i, err)
			}
		}(i)
	}

	wg.Wait()
}
