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

package topology

import (
	"context"
	"fmt"
	"sort"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Segment represents a topology segment. Entries are always sorted by
// key and keys are unique. In contrast to a map, segments therefore
// can be compared efficiently. A nil segment matches no nodes
// in a cluster, an empty segment all of them.
type Segment []SegmentEntry

var _ sort.Interface = Segment{}

// String returns the address *and* the content of the segment; the address
// is how the segment is identified when used as a hash key.
func (s *Segment) String() string {
	return fmt.Sprintf("%p = %s", s, s.SimpleString())
}

// SimpleString only returns the content.
func (s *Segment) SimpleString() string {
	var parts []string
	for _, entry := range *s {
		parts = append(parts, entry.String())
	}
	return strings.Join(parts, "+ ")
}

// Compare returns -1 if s is considered smaller than the other segment (less keys,
// keys and/or values smaller), 0 if equal and 1 otherwise.
func (s Segment) Compare(other Segment) int {
	if len(s) < len(other) {
		return -1
	}
	if len(s) > len(other) {
		return 1
	}
	for i := 0; i < len(s); i++ {
		cmp := s[i].Compare(other[i])
		if cmp != 0 {
			return cmp
		}
	}
	return 0
}

func (s Segment) Len() int           { return len(s) }
func (s Segment) Less(i, j int) bool { return s[i].Compare(s[j]) < 0 }
func (s Segment) Swap(i, j int) {
	entry := s[i]
	s[i] = s[j]
	s[j] = entry
}

// SegmentEntry represents one topology key/value pair.
type SegmentEntry struct {
	Key, Value string
}

func (se SegmentEntry) String() string {
	return se.Key + ": " + se.Value
}

// Compare returns -1 if se is considered smaller than the other segment entry (key or value smaller),
// 0 if equal and 1 otherwise.
func (se SegmentEntry) Compare(other SegmentEntry) int {
	cmp := strings.Compare(se.Key, other.Key)
	if cmp != 0 {
		return cmp
	}
	return strings.Compare(se.Value, other.Value)
}

// GetLabelSelector returns a LabelSelector with the key/value entries
// as label match criteria.
func (s Segment) GetLabelSelector() *metav1.LabelSelector {
	return &metav1.LabelSelector{
		MatchLabels: s.GetLabelMap(),
	}
}

// GetLabelMap returns nil if the Segment itself is nil,
// otherwise a map with all key/value pairs.
func (s Segment) GetLabelMap() map[string]string {
	if s == nil {
		return nil
	}
	labels := map[string]string{}
	for _, entry := range s {
		labels[entry.Key] = entry.Value
	}
	return labels
}

// Informer keeps a list of discovered topology segments and can
// notify one or more clients when it discovers changes. Segments
// are identified by their address and guaranteed to be unique.
type Informer interface {
	// AddCallback ensures that the function is called each time
	// changes to the list of segments are detected. It also gets
	// called immediately when adding the callback and there are
	// already some known segments.
	AddCallback(cb Callback)

	// List returns all known segments, in no particular order.
	List() []*Segment

	// HasSynced returns true once all segments have been found.
	HasSynced() bool

	// RunWorker starts a worker to process queue.
	RunWorker(ctx context.Context)
}

type Callback func(added []*Segment, removed []*Segment)
