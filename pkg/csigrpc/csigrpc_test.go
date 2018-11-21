/*
Copyright 2018 The Kubernetes Authors.

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

package csigrpc

import (
	"fmt"
	"testing"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/stretchr/testify/assert"
)

func TestStripSecrets(t *testing.T) {
	secretName := "secret-abc"
	secretValue := "123"
	createVolume := &csi.CreateVolumeRequest{
		Name: "foo",
		VolumeCapabilities: []*csi.VolumeCapability{
			&csi.VolumeCapability{
				AccessType: &csi.VolumeCapability_Mount{
					Mount: &csi.VolumeCapability_MountVolume{
						FsType: "ext4",
					},
				},
			},
		},
		CapacityRange: &csi.CapacityRange{
			RequiredBytes: 1024,
		},
		Secrets: map[string]string{
			secretName:   secretValue,
			"secret-xyz": "987",
		},
	}

	cases := []struct {
		original, stripped interface{}
	}{
		{nil, "null"},
		{1, "1"},
		{"hello world", `"hello world"`},
		{true, "true"},
		{false, "false"},
		{createVolume, `{"capacity_range":{"required_bytes":1024},"name":"foo","secrets":"***stripped***","volume_capabilities":[{"AccessType":{"Mount":{"fs_type":"ext4"}}}]}`},

		// There is currently no test case that can verify
		// that recursive stripping works, because there is no
		// message where that is necessary. The code
		// nevertheless implements it and it has been verified
		// manually that it recurses properly for single and
		// repeated values. One-of might require further work.
	}

	for _, c := range cases {
		original := c.original
		stripped := StripSecrets(original)
		assert.Equal(t, c.stripped, fmt.Sprintf("%s", stripped), "unexpected result for fmt s")
		assert.Equal(t, c.stripped, fmt.Sprintf("%v", stripped), "unexpected result for fmt v")
		assert.Equal(t, original, c.original, "original value modified")
	}

	// The secret is hidden because StripSecrets is a struct referencing it.
	dump := fmt.Sprintf("%#v", StripSecrets(createVolume))
	assert.NotContains(t, secretName, dump)
	assert.NotContains(t, secretValue, dump)
}
