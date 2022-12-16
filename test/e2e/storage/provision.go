package storage

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	snapshotclientset "github.com/kubernetes-csi/external-snapshotter/client/v6/clientset/versioned"
	v1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/kubernetes/test/e2e/framework"
	e2ekubectl "k8s.io/kubernetes/test/e2e/framework/kubectl"
	e2epv "k8s.io/kubernetes/test/e2e/framework/pv"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"
	"k8s.io/kubernetes/test/e2e/storage/utils"
)

var _ = ginkgo.Describe("provision volumes with different volume modes from volume snapshot dataSource", func() {
	var local struct {
		snapshotClient  *snapshotclientset.Clientset
		sc              *storagev1.StorageClass
		vsc             *snapshotv1.VolumeSnapshotClass
		volumesnapshots []*snapshotv1.VolumeSnapshot
		pvcs            []*v1.PersistentVolumeClaim
	}
	f := framework.NewDefaultFramework("pvcs-from-volume-snapshots")
	ginkgo.BeforeEach(func() {
		var err error
		output, err := e2ekubectl.RunKubectl("kube-system", "describe", "deployment", "snapshot-controller")
		if err != nil || !strings.Contains(output, "prevent-volume-mode-conversion=true") {
			e2eskipper.Skipf("--prevent-volume-mode-conversion feature flag is not enabled in snapshot-controller. Skipping volume mode conversion tests.")
		}
		output, err = e2ekubectl.RunKubectl("default", "describe", "sts", "csi-hostpathplugin")
		if err != nil || !strings.Contains(output, "prevent-volume-mode-conversion=true") {
			e2eskipper.Skipf("--prevent-volume-mode-conversion feature flag is not enabled in csi-hostpathplugin. Skipping volume mode conversion tests.")
		}

		ginkgo.By("Creating CSI Hostpath driver Storage Class")
		local.sc, err = createHostPathStorageClass(f.ClientSet)
		framework.ExpectNoError(err)
		local.snapshotClient, err = snapshotclientset.NewForConfig(f.ClientConfig())
		framework.ExpectNoError(err)
		ginkgo.By("Creating VolumeSnapshotClass")
		local.vsc, err = createVolumeSnapshotClass(local.snapshotClient, snapshotv1.VolumeSnapshotContentDelete)
		framework.ExpectNoError(err)
	})
	ginkgo.AfterEach(func() {
		var errs []error
		if local.snapshotClient != nil && local.vsc != nil {
			ginkgo.By("Deleting VolumeSnapshotClass")
			errs = append(errs, local.snapshotClient.SnapshotV1().VolumeSnapshotClasses().Delete(context.TODO(), local.vsc.Name, metav1.DeleteOptions{}))
		}
		if local.sc != nil {
			ginkgo.By("Deleting CSI Hostpath driver Storage Class")
			errs = append(errs, f.ClientSet.StorageV1().StorageClasses().Delete(context.TODO(), local.sc.Name, metav1.DeleteOptions{}))
		}
		for _, claim := range local.pvcs {
			ginkgo.By(fmt.Sprintf("Deleting PersistentVolumeClaim %s/%s", claim.Namespace, claim.Name))
			claim, err := f.ClientSet.CoreV1().PersistentVolumeClaims(claim.Namespace).Get(context.TODO(), claim.Name, metav1.GetOptions{})
			if err == nil {
				errs = append(errs, e2epv.DeletePersistentVolumeClaim(f.ClientSet, claim.Name, claim.Namespace))
				if claim.Spec.VolumeName != "" {
					errs = append(errs, e2epv.WaitForPersistentVolumeDeleted(f.ClientSet, claim.Spec.VolumeName, framework.Poll, 2*time.Minute))
				}
			}
		}
		for _, volumeSnapshot := range local.volumesnapshots {
			ginkgo.By(fmt.Sprintf("Deleting VolumeSnapshot %s/%s", volumeSnapshot.Namespace, volumeSnapshot.Name))
			vs, err := local.snapshotClient.SnapshotV1().VolumeSnapshots(volumeSnapshot.Namespace).Get(context.TODO(), volumeSnapshot.Name, metav1.GetOptions{})
			if err == nil {
				errs = append(errs, utils.DeleteAndWaitSnapshot(f.DynamicClient, vs.Namespace, vs.Name, framework.Poll, framework.SnapshotDeleteTimeout))
				if vs.Status != nil && vs.Status.BoundVolumeSnapshotContentName != nil {
					volumeSnapshotContentName := vs.Status.BoundVolumeSnapshotContentName
					ginkgo.By(fmt.Sprintf("Wait for VolumeSnapshotContent %s to be deleted", *volumeSnapshotContentName))
					errs = append(errs, utils.WaitForGVRDeletion(f.DynamicClient, utils.SnapshotContentGVR, *volumeSnapshotContentName, framework.Poll, framework.SnapshotDeleteTimeout))
				}
			}
		}
		err := utilerrors.NewAggregate(errs)
		framework.ExpectNoError(err, "while cleaning up after test")

	})

	// Attempt to create a PVC that alters the volume mode of the source volume snapshot.
	// The PVC is expected to remain in the Pending phase
	ginkgo.It("when the source volume mode is altered without permissions", func() {
		fs := v1.PersistentVolumeFilesystem
		pvcConfig := e2epv.PersistentVolumeClaimConfig{
			ClaimSize:        "1Mi",
			StorageClassName: &local.sc.Name,
			VolumeMode:       &fs,
		}
		pvc, err := e2epv.CreatePVC(f.ClientSet, f.Namespace.Name, e2epv.MakePersistentVolumeClaim(pvcConfig, f.Namespace.Name))
		framework.ExpectNoError(err)
		_, err = e2epv.WaitForPVClaimBoundPhase(f.ClientSet, []*v1.PersistentVolumeClaim{pvc}, framework.ClaimProvisionShortTimeout)
		framework.ExpectNoError(err)

		local.pvcs = append(local.pvcs, pvc)

		vs, err := local.snapshotClient.SnapshotV1().VolumeSnapshots(f.Namespace.Name).Create(context.TODO(), getVolumeSnapshotSpec(f.Namespace.Name, local.vsc.Name, pvc.Name), metav1.CreateOptions{})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		err = utils.WaitForSnapshotReady(f.DynamicClient, vs.Namespace, vs.Name, time.Second, time.Minute)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		local.volumesnapshots = append(local.volumesnapshots, vs)

		block := v1.PersistentVolumeBlock
		pvcConfig = e2epv.PersistentVolumeClaimConfig{
			ClaimSize:        "1Mi",
			StorageClassName: &local.sc.Name,
			VolumeMode:       &block,
		}

		pvc2 := e2epv.MakePersistentVolumeClaim(pvcConfig, f.Namespace.Name)
		snapshotGroup := "snapshot.storage.k8s.io"
		pvc2.Spec.DataSource = &v1.TypedLocalObjectReference{
			APIGroup: &snapshotGroup,
			Kind:     "VolumeSnapshot",
			Name:     vs.Name,
		}
		pvc2, err = e2epv.CreatePVC(f.ClientSet, f.Namespace.Name, pvc2)
		framework.ExpectNoError(err)
		_, err = e2epv.WaitForPVClaimBoundPhase(f.ClientSet, []*v1.PersistentVolumeClaim{pvc2}, framework.ClaimProvisionShortTimeout)
		framework.ExpectError(err)

		local.pvcs = append(local.pvcs, pvc2)

		isFailureFound := false
		eventList, _ := f.ClientSet.CoreV1().Events(f.Namespace.Name).List(context.TODO(), metav1.ListOptions{FieldSelector: fmt.Sprintf("involvedObject.name=%s", pvc2.Name)})
		for _, item := range eventList.Items {
			if strings.Contains(item.Message, volumeModeConversionFailureMessage) {
				isFailureFound = true
				break
			}
		}
		if !isFailureFound {
			framework.Failf("expected failure message [%s] not parsed in event list for PVC %s/%s", volumeModeConversionFailureMessage, pvc2.Namespace, pvc2.Name)
		}
	})
})
