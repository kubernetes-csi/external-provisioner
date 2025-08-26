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
	"strings"
	"testing"
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
