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
	"errors"
	"strings"

	flag "github.com/spf13/pflag"
)

// DeploymentMode determines how the capacity controller operates.
type DeploymentMode string

const (
	// DeploymentModeCentral enables the mode where there is only one
	// external-provisioner actively running in the cluster which
	// talks to the CSI driver's controller service.
	DeploymentModeCentral = DeploymentMode("central")

	// DeploymentModeLocal enables the mode where external-provisioner
	// is deployed on each node. Not implemented yet.
	DeploymentModeLocal = DeploymentMode("local")

	// DeploymentModeUnset disables the capacity feature completely.
	DeploymentModeUnset = DeploymentMode("")
)

// Set enables the named features. Multiple features can be listed, separated by commas,
// with optional whitespace.
func (mode *DeploymentMode) Set(value string) error {
	switch DeploymentMode(value) {
	case DeploymentModeCentral, DeploymentModeUnset:
		*mode = DeploymentMode(value)
	default:
		return errors.New("invalid value")
	}
	return nil
}

func (mode *DeploymentMode) String() string {
	return string(*mode)
}

func (mode *DeploymentMode) Type() string {
	return strings.Join([]string{string(DeploymentModeCentral) /*, string(DeploymentModeLocal) */}, "|")
}

var _ flag.Value = new(DeploymentMode)
