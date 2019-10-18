# Changelog since v0.4.2

## Bug Fixes

- Added extra verification of source Snapshot before provisioning. ([#358](https://github.com/kubernetes-csi/external-provisioner/pull/358))

# Changelog since v0.4.1

## Bug Fixes
- Secrets are now being stripped from GRPC request and response logs using protosanitizer library ([#192](https://github.com/kubernetes-csi/external-provisioner/pull/192))

# Changelog since v0.3.1

## New Features
- Added flag for leader election ([#152](https://github.com/kubernetes-csi/external-provisioner/pull/152))
- Topology alpha ([#141](https://github.com/kubernetes-csi/external-provisioner/pull/141))
- Restore volume from snapshot ([#123](https://github.com/kubernetes-csi/external-provisioner/pull/123))

## Bug Fixes

- Add the check for deleting snapshot while creating volume ([#138](https://github.com/kubernetes-csi/external-provisioner/pull/138))
- Support all Kubernetes access modes ([#133](https://github.com/kubernetes-csi/external-provisioner/pull/133))
- Change default pvName behavior to non-truncating ([#116](https://github.com/kubernetes-csi/external-provisioner/pull/116))
