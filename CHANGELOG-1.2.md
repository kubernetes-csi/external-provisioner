# Changelog since v1.2.0

## Notable Features

* Adds CSI Migration support for Azure Disk/File, adds some backwards compatibility fixes for GCE PD Migration ([#294](https://github.com/kubernetes-csi/external-provisioner/pull/294), [@davidz627](https://github.com/davidz627))
* Add strict topology option that restricts requisite topology to the selected node topology during delayed binding ([#288](https://github.com/kubernetes-csi/external-provisioner/pull/288), [@avalluri](https://github.com/avalluri))
* Add volume provisioning secret templating for storage class parameters: "provisioner-secret-name" and "provisioner-secret-namespace" ([#287](https://github.com/kubernetes-csi/external-provisioner/pull/287), [@oleksiys](https://github.com/oleksiys))


# Changelog since v1.1.0

## Breaking Changes

None

## Deprecations

None

## Notable Features

* Handle deletion of CSI migrated volumes ([#273](https://github.com/kubernetes-csi/external-provisioner/pull/273), [@ddebroy](https://github.com/ddebroy))

## Other Notable Changes

* Fixes migration scenarios for Topology, fstype, and accessmodes for the kubernetes.io/gce-pd in-tree plugin ([#277](https://github.com/kubernetes-csi/external-provisioner/pull/277), [@davidz627](https://github.com/davidz627))
* Vendor: update sigs.k8s.io/sig-storage-lib-external-provisioner to v4.0.0 ([#277](https://github.com/kubernetes-csi/external-provisioner/pull/277), [@davidz627](https://github.com/davidz627))
* Update build to Go 1.12.4 ([#267](https://github.com/kubernetes-csi/external-provisioner/pull/267), [@pohly](https://github.com/pohly))
