package controller

import (
	"fmt"
	"sync"
)

// TopologyInfo holds the data for NodeLabels and TopologyKeys
type TopologyInfo struct {
	NodeLabels       map[string]string
	TopologyKeys     []string
	SelectedNodeName string
}

// TopologyProvider is an interface that defines the behavior for looking up
// a TopologyInfo object by its name.
// The name is pvcNamespace/pvcName
type TopologyProvider interface {
	GetByName(name string) (*TopologyInfo, error)
	Delete(name string) error

	// Update methods now perform an "upsert" and don't return errors.
	UpdateNodeLabels(name string, newLabels map[string]string)
	UpdateTopologyKeys(name string, newKeys []string)
	UpdateSelectedNodeName(name string, newName string)
}

// InMemoryStore is a concrete implementation of TopologyProvider.
// It uses an in-memory map for quick lookups.
type InMemoryStore struct {
	// The map key is the object's name.
	data map[string]*TopologyInfo
	// Adding a mutex for thread-safe access
	mutex sync.Mutex
}

// NewInMemoryStore creates and initializes a new store.
func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		data: make(map[string]*TopologyInfo),
	}
}

// Add is a helper function to populate our store with data.
func (s *InMemoryStore) Add(name string, info *TopologyInfo) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.data[name] = info
}

// Delete implements the TopologyProvider interface.
// It uses the built-in delete() function to remove the item from the map.
func (s *InMemoryStore) Delete(name string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	// First, check if the key exists to provide a helpful error.
	_, found := s.data[name]
	if !found {
		return fmt.Errorf("cannot delete: object with name '%s' not found", name)
	}
	delete(s.data, name)
	return nil
}

// GetByName implements the TopologyProvider interface.
func (s *InMemoryStore) GetByName(name string) (*TopologyInfo, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if s == nil {
		return nil, fmt.Errorf("pvcNodeStore is nil")
	}
	info, found := s.data[name]
	if !found {
		return nil, fmt.Errorf("topology object with name '%s' not found", name)
	}
	return info, nil
}

// UpdateNodeLabels finds an object by name and replaces its NodeLabels.
func (s *InMemoryStore) UpdateNodeLabels(name string, newLabels map[string]string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	info, found := s.data[name]
	if !found {
		s.data[name] = &TopologyInfo{NodeLabels: newLabels}
	} else {
		info.NodeLabels = newLabels
	}
}

// UpdateTopologyKeys finds an object by name and replaces its TopologyKeys.
func (s *InMemoryStore) UpdateTopologyKeys(name string, newKeys []string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	info, found := s.data[name]
	if !found {
		s.data[name] = &TopologyInfo{TopologyKeys: newKeys}
	} else {
		info.TopologyKeys = newKeys
	}
}

// UpdateSelectedNodeName finds an object by name and replaces its SelectedNodeName.
func (s *InMemoryStore) UpdateSelectedNodeName(name string, newName string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	info, found := s.data[name]
	if !found {
		s.data[name] = &TopologyInfo{SelectedNodeName: newName}
	} else {
		info.SelectedNodeName = newName
	}
}
