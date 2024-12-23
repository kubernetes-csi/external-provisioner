/*
Copyright The Kubernetes Authors.

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

// Code generated by applyconfiguration-gen. DO NOT EDIT.

package v1

import (
	v1 "sigs.k8s.io/gateway-api/apis/v1"
)

// GRPCHeaderMatchApplyConfiguration represents a declarative configuration of the GRPCHeaderMatch type for use
// with apply.
type GRPCHeaderMatchApplyConfiguration struct {
	Type  *v1.GRPCHeaderMatchType `json:"type,omitempty"`
	Name  *v1.GRPCHeaderName      `json:"name,omitempty"`
	Value *string                 `json:"value,omitempty"`
}

// GRPCHeaderMatchApplyConfiguration constructs a declarative configuration of the GRPCHeaderMatch type for use with
// apply.
func GRPCHeaderMatch() *GRPCHeaderMatchApplyConfiguration {
	return &GRPCHeaderMatchApplyConfiguration{}
}

// WithType sets the Type field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the Type field is set to the value of the last call.
func (b *GRPCHeaderMatchApplyConfiguration) WithType(value v1.GRPCHeaderMatchType) *GRPCHeaderMatchApplyConfiguration {
	b.Type = &value
	return b
}

// WithName sets the Name field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the Name field is set to the value of the last call.
func (b *GRPCHeaderMatchApplyConfiguration) WithName(value v1.GRPCHeaderName) *GRPCHeaderMatchApplyConfiguration {
	b.Name = &value
	return b
}

// WithValue sets the Value field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the Value field is set to the value of the last call.
func (b *GRPCHeaderMatchApplyConfiguration) WithValue(value string) *GRPCHeaderMatchApplyConfiguration {
	b.Value = &value
	return b
}
