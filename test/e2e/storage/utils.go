package storage

import (
	"context"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v8/apis/volumesnapshot/v1"
	snapshotclientset "github.com/kubernetes-csi/external-snapshotter/client/v8/clientset/versioned"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	hostpathDriver                     = "hostpath.csi.k8s.io"
	volumeModeConversionFailureMessage = "modifies the mode of the source volume but does not have permission to do so"
)

func createHostPathStorageClass(k8sclient kubernetes.Interface) (*storagev1.StorageClass, error) {
	sc := &storagev1.StorageClass{
		TypeMeta: metav1.TypeMeta{
			Kind: "StorageClass",
		},
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "hostpath-sc-",
		},
		Provisioner: hostpathDriver,
	}

	sc, err := k8sclient.StorageV1().StorageClasses().Create(context.TODO(), sc, metav1.CreateOptions{})
	return sc, err
}

func createVolumeSnapshotClass(snapshotClient *snapshotclientset.Clientset, deletionPolicy snapshotv1.DeletionPolicy) (*snapshotv1.VolumeSnapshotClass, error) {
	volumesnapshotclass := &snapshotv1.VolumeSnapshotClass{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "volumesnapshotclass-",
		},
		Driver:         hostpathDriver,
		DeletionPolicy: deletionPolicy,
	}
	volumesnapshotclass, err := snapshotClient.SnapshotV1().VolumeSnapshotClasses().Create(context.TODO(), volumesnapshotclass, metav1.CreateOptions{})
	return volumesnapshotclass, err
}

func getVolumeSnapshotSpec(namespace string, snapshotclassname string, pvcName string) *snapshotv1.VolumeSnapshot {
	var volumesnapshotSpec = &snapshotv1.VolumeSnapshot{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "volumesnapshot-",
			Namespace:    namespace,
		},
		Spec: snapshotv1.VolumeSnapshotSpec{
			VolumeSnapshotClassName: &snapshotclassname,
			Source: snapshotv1.VolumeSnapshotSource{
				PersistentVolumeClaimName: &pvcName,
			},
		},
	}
	return volumesnapshotSpec
}
