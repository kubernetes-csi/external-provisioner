/*
Copyright 2022 The Kubernetes Authors.

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

package main

import (
	"fmt"
	"hash/fnv"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"

	"github.com/kubernetes-csi/external-provisioner/v6/pkg/capacity"
)

// nodeDeploymentPVLabels returns the labels that identify the PVs created by
// the external-provisioner instance with the given managed-by ID. They are the
// same labels that identify the CSIStorageCapacity objects of that instance.
func nodeDeploymentPVLabels(driverName, managedByID string) labels.Set {
	return labels.Set{
		capacity.DriverNameLabel: driverName,
		capacity.ManagedByLabel:  managedByID,
	}
}

// newLabelFilteredInformerFactory returns an informer factory whose informers
// only list and watch objects with all of the given labels.
func newLabelFilteredInformerFactory(client kubernetes.Interface, resync time.Duration, set labels.Set) informers.SharedInformerFactory {
	return informers.NewSharedInformerFactoryWithOptions(client,
		resync,
		informers.WithTweakListOptions(func(lo *metav1.ListOptions) {
			lo.LabelSelector = set.AsSelector().String()
		}),
	)
}

// getNameWithMaxLength returns a name given a base ("deployment-5") and a suffix ("deploy")
// It will first attempt to join them with a dash. If the resulting name is longer
// than maxLength: if the suffix is too long, it will truncate the base name and add
// an 8-character hash of the [base]-[suffix] string.  If the suffix is not too long,
// it will truncate the base, add the hash of the base and return [base]-[hash]-[suffix]
// copied from https://github.com/openshift/library-go/blob/master/pkg/build/naming/namer.go
func getNameWithMaxLength(base, suffix string, maxLength int) string {
	name := suffix
	if base != "" {
		name = fmt.Sprintf("%s-%s", base, suffix)
	}

	if len(name) <= maxLength {
		return name
	}

	baseLength := maxLength - 10 /*length of -hash-*/ - len(suffix)

	// if the suffix is too long, ignore it
	if baseLength <= 0 {
		shortName := hash(name)
		prefix := base[0:min(len(base), max(0, maxLength-9))]
		if prefix != "" {
			// Calculate hash on initial base-suffix string
			shortName = fmt.Sprintf("%s-%s", prefix, hash(name))
		}
		return shortName[:min(maxLength, len(shortName))]
	}

	prefix := base[0:baseLength]
	// Calculate hash on initial base-suffix string
	return fmt.Sprintf("%s-%s-%s", prefix, hash(base), suffix)
}

// hash calculates the hexadecimal representation (8-chars)
// of the hash of the passed in string using the FNV-a algorithm
func hash(s string) string {
	hash := fnv.New32a()
	hash.Write([]byte(s))
	intHash := hash.Sum32()
	result := fmt.Sprintf("%08x", intHash)
	return result
}

// max returns the greater of its 2 inputs
func max(a, b int) int {
	if b > a {
		return b
	}
	return a
}

// min returns the lesser of its 2 inputs
func min(a, b int) int {
	if b < a {
		return b
	}
	return a
}
