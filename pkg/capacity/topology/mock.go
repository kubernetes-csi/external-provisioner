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
)

// NewFixedNodeTopology creates topology informer for
// a driver with a fixed topology segment.
func NewFixedNodeTopology(segment *Segment) Informer {
	return NewMock(segment)
}

// NewMock creates a new mocked topology informer with a
// certain set of pre-defined segments.
func NewMock(segments ...*Segment) *Mock {
	return &Mock{
		segments: segments,
	}
}

var _ Informer = &Mock{}

// Mock simulates a driver installation on one or more nodes.
type Mock struct {
	segments  []*Segment
	callbacks []Callback
}

func (mt *Mock) AddCallback(cb Callback) {
	mt.callbacks = append(mt.callbacks, cb)
	cb(mt.segments, nil)
}

func (mt *Mock) List() []*Segment {
	return mt.segments
}

func (mt *Mock) RunWorker(ctx context.Context) {
}

func (mt *Mock) HasSynced() bool {
	return true
}

// Modify adds and/or removes segments.
func (mt *Mock) Modify(add, remove []*Segment) {
	var added, removed []*Segment
	for _, segment := range add {
		if mt.segmentIndex(segment) == -1 {
			added = append(added, segment)
			mt.segments = append(mt.segments, segment)
		}
	}
	for _, segment := range remove {
		index := mt.segmentIndex(segment)
		if index != -1 {
			removed = append(removed, segment)
			mt.segments = append(mt.segments[0:index], mt.segments[index+1:]...)
		}
	}
	for _, cb := range mt.callbacks {
		cb(added, removed)
	}
}

func (mt *Mock) segmentIndex(segment *Segment) int {
	for i, otherSegment := range mt.segments {
		if otherSegment == segment {
			return i
		}
	}
	return -1
}
