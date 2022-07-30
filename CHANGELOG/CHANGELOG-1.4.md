# Changelog since v1.3.0

## Deprecations

All external-provisioner versions < 1.4.0 are deprecated and will stop
functioning in Kubernetes v1.20. See
[#323](https://github.com/kubernetes-csi/external-provisioner/pull/323) and
[k/k#80978](https://github.com/kubernetes/kubernetes/pull/80978) for more
details. Upgrade your external-provisioner to v1.4+ before Kubernetes v1.20.

## New Features

None

## Bug Fixes

- Fixes migration scenarios for Topology, fstype, and accessmodes for the kubernetes.io/gce-pd in-tree plugin ([#277](https://github.com/kubernetes-csi/external-provisioner/pull/277), [@davidz627](https://github.com/davidz627))
- Checks if volume content source is populated if creating a volume from a snapshot source. ([#283](https://github.com/kubernetes-csi/external-provisioner/pull/283), [@zhucan](https://github.com/zhucan))
- Fixes issue when SelfLink removal is turned on in Kubernetes. ([#323](https://github.com/kubernetes-csi/external-provisioner/pull/323), [@msau42](https://github.com/msau42))
- CSI driver can return `CreateVolumeResponse` with size 0, which means unknown volume size.
In this case, Provisioner will use PVC requested size as PV size rather than 0 bytes ([#271](https://github.com/kubernetes-csi/external-provisioner/pull/271), [@hoyho](https://github.com/hoyho))
- Fixed potential leak of volumes after CSI driver timeouts. ([#312](https://github.com/kubernetes-csi/external-provisioner/pull/312), [@jsafrane](https://github.com/jsafrane))
- Fixes issue where provisioner provisions volumes for in-tree PVC's which have not been migrated ([#341](https://github.com/kubernetes-csi/external-provisioner/pull/341), [@davidz627](https://github.com/davidz627))
- Send the CSI volume_id instead of  PVC Name to the csi-driver in volumeCreate when datasource  is PVC ([#310](https://github.com/kubernetes-csi/external-provisioner/pull/310), [@Madhu-1](https://github.com/Madhu-1))
- Fixes nil pointer dereference in log when migration turned on ([#342](https://github.com/kubernetes-csi/external-provisioner/pull/342), [@davidz627](https://github.com/davidz627))
- Handle deletion of CSI migrated volumes ([#273](https://github.com/kubernetes-csi/external-provisioner/pull/273), [@ddebroy](https://github.com/ddebroy))
- Reduced logging noise of unrelated PVCs. Emit event on successful provisioning. ([#351](https://github.com/kubernetes-csi/external-provisioner/pull/351), [@jsafrane](https://github.com/jsafrane))
- Added extra verification of source Snapshot and PersistentVolumeClaim before provisioning. ([#352](https://github.com/kubernetes-csi/external-provisioner/pull/352), [@jsafrane](https://github.com/jsafrane))
- Fixes storageclass comparison during volume cloning.  ([#309](https://github.com/kubernetes-csi/external-provisioner/pull/309), [@madhu-1](https://github.com/madhu-1))
