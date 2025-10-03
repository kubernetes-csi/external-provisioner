package controller

import (
	"fmt"
	"reflect"
	"sync"
	"testing"

	"k8s.io/apimachinery/pkg/types"
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

// TestAddAndGet tests the Add and GetByPvcUID methods.
func TestAddAndGetInInMemoryStore(t *testing.T) {
	store := NewInMemoryStore()
	pvcUID := types.UID("a9e2d5a3-1c64-4787-97b7-1a22d5a0b123")
	labels := map[string]string{"foo": "bar"}
	store.UpdateNodeLabels(pvcUID, labels)

	retrieved, err := store.GetByPvcUID(pvcUID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedInfo := &TopologyInfo{NodeLabels: labels}
	if !reflect.DeepEqual(expectedInfo, retrieved) {
		t.Errorf("Expected %+v, got %+v", expectedInfo, retrieved)
	}

	_, err = store.GetByPvcUID("nonexistent")
	if err == nil {
		t.Error("Expected an error for a nonexistent entry, but got nil")
	}
}

// TestDelete tests the Delete method.
func TestDeleteInInMemoryStore(t *testing.T) {
	store := NewInMemoryStore()
	pvcUID := types.UID("a9e2d5a3-1c64-4787-97b7-1a22d5a0b123")
	labels := map[string]string{"foo": "bar"}
	store.UpdateNodeLabels(pvcUID, labels)

	err := store.Delete(pvcUID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = store.GetByPvcUID(pvcUID)
	if err == nil {
		t.Error("Expected an error after deleting the entry, but got nil")
	}

	err = store.Delete("nonexistent")
	if err != nil {
		t.Errorf("Did not expect an error for deleting a nonexistent entry, but got: %v", err)
	}
}

// TestUpdate tests the update methods.
func TestUpdateInInMemoryStore(t *testing.T) {
	store := NewInMemoryStore()
	pvcUID := types.UID("a9e2d5a3-1c64-4787-97b7-1a22d5a0b123")

	// Test updating a nonexistent entry
	store.UpdateNodeLabels(pvcUID, map[string]string{"foo": "bar"})
	retrieved, _ := store.GetByPvcUID(pvcUID)
	if retrieved.NodeLabels["foo"] != "bar" {
		t.Errorf("Expected NodeLabels to be updated")
	}

	// Test updating an existing entry
	store.UpdateNodeLabels(pvcUID, map[string]string{"foo": "baz"})
	retrieved, _ = store.GetByPvcUID(pvcUID)
	if retrieved.NodeLabels["foo"] != "baz" {
		t.Errorf("Expected NodeLabels to be updated")
	}

	store.UpdateTopologyKeys(pvcUID, []string{"key1"})
	retrieved, _ = store.GetByPvcUID(pvcUID)
	if retrieved.TopologyKeys[0] != "key1" {
		t.Errorf("Expected TopologyKeys to be updated")
	}
}

// TestUpdateTermsInInMemoryStore tests the UpdateRequisiteTerms methods.
func TestUpdateTermsInInMemoryStore(t *testing.T) {
	store := NewInMemoryStore()
	pvcUID := types.UID("a9e2d5a3-1c64-4787-97b7-1a22d5a0b123")

	// Define some topology terms for testing
	term1 := topologyTerm{
		{Key: "zone", Value: "zone1"},
		{Key: "rack", Value: "rack1"},
	}
	term2 := topologyTerm{
		{Key: "zone", Value: "zone2"},
		{Key: "rack", Value: "rack2"},
	}

	// Test updating requisite terms for a nonexistent entry
	store.UpdateRequisiteTerms(pvcUID, []topologyTerm{term1})
	retrieved, _ := store.GetByPvcUID(pvcUID)
	if !reflect.DeepEqual(retrieved.RequisiteTerms, []topologyTerm{term1}) {
		t.Errorf("Expected RequisiteTerms to be updated")
	}

	// Test updating requisite terms for an existing entry
	store.UpdateRequisiteTerms(pvcUID, []topologyTerm{term2})
	retrieved, _ = store.GetByPvcUID(pvcUID)
	if !reflect.DeepEqual(retrieved.RequisiteTerms, []topologyTerm{term2}) {
		t.Errorf("Expected RequisiteTerms to be updated")
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
			pvcUID := types.UID(fmt.Sprintf("item-%d", i))
			info := &TopologyInfo{TopologyKeys: []string{fmt.Sprintf("key-%d", i)}}

			store.UpdateTopologyKeys(pvcUID, info.TopologyKeys)

			retrieved, err := store.GetByPvcUID(pvcUID)
			if err != nil {
				t.Errorf("goroutine %d: unexpected error getting item: %v", i, err)
			}
			if !reflect.DeepEqual(retrieved.TopologyKeys, info.TopologyKeys) {
				t.Errorf("goroutine %d: retrieved wrong data", i)
			}

			store.UpdateTopologyKeys(pvcUID, []string{fmt.Sprintf("new-key-%d", i)})
			retrieved, err = store.GetByPvcUID(pvcUID)
			if err != nil {
				t.Errorf("goroutine %d: unexpected error getting item: %v", i, err)
			}
			if !reflect.DeepEqual(retrieved.TopologyKeys, []string{fmt.Sprintf("new-key-%d", i)}) {
				t.Errorf("goroutine %d: retrieved wrong data after update", i)
			}

			err = store.Delete(pvcUID)
			if err != nil {
				t.Errorf("goroutine %d: unexpected error deleting item: %v", i, err)
			}
		}(i)
	}

	wg.Wait()
}
