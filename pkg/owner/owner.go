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

// Package owner contains code for walking up the ownership chain,
// starting with an arbitrary object. RBAC rules must allow GET access
// to each object on the chain, at least including the starting
// object, more when walking up more than one level.
package owner

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Lookup walks up the ownership chain zero or more levels and returns an OwnerReference for the
// object. The object identified by name, namespace and type is the starting point and is
// returned when levels is zero. Only APIVersion, Kind, Name, and UID will be set.
// IsController is always true.
func Lookup(config *rest.Config, namespace, name string, gkv schema.GroupVersionKind, levels int) (*metav1.OwnerReference, error) {
	c, err := client.New(config, client.Options{})
	if err != nil {
		return nil, fmt.Errorf("build client: %v", err)
	}

	return lookupRecursive(c, namespace, name, gkv.Group, gkv.Version, gkv.Kind, levels)
}

func lookupRecursive(c client.Client, namespace, name, group, version, kind string, levels int) (*metav1.OwnerReference, error) {
	u := &unstructured.Unstructured{}
	apiVersion := metav1.GroupVersion{Group: group, Version: version}.String()
	u.SetAPIVersion(apiVersion)
	u.SetKind(kind)

	if err := c.Get(context.Background(), client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}, u); err != nil {
		return nil, fmt.Errorf("get object: %v", err)
	}

	if levels == 0 {
		isTrue := true
		return &metav1.OwnerReference{
			APIVersion: apiVersion,
			Kind:       kind,
			Name:       name,
			UID:        u.GetUID(),
			Controller: &isTrue,
		}, nil
	}
	owners := u.GetOwnerReferences()
	for _, owner := range owners {
		if owner.Controller != nil && *owner.Controller {
			gv, err := schema.ParseGroupVersion(owner.APIVersion)
			if err != nil {
				return nil, fmt.Errorf("parse OwnerReference.APIVersion: %v", err)
			}
			// With this special case here we avoid one lookup and thus the need for
			// RBAC GET permission for the parent. For example, when a Pod is controlled
			// by a StatefulSet, we only need GET permission for Pods (for the c.Get above)
			// but not for StatefulSets.
			if levels == 1 {
				isTrue := true
				return &metav1.OwnerReference{
					APIVersion: owner.APIVersion,
					Kind:       owner.Kind,
					Name:       owner.Name,
					UID:        owner.UID,
					Controller: &isTrue,
				}, nil
			}

			return lookupRecursive(c, namespace, owner.Name,
				gv.Group, gv.Version, owner.Kind,
				levels-1)
		}
	}
	return nil, fmt.Errorf("%s/%s %q in namespace %q has no controlling owner, cannot unwind the ownership further",
		apiVersion, kind, name, namespace)
}
