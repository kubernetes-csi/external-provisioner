package controller

import (
	"fmt"
	"sync"

	"k8s.io/apimachinery/pkg/types"
)

// TopologyInfo holds the data for NodeLabels and TopologyKeys
type TopologyInfo struct {
	NodeLabels     map[string]string
	TopologyKeys   []string
	RequisiteTerms []topologyTerm
}

// TopologyProvider is an interface that defines the behavior for looking up
// a TopologyInfo object by its pvc UID.
type TopologyProvider interface {
	GetByPvcUID(pvcUID types.UID) (*TopologyInfo, error)
	// The entry is deleted when provision succeeds or returns a final error.
	Delete(pvcUID types.UID) error

	// Update methods now perform an "upsert" and don't return errors.
	UpdateNodeLabels(pvcUID types.UID, newLabels map[string]string)
	UpdateTopologyKeys(pvcUID types.UID, newKeys []string)
	UpdateRequisiteTerms(pvcUID types.UID, requisiteTerms []topologyTerm)
}

// InMemoryStore is a concrete implementation of TopologyProvider.
// It uses an in-memory map for quick lookups.
type InMemoryStore struct {
	// The map key is the object's name.
	data map[types.UID]*TopologyInfo
	// Adding a mutex for thread-safe access
	mutex sync.RWMutex
}

// NewInMemoryStore creates and initializes a new store.
func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		data: make(map[types.UID]*TopologyInfo),
	}
}

// Delete implements the TopologyProvider interface.
// It uses the built-in delete() function to remove the item from the map.
func (s *InMemoryStore) Delete(pvcUID types.UID) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	// First, check if the key exists to provide a helpful error.
	_, found := s.data[pvcUID]
	if !found {
		return nil
	}
	delete(s.data, pvcUID)
	return nil
}

// GetByPvcUID implements the TopologyProvider interface.
func (s *InMemoryStore) GetByPvcUID(pvcUID types.UID) (*TopologyInfo, error) {
	if s == nil {
		return nil, fmt.Errorf("pvcNodeStore is nil")
	}
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	info, found := s.data[pvcUID]
	if !found {
		return nil, fmt.Errorf("topology object with pvcUID '%s' not found", pvcUID)
	}

	// Return a deep copy to prevent data races
	infoCopy := &TopologyInfo{}
	if info.NodeLabels != nil {
		infoCopy.NodeLabels = make(map[string]string)
		for k, v := range info.NodeLabels {
			infoCopy.NodeLabels[k] = v
		}
	}

	if info.TopologyKeys != nil {
		infoCopy.TopologyKeys = make([]string, len(info.TopologyKeys))
		copy(infoCopy.TopologyKeys, info.TopologyKeys)
	}

	if info.RequisiteTerms != nil {
		infoCopy.RequisiteTerms = make([]topologyTerm, len(info.RequisiteTerms))
		for i, term := range info.RequisiteTerms {
			newTerm := make(topologyTerm, len(term))
			copy(newTerm, term)
			infoCopy.RequisiteTerms[i] = newTerm
		}
	}

	return infoCopy, nil
}

// UpdateNodeLabels finds an object by pvcUID and replaces its NodeLabels.
func (s *InMemoryStore) UpdateNodeLabels(pvcUID types.UID, newLabels map[string]string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	info, found := s.data[pvcUID]
	if !found {
		s.data[pvcUID] = &TopologyInfo{NodeLabels: newLabels}
	} else {
		info.NodeLabels = newLabels
	}
}

// UpdateTopologyKeys finds an object by pvcUID and replaces its TopologyKeys.
func (s *InMemoryStore) UpdateTopologyKeys(pvcUID types.UID, newKeys []string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	info, found := s.data[pvcUID]
	if !found {
		s.data[pvcUID] = &TopologyInfo{TopologyKeys: newKeys}
	} else {
		info.TopologyKeys = newKeys
	}
}

// UpdateRequisiteTerms finds an object by pvcUID and replaces its RequisiteTerms.
func (s *InMemoryStore) UpdateRequisiteTerms(pvcUID types.UID, requisiteTerms []topologyTerm) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	info, found := s.data[pvcUID]
	if !found {
		s.data[pvcUID] = &TopologyInfo{RequisiteTerms: requisiteTerms}
	} else {
		info.RequisiteTerms = requisiteTerms
	}
}
