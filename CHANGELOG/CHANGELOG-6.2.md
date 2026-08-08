# Release notes for 6.2.1

[Documentation](https://kubernetes-csi.github.io)

# Changelog since 6.2.0

## Changes by Kind

### Bug or Regression

- Add controller to clean up orphaned snapshot source-protection finalizers that can block snapshot deletion after provisioner crashes or PVC deletions. ([#1524](https://github.com/kubernetes-csi/external-provisioner/pull/1524), [@ConnorJC3](https://github.com/ConnorJC3))

## Dependencies

### Added
_Nothing has changed._

### Changed
_Nothing has changed._

### Removed
_Nothing has changed._

# Release notes for 6.2.0

[Documentation](https://kubernetes-csi.github.io)

# Changelog since 6.1.0

## Changes by Kind

### Bug or Regression

- A StorageClass can use `csi.storage.k8s.io/controller-modify-secret-name` and `csi.storage.k8s.io/controller-modify-secret-namespace` to reference the credentials that should be used to modify a volume according to the parameters of a VolumeAttributeClass. ([#1440](https://github.com/kubernetes-csi/external-provisioner/pull/1440), [@nixpanic](https://github.com/nixpanic))
- Add provisioner.storage.kubernetes.io/volumesnapshot-as-source-protection finalizer on VolumeSnapshot as Source. Add rbac rules to watch/update volumesnapshots. The external=provisioner is able to work without these new RBAC permissions, but we strongly encourage the CSI driver vendors to update them. ([#1458](https://github.com/kubernetes-csi/external-provisioner/pull/1458), [@xing-yang](https://github.com/xing-yang))
- Allow provisioning to proceed when snapshot is being deleted to prevent leaking volumes and snapshots. ([#1448](https://github.com/kubernetes-csi/external-provisioner/pull/1448), [@xing-yang](https://github.com/xing-yang))
- Fixed a bug where retries could cause volumes to be provisioned in the wrong availability zone. ([#1466](https://github.com/kubernetes-csi/external-provisioner/pull/1466), [@torredil](https://github.com/torredil))
- Updated go version to fix CVE-2025-68121. ([#1467](https://github.com/kubernetes-csi/external-provisioner/pull/1467), [@jsafrane](https://github.com/jsafrane))

### Other (Cleanup or Flake)

- Bump k8s dependencies to v1.35.2 ([#1460](https://github.com/kubernetes-csi/external-provisioner/pull/1460), [@dfajmon](https://github.com/dfajmon))
- Remove general availability feature gate HonorPVReclaimPolicy ([#1476](https://github.com/kubernetes-csi/external-provisioner/pull/1476), [@carlory](https://github.com/carlory))

## Dependencies

### Added
- cyphar.com/go-pathrs: v0.2.3
- github.com/Masterminds/semver/v3: [v3.4.0](https://github.com/Masterminds/semver/tree/v3.4.0)
- github.com/cenkalti/backoff/v5: [v5.0.3](https://github.com/cenkalti/backoff/tree/v5.0.3)
- github.com/gkampitakis/ciinfo: [v0.3.2](https://github.com/gkampitakis/ciinfo/tree/v0.3.2)
- github.com/gkampitakis/go-diff: [v1.3.2](https://github.com/gkampitakis/go-diff/tree/v1.3.2)
- github.com/gkampitakis/go-snaps: [v0.5.15](https://github.com/gkampitakis/go-snaps/tree/v0.5.15)
- github.com/go-openapi/swag/jsonutils/fixtures_test: [v0.25.4](https://github.com/go-openapi/swag/tree/jsonutils/fixtures_test/v0.25.4)
- github.com/go-openapi/testify/enable/yaml/v2: [v2.0.2](https://github.com/go-openapi/testify/tree/enable/yaml/v2/v2.0.2)
- github.com/go-openapi/testify/v2: [v2.0.2](https://github.com/go-openapi/testify/tree/v2.0.2)
- github.com/joshdk/go-junit: [v1.0.0](https://github.com/joshdk/go-junit/tree/v1.0.0)
- github.com/maruel/natural: [v1.1.1](https://github.com/maruel/natural/tree/v1.1.1)
- github.com/mfridman/tparse: [v0.18.0](https://github.com/mfridman/tparse/tree/v0.18.0)
- github.com/tidwall/gjson: [v1.18.0](https://github.com/tidwall/gjson/tree/v1.18.0)
- github.com/tidwall/match: [v1.1.1](https://github.com/tidwall/match/tree/v1.1.1)
- github.com/tidwall/pretty: [v1.2.1](https://github.com/tidwall/pretty/tree/v1.2.1)
- github.com/tidwall/sjson: [v1.2.5](https://github.com/tidwall/sjson/tree/v1.2.5)

### Changed
- cel.dev/expr: v0.24.0 → v0.25.1
- cloud.google.com/go/compute/metadata: v0.7.0 → v0.9.0
- github.com/GoogleCloudPlatform/opentelemetry-operations-go/detectors/gcp: [v1.29.0 → v1.30.0](https://github.com/GoogleCloudPlatform/opentelemetry-operations-go/compare/detectors/gcp/v1.29.0...detectors/gcp/v1.30.0)
- github.com/Microsoft/hnslib: [v0.1.1 → v0.1.2](https://github.com/Microsoft/hnslib/compare/v0.1.1...v0.1.2)
- github.com/alecthomas/units: [b94a6e3 → 0f3dac3](https://github.com/alecthomas/units/compare/b94a6e3...0f3dac3)
- github.com/cncf/xds/go: [2ac532f → ee656c7](https://github.com/cncf/xds/compare/2ac532f...ee656c7)
- github.com/containerd/containerd/api: [v1.8.0 → v1.9.0](https://github.com/containerd/containerd/compare/api/v1.8.0...api/v1.9.0)
- github.com/containerd/ttrpc: [v1.2.6 → v1.2.7](https://github.com/containerd/ttrpc/compare/v1.2.6...v1.2.7)
- github.com/containerd/typeurl/v2: [v2.2.2 → v2.2.3](https://github.com/containerd/typeurl/compare/v2.2.2...v2.2.3)
- github.com/coredns/corefile-migration: [v1.0.26 → v1.0.29](https://github.com/coredns/corefile-migration/compare/v1.0.26...v1.0.29)
- github.com/coreos/go-systemd/v22: [v22.5.0 → v22.7.0](https://github.com/coreos/go-systemd/compare/v22.5.0...v22.7.0)
- github.com/cyphar/filepath-securejoin: [v0.4.1 → v0.6.1](https://github.com/cyphar/filepath-securejoin/compare/v0.4.1...v0.6.1)
- github.com/envoyproxy/go-control-plane/envoy: [v1.32.4 → v1.36.0](https://github.com/envoyproxy/go-control-plane/compare/envoy/v1.32.4...envoy/v1.36.0)
- github.com/envoyproxy/go-control-plane: [v0.13.4 → v0.14.0](https://github.com/envoyproxy/go-control-plane/compare/v0.13.4...v0.14.0)
- github.com/envoyproxy/protoc-gen-validate: [v1.2.1 → v1.3.0](https://github.com/envoyproxy/protoc-gen-validate/compare/v1.2.1...v1.3.0)
- github.com/go-jose/go-jose/v4: [v4.1.1 → v4.1.3](https://github.com/go-jose/go-jose/compare/v4.1.1...v4.1.3)
- github.com/go-openapi/jsonpointer: [v0.22.0 → v0.22.4](https://github.com/go-openapi/jsonpointer/compare/v0.22.0...v0.22.4)
- github.com/go-openapi/jsonreference: [v0.21.1 → v0.21.4](https://github.com/go-openapi/jsonreference/compare/v0.21.1...v0.21.4)
- github.com/go-openapi/swag/cmdutils: [v0.24.0 → v0.25.4](https://github.com/go-openapi/swag/compare/cmdutils/v0.24.0...cmdutils/v0.25.4)
- github.com/go-openapi/swag/conv: [v0.24.0 → v0.25.4](https://github.com/go-openapi/swag/compare/conv/v0.24.0...conv/v0.25.4)
- github.com/go-openapi/swag/fileutils: [v0.24.0 → v0.25.4](https://github.com/go-openapi/swag/compare/fileutils/v0.24.0...fileutils/v0.25.4)
- github.com/go-openapi/swag/jsonname: [v0.24.0 → v0.25.4](https://github.com/go-openapi/swag/compare/jsonname/v0.24.0...jsonname/v0.25.4)
- github.com/go-openapi/swag/jsonutils: [v0.24.0 → v0.25.4](https://github.com/go-openapi/swag/compare/jsonutils/v0.24.0...jsonutils/v0.25.4)
- github.com/go-openapi/swag/loading: [v0.24.0 → v0.25.4](https://github.com/go-openapi/swag/compare/loading/v0.24.0...loading/v0.25.4)
- github.com/go-openapi/swag/mangling: [v0.24.0 → v0.25.4](https://github.com/go-openapi/swag/compare/mangling/v0.24.0...mangling/v0.25.4)
- github.com/go-openapi/swag/netutils: [v0.24.0 → v0.25.4](https://github.com/go-openapi/swag/compare/netutils/v0.24.0...netutils/v0.25.4)
- github.com/go-openapi/swag/stringutils: [v0.24.0 → v0.25.4](https://github.com/go-openapi/swag/compare/stringutils/v0.24.0...stringutils/v0.25.4)
- github.com/go-openapi/swag/typeutils: [v0.24.0 → v0.25.4](https://github.com/go-openapi/swag/compare/typeutils/v0.24.0...typeutils/v0.25.4)
- github.com/go-openapi/swag/yamlutils: [v0.24.0 → v0.25.4](https://github.com/go-openapi/swag/compare/yamlutils/v0.24.0...yamlutils/v0.25.4)
- github.com/go-openapi/swag: [v0.24.1 → v0.25.4](https://github.com/go-openapi/swag/compare/v0.24.1...v0.25.4)
- github.com/golang-jwt/jwt/v5: [v5.2.2 → v5.3.0](https://github.com/golang-jwt/jwt/compare/v5.2.2...v5.3.0)
- github.com/google/cadvisor: [v0.52.1 → v0.53.0](https://github.com/google/cadvisor/compare/v0.52.1...v0.53.0)
- github.com/google/cel-go: [v0.26.0 → v0.27.0](https://github.com/google/cel-go/compare/v0.26.0...v0.27.0)
- github.com/google/gnostic-models: [v0.7.0 → v0.7.1](https://github.com/google/gnostic-models/compare/v0.7.0...v0.7.1)
- github.com/google/pprof: [27863c8 → cb029da](https://github.com/google/pprof/compare/27863c8...cb029da)
- github.com/grpc-ecosystem/grpc-gateway/v2: [v2.26.3 → v2.28.0](https://github.com/grpc-ecosystem/grpc-gateway/compare/v2.26.3...v2.28.0)
- github.com/ianlancetaylor/demangle: [bd984b5 → f615e6b](https://github.com/ianlancetaylor/demangle/compare/bd984b5...f615e6b)
- github.com/kubernetes-csi/csi-lib-utils: [v0.22.0 → v0.23.2](https://github.com/kubernetes-csi/csi-lib-utils/compare/v0.22.0...v0.23.2)
- github.com/kubernetes-csi/external-snapshotter/client/v8: [v8.2.0 → v8.4.0](https://github.com/kubernetes-csi/external-snapshotter/compare/client/v8/v8.2.0...client/v8/v8.4.0)
- github.com/mailru/easyjson: [v0.9.0 → v0.9.1](https://github.com/mailru/easyjson/compare/v0.9.0...v0.9.1)
- github.com/miekg/dns: [v1.1.68 → v1.1.72](https://github.com/miekg/dns/compare/v1.1.68...v1.1.72)
- github.com/onsi/ginkgo/v2: [v2.23.4 → v2.28.1](https://github.com/onsi/ginkgo/compare/v2.23.4...v2.28.1)
- github.com/onsi/gomega: [v1.37.0 → v1.39.1](https://github.com/onsi/gomega/compare/v1.37.0...v1.39.1)
- github.com/opencontainers/cgroups: [v0.0.1 → v0.0.3](https://github.com/opencontainers/cgroups/compare/v0.0.1...v0.0.3)
- github.com/opencontainers/runtime-spec: [v1.2.0 → v1.2.1](https://github.com/opencontainers/runtime-spec/compare/v1.2.0...v1.2.1)
- github.com/opencontainers/selinux: [v1.12.0 → v1.13.1](https://github.com/opencontainers/selinux/compare/v1.12.0...v1.13.1)
- github.com/prometheus/common: [v0.66.1 → v0.67.5](https://github.com/prometheus/common/compare/v0.66.1...v0.67.5)
- github.com/prometheus/procfs: [v0.17.0 → v0.20.0](https://github.com/prometheus/procfs/compare/v0.17.0...v0.20.0)
- github.com/rogpeppe/go-internal: [v1.13.1 → v1.14.1](https://github.com/rogpeppe/go-internal/compare/v1.13.1...v1.14.1)
- github.com/spf13/cobra: [v1.9.1 → v1.10.2](https://github.com/spf13/cobra/compare/v1.9.1...v1.10.2)
- github.com/spf13/pflag: [v1.0.7 → v1.0.10](https://github.com/spf13/pflag/compare/v1.0.7...v1.0.10)
- github.com/spiffe/go-spiffe/v2: [v2.5.0 → v2.6.0](https://github.com/spiffe/go-spiffe/compare/v2.5.0...v2.6.0)
- go.etcd.io/bbolt: v1.4.2 → v1.4.3
- go.etcd.io/etcd/api/v3: v3.6.4 → v3.6.8
- go.etcd.io/etcd/client/pkg/v3: v3.6.4 → v3.6.8
- go.etcd.io/etcd/client/v3: v3.6.4 → v3.6.8
- go.etcd.io/etcd/pkg/v3: v3.6.4 → v3.6.5
- go.etcd.io/etcd/server/v3: v3.6.4 → v3.6.5
- go.opentelemetry.io/auto/sdk: v1.1.0 → v1.2.1
- go.opentelemetry.io/contrib/detectors/gcp: v1.36.0 → v1.39.0
- go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc: v0.60.0 → v0.65.0
- go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp: v0.60.0 → v0.65.0
- go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc: v1.35.0 → v1.40.0
- go.opentelemetry.io/otel/exporters/otlp/otlptrace: v1.35.0 → v1.40.0
- go.opentelemetry.io/otel/metric: v1.37.0 → v1.40.0
- go.opentelemetry.io/otel/sdk/metric: v1.37.0 → v1.40.0
- go.opentelemetry.io/otel/sdk: v1.37.0 → v1.40.0
- go.opentelemetry.io/otel/trace: v1.37.0 → v1.40.0
- go.opentelemetry.io/otel: v1.37.0 → v1.40.0
- go.opentelemetry.io/proto/otlp: v1.6.0 → v1.9.0
- go.uber.org/zap: v1.27.0 → v1.27.1
- go.yaml.in/yaml/v2: v2.4.2 → v2.4.3
- golang.org/x/crypto: v0.41.0 → v0.48.0
- golang.org/x/exp: 7588d65 → 3dfff04
- golang.org/x/mod: v0.27.0 → v0.33.0
- golang.org/x/net: v0.43.0 → v0.51.0
- golang.org/x/oauth2: v0.30.0 → v0.35.0
- golang.org/x/sync: v0.16.0 → v0.19.0
- golang.org/x/sys: v0.35.0 → v0.41.0
- golang.org/x/telemetry: 1a19826 → e7419c6
- golang.org/x/term: v0.34.0 → v0.40.0
- golang.org/x/text: v0.28.0 → v0.34.0
- golang.org/x/time: v0.12.0 → v0.14.0
- golang.org/x/tools: v0.36.0 → v0.42.0
- google.golang.org/genproto/googleapis/api: 8d1bb00 → a57be14
- google.golang.org/genproto/googleapis/rpc: ef028d9 → a57be14
- google.golang.org/grpc: v1.75.1 → v1.79.1
- google.golang.org/protobuf: v1.36.8 → v1.36.11
- k8s.io/api: v0.34.0 → v0.35.2
- k8s.io/apiextensions-apiserver: v0.34.0 → v0.35.2
- k8s.io/apimachinery: v0.34.1 → v0.35.2
- k8s.io/apiserver: v0.34.1 → v0.35.2
- k8s.io/cli-runtime: v0.34.0 → v0.35.2
- k8s.io/client-go: v0.34.1 → v0.35.2
- k8s.io/cloud-provider: v0.34.0 → v0.35.2
- k8s.io/cluster-bootstrap: v0.34.0 → v0.35.2
- k8s.io/code-generator: v0.34.0 → v0.35.2
- k8s.io/component-base: v0.34.0 → v0.35.2
- k8s.io/component-helpers: v0.34.0 → v0.35.2
- k8s.io/controller-manager: v0.34.0 → v0.35.2
- k8s.io/cri-api: v0.34.1 → v0.35.2
- k8s.io/cri-client: v0.34.0 → v0.35.2
- k8s.io/csi-translation-lib: v0.34.0 → v0.35.2
- k8s.io/dynamic-resource-allocation: v0.34.0 → v0.35.2
- k8s.io/endpointslice: v0.34.0 → v0.35.2
- k8s.io/externaljwt: v0.34.1 → v0.35.2
- k8s.io/gengo/v2: c297c0c → ec3ebc5
- k8s.io/kms: v0.34.0 → v0.35.2
- k8s.io/kube-aggregator: v0.34.0 → v0.35.2
- k8s.io/kube-controller-manager: v0.34.0 → v0.35.2
- k8s.io/kube-openapi: 7fc2783 → a19766b
- k8s.io/kube-proxy: v0.34.0 → v0.35.2
- k8s.io/kube-scheduler: v0.34.0 → v0.35.2
- k8s.io/kubectl: v0.34.0 → v0.35.2
- k8s.io/kubelet: v0.34.0 → v0.35.2
- k8s.io/kubernetes: v1.34.1 → v1.35.2
- k8s.io/metrics: v0.34.0 → v0.35.2
- k8s.io/mount-utils: v0.34.1 → v0.35.2
- k8s.io/pod-security-admission: v0.34.0 → v0.35.2
- k8s.io/sample-apiserver: v0.34.0 → v0.35.2
- k8s.io/system-validators: v1.10.1 → v1.12.1
- k8s.io/utils: 0af2bda → b8788ab
- sigs.k8s.io/apiserver-network-proxy/konnectivity-client: v0.33.0 → v0.34.0
- sigs.k8s.io/controller-runtime: v0.22.3 → v0.23.1
- sigs.k8s.io/gateway-api: v1.4.0 → v1.5.0
- sigs.k8s.io/structured-merge-diff/v4: v4.6.0 → v4.4.1
- sigs.k8s.io/structured-merge-diff/v6: v6.3.0 → v6.3.2

### Removed
- github.com/Masterminds/goutils: [v1.1.1](https://github.com/Masterminds/goutils/tree/v1.1.1)
- github.com/Masterminds/semver: [v1.5.0](https://github.com/Masterminds/semver/tree/v1.5.0)
- github.com/Masterminds/sprig: [v2.22.0+incompatible](https://github.com/Masterminds/sprig/tree/v2.22.0)
- github.com/elastic/crd-ref-docs: [v0.2.0](https://github.com/elastic/crd-ref-docs/tree/v0.2.0)
- github.com/fatih/color: [v1.18.0](https://github.com/fatih/color/tree/v1.18.0)
- github.com/gobuffalo/flect: [v1.0.3](https://github.com/gobuffalo/flect/tree/v1.0.3)
- github.com/huandu/xstrings: [v1.3.3](https://github.com/huandu/xstrings/tree/v1.3.3)
- github.com/imdario/mergo: [v0.3.11](https://github.com/imdario/mergo/tree/v0.3.11)
- github.com/mattn/go-colorable: [v0.1.13](https://github.com/mattn/go-colorable/tree/v0.1.13)
- github.com/mattn/go-isatty: [v0.0.20](https://github.com/mattn/go-isatty/tree/v0.0.20)
- github.com/mitchellh/copystructure: [v1.2.0](https://github.com/mitchellh/copystructure/tree/v1.2.0)
- github.com/mitchellh/reflectwalk: [v1.0.2](https://github.com/mitchellh/reflectwalk/tree/v1.0.2)
- github.com/zeebo/errs: [v1.4.0](https://github.com/zeebo/errs/tree/v1.4.0)
- google.golang.org/grpc/cmd/protoc-gen-go-grpc: v1.5.1
- sigs.k8s.io/controller-tools: v0.19.0
