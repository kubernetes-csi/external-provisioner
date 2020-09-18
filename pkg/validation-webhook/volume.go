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

package webhook

import (
	"fmt"

	popv1alpha1 "github.com/kubernetes-csi/external-provisioner/client/listers/volumepopulator/v1alpha1"
	volumesnapshotv1beta1 "github.com/kubernetes-csi/external-snapshotter/client/v2/apis/volumesnapshot/v1beta1"
	v1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/klog"
)

var (
	PVCV1GVR = metav1.GroupVersionResource{Group: corev1.GroupName, Version: "v1", Resource: "persistentvolumeclaims"}

	PVCGK            = metav1.GroupKind{Group: corev1.GroupName, Kind: "PersistentVolumeClaim"}
	volumeSnapshotGK = metav1.GroupKind{Group: volumesnapshotv1beta1.GroupName, Kind: "VolumeSnapshot"}
)

var popLister popv1alpha1.VolumePopulatorLister

// Add a label {"added-label": "yes"} to the object
func admitPVC(ar v1.AdmissionReview) *v1.AdmissionResponse {
	klog.V(2).Info("admitting PVCs")

	reviewResponse := &v1.AdmissionResponse{
		Allowed: true,
		Result:  &metav1.Status{},
	}

	// Admit requests other than Create
	if ar.Request.Operation != v1.Create {
		return reviewResponse
	}

	if PVCV1GVR == ar.Request.Resource {
		deserializer := codecs.UniversalDeserializer()
		pvc := &corev1.PersistentVolumeClaim{}
		if _, _, err := deserializer.Decode(ar.Request.Object.Raw, nil, pvc); err != nil {
			klog.Error(err)
			return toV1AdmissionResponse(err)
		}
		return decidePVC(pvc)
	} else {
		err := fmt.Errorf("expect resource to be %s", PVCV1GVR)
		klog.Error(err)
		return toV1AdmissionResponse(err)
	}
}

func validateDataSource(gk metav1.GroupKind) error {
	switch gk {
	case PVCGK, volumeSnapshotGK:
		// Cloning PVCs and Volume Snapshots are special cases, allowed by the
		// core, so don't reject these.
		klog.V(4).Infof("Allowing %s as a special case", gk.String())
		return nil
	}
	populators, err := popLister.List(labels.Everything())
	if nil != err {
		klog.Errorf("Failed to list populators: %v", err)
		return err
	}
	for _, populator := range populators {
		if populator.SourceKind == gk {
			klog.V(4).Infof("Allowing %s due to %s populator", gk.String(), populator.Name)
			return nil
		}
	}
	klog.Warningf("No populator matches %s", gk.String())
	return fmt.Errorf("unsupported data source: %s", gk.String())
}

func decidePVC(pvc *corev1.PersistentVolumeClaim) *v1.AdmissionResponse {
	reviewResponse := &v1.AdmissionResponse{
		Allowed: true,
		Result:  &metav1.Status{},
	}

	klog.V(5).Infof("Got PVC %s", pvc.Name)

	dataSource := pvc.Spec.DataSource
	if nil != dataSource && nil != dataSource.APIGroup {
		klog.V(3).Infof("PVC %s datasource is %s.%s", pvc.Name, *dataSource.APIGroup, dataSource.Kind)
		gk := metav1.GroupKind{
			Group: *dataSource.APIGroup,
			Kind:  dataSource.Kind,
		}
		err := validateDataSource(gk)
		if nil != err {
			reviewResponse.Allowed = false
			reviewResponse.Result.Message = err.Error()
		}
	}

	return reviewResponse
}
