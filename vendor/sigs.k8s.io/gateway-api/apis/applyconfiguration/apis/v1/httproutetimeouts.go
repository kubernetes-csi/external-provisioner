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

// HTTPRouteTimeoutsApplyConfiguration represents a declarative configuration of the HTTPRouteTimeouts type for use
// with apply.
type HTTPRouteTimeoutsApplyConfiguration struct {
	Request        *v1.Duration `json:"request,omitempty"`
	BackendRequest *v1.Duration `json:"backendRequest,omitempty"`
}

// HTTPRouteTimeoutsApplyConfiguration constructs a declarative configuration of the HTTPRouteTimeouts type for use with
// apply.
func HTTPRouteTimeouts() *HTTPRouteTimeoutsApplyConfiguration {
	return &HTTPRouteTimeoutsApplyConfiguration{}
}

// WithRequest sets the Request field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the Request field is set to the value of the last call.
func (b *HTTPRouteTimeoutsApplyConfiguration) WithRequest(value v1.Duration) *HTTPRouteTimeoutsApplyConfiguration {
	b.Request = &value
	return b
}

// WithBackendRequest sets the BackendRequest field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the BackendRequest field is set to the value of the last call.
func (b *HTTPRouteTimeoutsApplyConfiguration) WithBackendRequest(value v1.Duration) *HTTPRouteTimeoutsApplyConfiguration {
	b.BackendRequest = &value
	return b
}
