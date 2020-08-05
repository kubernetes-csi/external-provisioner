# Changes since v1.4.0

## Breaking Changes

- Updates VolumeSnapshot CRD to v1beta1. v1alpha1 is no longer supported. ([#335](https://github.com/kubernetes-csi/external-provisioner/pull/335), [@xing-yang](https://github.com/xing-yang))

## New Features

- Add prometheus metrics to CSI external-provisioner under the /metrics endpoint. This can be enabled via the "--metrics-address" and "--metrics-path" options. ([#388](https://github.com/kubernetes-csi/external-provisioner/pull/388), [@saad-ali](https://github.com/saad-ali))
- Updates VolumeSnapshot CRD to v1beta1. v1alpha1 is no longer supported. ([#335](https://github.com/kubernetes-csi/external-provisioner/pull/335), [@xing-yang](https://github.com/xing-yang))

## Other Notable Changes

- Migrated to Go modules, so the source builds also outside of GOPATH. ([#369](https://github.com/kubernetes-csi/external-provisioner/pull/369), [@pohly](https://github.com/pohly))
- Fixes Azure translation lib nil dereferences. ([#359](https://github.com/kubernetes-csi/external-provisioner/pull/359), [@davidz627](https://github.com/davidz627))
- Use informers for Node objects. ([#337](https://github.com/kubernetes-csi/external-provisioner/pull/337), [@muchahitkurt](https://github.com/muchahitkurt))
- Use informers for CSINode objects. ([#327](https://github.com/kubernetes-csi/external-provisioner/pull/327), [@muchahitkurt](https://github.com/muchahitkurt))
