# Release notes for 6.1.1

[Documentation](https://kubernetes-csi.github.io)

# Changelog since 6.1.0

## Changes by Kind

### Bug or Regression

- Add provisioner.storage.kubernetes.io/volumesnapshot-as-source-protection finalizer on VolumeSnapshot as Source. Add rbac rules to watch/update volumesnapshots. ([#1458](https://github.com/kubernetes-csi/external-provisioner/pull/1458), [@xing-yang](https://github.com/xing-yang))
- Allow provisioning to proceed when snapshot is being deleted to prevent leaking volumes and snapshots. ([#1448](https://github.com/kubernetes-csi/external-provisioner/pull/1448), [@xing-yang](https://github.com/xing-yang))
- Fix topology cache corruption on retry. ([#1472](https://github.com/kubernetes-csi/external-provisioner/pull/1472), [@torredil](https://github.com/torredil))

## Dependencies

### Added
_Nothing has changed._

### Changed
_Nothing has changed._

### Removed
_Nothing has changed._

# Release notes for 6.1.0

# Changelog since 6.0.0

## Changes by Kind

### Bug or Regression

- Fixed infinite retry loop during provisioning if node was deleted in the meantime. ([#1438](https://github.com/kubernetes-csi/external-provisioner/pull/1438), [@Fricounet](https://github.com/Fricounet))

