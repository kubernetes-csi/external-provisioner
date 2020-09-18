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
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	popv1alpha1 "github.com/kubernetes-csi/external-provisioner/client/apis/volumepopulator/v1alpha1"
	volumesnapshotv1beta1 "github.com/kubernetes-csi/external-snapshotter/client/v2/apis/volumesnapshot/v1beta1"
	v1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
)

type fakeVolumeLister struct {
	populators []*popv1alpha1.VolumePopulator
}

func (l *fakeVolumeLister) List(labels.Selector) ([]*popv1alpha1.VolumePopulator, error) {
	return l.populators, nil
}

func (l *fakeVolumeLister) Get(name string) (*popv1alpha1.VolumePopulator, error) {
	for _, populator := range l.populators {
		if populator.Name == name {
			return populator, nil
		}
	}
	return nil, errors.New("not found")
}

func TestAdmitVolumeSnapshot(t *testing.T) {
	pvcname := "pvcname1"
	accessMode := corev1.ReadWriteOnce
	storageClassName := "storage-class-1"
	snapshotApiGroup := volumesnapshotv1beta1.GroupName
	invalidApiGroup := "invalid.storage.k8s.io"
	validApiGroup := "valid.storage.k8s.io"
	resources := corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceStorage: *resource.NewQuantity(1<<30, resource.BinarySI),
		},
	}

	populator := popv1alpha1.VolumePopulator{
		TypeMeta: metav1.TypeMeta{
			Kind:       "VolumePopulator",
			APIVersion: "populator.storage.k8s.io",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "valid",
		},
		SourceKind: metav1.GroupKind{
			Group: validApiGroup,
			Kind:  "Valid",
		},
	}
	popLister = &fakeVolumeLister{
		populators: []*popv1alpha1.VolumePopulator{&populator},
	}

	testCases := []struct {
		name        string
		pvc         *corev1.PersistentVolumeClaim
		shouldAdmit bool
		msg         string
	}{
		{
			name: "Create: no data source",
			pvc: &corev1.PersistentVolumeClaim{
				Spec: corev1.PersistentVolumeClaimSpec{
					VolumeName:       pvcname,
					StorageClassName: &storageClassName,
					AccessModes:      []corev1.PersistentVolumeAccessMode{accessMode},
					Resources:        resources,
					DataSource:       nil,
				},
			},
			shouldAdmit: true,
		},
		{
			name: "Create: PVC data source",
			pvc: &corev1.PersistentVolumeClaim{
				Spec: corev1.PersistentVolumeClaimSpec{
					VolumeName:       pvcname,
					StorageClassName: &storageClassName,
					AccessModes:      []corev1.PersistentVolumeAccessMode{accessMode},
					Resources:        resources,
					DataSource: &corev1.TypedLocalObjectReference{
						APIGroup: nil,
						Kind:     "PersistentVolumeClaim",
						Name:     "pvcname2",
					},
				},
			},
			shouldAdmit: true,
		},
		{
			name: "Create: snapshot data source",
			pvc: &corev1.PersistentVolumeClaim{
				Spec: corev1.PersistentVolumeClaimSpec{
					VolumeName:       pvcname,
					StorageClassName: &storageClassName,
					AccessModes:      []corev1.PersistentVolumeAccessMode{accessMode},
					Resources:        resources,
					DataSource: &corev1.TypedLocalObjectReference{
						APIGroup: &snapshotApiGroup,
						Kind:     "VolumeSnapshot",
						Name:     "snap1",
					},
				},
			},
			shouldAdmit: true,
		},
		{
			name: "Create: valid data source",
			pvc: &corev1.PersistentVolumeClaim{
				Spec: corev1.PersistentVolumeClaimSpec{
					VolumeName:       pvcname,
					StorageClassName: &storageClassName,
					AccessModes:      []corev1.PersistentVolumeAccessMode{accessMode},
					Resources:        resources,
					DataSource: &corev1.TypedLocalObjectReference{
						APIGroup: &validApiGroup,
						Kind:     "Valid",
						Name:     "valid1",
					},
				},
			},
			shouldAdmit: true,
		},
		{
			name: "Create: invalid data source",
			pvc: &corev1.PersistentVolumeClaim{
				Spec: corev1.PersistentVolumeClaimSpec{
					VolumeName:       pvcname,
					StorageClassName: &storageClassName,
					AccessModes:      []corev1.PersistentVolumeAccessMode{accessMode},
					Resources:        resources,
					DataSource: &corev1.TypedLocalObjectReference{
						APIGroup: &invalidApiGroup,
						Kind:     "Invalid",
						Name:     "invalid1",
					},
				},
			},
			shouldAdmit: false,
			msg:         fmt.Sprintf("unsupported data source: %s.%s", "Invalid", invalidApiGroup),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			pvc := tc.pvc
			raw, err := json.Marshal(pvc)
			if err != nil {
				t.Fatal(err)
			}
			review := v1.AdmissionReview{
				Request: &v1.AdmissionRequest{
					Object: runtime.RawExtension{
						Raw: raw,
					},
					Resource:  PVCV1GVR,
					Operation: v1.Create,
				},
			}
			response := admitPVC(review)
			shouldAdmit := response.Allowed
			msg := response.Result.Message

			expectedResponse := tc.shouldAdmit
			expectedMsg := tc.msg

			if shouldAdmit != expectedResponse {
				t.Errorf("expected \"%v\" to equal \"%v\"", shouldAdmit, expectedResponse)
			}
			if msg != expectedMsg {
				t.Errorf("expected \"%v\" to equal \"%v\"", msg, expectedMsg)
			}
		})
	}
}
