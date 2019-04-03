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

package features

import utilfeature "k8s.io/apiserver/pkg/util/feature"

const (
	// owner: @verult
	// alpha: v0.4
	// beta: v2.0
	Topology utilfeature.Feature = "Topology"
)

func init() {
	utilfeature.DefaultMutableFeatureGate.Add(defaultKubernetesFeatureGates)
}

// defaultKubernetesFeatureGates consists of all known feature keys specific to external-provisioner.
// To add a new feature, define a key for it above and add it here.
var defaultKubernetesFeatureGates = map[utilfeature.Feature]utilfeature.FeatureSpec{
	Topology: {Default: false, PreRelease: utilfeature.Beta},
}
