# Release notes for v2.2.2

[Documentation](https://kubernetes-csi.github.io)

# Changelog since v2.2.1

## Changes by Kind

### Bug or Regression
 - Fix env name from POD_NAMESPACE to NAMESPACE for capacity-ownerref-level option. ([#636](https://github.com/kubernetes-csi/external-provisioner/pull/636), [@bells17](https://github.com/bells17))
 - Fix a bug that not being able to use block device mode when enable a storage capacity tracking mode. ([#635](https://github.com/kubernetes-csi/external-provisioner/pull/635), [@bells17](https://github.com/bells17))

## Dependencies

### Added
_Nothing has changed._

### Changed
_Nothing has changed._

### Removed
_Nothing has changed._

# Release notes for v2.2.1

# Changelog since v2.2.0

## Changes by Kind

### Bug or Regression
 - Fix capacity information updates when topology changes. Only affected central deployment and network attached storage, not deployment on each node. This broke in v2.2.0 as part of a bug fix for capacity informer handling. ([#617](https://github.com/kubernetes-csi/external-provisioner/pull/617), [@bai3shuo4](https://github.com/bai3shuo4))
 - Fixed reporting of metrics when a migratable CSI driver is used. ([#620](https://github.com/kubernetes-csi/external-provisioner/pull/620), [@jsafrane](https://github.com/jsafrane))

## Dependencies

### Added
_Nothing has changed._

### Changed
_Nothing has changed._

### Removed
_Nothing has changed._

# Changelog since v2.1.0

## Urgent Upgrade Notes

### (No, really, you MUST read this before you upgrade)

- For drivers that support CSI Migration, a "migrated" label was added to the csi_sidecar_operations_seconds metric that indicates if the call is from the migration path. Metric collectors should be updated with the new field. ([#560](https://github.com/kubernetes-csi/external-provisioner/pull/560), [@nearora-msft](https://github.com/nearora-msft))

## Deprecations

- Support for the v1beta1 VolumeSnapshot API is deprecated and will be removed in the following release, which will require minimum Kubernetes version 1.20.

## Changes by Kind

### API Change

- Adds support for the CSI `GetCapacityResponse.MaximumVolumeSize` field added in CSI spec v1.4.0. If reported by the CSI driver, that value is also published in the `CSIStorageCapacity` objects. The v1beta1 storage API from Kubernetes >= 1.21 is required. ([#584](https://github.com/kubernetes-csi/external-provisioner/pull/584), [@pohly](https://github.com/pohly))

### Feature

- After volume creation or deletion, those CSIStorageCapacity objects most likely affected by that get refreshed sooner than the others. ([#586](https://github.com/kubernetes-csi/external-provisioner/pull/586), [@pohly](https://github.com/pohly))
- New metrics data (storage capacity tracking, process and Go runtime, work queues, leaderelection) ([#579](https://github.com/kubernetes-csi/external-provisioner/pull/579), [@pohly](https://github.com/pohly))

### Bug or Regression

- Fix races where external-provisioner may have [stopped provisioning](https://github.com/kubernetes-sigs/sig-storage-lib-external-provisioner/pull/103) for a PVC depending on certain timing conditions (provisioning failed once, next attempt currently running, PVC update arrives from API server). ([#564](https://github.com/kubernetes-csi/external-provisioner/pull/564), [@pohly](https://github.com/pohly))
- Fixes CSI migration to provision PV objects with in-tree topology labels instead of CSI topology labels for aws-ebs, gce-pd, cinder plugins ([#566](https://github.com/kubernetes-csi/external-provisioner/pull/566), [@Jiawei0227](https://github.com/Jiawei0227))
- Fixes an issue when using immediate binding, allowed topologies and distributed provisioning, where PVCs may have been assigned to nodes which did not fit the allowed topologies, leading to permanent `error generating accessibility requirements` failures. ([#612](https://github.com/kubernetes-csi/external-provisioner/pull/612), [@pohly](https://github.com/pohly))
- Fixes an issue where redundant CSIStorageCapacity objects might have been created because informers were not handled correctly. ([#590](https://github.com/kubernetes-csi/external-provisioner/pull/590), [@bai3shuo4](https://github.com/bai3shuo4))
- Fixes race where producing StorageCapacity objects may have failed with `the object has been modified` errors ([#565](https://github.com/kubernetes-csi/external-provisioner/pull/565), [@pohly](https://github.com/pohly))
- If the in-tree providers are depending on StorageClass for DeleteVolume, volumes could not get deleted after migrating from them. So translate in-tree StorageClass to CSI to get SecretReference in DeleteVolume call for CSI Migration scenario. ([#567](https://github.com/kubernetes-csi/external-provisioner/pull/567), [@huchengze](https://github.com/huchengze))

### Other (Cleanup or Flake)

- Removed redundant log lines at log level 5+ for CreateVolume and GetCapacity requests that could leak secrets. ([#604](https://github.com/kubernetes-csi/external-provisioner/pull/604), [@chrishenzie](https://github.com/chrishenzie))

### Uncategorized

- Updated runtime (Go 1.16) and dependencies ([#588](https://github.com/kubernetes-csi/external-provisioner/pull/588), [@pohly](https://github.com/pohly))

## Dependencies

### Added
- github.com/go-errors/errors: [v1.0.1](https://github.com/go-errors/errors/tree/v1.0.1)
- github.com/gobuffalo/here: [v0.6.0](https://github.com/gobuffalo/here/tree/v0.6.0)
- github.com/google/shlex: [e7afc7f](https://github.com/google/shlex/tree/e7afc7f)
- github.com/markbates/pkger: [v0.17.1](https://github.com/markbates/pkger/tree/v0.17.1)
- github.com/moby/spdystream: [v0.2.0](https://github.com/moby/spdystream/tree/v0.2.0)
- github.com/monochromegane/go-gitignore: [205db1a](https://github.com/monochromegane/go-gitignore/tree/205db1a)
- github.com/niemeyer/pretty: [a10e7ca](https://github.com/niemeyer/pretty/tree/a10e7ca)
- github.com/xlab/treeprint: [a009c39](https://github.com/xlab/treeprint/tree/a009c39)
- go.starlark.net: 8dd3e2e
- sigs.k8s.io/kustomize/api: v0.8.5
- sigs.k8s.io/kustomize/cmd/config: v0.9.7
- sigs.k8s.io/kustomize/kustomize/v4: v4.0.5
- sigs.k8s.io/kustomize/kyaml: v0.10.15

### Changed
- dmitri.shuralyov.com/gpu/mtl: 666a987 → 28db891
- github.com/Azure/go-autorest/autorest: [v0.11.1 → v0.11.12](https://github.com/Azure/go-autorest/autorest/compare/v0.11.1...v0.11.12)
- github.com/NYTimes/gziphandler: [56545f4 → v1.1.1](https://github.com/NYTimes/gziphandler/compare/56545f4...v1.1.1)
- github.com/cilium/ebpf: [1c8d4c9 → v0.2.0](https://github.com/cilium/ebpf/compare/1c8d4c9...v0.2.0)
- github.com/cncf/udpa/go: [efcf912 → 5459f2c](https://github.com/cncf/udpa/go/compare/efcf912...5459f2c)
- github.com/container-storage-interface/spec: [v1.3.0 → v1.4.0](https://github.com/container-storage-interface/spec/compare/v1.3.0...v1.4.0)
- github.com/containerd/console: [v1.0.0 → v1.0.1](https://github.com/containerd/console/compare/v1.0.0...v1.0.1)
- github.com/containerd/containerd: [v1.4.1 → v1.4.4](https://github.com/containerd/containerd/compare/v1.4.1...v1.4.4)
- github.com/coredns/corefile-migration: [v1.0.10 → v1.0.11](https://github.com/coredns/corefile-migration/compare/v1.0.10...v1.0.11)
- github.com/creack/pty: [v1.1.7 → v1.1.11](https://github.com/creack/pty/compare/v1.1.7...v1.1.11)
- github.com/docker/docker: [bd33bbf → v20.10.2+incompatible](https://github.com/docker/docker/compare/bd33bbf...v20.10.2)
- github.com/envoyproxy/go-control-plane: [v0.9.7 → fd9021f](https://github.com/envoyproxy/go-control-plane/compare/v0.9.7...fd9021f)
- github.com/go-logr/logr: [v0.3.0 → v0.4.0](https://github.com/go-logr/logr/compare/v0.3.0...v0.4.0)
- github.com/go-openapi/spec: [v0.19.3 → v0.19.5](https://github.com/go-openapi/spec/compare/v0.19.3...v0.19.5)
- github.com/go-openapi/strfmt: [v0.19.3 → v0.19.5](https://github.com/go-openapi/strfmt/compare/v0.19.3...v0.19.5)
- github.com/go-openapi/validate: [v0.19.5 → v0.19.8](https://github.com/go-openapi/validate/compare/v0.19.5...v0.19.8)
- github.com/gogo/protobuf: [v1.3.1 → v1.3.2](https://github.com/gogo/protobuf/compare/v1.3.1...v1.3.2)
- github.com/golang/protobuf: [v1.4.3 → v1.5.1](https://github.com/golang/protobuf/compare/v1.4.3...v1.5.1)
- github.com/google/cadvisor: [v0.38.5 → v0.39.0](https://github.com/google/cadvisor/compare/v0.38.5...v0.39.0)
- github.com/google/go-cmp: [v0.5.4 → v0.5.5](https://github.com/google/go-cmp/compare/v0.5.4...v0.5.5)
- github.com/google/uuid: [v1.1.2 → v1.2.0](https://github.com/google/uuid/compare/v1.1.2...v1.2.0)
- github.com/googleapis/gnostic: [v0.5.3 → v0.5.4](https://github.com/googleapis/gnostic/compare/v0.5.3...v0.5.4)
- github.com/heketi/heketi: [c2e2a4a → v10.2.0+incompatible](https://github.com/heketi/heketi/compare/c2e2a4a...v10.2.0)
- github.com/imdario/mergo: [v0.3.11 → v0.3.12](https://github.com/imdario/mergo/compare/v0.3.11...v0.3.12)
- github.com/kisielk/errcheck: [v1.2.0 → v1.5.0](https://github.com/kisielk/errcheck/compare/v1.2.0...v1.5.0)
- github.com/kr/text: [v0.1.0 → v0.2.0](https://github.com/kr/text/compare/v0.1.0...v0.2.0)
- github.com/kubernetes-csi/csi-lib-utils: [v0.9.0 → v0.9.1](https://github.com/kubernetes-csi/csi-lib-utils/compare/v0.9.0...v0.9.1)
- github.com/mattn/go-runewidth: [v0.0.2 → v0.0.7](https://github.com/mattn/go-runewidth/compare/v0.0.2...v0.0.7)
- github.com/miekg/dns: [v1.1.35 → v1.1.40](https://github.com/miekg/dns/compare/v1.1.35...v1.1.40)
- github.com/moby/sys/mountinfo: [v0.1.3 → v0.4.0](https://github.com/moby/sys/mountinfo/compare/v0.1.3...v0.4.0)
- github.com/moby/term: [672ec06 → df9cb8a](https://github.com/moby/term/compare/672ec06...df9cb8a)
- github.com/mrunalp/fileutils: [abd8a0e → v0.5.0](https://github.com/mrunalp/fileutils/compare/abd8a0e...v0.5.0)
- github.com/olekukonko/tablewriter: [a0225b3 → v0.0.4](https://github.com/olekukonko/tablewriter/compare/a0225b3...v0.0.4)
- github.com/opencontainers/runc: [v1.0.0-rc92 → v1.0.0-rc93](https://github.com/opencontainers/runc/compare/v1.0.0-rc92...v1.0.0-rc93)
- github.com/opencontainers/runtime-spec: [4d89ac9 → e6143ca](https://github.com/opencontainers/runtime-spec/compare/4d89ac9...e6143ca)
- github.com/opencontainers/selinux: [v1.6.0 → v1.8.0](https://github.com/opencontainers/selinux/compare/v1.6.0...v1.8.0)
- github.com/prometheus/client_golang: [v1.8.0 → v1.9.0](https://github.com/prometheus/client_golang/compare/v1.8.0...v1.9.0)
- github.com/prometheus/common: [v0.15.0 → v0.19.0](https://github.com/prometheus/common/compare/v0.15.0...v0.19.0)
- github.com/prometheus/procfs: [v0.2.0 → v0.6.0](https://github.com/prometheus/procfs/compare/v0.2.0...v0.6.0)
- github.com/sergi/go-diff: [v1.0.0 → v1.1.0](https://github.com/sergi/go-diff/compare/v1.0.0...v1.1.0)
- github.com/sirupsen/logrus: [v1.6.0 → v1.7.0](https://github.com/sirupsen/logrus/compare/v1.6.0...v1.7.0)
- github.com/stretchr/testify: [v1.6.1 → v1.7.0](https://github.com/stretchr/testify/compare/v1.6.1...v1.7.0)
- github.com/syndtr/gocapability: [d983527 → 42c35b4](https://github.com/syndtr/gocapability/compare/d983527...42c35b4)
- github.com/willf/bitset: [d5bec33 → v1.1.11](https://github.com/willf/bitset/compare/d5bec33...v1.1.11)
- github.com/yuin/goldmark: [v1.1.32 → v1.2.1](https://github.com/yuin/goldmark/compare/v1.1.32...v1.2.1)
- golang.org/x/crypto: 5f87f34 → 513c2a4
- golang.org/x/exp: 6cc2880 → 85be41e
- golang.org/x/mobile: d2bd2a2 → e6ae53a
- golang.org/x/mod: v0.3.0 → ce943fd
- golang.org/x/net: ac852fb → d523dce
- golang.org/x/oauth2: 08078c5 → cd4f82c
- golang.org/x/sync: 6e8e738 → 09787c9
- golang.org/x/sys: aec9a39 → c4fcb01
- golang.org/x/term: 2321bbc → de623e6
- golang.org/x/text: v0.3.4 → v0.3.5
- golang.org/x/time: 7e3f01d → f8bda1e
- golang.org/x/tools: b303f43 → v0.1.0
- google.golang.org/genproto: 40ec1c2 → 75c7a85
- google.golang.org/grpc: v1.34.0 → v1.36.0
- google.golang.org/protobuf: v1.25.0 → v1.26.0
- gopkg.in/check.v1: 41f04d3 → 8fa4692
- gopkg.in/yaml.v3: eeeca48 → 496545a
- gotest.tools/v3: v3.0.2 → v3.0.3
- k8s.io/api: v0.20.0 → v0.21.0
- k8s.io/apiextensions-apiserver: v0.20.0 → v0.21.0
- k8s.io/apimachinery: v0.20.0 → v0.21.0
- k8s.io/apiserver: v0.20.0 → v0.21.0
- k8s.io/cli-runtime: v0.20.0 → v0.21.0
- k8s.io/client-go: v0.20.0 → v0.21.0
- k8s.io/cloud-provider: v0.20.0 → v0.21.0
- k8s.io/cluster-bootstrap: v0.20.0 → v0.21.0
- k8s.io/code-generator: v0.20.0 → v0.21.0
- k8s.io/component-base: v0.20.0 → v0.21.0
- k8s.io/component-helpers: v0.20.0 → v0.21.0
- k8s.io/controller-manager: v0.20.0 → v0.21.0
- k8s.io/cri-api: v0.20.0 → v0.21.0
- k8s.io/csi-translation-lib: v0.20.0 → v0.21.0
- k8s.io/gengo: 83324d8 → b6c5ce2
- k8s.io/klog/v2: v2.4.0 → v2.8.0
- k8s.io/kube-aggregator: v0.20.0 → v0.21.0
- k8s.io/kube-controller-manager: v0.20.0 → v0.21.0
- k8s.io/kube-openapi: d219536 → f622666
- k8s.io/kube-proxy: v0.20.0 → v0.21.0
- k8s.io/kube-scheduler: v0.20.0 → v0.21.0
- k8s.io/kubectl: v0.20.0 → v0.21.0
- k8s.io/kubelet: v0.20.0 → v0.21.0
- k8s.io/kubernetes: v1.20.0 → v1.21.0
- k8s.io/legacy-cloud-providers: v0.20.0 → v0.21.0
- k8s.io/metrics: v0.20.0 → v0.21.0
- k8s.io/mount-utils: v0.20.0 → v0.21.0
- k8s.io/sample-apiserver: v0.20.0 → v0.21.0
- k8s.io/system-validators: v1.2.0 → v1.4.0
- k8s.io/utils: 67b214c → 2afb431
- sigs.k8s.io/apiserver-network-proxy/konnectivity-client: v0.0.14 → v0.0.15
- sigs.k8s.io/controller-runtime: v0.7.0 → v0.8.3
- sigs.k8s.io/structured-merge-diff/v4: v4.0.2 → v4.1.0

### Removed
- github.com/codegangsta/negroni: [v1.0.0](https://github.com/codegangsta/negroni/tree/v1.0.0)
- github.com/docker/spdystream: [449fdfc](https://github.com/docker/spdystream/tree/449fdfc)
- github.com/golangplus/bytes: [45c989f](https://github.com/golangplus/bytes/tree/45c989f)
- github.com/golangplus/fmt: [2a5d6d7](https://github.com/golangplus/fmt/tree/2a5d6d7)
- sigs.k8s.io/kustomize: v2.0.3+incompatible
