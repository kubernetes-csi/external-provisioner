# Release notes for v1.6.1 (Changelog since v1.6.0)

## Changes by Kind

### Uncategorized

- release-1.6: update release-tools ([#496](https://github.com/kubernetes-csi/external-provisioner/pull/496), [@Jiawei0227](https://github.com/Jiawei0227))
  - Build with Go 1.15


## Dependencies

### Added
_Nothing has changed._

### Changed
- github.com/kubernetes-csi/csi-lib-utils: [v0.7.0 → v0.7.1](https://github.com/kubernetes-csi/csi-lib-utils/compare/v0.7.0...v0.7.1)

### Removed
_Nothing has changed._

# Release notes for v1.6.0 (Changelog since v1.5.0)

## Deprecations

- Support for `storage.k8s.io/v1beta1.CSINode` and the `nodeid` annotation will be
  removed in a future release. Only cluster versions that support
  `storage.k8s.io/v1.CSINode` will be supported. The topology feature gate will
  also be removed and will always be on if the plugin supports the
  `VOLUME_ACCESSIBILITY_CONSTRAINTS` capability.
  ([#385](https://github.com/kubernetes-csi/external-provisioner/issues/385),
  [#257](https://github.com/kubernetes-csi/external-provisioner/issues/257))
- The current leader election lock name will be changed in a future release.
  Rolling upgrades to a newer sidecar will not maintain proper leadership, and a
  full teardown and bringup will be required if the driver cannot handle
  multiple leaders.
  ([#295](https://github.com/kubernetes-csi/external-provisioner/issues/295))
- The `--leader-election-type` argument will be removed in a future release.
  Only Leases-type leader election will be supported. The
  `--enable-leader-election` argument will also be renamed to
  `--leader-election`.
  ([#401](https://github.com/kubernetes-csi/external-provisioner/issues/401))
- The default value of `--worker-threads` will be lowered to 10 to match the
  other CSI sidecars in a future release.
  ([#322](https://github.com/kubernetes-csi/external-provisioner/issues/322))
- The external-provisioner will no longer default an empty fstype to `ext4`.
  A new option to set the default will be added, otherwise the CSI driver
  must be able to handle an empty fstype according to the CSI spec.
  ([#328](https://github.com/kubernetes-csi/external-provisioner/issues/328))
- Already deprecated arguments `--connection-timeout` and `--provisioner` will
  be removed in a future release.

## New Features

- `StorageClass.allowedtopologies` can specify a subset of the topology keys
  supported by the driver.
  ([#421](https://github.com/kubernetes-csi/external-provisioner/pull/421),
  [@pawanpraka1](https://github.com/pawanpraka1))
- Added a new flag, `--cloning-protection-threads` which defaults to 1,
  managing how many threads will simultaneously serve the
  `provisioner.storage.kubernetes.io/cloning-protection` finalizer removal
  ([#424](https://github.com/kubernetes-csi/external-provisioner/pull/424),
  [@Danil-Grigorev](https://github.com/Danil-Grigorev))
- finalizer `provisioner.storage.kubernetes.io/cloning-protection`
  is now set on the source PVC preventing their removal before the cloning finishes.
  ([#422](https://github.com/kubernetes-csi/external-provisioner/pull/422),
  [@Danil-Grigorev](https://github.com/Danil-Grigorev))
- Adds `--extraCreateMetadata` flag which, when enabled, will inject parameters onto CreateVolume driver requests with PVC and PV metadata.
  Injected keys:
  - csi.storage.k8s.io/pvc/name
  - csi.storage.k8s.io/pvc/namespace
  - csi.storage.k8s.io/pv/name ([#399](https://github.com/kubernetes-csi/external-provisioner/pull/399), [@zetsub0u](https://github.com/zetsub0u))
- Default StorageClass secrets are added:
  - csi.storage.k8s.io/secret-name: ${pvc.name}
  - csi.storage.k8s.io/secret-namespace: ${pvc.namespace}

  Note: Default secrets for storage class feature does work only when both parameters are added. ([#393](https://github.com/kubernetes-csi/external-provisioner/pull/393), [@taaraora](https://github.com/taaraora))

## Bug Fixes

- If a CSI driver returns ResourceExhausted for CreateVolume, supports topology,
  and the `StorageClass.volumeBindingMode` is `WaitForFirstConsumer`,
  then the pod will be rescheduled. This may then result in a CreateVolume retry
  with a different topology.
  ([#405](https://github.com/kubernetes-csi/external-provisioner/pull/405),
  [@pohly](https://github.com/pohly))
- CSI Migration: Fixes CSI migration issue where PVCs provisioned before
  migration was turned on could not be deleted
  ([#412](https://github.com/kubernetes-csi/external-provisioner/pull/412),
  [@davidz627](https://github.com/davidz627))
- Cloning: ensure source and target PVCs have matching volumeMode.
  ([#410](https://github.com/kubernetes-csi/external-provisioner/pull/410),
  [@j-griffith](https://github.com/j-griffith))


## Other Notable Changes

- Use informers for StorageClass ([#387](https://github.com/kubernetes-csi/external-provisioner/pull/387), [@bertinatto](https://github.com/bertinatto))


