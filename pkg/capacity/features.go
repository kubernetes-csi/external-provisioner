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
	"fmt"
	"strings"

	flag "github.com/spf13/pflag"
)

// Feature is the type for named features supported by the capacity
// controller.
type Feature string

// Features are disabled by default.
type Features map[Feature]bool

const (
	// FeatureCentral enables the mode where there is only one
	// external-provisioner actively running in the cluster which
	// talks to the CSI driver's controller.
	FeatureCentral = Feature("central")

	// FeatureLocal enables the mode where external-provisioner
	// is deployed on each node. Not implemented yet.
	FeatureLocal = Feature("local")
)

// Set enables the named features. Multiple features can be listed, separated by commas,
// with optional whitespace.
func (features *Features) Set(value string) error {
	for _, part := range strings.Split(value, ",") {
		part := Feature(strings.TrimSpace(part))
		switch part {
		case FeatureCentral:
			if *features == nil {
				*features = Features{}
			}
			(*features)[part] = true
		case FeatureLocal:
			return fmt.Errorf("%s: not implemented yet", part)
		case "":
		default:
			return fmt.Errorf("%s: unknown feature", part)
		}
	}
	return nil
}

func (features *Features) String() string {
	var parts []string
	for feature := range *features {
		parts = append(parts, string(feature))
	}
	return strings.Join(parts, ",")
}

func (features *Features) Type() string {
	return "enumeration"
}

var _ flag.Value = &Features{}
