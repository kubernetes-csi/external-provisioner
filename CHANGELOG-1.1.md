# Changelog since v1.0.1

## Breaking Changes
* Support for the alpha Topology feature and CSINodeInfo CRD has been removed.

## Deprecations
* Command line flag `--connection-timeout` is deprecated and has no effect.
* Command line flag `--provisioner` is deprecated and has no effect.
* Command line flag `--leader-election-type` is deprecated. Support for Endpoints-based
  leader election will be removed in the future in favor of Lease-based leader election.
  The default currently remains as `endpoints` for backwards compatibility.

## Notable Features
* The Topology feature has been promoted to beta and uses the `storage.k8s.io/v1beta1` CSINode object ([#238](https://github.com/kubernetes-csi/external-provisioner/pull/238))
* [In-tree storage plugin to CSI Driver Migration](https://github.com/kubernetes/enhancements/blob/master/keps/sig-storage/20190129-csi-migration.md) is now alpha ([#253](https://github.com/kubernetes-csi/external-provisioner/pull/253))
* The external provisioner now tries to connect to the CSI driver indefinitely ([#234](https://github.com/kubernetes-csi/external-provisioner/pull/234))
* A new --timeout parameter has been added for CSI operations ([#230](https://github.com/kubernetes-csi/external-provisioner/pull/230))
* README.md has been signficantly enhanced ([#249](https://github.com/kubernetes-csi/external-provisioner/pull/249))
* Add support for  Lease based leader election. Enable this by setting
  `--leader-election-type=leases` ([#261](https://github.com/kubernetes-csi/external-provisioner/pull/261))

## Other Notable Changes
* vendor: update to k8s.io 1.14, avoid glog ([#262](https://github.com/kubernetes-csi/external-provisioner/pull/262))
* Deprecate provisioner arguments ([#255](https://github.com/kubernetes-csi/external-provisioner/pull/255))
* Do not stop saving PVs to API server ([#251](https://github.com/kubernetes-csi/external-provisioner/pull/251))
* filter resizer related params from storageclass before passing them to the driver ([#248](https://github.com/kubernetes-csi/external-provisioner/pull/248))
* Cache driver capabilities ([#241](https://github.com/kubernetes-csi/external-provisioner/pull/241))
* Use distroless as base image ([#247](https://github.com/kubernetes-csi/external-provisioner/pull/247))
* Fix retry loop issues ([#216](https://github.com/kubernetes-csi/external-provisioner/pull/216))
* Cache driver name ([#215](https://github.com/kubernetes-csi/external-provisioner/pull/215))
* Add prune to gopkg.toml ([#196](https://github.com/kubernetes-csi/external-provisioner/pull/196))
* Don't provide access to Secrets in default RBAC ([#188](https://github.com/kubernetes-csi/external-provisioner/pull/188))
