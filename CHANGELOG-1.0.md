# Changes since v1.0.1

## New Features

- Users can now provide a secret name and namespace during provision by passing the correct storage class parameters: `csi.storage.k8s.io/provisioner-secret-name` and `csi.storage.k8s.io/provisioner-secret-namespace` ([#281](https://github.com/kubernetes-csi/external-provisioner/pull/281), [@oleksiys](https://github.com/oleksiys))

## Bug Fixes

- Added extra verification of source Snapshot before provisioning. ([#357](https://github.com/kubernetes-csi/external-provisioner/pull/357), [@jsafrane](https://github.com/jsafrane))


# Changes since v0.4.1

# Breaking Changes
- Update external-provisioner to use csi spec 1.0 ([#166](https://github.com/kubernetes-csi/external-provisioner/pull/166))
- Update dependencies to use external-snapshotter v1.0.0-rc4 ([#175](https://github.com/kubernetes-csi/external-provisioner/pull/175))

# Action Required
* CSI plugin must support the 1.0 spec.  CSI spec versions < 1.0 are no longer supported

# Deprecations
* The following StorageClass parameters are deprecated and will be removed in a future release:

| Deprecated | Replacement |
| --------------- | ------------------ |
| csiProvisionerSecretName | csi.storage.k8s.io/provisioner-secret-name |
| csiProvisionerSecretNamespace | csi.storage.k8s.io/provisioner-secret-namespace |
| csiControllerPublishSecretName | csi.storage.k8s.io/controller-publish-secret-name |
| csiControllerPublishSecretNamespace | csi.storage.k8s.io/controller-publish-secret-namespace |
| csiNodeStageSecretName | csi.storage.k8s.io/node-stage-secret-name |
| csiNodeStageSecretNamespace | csi.storage.k8s.io/node-stage-secret-namespace |
| csiNodePublishSecretName |  csi.storage.k8s.io/node-publish-secret-name |
| csiNodePublishSecretNamespace | csi.storage.k8s.io/node-publish-secret-namespace |
| fstype |  csi.storage.k8s.io/fstype |

# New Features
* Topology (alpha)
* Raw block (alpha)

# Major Changes
* Make the provisioner name optional ([#142](https://github.com/kubernetes-csi/external-provisioner/pull/142))
* Add mount options ([#126](https://github.com/kubernetes-csi/external-provisioner/pull/126))
* Added feature gate mechanism; added topology feature gate ([#148](https://github.com/kubernetes-csi/external-provisioner/pull/148))
* Add flag for leader election ([#148](https://github.com/kubernetes-csi/external-provisioner/pull/148))
* Split out RBAC, fix leadership election permissions ([#156](https://github.com/kubernetes-csi/external-provisioner/pull/156))
* Evenly spread volumes of a StatefuleSet across nodes based on topology ([#151](https://github.com/kubernetes-csi/external-provisioner/pull/151))
* rbac: fix kubectl validation error ([#163](https://github.com/kubernetes-csi/external-provisioner/pull/163))
* Updating topology logic to use the latest version of CSINodeInfo ([#175](https://github.com/kubernetes-csi/external-provisioner/pull/175))
* Add Block volume support for CSI provisioner ([#175](https://github.com/kubernetes-csi/external-provisioner/pull/175))
* Use protosanitizer library to avoid logging secrets ([#180](https://github.com/kubernetes-csi/external-provisioner/pull/180))
* Add new reserved prefixed parameter keys which are stripped out of the parameter key ([#182](https://github.com/kubernetes-csi/external-provisioner/pull/182))
