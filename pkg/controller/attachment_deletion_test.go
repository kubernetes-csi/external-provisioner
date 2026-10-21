/*
Copyright 2026 The Kubernetes Authors.

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

package controller

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"go.uber.org/mock/gomock"
	v1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	storagelisters "k8s.io/client-go/listers/storage/v1"
	"k8s.io/client-go/tools/cache"
	csitrans "k8s.io/csi-translation-lib"
)

// A non-publishing driver may still use VolumeAttachments via the trivial
// attacher. Its volume must not be deleted until the last attachment is gone.
func TestDeleteWithoutPublishCapabilityWaitsForAttachments(t *testing.T) {
	mockController, driver, _, server, conn, err := createMockServer(t, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer mockController.Finish()
	defer driver.Stop()

	pluginCaps, controllerCaps := provisionCapabilities()
	if controllerCaps[csi.ControllerServiceCapability_RPC_PUBLISH_UNPUBLISH_VOLUME] {
		t.Fatal("test requires a driver without controller publishing")
	}
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	p := &csiProvisioner{
		csiClient:              csi.NewControllerClient(conn.conn),
		timeout:                5 * time.Second,
		translator:             csitrans.New(),
		pluginCapabilities:     pluginCaps,
		controllerCapabilities: controllerCaps,
		vaLister:               storagelisters.NewVolumeAttachmentLister(indexer),
	}
	pv := &v1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{Name: "backup-clone"},
		Spec: v1.PersistentVolumeSpec{PersistentVolumeSource: v1.PersistentVolumeSource{
			CSI: &v1.CSIPersistentVolumeSource{Driver: driverName, VolumeHandle: "clone-handle"},
		}},
	}
	attachment := &storagev1.VolumeAttachment{
		ObjectMeta: metav1.ObjectMeta{Name: "clone-attachment"},
		Spec: storagev1.VolumeAttachmentSpec{
			Attacher: driverName,
			NodeName: "node-1",
			Source:   storagev1.VolumeAttachmentSource{PersistentVolumeName: &pv.Name},
		},
		Status: storagev1.VolumeAttachmentStatus{Attached: true},
	}

	// No DeleteVolume RPC is expected during any of these states.
	for _, state := range []string{"attached", "not attached", "deleting"} {
		t.Run(state, func(t *testing.T) {
			va := attachment.DeepCopy()
			if state != "attached" {
				va.Status.Attached = false
			}
			if state == "deleting" {
				now := metav1.Now()
				va.DeletionTimestamp = &now
			}
			if err := indexer.Update(va); err != nil {
				t.Fatal(err)
			}
			if err := p.Delete(context.Background(), pv); err == nil || !strings.Contains(err.Error(), "is still attached") {
				t.Fatalf("expected attachment guard error, got %v", err)
			}
		})
	}

	// A second node still protects the volume after the first attachment goes.
	second := attachment.DeepCopy()
	second.Name = "second-attachment"
	second.Spec.NodeName = "node-2"
	if err := indexer.Add(second); err != nil {
		t.Fatal(err)
	}
	if err := indexer.Delete(attachment); err != nil {
		t.Fatal(err)
	}
	if err := p.Delete(context.Background(), pv); err == nil || !strings.Contains(err.Error(), "node-2") {
		t.Fatalf("expected remaining attachment to block deletion, got %v", err)
	}
	if err := indexer.Delete(second); err != nil {
		t.Fatal(err)
	}

	// An unrelated attachment must not prevent reclamation of this volume.
	unrelated := attachment.DeepCopy()
	unrelated.Name = "unrelated-attachment"
	otherPV := "other-volume"
	unrelated.Spec.Source.PersistentVolumeName = &otherPV
	if err := indexer.Add(unrelated); err != nil {
		t.Fatal(err)
	}
	server.EXPECT().DeleteVolume(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, request *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
			if request.VolumeId != "clone-handle" {
				t.Errorf("unexpected volume ID: %q", request.VolumeId)
			}
			return &csi.DeleteVolumeResponse{}, nil
		}).Times(1)
	if err := p.Delete(context.Background(), pv); err != nil {
		t.Fatalf("deletion after the last attachment was removed failed: %v", err)
	}
}
