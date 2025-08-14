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

package owner

import (
	"fmt"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var (
	testNamespace  = "test-namespace"
	otherNamespace = "other-namespace"
	statefulsetGkv = schema.GroupVersionKind{
		Group:   "apps",
		Version: "v1",
		Kind:    "StatefulSet",
	}
	deploymentGkv = schema.GroupVersionKind{
		Group:   "apps",
		Version: "v1",
		Kind:    "Deployment",
	}
	replicasetGkv = schema.GroupVersionKind{
		Group:   "apps",
		Version: "v1",
		Kind:    "ReplicaSet",
	}
	podGkv = schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "Pod",
	}

	pod                  = makeObject(testNamespace, "foo", podGkv, nil)
	statefulset          = makeObject(testNamespace, "foo", statefulsetGkv, nil)
	statefulsetPod       = makeObject(testNamespace, "foo", podGkv, &statefulset)
	deployment           = makeObject(testNamespace, "foo", deploymentGkv, nil)
	replicaset           = makeObject(testNamespace, "foo", replicasetGkv, &deployment)
	otherReplicaset      = makeObject(testNamespace, "bar", replicasetGkv, &deployment)
	yetAnotherReplicaset = makeObject(otherNamespace, "foo", replicasetGkv, &deployment)
	deploymentsetPod     = makeObject(testNamespace, "foo", podGkv, &replicaset)
)

// TestNodeTopology checks that node labels are correctly transformed
// into topology segments.
func TestNodeTopology(t *testing.T) {
	testcases := map[string]struct {
		objects     []runtime.Object
		start       unstructured.Unstructured
		levels      int
		expectError bool
		expectOwner unstructured.Unstructured
	}{
		"empty": {
			start:       pod,
			expectError: true,
		},
		"pod-itself": {
			objects:     []runtime.Object{&pod},
			start:       pod,
			levels:      0,
			expectOwner: pod,
		},
		"no-parent": {
			objects:     []runtime.Object{&pod},
			start:       pod,
			levels:      1,
			expectError: true,
		},
		"parent": {
			objects: []runtime.Object{&statefulsetPod},
			start:   statefulsetPod,
			levels:  1,
			// The object doesn't have to exist.
			expectOwner: statefulset,
		},
		"missing-parent": {
			objects:     []runtime.Object{&deploymentsetPod},
			start:       deploymentsetPod,
			levels:      2,
			expectError: true,
		},
		"wrong-parent": {
			objects:     []runtime.Object{&deploymentsetPod, &otherReplicaset},
			start:       deploymentsetPod,
			levels:      2,
			expectError: true,
		},
		"another-wrong-parent": {
			objects:     []runtime.Object{&deploymentsetPod, &yetAnotherReplicaset},
			start:       deploymentsetPod,
			levels:      2,
			expectError: true,
		},
		"grandparent": {
			objects: []runtime.Object{&deploymentsetPod, &replicaset},
			start:   deploymentsetPod,
			levels:  2,
			// The object doesn't have to exist.
			expectOwner: deployment,
		},
	}

	for name, tc := range testcases {
		t.Run(name, func(t *testing.T) {
			c := fake.NewFakeClient(tc.objects...)
			gkv := tc.start.GroupVersionKind()
			ownerRef, err := lookupRecursive(c,
				tc.start.GetNamespace(),
				tc.start.GetName(),
				gkv.Group,
				gkv.Version,
				gkv.Kind,
				tc.levels)
			if err != nil && !tc.expectError {
				t.Fatalf("unexpected error: %v", err)
			}
			if err == nil && tc.expectError {
				t.Fatal("unexpected success")
			}
			if err == nil {
				if ownerRef == nil {
					t.Fatal("unexpected nil owner")
				}
				gkv := tc.expectOwner.GroupVersionKind()
				apiVersion := metav1.GroupVersion{Group: gkv.Group, Version: gkv.Version}.String()
				if ownerRef.APIVersion != apiVersion {
					t.Errorf("expected APIVersion %q, got %q", apiVersion, ownerRef.APIVersion)
				}
				if ownerRef.Kind != gkv.Kind {
					t.Errorf("expected Kind %q, got %q", gkv.Kind, ownerRef.Kind)
				}
				if ownerRef.Name != tc.expectOwner.GetName() {
					t.Errorf("expected Name %q, got %q", tc.expectOwner.GetName(), ownerRef.Name)
				}
				if ownerRef.UID != tc.expectOwner.GetUID() {
					t.Errorf("expected UID %q, got %q", tc.expectOwner.GetUID(), ownerRef.UID)
				}
				if ownerRef.Controller == nil || !*ownerRef.Controller {
					t.Error("Controller field should true")
				}
				if ownerRef.BlockOwnerDeletion != nil && *ownerRef.BlockOwnerDeletion {
					t.Error("BlockOwnerDeletion field should false")
				}
			}
		})
	}
}

var uidCounter int

func makeObject(namespace, name string, gkv schema.GroupVersionKind, owner *unstructured.Unstructured) unstructured.Unstructured {
	u := unstructured.Unstructured{}
	u.SetNamespace(namespace)
	u.SetName(name)
	u.SetGroupVersionKind(gkv)
	uidCounter++
	u.SetUID(types.UID(fmt.Sprintf("FAKE-UID-%d", uidCounter)))
	if owner != nil {
		isTrue := true
		u.SetOwnerReferences([]metav1.OwnerReference{
			{
				APIVersion: owner.GetAPIVersion(),
				Kind:       owner.GetKind(),
				Name:       owner.GetName(),
				UID:        owner.GetUID(),
				Controller: &isTrue,
			},
		})
	}
	return u
}
