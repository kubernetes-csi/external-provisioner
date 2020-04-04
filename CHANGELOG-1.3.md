# Changelog since v1.2.0

## Breaking Changes

- The alpha resizer secret name and namespace keys have been changed from the following values:
  - `csi.storage.k8s.io/resizer-secret-name`
  - `csi.storage.k8s.io/resizer-secret-namespace`

  to be the following values:
  - `csi.storage.k8s.io/controller-expand-secret-name`
  - `csi.storage.k8s.io/controller-expand-secret-namespace`

  This is a breaking change and is being introduced so that these keys match the naming convention for other secret name/namespace keys. ([#301](https://github.com/kubernetes-csi/external-provisioner/pull/301), [@ggriffiths](https://github.com/ggriffiths))

## New Features

- A new flag `--leader-election-namespace` is introduced to allow the user to set where the leader election lock resource lives. ([#296](https://github.com/kubernetes-csi/external-provisioner/pull/296), [@verult](https://github.com/verult))
- Adds CSI Migration support for Azure Disk/File, adds some backwards compatibility fixes for GCE PD Migration ([#292](https://github.com/kubernetes-csi/external-provisioner/pull/292), [@davidz627](https://github.com/davidz627))
- Add `--strict-topology` option that restricts requisite topology to the selected node topology during delayed binding ([#282](https://github.com/kubernetes-csi/external-provisioner/pull/282), [@avalluri](https://github.com/avalluri))
- Add volume provisioning secret templating for storage class parameters: `csi.storage.k8s.io/provisioner-secret-name` and `csi.storage.k8s.io/provisioner-secret-namespace` ([#274](https://github.com/kubernetes-csi/external-provisioner/pull/274), [@ggriffiths](https://github.com/ggriffiths))
- Adds the ability to handle PVC as a DataSource to enable cloning for plugins that support it.  ([#220](https://github.com/kubernetes-csi/external-provisioner/pull/220), [@j-griffith](https://github.com/j-griffith))

## Bug Fixes

- Fixes issue where leader election in the CSI provisioner and lib-external-provisioner conflicts. ([#296](https://github.com/kubernetes-csi/external-provisioner/pull/296), [@verult](https://github.com/verult))
