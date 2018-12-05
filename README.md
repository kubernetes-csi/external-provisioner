[![Build Status](https://travis-ci.org/kubernetes-csi/external-provisioner.svg?branch=master)](https://travis-ci.org/kubernetes-csi/external-provisioner)
# Kubernetes external provisioner that works with CSI volumes.

This is an example external provisioner for Kubernetes which provisions using CSI Volume drivers..  It's under heavy development, so at this time README.md is notes for the developers coding.  Once complete this will change to something user friendly.

# User Guide

## Parameters

The CSI dynamic provisioner makes `CreateVolumeRequest` and `DeleteVolumeRequest` calls to CSI drivers.
The `controllerCreateSecrets` and `controllerDeleteSecrets` fields in those requests can be populated 
with data from a Kubernetes `Secret` object by setting `csiProvisionerSecretName` and `csiProvisionerSecretNamespace`
parameters in the `StorageClass`. For example:

```yaml
kind: StorageClass
apiVersion: storage.k8s.io/v1
metadata:
  name: fast-storage
provisioner: com.example.team/csi-driver
parameters:
  type: pd-ssd
  csiProvisionerSecretName: fast-storage-provision-key
  csiProvisionerSecretNamespace: pd-ssd-credentials
```

The `csiProvisionerSecretName` and `csiProvisionerSecretNamespace` parameters
may specify literal values, or a template containing the following variables:
* `${pv.name}` - replaced with the name of the PersistentVolume object being provisioned

Once the CSI volume is created, a corresponding Kubernetes `PersistentVolume` object is created.
The `controllerPublishSecretRef`, `nodeStageSecretRef`, and `nodePublishSecretRef` fields in the 
`PersistentVolume` object can be populated via the following storage class parameters:

* `controllerPublishSecretRef` in the PersistentVolume is populated by setting these StorageClass parameters:
  * `csiControllerPublishSecretName`
  * `csiControllerPublishSecretNamespace`
* `nodeStageSecretRef` in the PersistentVolume is populated by setting these StorageClass parameters:
  * `csiNodeStageSecretName`
  * `csiNodeStageSecretNamespace`
* `nodePublishSecretRef` in the PersistentVolume is populated by setting these StorageClass parameters:
  * `csiNodePublishSecretName`
  * `csiNodePublishSecretNamespace`

The `csiControllerPublishSecretName`, `csiNodeStageSecretName`, and `csiNodePublishSecretName` parameters
may specify a literal secret name, or a template containing the following variables:
* `${pv.name}` - replaced with the name of the PersistentVolume
* `${pvc.name}` - replaced with the name of the PersistentVolumeClaim
* `${pvc.namespace}` - replaced with the namespace of the PersistentVolumeClaim
* `${pvc.annotations['<ANNOTATION_KEY>']}` (e.g. `${pvc.annotations['example.com/key']}`) - replaced with the value of the specified annotation in the PersistentVolumeClaim

The `csiControllerPublishSecretNamespace`, `csiNodeStageSecretNamespace`, and `csiNodePublishSecretNamespace` parameters
may specify a literal namespace name, or a template containing the following variables:
* `${pv.name}` - replaced with the name of the PersistentVolume
* `${pvc.namespace}` - replaced with the namespace of the PersistentVolumeClaim

As an example, consider this StorageClass:

```yaml
kind: StorageClass
apiVersion: storage.k8s.io/v1
metadata:
  name: fast-storage
provisioner: com.example.team/csi-driver
parameters:
  type: pd-ssd

  csiProvisionerSecretName: fast-storage-provision-key
  csiProvisionerSecretNamespace: pd-ssd-credentials

  csiControllerPublishSecretName: ${pv.name}-publish
  csiControllerPublishSecretNamespace: pd-ssd-credentials

  csiNodeStageSecretName: ${pv.name}-stage
  csiNodeStageSecretNamespace: pd-ssd-credentials

  csiNodePublishSecretName: ${pvc.annotations['com.example.team/key']}
  csiNodePublishSecretNamespace: ${pvc.namespace}
```

This StorageClass instructs the CSI provisioner to do the following:
* send the data in the `fast-storage-provision-key` secret in the `pd-ssd-credentials` namespace as part of the create request to the CSI driver
* create a PersistentVolume with:
  * a per-volume controller publish and node stage secret, both in the `pd-ssd-credentials` (those secrets would need to be created separately in response to the PersistentVolume creation before the PersistentVolume could be attached/mounted)
  * a node publish secret in the same namespace as the PersistentVolumeClaim that triggered the provisioning, with a name specified as an annotation on the PersistentVolumeClaim. This could be used to give the creator of the PersistentVolumeClaim the ability to specify a secret containing a decryption key they have control over.

# Build

```bash
make csi-provisioner
```

# Test

### Start Kubernetes

Run a local kubernetes cluster built from latest master code

## Run Storage Provider

### Use HostPath drivers

Go to drivers and run:

```bash
bin/hostpathplugin --drivername mydriver  --endpoint unix://tmp/csi.sock --nodeid foobar -v=5
```

### Start external provisioner

```bash
bin/csi-provisioner -kubeconfig /var/run/kubernetes/admin.kubeconfig -alsologtostderr -provisioner csi-flex
```

### Create Storage class, PVC, and Pod

```bash
kubectl create -f examples/sc.yaml
kubectl create -f example/pvc2.yaml
kubectl create -f example/pod.yaml
```

### Delete PVC
```bash
kubectl delete -f example/pvc1.yaml
```

## Community, discussion, contribution, and support

Learn how to engage with the Kubernetes community on the [community page](http://kubernetes.io/community/).

You can reach the maintainers of this project at:

- [Slack channel](https://kubernetes.slack.com/messages/sig-storage)
- [Mailing list](https://groups.google.com/forum/#!forum/kubernetes-sig-storage)

### Code of conduct

Participation in the Kubernetes community is governed by the [Kubernetes Code of Conduct](code-of-conduct.md).
