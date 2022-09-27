# Release notes for v3.3.0

[Documentation](https://kubernetes-csi.github.io)

# Changelog since v3.2.0

## Changes by Kind

### Feature

- This release add the support for CSINodeExpandSecret feature (https://github.com/kubernetes/enhancements/issues/3107). The CSI drivers can make use of the secret as part of the nodeExpandVolume RPC call from now onwards. ([#758](https://github.com/kubernetes-csi/external-provisioner/pull/758), [@zhucan](https://github.com/zhucan))

### Documentation

- Updated README and design documentation. ([#757](https://github.com/kubernetes-csi/external-provisioner/pull/757), [@LetFu](https://github.com/LetFu))

### Bug or Regression

- The CSIStorageCapacity version check will now use the namespace specified in the NAMESPACE env variable. This avoids a fatal unexpected error when checking for the V1 CSIStorageCapacity API error during startup. ([#753](https://github.com/kubernetes-csi/external-provisioner/pull/753), [@nbalacha](https://github.com/nbalacha))

### Uncategorized

- This release update kubernetes module dependencies to v1.25 ([#776](https://github.com/kubernetes-csi/external-provisioner/pull/776), [@humblec](https://github.com/humblec))
- Update volumesnapshot client to 6.1.0 and K8s deps to 1.25.2 ([#789](https://github.com/kubernetes-csi/external-provisioner/pull/789), [@xing-yang](https://github.com/xing-yang))

## Dependencies

### Added
- github.com/emicklei/go-restful/v3: [v3.9.0](https://github.com/emicklei/go-restful/v3/tree/v3.9.0)
- github.com/evanphx/json-patch/v5: [v5.6.0](https://github.com/evanphx/json-patch/v5/tree/v5.6.0)
- github.com/go-task/slim-sprig: [348f09d](https://github.com/go-task/slim-sprig/tree/348f09d)
- github.com/golang-jwt/jwt/v4: [v4.2.0](https://github.com/golang-jwt/jwt/v4/tree/v4.2.0)
- github.com/golang/snappy: [v0.0.3](https://github.com/golang/snappy/tree/v0.0.3)
- github.com/jessevdk/go-flags: [v1.4.0](https://github.com/jessevdk/go-flags/tree/v1.4.0)
- github.com/kubernetes-csi/csi-test/v5: [v5.0.0](https://github.com/kubernetes-csi/csi-test/v5/tree/v5.0.0)
- github.com/onsi/ginkgo/v2: [v2.1.6](https://github.com/onsi/ginkgo/v2/tree/v2.1.6)
- google.golang.org/grpc/cmd/protoc-gen-go-grpc: v1.1.0

### Changed
- cloud.google.com/go: v0.81.0 → v0.97.0
- github.com/Azure/go-autorest/autorest/adal: [v0.9.13 → v0.9.20](https://github.com/Azure/go-autorest/autorest/adal/compare/v0.9.13...v0.9.20)
- github.com/Azure/go-autorest/autorest/mocks: [v0.4.1 → v0.4.2](https://github.com/Azure/go-autorest/autorest/mocks/compare/v0.4.1...v0.4.2)
- github.com/Azure/go-autorest/autorest: [v0.11.18 → v0.11.27](https://github.com/Azure/go-autorest/autorest/compare/v0.11.18...v0.11.27)
- github.com/envoyproxy/go-control-plane: [cf90f65 → 49ff273](https://github.com/envoyproxy/go-control-plane/compare/cf90f65...49ff273)
- github.com/fsnotify/fsnotify: [v1.5.1 → v1.5.4](https://github.com/fsnotify/fsnotify/compare/v1.5.1...v1.5.4)
- github.com/go-logr/zapr: [v1.2.0 → v1.2.3](https://github.com/go-logr/zapr/compare/v1.2.0...v1.2.3)
- github.com/go-openapi/jsonreference: [v0.19.6 → v0.20.0](https://github.com/go-openapi/jsonreference/compare/v0.19.6...v0.20.0)
- github.com/go-openapi/swag: [v0.21.1 → v0.22.3](https://github.com/go-openapi/swag/compare/v0.21.1...v0.22.3)
- github.com/golang/glog: [v1.0.0 → 23def4e](https://github.com/golang/glog/compare/v1.0.0...23def4e)
- github.com/golang/mock: [v1.5.0 → v1.6.0](https://github.com/golang/mock/compare/v1.5.0...v1.6.0)
- github.com/google/gnostic: [v0.6.8 → v0.6.9](https://github.com/google/gnostic/compare/v0.6.8...v0.6.9)
- github.com/google/go-cmp: [v0.5.7 → v0.5.8](https://github.com/google/go-cmp/compare/v0.5.7...v0.5.8)
- github.com/google/martian/v3: [v3.1.0 → v3.2.1](https://github.com/google/martian/v3/compare/v3.1.0...v3.2.1)
- github.com/google/pprof: [cbba55b → 4bb14d4](https://github.com/google/pprof/compare/cbba55b...4bb14d4)
- github.com/googleapis/gax-go/v2: [v2.0.5 → v2.1.0](https://github.com/googleapis/gax-go/v2/compare/v2.0.5...v2.1.0)
- github.com/kubernetes-csi/external-snapshotter/client/v6: [v6.0.1 → v6.1.0](https://github.com/kubernetes-csi/external-snapshotter/client/v6/compare/v6.0.1...v6.1.0)
- github.com/mitchellh/mapstructure: [v1.4.1 → v1.1.2](https://github.com/mitchellh/mapstructure/compare/v1.4.1...v1.1.2)
- github.com/onsi/gomega: [v1.17.0 → v1.20.1](https://github.com/onsi/gomega/compare/v1.17.0...v1.20.1)
- github.com/pquerna/cachecontrol: [0dec1b3 → v0.1.0](https://github.com/pquerna/cachecontrol/compare/0dec1b3...v0.1.0)
- github.com/prometheus/client_golang: [v1.12.1 → v1.13.0](https://github.com/prometheus/client_golang/compare/v1.12.1...v1.13.0)
- github.com/prometheus/common: [v0.33.0 → v0.37.0](https://github.com/prometheus/common/compare/v0.33.0...v0.37.0)
- github.com/prometheus/procfs: [v0.7.3 → v0.8.0](https://github.com/prometheus/procfs/compare/v0.7.3...v0.8.0)
- github.com/stretchr/objx: [v0.1.1 → v0.4.0](https://github.com/stretchr/objx/compare/v0.1.1...v0.4.0)
- github.com/stretchr/testify: [v1.7.0 → v1.8.0](https://github.com/stretchr/testify/compare/v1.7.0...v1.8.0)
- github.com/yuin/goldmark: [v1.4.1 → v1.4.13](https://github.com/yuin/goldmark/compare/v1.4.1...v1.4.13)
- go.etcd.io/etcd/api/v3: v3.5.1 → v3.5.4
- go.etcd.io/etcd/client/pkg/v3: v3.5.1 → v3.5.4
- go.etcd.io/etcd/client/v2: v2.305.0 → v2.305.4
- go.etcd.io/etcd/client/v3: v3.5.1 → v3.5.4
- go.etcd.io/etcd/pkg/v3: v3.5.0 → v3.5.4
- go.etcd.io/etcd/raft/v3: v3.5.0 → v3.5.4
- go.etcd.io/etcd/server/v3: v3.5.0 → v3.5.4
- go.uber.org/zap: v1.19.1 → v1.21.0
- golang.org/x/crypto: 8634188 → 3147a52
- golang.org/x/mod: 9b9b3d8 → 86c51ed
- golang.org/x/net: 749bd19 → bea034e
- golang.org/x/sync: 036812b → 886fb93
- golang.org/x/sys: 3f8b815 → fb04ddd
- golang.org/x/time: 0e9765c → 579cf78
- golang.org/x/tools: v0.1.10 → v0.1.12
- google.golang.org/api: v0.43.0 → v0.57.0
- google.golang.org/genproto: 9d70989 → c8bf987
- google.golang.org/grpc: v1.45.0 → v1.49.0
- google.golang.org/protobuf: v1.28.0 → v1.28.1
- gopkg.in/check.v1: 8fa4692 → 10cb982
- k8s.io/api: v0.24.0 → v0.25.2
- k8s.io/apiextensions-apiserver: v0.24.0 → v0.25.2
- k8s.io/apimachinery: v0.24.0 → v0.25.2
- k8s.io/apiserver: v0.24.0 → v0.25.2
- k8s.io/client-go: v0.24.0 → v0.25.2
- k8s.io/code-generator: v0.24.0 → v0.25.2
- k8s.io/component-base: v0.24.0 → v0.25.2
- k8s.io/component-helpers: v0.24.0 → v0.25.2
- k8s.io/csi-translation-lib: v0.24.0 → v0.25.2
- k8s.io/gengo: c02415c → 3913671
- k8s.io/klog/v2: v2.60.1 → v2.80.1
- k8s.io/kube-openapi: b28bf28 → a70c9af
- k8s.io/utils: 3a6ce19 → ee6ede2
- sigs.k8s.io/apiserver-network-proxy/konnectivity-client: v0.0.30 → v0.0.32
- sigs.k8s.io/controller-runtime: v0.11.2 → v0.13.0
- sigs.k8s.io/json: 9f7c6b3 → f223a00
- sigs.k8s.io/structured-merge-diff/v4: v4.2.1 → v4.2.3

### Removed
- cloud.google.com/go/firestore: v1.1.0
- github.com/antlr/antlr4/runtime/Go/antlr: [b48c857](https://github.com/antlr/antlr4/runtime/Go/antlr/tree/b48c857)
- github.com/armon/circbuf: [bbbad09](https://github.com/armon/circbuf/tree/bbbad09)
- github.com/armon/go-metrics: [f0300d1](https://github.com/armon/go-metrics/tree/f0300d1)
- github.com/armon/go-radix: [7fddfc3](https://github.com/armon/go-radix/tree/7fddfc3)
- github.com/bgentry/speakeasy: [v0.1.0](https://github.com/bgentry/speakeasy/tree/v0.1.0)
- github.com/bketelsen/crypt: [5cbc8cc](https://github.com/bketelsen/crypt/tree/5cbc8cc)
- github.com/certifi/gocertifi: [2c3bb06](https://github.com/certifi/gocertifi/tree/2c3bb06)
- github.com/cockroachdb/datadriven: [bf6692d](https://github.com/cockroachdb/datadriven/tree/bf6692d)
- github.com/cockroachdb/errors: [v1.2.4](https://github.com/cockroachdb/errors/tree/v1.2.4)
- github.com/cockroachdb/logtags: [eb05cc2](https://github.com/cockroachdb/logtags/tree/eb05cc2)
- github.com/coreos/bbolt: [v1.3.2](https://github.com/coreos/bbolt/tree/v1.3.2)
- github.com/coreos/etcd: [v3.3.13+incompatible](https://github.com/coreos/etcd/tree/v3.3.13)
- github.com/coreos/go-systemd: [95778df](https://github.com/coreos/go-systemd/tree/95778df)
- github.com/coreos/pkg: [399ea9e](https://github.com/coreos/pkg/tree/399ea9e)
- github.com/dgrijalva/jwt-go: [v3.2.0+incompatible](https://github.com/dgrijalva/jwt-go/tree/v3.2.0)
- github.com/dgryski/go-sip13: [e10d5fe](https://github.com/dgryski/go-sip13/tree/e10d5fe)
- github.com/emicklei/go-restful: [v2.15.0+incompatible](https://github.com/emicklei/go-restful/tree/v2.15.0)
- github.com/fatih/color: [v1.7.0](https://github.com/fatih/color/tree/v1.7.0)
- github.com/getsentry/raven-go: [v0.2.0](https://github.com/getsentry/raven-go/tree/v0.2.0)
- github.com/godbus/dbus/v5: [v5.0.4](https://github.com/godbus/dbus/v5/tree/v5.0.4)
- github.com/google/cel-go: [v0.10.1](https://github.com/google/cel-go/tree/v0.10.1)
- github.com/google/cel-spec: [v0.6.0](https://github.com/google/cel-spec/tree/v0.6.0)
- github.com/googleapis/gnostic: [v0.5.5](https://github.com/googleapis/gnostic/tree/v0.5.5)
- github.com/gopherjs/gopherjs: [0766667](https://github.com/gopherjs/gopherjs/tree/0766667)
- github.com/hashicorp/consul/api: [v1.1.0](https://github.com/hashicorp/consul/api/tree/v1.1.0)
- github.com/hashicorp/consul/sdk: [v0.1.1](https://github.com/hashicorp/consul/sdk/tree/v0.1.1)
- github.com/hashicorp/errwrap: [v1.0.0](https://github.com/hashicorp/errwrap/tree/v1.0.0)
- github.com/hashicorp/go-cleanhttp: [v0.5.1](https://github.com/hashicorp/go-cleanhttp/tree/v0.5.1)
- github.com/hashicorp/go-immutable-radix: [v1.0.0](https://github.com/hashicorp/go-immutable-radix/tree/v1.0.0)
- github.com/hashicorp/go-msgpack: [v0.5.3](https://github.com/hashicorp/go-msgpack/tree/v0.5.3)
- github.com/hashicorp/go-multierror: [v1.0.0](https://github.com/hashicorp/go-multierror/tree/v1.0.0)
- github.com/hashicorp/go-rootcerts: [v1.0.0](https://github.com/hashicorp/go-rootcerts/tree/v1.0.0)
- github.com/hashicorp/go-sockaddr: [v1.0.0](https://github.com/hashicorp/go-sockaddr/tree/v1.0.0)
- github.com/hashicorp/go-syslog: [v1.0.0](https://github.com/hashicorp/go-syslog/tree/v1.0.0)
- github.com/hashicorp/go-uuid: [v1.0.1](https://github.com/hashicorp/go-uuid/tree/v1.0.1)
- github.com/hashicorp/go.net: [v0.0.1](https://github.com/hashicorp/go.net/tree/v0.0.1)
- github.com/hashicorp/hcl: [v1.0.0](https://github.com/hashicorp/hcl/tree/v1.0.0)
- github.com/hashicorp/logutils: [v1.0.0](https://github.com/hashicorp/logutils/tree/v1.0.0)
- github.com/hashicorp/mdns: [v1.0.0](https://github.com/hashicorp/mdns/tree/v1.0.0)
- github.com/hashicorp/memberlist: [v0.1.3](https://github.com/hashicorp/memberlist/tree/v0.1.3)
- github.com/hashicorp/serf: [v0.8.2](https://github.com/hashicorp/serf/tree/v0.8.2)
- github.com/jtolds/gls: [v4.20.0+incompatible](https://github.com/jtolds/gls/tree/v4.20.0)
- github.com/kr/fs: [v0.1.0](https://github.com/kr/fs/tree/v0.1.0)
- github.com/kubernetes-csi/csi-test/v4: [v4.0.2](https://github.com/kubernetes-csi/csi-test/v4/tree/v4.0.2)
- github.com/magiconair/properties: [v1.8.1](https://github.com/magiconair/properties/tree/v1.8.1)
- github.com/mattn/go-colorable: [v0.0.9](https://github.com/mattn/go-colorable/tree/v0.0.9)
- github.com/mattn/go-isatty: [v0.0.3](https://github.com/mattn/go-isatty/tree/v0.0.3)
- github.com/mitchellh/cli: [v1.0.0](https://github.com/mitchellh/cli/tree/v1.0.0)
- github.com/mitchellh/go-homedir: [v1.1.0](https://github.com/mitchellh/go-homedir/tree/v1.1.0)
- github.com/mitchellh/go-testing-interface: [v1.0.0](https://github.com/mitchellh/go-testing-interface/tree/v1.0.0)
- github.com/mitchellh/gox: [v0.4.0](https://github.com/mitchellh/gox/tree/v0.4.0)
- github.com/mitchellh/iochan: [v1.0.0](https://github.com/mitchellh/iochan/tree/v1.0.0)
- github.com/oklog/ulid: [v1.3.1](https://github.com/oklog/ulid/tree/v1.3.1)
- github.com/opentracing/opentracing-go: [v1.1.0](https://github.com/opentracing/opentracing-go/tree/v1.1.0)
- github.com/pascaldekloe/goe: [57f6aae](https://github.com/pascaldekloe/goe/tree/57f6aae)
- github.com/pelletier/go-toml: [v1.2.0](https://github.com/pelletier/go-toml/tree/v1.2.0)
- github.com/pkg/sftp: [v1.10.1](https://github.com/pkg/sftp/tree/v1.10.1)
- github.com/posener/complete: [v1.1.1](https://github.com/posener/complete/tree/v1.1.1)
- github.com/prometheus/tsdb: [v0.7.1](https://github.com/prometheus/tsdb/tree/v0.7.1)
- github.com/robertkrimen/otto: [c382bd3](https://github.com/robertkrimen/otto/tree/c382bd3)
- github.com/ryanuber/columnize: [9b3edd6](https://github.com/ryanuber/columnize/tree/9b3edd6)
- github.com/sean-/seed: [e2103e2](https://github.com/sean-/seed/tree/e2103e2)
- github.com/shurcooL/sanitized_anchor_name: [v1.0.0](https://github.com/shurcooL/sanitized_anchor_name/tree/v1.0.0)
- github.com/smartystreets/assertions: [b2de0cb](https://github.com/smartystreets/assertions/tree/b2de0cb)
- github.com/smartystreets/goconvey: [v1.6.4](https://github.com/smartystreets/goconvey/tree/v1.6.4)
- github.com/spf13/cast: [v1.3.0](https://github.com/spf13/cast/tree/v1.3.0)
- github.com/spf13/jwalterweatherman: [v1.0.0](https://github.com/spf13/jwalterweatherman/tree/v1.0.0)
- github.com/spf13/viper: [v1.7.0](https://github.com/spf13/viper/tree/v1.7.0)
- github.com/subosito/gotenv: [v1.2.0](https://github.com/subosito/gotenv/tree/v1.2.0)
- gopkg.in/ini.v1: v1.51.0
- gopkg.in/resty.v1: v1.12.0
- gopkg.in/sourcemap.v1: v1.0.5
- k8s.io/klog: v1.0.0
