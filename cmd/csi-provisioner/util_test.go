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
	"context"
	"fmt"
	"strings"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/validation"
	fakeclientset "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

const (
	externalProvisioner = "external-provisioner"
)

func TestGetNameWithMaxLength(t *testing.T) {
	testcases := map[string]struct {
		expected string
		nodeName string
		base     string
	}{
		"unchanged": {
			expected: fmt.Sprintf("%s-%s", externalProvisioner, "node01"),
			nodeName: "node01",
			base:     externalProvisioner,
		},
		"with empty base": {
			expected: "node01",
			nodeName: "node01",
			base:     "",
		},
		"exactly63": {
			// 20 (external-provisioner) + 1 (-) + 4 (node) + 38 = 63
			expected: fmt.Sprintf("%s-%s", externalProvisioner, fmt.Sprintf("node%s", strings.Repeat("a", 38))),
			nodeName: fmt.Sprintf("node%s", strings.Repeat("a", 38)),
			base:     externalProvisioner,
		},
		"one over": {
			expected: fmt.Sprintf("%s-%s-%s", "external-p", "53f40b57", fmt.Sprintf("node%s", strings.Repeat("a", 39))),
			nodeName: fmt.Sprintf("node%s", strings.Repeat("a", 39)),
			base:     externalProvisioner,
		},
		"long node name with 52": {
			expected: fmt.Sprintf("%s-%s-%s", "e", "53f40b57", fmt.Sprintf("node%s", strings.Repeat("a", 48))),
			nodeName: fmt.Sprintf("node%s", strings.Repeat("a", 48)),
			base:     externalProvisioner,
		},

		"long node name with 53, ignore suffix ": {
			expected: fmt.Sprintf("%s-%s", externalProvisioner, "a3607ff1"),
			nodeName: fmt.Sprintf("node%s", strings.Repeat("a", 49)),
			base:     externalProvisioner,
		},
		"very long, ignore suffix": {
			expected: fmt.Sprintf("%s-%s", externalProvisioner, "df38e37f"),
			nodeName: fmt.Sprintf("node%s", strings.Repeat("a", 63)),
			base:     externalProvisioner,
		},
		"long node name ,with empty base": {
			expected: "fa65587e",
			nodeName: fmt.Sprintf("node%s", strings.Repeat("a", 63)),
			base:     "",
		},
	}

	for name, c := range testcases {
		t.Run(name, func(t *testing.T) {
			expected := c.expected
			res := getNameWithMaxLength(c.base, c.nodeName, 63)
			if expected != res {
				t.Errorf("Expected: %s, does not match result: %s", expected, res)
			}
		})
	}
}

func TestNodeDeploymentPVLabels(t *testing.T) {
	testcases := map[string]struct {
		nodeName string
		expected labels.Set
	}{
		"short node name": {
			nodeName: "node01",
			expected: labels.Set{
				"csi.storage.k8s.io/drivername": "hostpath.csi.k8s.io",
				"csi.storage.k8s.io/managed-by": "external-provisioner-node01",
			},
		},
		"long node name": {
			nodeName: fmt.Sprintf("node%s", strings.Repeat("a", 63)),
			expected: labels.Set{
				"csi.storage.k8s.io/drivername": "hostpath.csi.k8s.io",
				"csi.storage.k8s.io/managed-by": fmt.Sprintf("%s-%s", externalProvisioner, "df38e37f"),
			},
		},
	}

	for name, c := range testcases {
		t.Run(name, func(t *testing.T) {
			managedByID := getNameWithMaxLength(externalProvisioner, c.nodeName, validation.DNS1035LabelMaxLength)
			res := nodeDeploymentPVLabels("hostpath.csi.k8s.io", managedByID)
			if !labels.Equals(res, c.expected) {
				t.Errorf("Expected: %v, does not match result: %v", c.expected, res)
			}
			for key, value := range res {
				if errs := validation.IsValidLabelValue(value); len(errs) > 0 {
					t.Errorf("Invalid value %q for label %q: %v", value, key, errs)
				}
			}
		})
	}
}

func TestNewLabelFilteredInformerFactory(t *testing.T) {
	pvLabels := nodeDeploymentPVLabels("hostpath.csi.k8s.io", "external-provisioner-node01")
	newPV := func(name string, objLabels map[string]string) *v1.PersistentVolume {
		return &v1.PersistentVolume{
			ObjectMeta: metav1.ObjectMeta{
				Name:   name,
				Labels: objLabels,
			},
		}
	}
	client := fakeclientset.NewSimpleClientset(
		newPV("own", pvLabels),
		newPV("other-node", nodeDeploymentPVLabels("hostpath.csi.k8s.io", "external-provisioner-node02")),
		newPV("other-driver", nodeDeploymentPVLabels("other.csi.k8s.io", "external-provisioner-node01")),
		newPV("unlabeled", nil),
	)

	factory := newLabelFilteredInformerFactory(client, 0, pvLabels)
	pvInformer := factory.Core().V1().PersistentVolumes()
	pvLister := pvInformer.Lister()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	factory.Start(ctx.Done())
	for informerType, synced := range factory.WaitForCacheSync(ctx.Done()) {
		if !synced {
			t.Fatalf("Informer %v did not sync", informerType)
		}
	}

	pvs, err := pvLister.List(labels.Everything())
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(pvs) != 1 || pvs[0].Name != "own" {
		t.Errorf("Expected only PV %q, got: %v", "own", pvs)
	}

	expectedSelector := "csi.storage.k8s.io/drivername=hostpath.csi.k8s.io,csi.storage.k8s.io/managed-by=external-provisioner-node01"
	listed := false
	for _, action := range client.Actions() {
		if !action.Matches("list", "persistentvolumes") {
			continue
		}
		listed = true
		selector := action.(k8stesting.ListAction).GetListRestrictions().Labels.String()
		if selector != expectedSelector {
			t.Errorf("Expected label selector %q, got: %q", expectedSelector, selector)
		}
	}
	if !listed {
		t.Error("PersistentVolumes were not listed")
	}
}
