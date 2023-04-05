# Release notes for v3.2.2

# Changelog since v3.2.1

### Bug or Regression

- Update sig-storage-lib-external-provisioner to v8.0.1 to fix stuck pod scheduled to a non exist node([#905](https://github.com/kubernetes-csi/external-provisioner/pull/905), [@sunnylovestiramisu])

# Release notes for v3.2.1

[Documentation](https://kubernetes-csi.github.io)

# Changelog since v3.2.0

## Changes by Kind

### Bug or Regression
 - The CSIStorageCapacity version check will now use the namespace specified in the NAMESPACE env variable. This avoids a fatal `unexpected error when checking for the V1 CSIStorageCapacity API` error during startup. ([#753](https://github.com/kubernetes-csi/external-provisioner/pull/753), [@nbalacha](https://github.com/nbalacha))

## Dependencies

### Added
_Nothing has changed._

### Changed
_Nothing has changed._

### Removed
_Nothing has changed._

# Changelog since v3.1.0

## Changes by Kind

### Feature
 - Added an option to enable pprof profiling with --enable-pprof. The profiling server port can be configured using --http-endpoint. ([#682](https://github.com/kubernetes-csi/external-provisioner/pull/682), [@aramase](https://github.com/aramase))
 - Provisioner changes to prevent unauthorised volume mode conversion ([#726](https://github.com/kubernetes-csi/external-provisioner/pull/726), [@RaunakShah](https://github.com/RaunakShah))
 - QPS and burst can be set separately for storage capacity tracking. The defaults are lower than before to reduce the risk of overloading the apiserver. ([#711](https://github.com/kubernetes-csi/external-provisioner/pull/711), [@pohly](https://github.com/pohly))
 - Previous to this release, if the SC does not exist at time of PVC deletion where provisioner need credentials to delete volume which was passed as SC parameter at creation time, the PV deletion will fail as it can not fetch credentials by reading the SC from the API server. However this workflow is improved from this release which allows the PV deletion to succeed even if SC was deleted or does not exist at time of PV deletion. ([#713](https://github.com/kubernetes-csi/external-provisioner/pull/713), [@humblec](https://github.com/humblec))
 - Users will be able to clone a volume with a different Storage Class. The destination volume no longer has to be the same storage class as the source. ([#699](https://github.com/kubernetes-csi/external-provisioner/pull/699), [@amacaskill](https://github.com/amacaskill))

### Documentation
 - Updates the documentation / README file. ([#722](https://github.com/kubernetes-csi/external-provisioner/pull/722), [@gregth](https://github.com/gregth))

### Bug or Regression
 - Certain informer delete events were not handled correctly. ([#745](https://github.com/kubernetes-csi/external-provisioner/pull/745), [@pohly](https://github.com/pohly))
 - Ensure that the identifier for the lease holder during leader election is unique. ([#690](https://github.com/kubernetes-csi/external-provisioner/pull/690), [@SunnyFenng](https://github.com/SunnyFenng))
 - Storage capacity: managed-by label potentially too long with a long node name and enableNodeDeployment set to true ([#717](https://github.com/kubernetes-csi/external-provisioner/pull/717), [@awels](https://github.com/awels))
 - Storage capacity: maximumVolumeSize was not updated when it changed ([#696](https://github.com/kubernetes-csi/external-provisioner/pull/696), [@mrpre](https://github.com/mrpre))

### Other (Cleanup or Flake)
 - Upgrade external-snapshotter client to v6.0.1 ([#738](https://github.com/kubernetes-csi/external-provisioner/pull/738), [@RaunakShah](https://github.com/RaunakShah))
 - When running on a Kubernetes cluster where the v1 CSIStorageCapacity API is available, external-provisioner automatically switches to that version instead of using the deprecated v1beta1 API. Kubernetes 1.24 will mark the v1beta1 CSIStorageCapacity API as deprecated and introduces it as v1. ([#710](https://github.com/kubernetes-csi/external-provisioner/pull/710), [@pohly](https://github.com/pohly))
 - Kubernetes client dependencies are updated to v1.24.0. ([#733](https://github.com/kubernetes-csi/external-provisioner/pull/733), [@humblec](https://github.com/humblec))

## Dependencies

### Added
- github.com/armon/go-socks5: [e753329](https://github.com/armon/go-socks5/tree/e753329)
- github.com/blang/semver/v4: [v4.0.0](https://github.com/blang/semver/v4/tree/v4.0.0)
- github.com/buger/jsonparser: [v1.1.1](https://github.com/buger/jsonparser/tree/v1.1.1)
- github.com/flowstack/go-jsonschema: [v0.1.1](https://github.com/flowstack/go-jsonschema/tree/v0.1.1)
- github.com/google/gnostic: [v0.6.8](https://github.com/google/gnostic/tree/v0.6.8)
- github.com/kubernetes-csi/external-snapshotter/client/v6: [v6.0.1](https://github.com/kubernetes-csi/external-snapshotter/client/v6/tree/v6.0.1)
- github.com/xeipuuv/gojsonpointer: [4e3ac27](https://github.com/xeipuuv/gojsonpointer/tree/4e3ac27)
- github.com/xeipuuv/gojsonreference: [bd5ef7b](https://github.com/xeipuuv/gojsonreference/tree/bd5ef7b)
- github.com/xeipuuv/gojsonschema: [v1.2.0](https://github.com/xeipuuv/gojsonschema/tree/v1.2.0)

### Changed
- github.com/bketelsen/crypt: [v0.0.4 → 5cbc8cc](https://github.com/bketelsen/crypt/compare/v0.0.4...5cbc8cc)
- github.com/cespare/xxhash/v2: [v2.1.1 → v2.1.2](https://github.com/cespare/xxhash/v2/compare/v2.1.1...v2.1.2)
- github.com/cncf/udpa/go: [5459f2c → 04548b0](https://github.com/cncf/udpa/go/compare/5459f2c...04548b0)
- github.com/cncf/xds/go: [fbca930 → cb28da3](https://github.com/cncf/xds/go/compare/fbca930...cb28da3)
- github.com/container-storage-interface/spec: [v1.5.0 → v1.6.0](https://github.com/container-storage-interface/spec/compare/v1.5.0...v1.6.0)
- github.com/cpuguy83/go-md2man/v2: [v2.0.0 → v2.0.1](https://github.com/cpuguy83/go-md2man/v2/compare/v2.0.0...v2.0.1)
- github.com/emicklei/go-restful: [v2.9.5+incompatible → v2.15.0+incompatible](https://github.com/emicklei/go-restful/compare/v2.9.5...v2.15.0)
- github.com/envoyproxy/go-control-plane: [63b5d3c → cf90f65](https://github.com/envoyproxy/go-control-plane/compare/63b5d3c...cf90f65)
- github.com/evanphx/json-patch: [v4.12.0+incompatible → v5.6.0+incompatible](https://github.com/evanphx/json-patch/compare/v4.12.0...v5.6.0)
- github.com/fsnotify/fsnotify: [v1.4.9 → v1.5.1](https://github.com/fsnotify/fsnotify/compare/v1.4.9...v1.5.1)
- github.com/go-kit/log: [v0.1.0 → v0.2.0](https://github.com/go-kit/log/compare/v0.1.0...v0.2.0)
- github.com/go-logfmt/logfmt: [v0.5.0 → v0.5.1](https://github.com/go-logfmt/logfmt/compare/v0.5.0...v0.5.1)
- github.com/go-logr/logr: [v1.2.1 → v1.2.3](https://github.com/go-logr/logr/compare/v1.2.1...v1.2.3)
- github.com/go-openapi/jsonreference: [v0.19.5 → v0.19.6](https://github.com/go-openapi/jsonreference/compare/v0.19.5...v0.19.6)
- github.com/go-openapi/swag: [v0.19.14 → v0.21.1](https://github.com/go-openapi/swag/compare/v0.19.14...v0.21.1)
- github.com/google/cel-go: [v0.9.0 → v0.10.1](https://github.com/google/cel-go/compare/v0.9.0...v0.10.1)
- github.com/google/go-cmp: [v0.5.6 → v0.5.7](https://github.com/google/go-cmp/compare/v0.5.6...v0.5.7)
- github.com/google/uuid: [v1.2.0 → v1.3.0](https://github.com/google/uuid/compare/v1.2.0...v1.3.0)
- github.com/hashicorp/golang-lru: [v0.5.4 → v0.5.1](https://github.com/hashicorp/golang-lru/compare/v0.5.4...v0.5.1)
- github.com/kubernetes-csi/csi-lib-utils: [v0.10.0 → v0.11.0](https://github.com/kubernetes-csi/csi-lib-utils/compare/v0.10.0...v0.11.0)
- github.com/magiconair/properties: [v1.8.5 → v1.8.1](https://github.com/magiconair/properties/compare/v1.8.5...v1.8.1)
- github.com/mailru/easyjson: [v0.7.6 → v0.7.7](https://github.com/mailru/easyjson/compare/v0.7.6...v0.7.7)
- github.com/miekg/dns: [v1.1.40 → v1.1.48](https://github.com/miekg/dns/compare/v1.1.40...v1.1.48)
- github.com/moby/term: [9d4ed18 → 3f7ff69](https://github.com/moby/term/compare/9d4ed18...3f7ff69)
- github.com/nxadm/tail: [v1.4.4 → v1.4.8](https://github.com/nxadm/tail/compare/v1.4.4...v1.4.8)
- github.com/onsi/ginkgo: [v1.14.1 → v1.16.5](https://github.com/onsi/ginkgo/compare/v1.14.1...v1.16.5)
- github.com/onsi/gomega: [v1.10.2 → v1.17.0](https://github.com/onsi/gomega/compare/v1.10.2...v1.17.0)
- github.com/pelletier/go-toml: [v1.9.3 → v1.2.0](https://github.com/pelletier/go-toml/compare/v1.9.3...v1.2.0)
- github.com/prometheus/client_golang: [v1.11.0 → v1.12.1](https://github.com/prometheus/client_golang/compare/v1.11.0...v1.12.1)
- github.com/prometheus/common: [v0.28.0 → v0.33.0](https://github.com/prometheus/common/compare/v0.28.0...v0.33.0)
- github.com/prometheus/procfs: [v0.6.0 → v0.7.3](https://github.com/prometheus/procfs/compare/v0.6.0...v0.7.3)
- github.com/russross/blackfriday/v2: [v2.0.1 → v2.1.0](https://github.com/russross/blackfriday/v2/compare/v2.0.1...v2.1.0)
- github.com/spf13/cast: [v1.3.1 → v1.3.0](https://github.com/spf13/cast/compare/v1.3.1...v1.3.0)
- github.com/spf13/cobra: [v1.2.1 → v1.4.0](https://github.com/spf13/cobra/compare/v1.2.1...v1.4.0)
- github.com/spf13/jwalterweatherman: [v1.1.0 → v1.0.0](https://github.com/spf13/jwalterweatherman/compare/v1.1.0...v1.0.0)
- github.com/spf13/viper: [v1.8.1 → v1.7.0](https://github.com/spf13/viper/compare/v1.8.1...v1.7.0)
- github.com/yuin/goldmark: [v1.4.0 → v1.4.1](https://github.com/yuin/goldmark/compare/v1.4.0...v1.4.1)
- go.etcd.io/etcd/api/v3: v3.5.0 → v3.5.1
- go.etcd.io/etcd/client/pkg/v3: v3.5.0 → v3.5.1
- go.etcd.io/etcd/client/v3: v3.5.0 → v3.5.1
- go.uber.org/goleak: v1.1.10 → v1.1.12
- go.uber.org/zap: v1.19.0 → v1.19.1
- golang.org/x/crypto: 32db794 → 8634188
- golang.org/x/mod: v0.4.2 → 9b9b3d8
- golang.org/x/net: 491a49a → 749bd19
- golang.org/x/oauth2: 2bc19b1 → 6242fa9
- golang.org/x/sys: f4d4317 → 3f8b815
- golang.org/x/term: 6886f2d → 03fcf44
- golang.org/x/time: 1f47c86 → 0e9765c
- golang.org/x/tools: d4cc65f → v0.1.10
- gomodules.xyz/jsonpatch/v2: v2.1.0 → v2.2.0
- google.golang.org/api: v0.44.0 → v0.43.0
- google.golang.org/genproto: fe13028 → 9d70989
- google.golang.org/grpc: v1.40.0 → v1.45.0
- google.golang.org/protobuf: v1.27.1 → v1.28.0
- gopkg.in/ini.v1: v1.62.0 → v1.51.0
- gopkg.in/yaml.v3: 496545a → v3.0.1
- k8s.io/api: v0.23.0 → v0.24.0
- k8s.io/apiextensions-apiserver: v0.23.0 → v0.24.0
- k8s.io/apimachinery: v0.23.0 → v0.24.0
- k8s.io/apiserver: v0.23.0 → v0.24.0
- k8s.io/client-go: v0.23.0 → v0.24.0
- k8s.io/code-generator: v0.23.0 → v0.24.0
- k8s.io/component-base: v0.23.0 → v0.24.0
- k8s.io/component-helpers: v0.23.0 → v0.24.0
- k8s.io/csi-translation-lib: 135d37b → v0.24.0
- k8s.io/gengo: 485abfe → c02415c
- k8s.io/klog/v2: v2.30.0 → v2.60.1
- k8s.io/kube-openapi: e816edb → b28bf28
- k8s.io/utils: 7d6a63d → 3a6ce19
- sigs.k8s.io/apiserver-network-proxy/konnectivity-client: v0.0.25 → v0.0.30
- sigs.k8s.io/controller-runtime: v0.8.3 → v0.11.2
- sigs.k8s.io/structured-merge-diff/v4: v4.2.0 → v4.2.1
- sigs.k8s.io/yaml: v1.2.0 → v1.3.0

### Removed
- github.com/blang/semver: [v3.5.1+incompatible](https://github.com/blang/semver/tree/v3.5.1)
- github.com/kubernetes-csi/external-snapshotter/client/v4: [v4.1.0](https://github.com/kubernetes-csi/external-snapshotter/client/v4/tree/v4.1.0)
- go.uber.org/tools: 2cfd321
