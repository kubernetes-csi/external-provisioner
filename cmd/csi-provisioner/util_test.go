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
	}{
		"unchanged": {
			expected: fmt.Sprintf("%s-%s", externalProvisioner, "node01"),
			nodeName: "node01",
		},
		"exactly63": {
			// 20 (external-provisioner) + 1 (-) + 4 (node) + 38 = 63
			expected: fmt.Sprintf("%s-%s", externalProvisioner, fmt.Sprintf("node%s", strings.Repeat("a", 38))),
			nodeName: fmt.Sprintf("node%s", strings.Repeat("a", 38)),
		},
		"one over": {
			expected: fmt.Sprintf("%s-%s-%s", "external-p", "53f40b57", fmt.Sprintf("node%s", strings.Repeat("a", 39))),
			nodeName: fmt.Sprintf("node%s", strings.Repeat("a", 39)),
		},
		"very long, ignore suffix": {
			expected: fmt.Sprintf("%s-%s", externalProvisioner, "df38e37f"),
			nodeName: fmt.Sprintf("node%s", strings.Repeat("a", 63)),
		},
	}

	for name, c := range testcases {
		t.Run(name, func(t *testing.T) {
			expected := c.expected
			res := getNameWithMaxLength(externalProvisioner, c.nodeName, 63)
			if expected != res {
				t.Errorf("Expected: %s, does not match result: %s", expected, res)
			}
		})
	}
}
