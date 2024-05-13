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

import (
	"k8s.io/apiserver/pkg/util/feature"
	"k8s.io/component-base/featuregate"
)

const (
	// owner: @verult
	// alpha: v0.4
	// beta: v1.2
	Topology featuregate.Feature = "Topology"

	// owner: @deepakkinni @xing-yang
	// kep: http://kep.k8s.io/2680
	// alpha: v1.23
	// beta: v1.31
	//
	// Honor Persistent Volume Reclaim Policy when it is "Delete" irrespective of PV-PVC
	// deletion ordering.
	HonorPVReclaimPolicy featuregate.Feature = "HonorPVReclaimPolicy"

	// owner: @ttakahashi21 @mkimuram
	// kep: http://kep.k8s.io/3294
	// alpha: v1.26
	//
	// Enable usage of Provision of PVCs from snapshots in other namespaces
	CrossNamespaceVolumeDataSource featuregate.Feature = "CrossNamespaceVolumeDataSource"

	// owner: @sunnylovestiramisu @ConnorJC3
	// kep: https://kep.k8s.io/3751
	// alpha: v1.29
	//
	// Pass VolumeAttributesClass parameters to supporting CSI drivers during CreateVolume
	VolumeAttributesClass featuregate.Feature = "VolumeAttributesClass"
)

func init() {
	feature.DefaultMutableFeatureGate.Add(defaultKubernetesFeatureGates)
}

// defaultKubernetesFeatureGates consists of all known feature keys specific to external-provisioner.
// To add a new feature, define a key for it above and add it here.
var defaultKubernetesFeatureGates = map[featuregate.Feature]featuregate.FeatureSpec{
	Topology:                       {Default: false, PreRelease: featuregate.GA},
	HonorPVReclaimPolicy:           {Default: true, PreRelease: featuregate.Beta},
	CrossNamespaceVolumeDataSource: {Default: false, PreRelease: featuregate.Alpha},
	VolumeAttributesClass:          {Default: false, PreRelease: featuregate.Alpha},
}
