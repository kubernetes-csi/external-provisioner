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

package v1alpha3

import (
	v1 "sigs.k8s.io/gateway-api/apis/applyconfiguration/apis/v1"
	apisv1 "sigs.k8s.io/gateway-api/apis/v1"
	v1alpha3 "sigs.k8s.io/gateway-api/apis/v1alpha3"
)

// BackendTLSPolicyValidationApplyConfiguration represents a declarative configuration of the BackendTLSPolicyValidation type for use
// with apply.
type BackendTLSPolicyValidationApplyConfiguration struct {
	CACertificateRefs       []v1.LocalObjectReferenceApplyConfiguration `json:"caCertificateRefs,omitempty"`
	WellKnownCACertificates *v1alpha3.WellKnownCACertificatesType       `json:"wellKnownCACertificates,omitempty"`
	Hostname                *apisv1.PreciseHostname                     `json:"hostname,omitempty"`
	SubjectAltNames         []SubjectAltNameApplyConfiguration          `json:"subjectAltNames,omitempty"`
}

// BackendTLSPolicyValidationApplyConfiguration constructs a declarative configuration of the BackendTLSPolicyValidation type for use with
// apply.
func BackendTLSPolicyValidation() *BackendTLSPolicyValidationApplyConfiguration {
	return &BackendTLSPolicyValidationApplyConfiguration{}
}

// WithCACertificateRefs adds the given value to the CACertificateRefs field in the declarative configuration
// and returns the receiver, so that objects can be build by chaining "With" function invocations.
// If called multiple times, values provided by each call will be appended to the CACertificateRefs field.
func (b *BackendTLSPolicyValidationApplyConfiguration) WithCACertificateRefs(values ...*v1.LocalObjectReferenceApplyConfiguration) *BackendTLSPolicyValidationApplyConfiguration {
	for i := range values {
		if values[i] == nil {
			panic("nil value passed to WithCACertificateRefs")
		}
		b.CACertificateRefs = append(b.CACertificateRefs, *values[i])
	}
	return b
}

// WithWellKnownCACertificates sets the WellKnownCACertificates field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the WellKnownCACertificates field is set to the value of the last call.
func (b *BackendTLSPolicyValidationApplyConfiguration) WithWellKnownCACertificates(value v1alpha3.WellKnownCACertificatesType) *BackendTLSPolicyValidationApplyConfiguration {
	b.WellKnownCACertificates = &value
	return b
}

// WithHostname sets the Hostname field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the Hostname field is set to the value of the last call.
func (b *BackendTLSPolicyValidationApplyConfiguration) WithHostname(value apisv1.PreciseHostname) *BackendTLSPolicyValidationApplyConfiguration {
	b.Hostname = &value
	return b
}

// WithSubjectAltNames adds the given value to the SubjectAltNames field in the declarative configuration
// and returns the receiver, so that objects can be build by chaining "With" function invocations.
// If called multiple times, values provided by each call will be appended to the SubjectAltNames field.
func (b *BackendTLSPolicyValidationApplyConfiguration) WithSubjectAltNames(values ...*SubjectAltNameApplyConfiguration) *BackendTLSPolicyValidationApplyConfiguration {
	for i := range values {
		if values[i] == nil {
			panic("nil value passed to WithSubjectAltNames")
		}
		b.SubjectAltNames = append(b.SubjectAltNames, *values[i])
	}
	return b
}
