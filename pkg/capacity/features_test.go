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

package capacity

import (
	"reflect"
	"testing"
)

func TestFeatures(t *testing.T) {
	tests := []struct {
		name           string
		input          []string
		expectedOutput Features
		expectedError  string
	}{
		{
			name: "empty",
		},
		{
			name:           "central",
			input:          []string{string(FeatureCentral)},
			expectedOutput: Features{FeatureCentral: true},
		},
		{
			name:          "local",
			input:         []string{string(FeatureLocal)},
			expectedError: string(FeatureLocal) + ": not implemented yet",
		},
		{
			name:          "invalid",
			input:         []string{"no-such-feature"},
			expectedError: "no-such-feature: unknown feature",
		},
		{
			name:           "multi",
			input:          []string{string(FeatureCentral), string(FeatureCentral)},
			expectedOutput: Features{FeatureCentral: true},
		},
		{
			name:           "comma",
			input:          []string{string(FeatureCentral) + "  ," + string(FeatureCentral) + "  "},
			expectedOutput: Features{FeatureCentral: true},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var actual Features
			for _, value := range test.input {
				err := actual.Set(value)
				if err != nil && test.expectedError != "" {
					if err.Error() == test.expectedError {
						return
					}
					t.Fatalf("expected error %q, got %v", test.expectedError, err)
				}
				if err == nil && test.expectedError != "" {
					t.Fatalf("expected error %q, got no error", test.expectedError)
				}
			}
			if !reflect.DeepEqual(actual, test.expectedOutput) {
				t.Fatalf("expected %v, got %v", test.expectedOutput, actual)
			}
		})
	}
}
