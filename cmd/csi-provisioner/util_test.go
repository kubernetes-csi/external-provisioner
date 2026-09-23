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
	"time"

	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
)

func TestVolumeAttachmentLister(t *testing.T) {
	for _, tc := range []struct {
		name            string
		supportsPublish bool
		watch           bool
		wantLister      bool
	}{
		{"default without controller publishing", false, false, false},
		{"opt in without controller publishing", false, true, true},
		{"publishing cannot be opted out", true, false, true},
		{"publishing with explicit watch", true, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := fake.NewSimpleClientset(&storagev1.VolumeAttachment{ObjectMeta: metav1.ObjectMeta{Name: "existing-attachment"}})
			factory := informers.NewSharedInformerFactory(client, 0)
			lister := volumeAttachmentLister(factory, tc.supportsPublish, tc.watch)
			if (lister != nil) != tc.wantLister {
				t.Fatalf("lister present = %t, want %t", lister != nil, tc.wantLister)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			factory.Start(ctx.Done())
			for _, synced := range factory.WaitForCacheSync(ctx.Done()) {
				if !synced {
					t.Fatal("attachment informer failed to sync")
				}
			}
			if lister == nil {
				if len(client.Actions()) != 0 {
					t.Fatalf("default non-publishing driver made API requests: %v", client.Actions())
				}
				return
			}
			attachments, err := lister.List(labels.Everything())
			if err != nil || len(attachments) != 1 || attachments[0].Name != "existing-attachment" {
				t.Fatalf("existing attachment was not loaded at startup: %v, %v", attachments, err)
			}
		})
	}
}

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
