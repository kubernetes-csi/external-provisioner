# Release notes for v3.1.1

[Documentation](https://kubernetes-csi.github.io)

# Changelog since v3.1.0

## Changes by Kind

### Bug or Regression
 - Storage capacity: managed-by label potentially too long with a long node name and enableNodeDeployment set to true ([#739](https://github.com/kubernetes-csi/external-provisioner/pull/739), [@akalenyu](https://github.com/akalenyu))

## Dependencies

### Added
_Nothing has changed._

### Changed
_Nothing has changed._

### Removed
_Nothing has changed._

# Changelog since v3.0.0

## Changes by Kind

### Feature
 - Introduces `HonorPVReclaimPolicy` feature gate that ensures PV honors the Reclaim policy irrespective of deletion order. ([#679](https://github.com/kubernetes-csi/external-provisioner/pull/679), [@deepakkinni](https://github.com/deepakkinni))
 - This release update brings the csi-translation-lib migration support for  in-tree plugin Ceph RBD ( `kubernetes.io/rbd`) to `rbd.csi.ceph.com` CSI driver and Portworx (`kubernetes.io/portworx-volume`)  in-tree plugin to `pxd.portworx.com` CSI driver. ([#689](https://github.com/kubernetes-csi/external-provisioner/pull/689), [@humblec](https://github.com/humblec))

### Bug or Regression
 - Add getCapacity request timeout to avoid hang forever ([#688](https://github.com/kubernetes-csi/external-provisioner/pull/688), [@bai3shuo4](https://github.com/bai3shuo4))
 - Fixed retry of CreateVolume calls when the storage backend runs out of space. Such calls won't be repeated when user deletes corresponding PVC. ([#675](https://github.com/kubernetes-csi/external-provisioner/pull/675), [@jsafrane](https://github.com/jsafrane))

## Dependencies

### Added
- github.com/antlr/antlr4/runtime/Go/antlr: [b48c857](https://github.com/antlr/antlr4/runtime/Go/antlr/tree/b48c857)
- github.com/cncf/xds/go: [fbca930](https://github.com/cncf/xds/go/tree/fbca930)
- github.com/getkin/kin-openapi: [v0.76.0](https://github.com/getkin/kin-openapi/tree/v0.76.0)
- github.com/google/cel-go: [v0.9.0](https://github.com/google/cel-go/tree/v0.9.0)
- github.com/google/cel-spec: [v0.6.0](https://github.com/google/cel-spec/tree/v0.6.0)
- github.com/gorilla/mux: [v1.8.0](https://github.com/gorilla/mux/tree/v1.8.0)
- github.com/kr/fs: [v0.1.0](https://github.com/kr/fs/tree/v0.1.0)
- github.com/pkg/sftp: [v1.10.1](https://github.com/pkg/sftp/tree/v1.10.1)
- sigs.k8s.io/json: 9f7c6b3
- sigs.k8s.io/sig-storage-lib-external-provisioner/v8: v8.0.0

### Changed
- cloud.google.com/go: v0.65.0 → v0.81.0
- github.com/benbjohnson/clock: [v1.0.3 → v1.1.0](https://github.com/benbjohnson/clock/compare/v1.0.3...v1.1.0)
- github.com/bketelsen/crypt: [5cbc8cc → v0.0.4](https://github.com/bketelsen/crypt/compare/5cbc8cc...v0.0.4)
- github.com/envoyproxy/go-control-plane: [668b12f → 63b5d3c](https://github.com/envoyproxy/go-control-plane/compare/668b12f...63b5d3c)
- github.com/evanphx/json-patch: [v4.11.0+incompatible → v4.12.0+incompatible](https://github.com/evanphx/json-patch/compare/v4.11.0...v4.12.0)
- github.com/go-logr/logr: [v0.4.0 → v1.2.1](https://github.com/go-logr/logr/compare/v0.4.0...v1.2.1)
- github.com/go-logr/zapr: [v0.2.0 → v1.2.0](https://github.com/go-logr/zapr/compare/v0.2.0...v1.2.0)
- github.com/golang/glog: [23def4e → v1.0.0](https://github.com/golang/glog/compare/23def4e...v1.0.0)
- github.com/golang/mock: [v1.4.4 → v1.5.0](https://github.com/golang/mock/compare/v1.4.4...v1.5.0)
- github.com/google/go-cmp: [v0.5.5 → v0.5.6](https://github.com/google/go-cmp/compare/v0.5.5...v0.5.6)
- github.com/google/martian/v3: [v3.0.0 → v3.1.0](https://github.com/google/martian/v3/compare/v3.0.0...v3.1.0)
- github.com/google/pprof: [1a94d86 → cbba55b](https://github.com/google/pprof/compare/1a94d86...cbba55b)
- github.com/ianlancetaylor/demangle: [5e5cf60 → 28f6c0f](https://github.com/ianlancetaylor/demangle/compare/5e5cf60...28f6c0f)
- github.com/json-iterator/go: [v1.1.11 → v1.1.12](https://github.com/json-iterator/go/compare/v1.1.11...v1.1.12)
- github.com/kr/pty: [v1.1.5 → v1.1.1](https://github.com/kr/pty/compare/v1.1.5...v1.1.1)
- github.com/magiconair/properties: [v1.8.1 → v1.8.5](https://github.com/magiconair/properties/compare/v1.8.1...v1.8.5)
- github.com/mattn/go-isatty: [v0.0.4 → v0.0.3](https://github.com/mattn/go-isatty/compare/v0.0.4...v0.0.3)
- github.com/mitchellh/mapstructure: [v1.1.2 → v1.4.1](https://github.com/mitchellh/mapstructure/compare/v1.1.2...v1.4.1)
- github.com/modern-go/reflect2: [v1.0.1 → v1.0.2](https://github.com/modern-go/reflect2/compare/v1.0.1...v1.0.2)
- github.com/pelletier/go-toml: [v1.2.0 → v1.9.3](https://github.com/pelletier/go-toml/compare/v1.2.0...v1.9.3)
- github.com/prometheus/common: [v0.26.0 → v0.28.0](https://github.com/prometheus/common/compare/v0.26.0...v0.28.0)
- github.com/spf13/afero: [v1.2.2 → v1.6.0](https://github.com/spf13/afero/compare/v1.2.2...v1.6.0)
- github.com/spf13/cast: [v1.3.0 → v1.3.1](https://github.com/spf13/cast/compare/v1.3.0...v1.3.1)
- github.com/spf13/cobra: [v1.1.3 → v1.2.1](https://github.com/spf13/cobra/compare/v1.1.3...v1.2.1)
- github.com/spf13/jwalterweatherman: [v1.0.0 → v1.1.0](https://github.com/spf13/jwalterweatherman/compare/v1.0.0...v1.1.0)
- github.com/spf13/viper: [v1.7.0 → v1.8.1](https://github.com/spf13/viper/compare/v1.7.0...v1.8.1)
- github.com/stretchr/objx: [v0.2.0 → v0.1.1](https://github.com/stretchr/objx/compare/v0.2.0...v0.1.1)
- github.com/yuin/goldmark: [v1.3.5 → v1.4.0](https://github.com/yuin/goldmark/compare/v1.3.5...v1.4.0)
- go.opencensus.io: v0.22.4 → v0.23.0
- go.uber.org/zap: v1.17.0 → v1.19.0
- golang.org/x/crypto: 513c2a4 → 32db794
- golang.org/x/net: 37e1c6a → 491a49a
- golang.org/x/oauth2: cd4f82c → 2bc19b1
- golang.org/x/sys: 59db8d7 → f4d4317
- golang.org/x/term: de623e6 → 6886f2d
- golang.org/x/text: v0.3.6 → v0.3.7
- golang.org/x/tools: v0.1.2 → d4cc65f
- google.golang.org/api: v0.30.0 → v0.44.0
- google.golang.org/genproto: f16073e → fe13028
- google.golang.org/grpc: v1.38.0 → v1.40.0
- google.golang.org/protobuf: v1.26.0 → v1.27.1
- gopkg.in/ini.v1: v1.51.0 → v1.62.0
- k8s.io/api: v0.22.0 → v0.23.0
- k8s.io/apiextensions-apiserver: v0.20.1 → v0.23.0
- k8s.io/apimachinery: v0.22.0 → v0.23.0
- k8s.io/apiserver: v0.22.0 → v0.23.0
- k8s.io/client-go: v0.22.0 → v0.23.0
- k8s.io/code-generator: v0.20.1 → v0.23.0
- k8s.io/component-base: v0.22.0 → v0.23.0
- k8s.io/component-helpers: v0.22.0 → v0.23.0
- k8s.io/csi-translation-lib: v0.22.0 → 135d37b
- k8s.io/gengo: 83324d8 → 485abfe
- k8s.io/klog/v2: v2.9.0 → v2.30.0
- k8s.io/kube-openapi: 9528897 → e816edb
- k8s.io/utils: 4b05e18 → 7d6a63d
- sigs.k8s.io/apiserver-network-proxy/konnectivity-client: v0.0.22 → v0.0.25
- sigs.k8s.io/structured-merge-diff/v4: v4.1.2 → v4.2.0

### Removed
- github.com/docker/spdystream: [449fdfc](https://github.com/docker/spdystream/tree/449fdfc)
- github.com/go-openapi/spec: [v0.19.3](https://github.com/go-openapi/spec/tree/v0.19.3)
- github.com/mattn/go-runewidth: [v0.0.2](https://github.com/mattn/go-runewidth/tree/v0.0.2)
- github.com/olekukonko/tablewriter: [a0225b3](https://github.com/olekukonko/tablewriter/tree/a0225b3)
- github.com/urfave/cli: [v1.20.0](https://github.com/urfave/cli/tree/v1.20.0)
- go.etcd.io/etcd: dd1b699
- gopkg.in/cheggaaa/pb.v1: v1.0.25
- gotest.tools: v2.2.0+incompatible
- sigs.k8s.io/sig-storage-lib-external-provisioner/v7: v7.0.1
