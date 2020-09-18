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

package populator_controller

import (
	"errors"
	"testing"

	popv1alpha1 "github.com/kubernetes-csi/external-provisioner/client/apis/volumepopulator/v1alpha1"
	volumesnapshotv1beta1 "github.com/kubernetes-csi/external-snapshotter/client/v3/apis/volumesnapshot/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
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

type brokenVolumeLister struct {
}

func (l *brokenVolumeLister) List(labels.Selector) ([]*popv1alpha1.VolumePopulator, error) {
	return nil, errors.New("failed")
}

func (l *brokenVolumeLister) Get(name string) (*popv1alpha1.VolumePopulator, error) {
	return nil, errors.New("failed")
}

func TestValidateGroupKind(t *testing.T) {
	ctrl := new(populatorController)

	populator := popv1alpha1.VolumePopulator{
		TypeMeta: metav1.TypeMeta{
			Kind:       "VolumePopulator",
			APIVersion: "populator.storage.k8s.io",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "valid",
		},
		SourceKind: metav1.GroupKind{
			Group: "valid.storage.k8s.io",
			Kind:  "Valid",
		},
	}
	ctrl.popLister = &fakeVolumeLister{
		populators: []*popv1alpha1.VolumePopulator{&populator},
	}

	testCases := []struct {
		name  string
		gk    metav1.GroupKind
		valid bool
	}{
		{
			name: "Create: PVC data source",
			gk: metav1.GroupKind{
				Group: "",
				Kind:  "PersistentVolumeClaim",
			},
			valid: true,
		},
		{
			name: "Create: snapshot data source",
			gk: metav1.GroupKind{
				Group: volumesnapshotv1beta1.GroupName,
				Kind:  "VolumeSnapshot",
			},
			valid: true,
		},
		{
			name: "Create: valid data source",
			gk: metav1.GroupKind{
				Group: "valid.storage.k8s.io",
				Kind:  "Valid",
			},
			valid: true,
		},
		{
			name: "Create: invalid data source",
			gk: metav1.GroupKind{
				Group: "invalid.storage.k8s.io",
				Kind:  "Invalid",
			},
			valid: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			valid, err := ctrl.validateGroupKind(tc.gk)
			if valid != tc.valid {
				t.Errorf(`expected "%v" to equal "%v"`, valid, tc.valid)
			}
			if err != nil {
				t.Errorf(`expected nil error, got "%v"`, err)
			}
		})
	}
}

func TestPopListError(t *testing.T) {
	ctrl := new(populatorController)
	ctrl.popLister = new(brokenVolumeLister)

	valid, err := ctrl.validateGroupKind(metav1.GroupKind{
		Group: "valid.storage.k8s.io",
		Kind:  "Valid",
	})
	if valid {
		t.Error("expected invalid")
	}
	if nil == err {
		t.Error("expected error")
	}
	if err.Error() != "failed" {
		t.Errorf(`expected "%v" to equal "failed"`, err)
	}
}
