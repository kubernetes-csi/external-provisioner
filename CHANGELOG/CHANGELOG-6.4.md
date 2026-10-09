# Release notes for 6.4.0

[Documentation](https://kubernetes-csi.github.io)

# Changelog since 6.3.0

## Changes by Kind

### Feature

- Issue a `Normal` event instead of `Warning` when deleting a PV about to be detached. ([#1588](https://github.com/kubernetes-csi/external-provisioner/pull/1588), [@WanzenBug](https://github.com/WanzenBug))

### Bug or Regression

- Aggregate topology per registered key set for immediate binding.
  With immediate binding, topology requirements now cover every set of topology keys registered by the driver, rather than one set picked at random. Additionally (with both immediate and delayed binding) nodes are no longer included in topology requirements unless the driver registered those topology keys on that node. ([#1582](https://github.com/kubernetes-csi/external-provisioner/pull/1582), [@divyenpatel](https://github.com/divyenpatel))
- This PR send the volume_id instead of  PVC Name to the csi-driver in volumeCreate when datasource  is PVC ([#310](https://github.com/kubernetes-csi/external-provisioner/pull/310), [@Madhu-1](https://github.com/Madhu-1))

### Other (Cleanup or Flake)

- Removed support for v1beta1 CSIStorageCapacity API. It was removed in [Kubernetes 1.27](https://github.com/kubernetes/kubernetes/blob/master/CHANGELOG/CHANGELOG-1.27.md). The external-provisioner now uses only v1 API and thus requires at least Kubernetes 1.24, where the v1 API was introduced. ([#1573](https://github.com/kubernetes-csi/external-provisioner/pull/1573), [@jsafrane](https://github.com/jsafrane))

### Uncategorized

- Fixed potential leak of volumes after CSI driver timeouts. ([#319](https://github.com/kubernetes-csi/external-provisioner/pull/319), [@jsafrane](https://github.com/jsafrane))

## Dependencies

### Added
- cloud.google.com/go/auth: v0.20.0
- github.com/BurntSushi/toml: [a339e1f](https://github.com/BurntSushi/toml/tree/a339e1f)
- github.com/apapsch/go-jsonmerge/v2: [v2.0.0](https://github.com/apapsch/go-jsonmerge/tree/v2.0.0)
- github.com/go-openapi/analysis: [v1.0.0](https://github.com/go-openapi/analysis/tree/v1.0.0)
- github.com/go-openapi/errors: [v0.22.9](https://github.com/go-openapi/errors/tree/v0.22.9)
- github.com/go-openapi/loads: [v0.25.2](https://github.com/go-openapi/loads/tree/v0.25.2)
- github.com/go-openapi/runtime/server-middleware: [v0.33.2](https://github.com/go-openapi/runtime/tree/server-middleware/v0.33.2)
- github.com/go-openapi/runtime: [v0.33.2](https://github.com/go-openapi/runtime/tree/v0.33.2)
- github.com/go-openapi/spec: [v1.0.0](https://github.com/go-openapi/spec/tree/v1.0.0)
- github.com/go-openapi/strfmt: [v0.27.2](https://github.com/go-openapi/strfmt/tree/v0.27.2)
- github.com/go-openapi/swag/pools: [v0.29.2](https://github.com/go-openapi/swag/tree/pools/v0.29.2)
- github.com/go-openapi/validate: [v1.0.0](https://github.com/go-openapi/validate/tree/v1.0.0)
- github.com/go-viper/mapstructure/v2: [v2.5.0](https://github.com/go-viper/mapstructure/tree/v2.5.0)
- github.com/google/s2a-go: [v0.1.9](https://github.com/google/s2a-go/tree/v0.1.9)
- github.com/googleapis/enterprise-certificate-proxy: [v0.3.15](https://github.com/googleapis/enterprise-certificate-proxy/tree/v0.3.15)
- github.com/googleapis/gax-go/v2: [v2.22.0](https://github.com/googleapis/gax-go/tree/v2.22.0)
- github.com/oapi-codegen/runtime: [v1.7.0](https://github.com/oapi-codegen/runtime/tree/v1.7.0)
- github.com/oklog/ulid/v2: [v2.1.2](https://github.com/oklog/ulid/tree/v2.1.2)
- go.opentelemetry.io/otel/exporters/stdout/stdouttrace: v1.46.0
- go.uber.org/mock: v0.6.0
- golang.org/x/exp/typeparams: 2478ac8
- google.golang.org/api: v0.278.0
- honnef.co/go/tools: v0.8.1

### Changed
- buf.build/gen/go/bufbuild/protovalidate/protocolbuffers/go: 8976f5b → 52f3232
- buf.build/go/protovalidate: v0.12.0 → v1.0.0
- cel.dev/expr: v0.25.2 → v0.25.3
- cyphar.com/go-pathrs: v0.2.4 → v0.2.6
- github.com/GoogleCloudPlatform/opentelemetry-operations-go/detectors/gcp: [v1.31.0 → v1.34.0](https://github.com/GoogleCloudPlatform/opentelemetry-operations-go/compare/detectors/gcp/v1.31.0...detectors/gcp/v1.34.0)
- github.com/container-storage-interface/spec: [v1.12.0 → v1.13.0](https://github.com/container-storage-interface/spec/compare/v1.12.0...v1.13.0)
- github.com/cyphar/filepath-securejoin: [v0.6.1 → v0.7.0](https://github.com/cyphar/filepath-securejoin/compare/v0.6.1...v0.7.0)
- github.com/felixge/httpsnoop: [v1.0.4 → v1.1.0](https://github.com/felixge/httpsnoop/compare/v1.0.4...v1.1.0)
- github.com/fxamacker/cbor/v2: [v2.9.2 → v2.9.4](https://github.com/fxamacker/cbor/compare/v2.9.2...v2.9.4)
- github.com/go-logr/logr: [v1.4.3 → v1.4.4](https://github.com/go-logr/logr/compare/v1.4.3...v1.4.4)
- github.com/go-openapi/jsonpointer: [v0.23.1 → v1.0.2](https://github.com/go-openapi/jsonpointer/compare/v0.23.1...v1.0.2)
- github.com/go-openapi/jsonreference: [v0.21.6 → v1.0.3](https://github.com/go-openapi/jsonreference/compare/v0.21.6...v1.0.3)
- github.com/go-openapi/swag/cmdutils: [v0.26.0 → v0.29.2](https://github.com/go-openapi/swag/compare/cmdutils/v0.26.0...cmdutils/v0.29.2)
- github.com/go-openapi/swag/conv: [v0.26.0 → v0.29.2](https://github.com/go-openapi/swag/compare/conv/v0.26.0...conv/v0.29.2)
- github.com/go-openapi/swag/fileutils: [v0.26.0 → v0.29.2](https://github.com/go-openapi/swag/compare/fileutils/v0.26.0...fileutils/v0.29.2)
- github.com/go-openapi/swag/jsonutils/fixtures_test: [v0.26.0 → v0.29.2](https://github.com/go-openapi/swag/compare/jsonutils/fixtures_test/v0.26.0...jsonutils/fixtures_test/v0.29.2)
- github.com/go-openapi/swag/jsonutils: [v0.26.0 → v0.29.2](https://github.com/go-openapi/swag/compare/jsonutils/v0.26.0...jsonutils/v0.29.2)
- github.com/go-openapi/swag/loading: [v0.26.0 → v0.29.2](https://github.com/go-openapi/swag/compare/loading/v0.26.0...loading/v0.29.2)
- github.com/go-openapi/swag/mangling: [v0.26.0 → v0.29.2](https://github.com/go-openapi/swag/compare/mangling/v0.26.0...mangling/v0.29.2)
- github.com/go-openapi/swag/netutils: [v0.26.0 → v0.29.2](https://github.com/go-openapi/swag/compare/netutils/v0.26.0...netutils/v0.29.2)
- github.com/go-openapi/swag/stringutils: [v0.26.0 → v0.29.2](https://github.com/go-openapi/swag/compare/stringutils/v0.26.0...stringutils/v0.29.2)
- github.com/go-openapi/swag/typeutils: [v0.26.0 → v0.29.2](https://github.com/go-openapi/swag/compare/typeutils/v0.26.0...typeutils/v0.29.2)
- github.com/go-openapi/swag/yamlutils: [v0.26.0 → v0.29.2](https://github.com/go-openapi/swag/compare/yamlutils/v0.26.0...yamlutils/v0.29.2)
- github.com/go-openapi/swag: [v0.26.0 → v0.29.2](https://github.com/go-openapi/swag/compare/v0.26.0...v0.29.2)
- github.com/go-openapi/testify/enable/yaml/v2: [v2.4.2 → v2.7.0](https://github.com/go-openapi/testify/compare/enable/yaml/v2/v2.4.2...enable/yaml/v2/v2.7.0)
- github.com/go-openapi/testify/v2: [v2.5.1 → v2.8.0](https://github.com/go-openapi/testify/compare/v2.5.1...v2.8.0)
- github.com/google/cel-go: [v0.27.0 → v0.31.0](https://github.com/google/cel-go/compare/v0.27.0...v0.31.0)
- github.com/grpc-ecosystem/go-grpc-middleware/v2: [v2.3.3 → v2.3.4](https://github.com/grpc-ecosystem/go-grpc-middleware/compare/v2.3.3...v2.3.4)
- github.com/grpc-ecosystem/grpc-gateway/v2: [v2.29.0 → v2.31.0](https://github.com/grpc-ecosystem/grpc-gateway/compare/v2.29.0...v2.31.0)
- github.com/klauspost/compress: [v1.18.0 → v1.19.1](https://github.com/klauspost/compress/compare/v1.18.0...v1.19.1)
- github.com/kubernetes-csi/csi-lib-utils: [v0.24.0 → v0.25.0](https://github.com/kubernetes-csi/csi-lib-utils/compare/v0.24.0...v0.25.0)
- github.com/kubernetes-csi/csi-test/v5: [v5.4.0 → v5.6.0](https://github.com/kubernetes-csi/csi-test/compare/v5.4.0...v5.6.0)
- github.com/kubernetes-csi/external-snapshotter/client/v8: [v8.4.0 → v8.6.0](https://github.com/kubernetes-csi/external-snapshotter/compare/client/v8/v8.4.0...client/v8/v8.6.0)
- github.com/mailru/easyjson: [v0.9.1 → v0.7.7](https://github.com/mailru/easyjson/compare/v0.9.1...v0.7.7)
- github.com/miekg/dns: [v1.1.72 → v1.1.73](https://github.com/miekg/dns/compare/v1.1.72...v1.1.73)
- github.com/onsi/ginkgo/v2: [v2.29.0 → v2.33.0](https://github.com/onsi/ginkgo/compare/v2.29.0...v2.33.0)
- github.com/onsi/gomega: [v1.41.0 → v1.44.0](https://github.com/onsi/gomega/compare/v1.41.0...v1.44.0)
- github.com/opencontainers/selinux: [v1.15.0 → v1.15.1](https://github.com/opencontainers/selinux/compare/v1.15.0...v1.15.1)
- github.com/prometheus/client_golang: [v1.23.2 → v1.24.1](https://github.com/prometheus/client_golang/compare/v1.23.2...v1.24.1)
- github.com/prometheus/client_model: [v0.6.2 → v0.6.3](https://github.com/prometheus/client_model/compare/v0.6.2...v0.6.3)
- github.com/prometheus/common: [v0.68.0 → v0.72.0](https://github.com/prometheus/common/compare/v0.68.0...v0.72.0)
- github.com/prometheus/procfs: [v0.20.1 → v0.22.0](https://github.com/prometheus/procfs/compare/v0.20.1...v0.22.0)
- github.com/spiffe/go-spiffe/v2: [v2.6.0 → v2.8.1](https://github.com/spiffe/go-spiffe/compare/v2.6.0...v2.8.1)
- github.com/stoewer/go-strcase: [v1.3.0 → v1.3.1](https://github.com/stoewer/go-strcase/compare/v1.3.0...v1.3.1)
- github.com/stretchr/objx: [v0.5.2 → v0.5.3](https://github.com/stretchr/objx/compare/v0.5.2...v0.5.3)
- github.com/stretchr/testify: [v1.11.1 → v1.12.1](https://github.com/stretchr/testify/compare/v1.11.1...v1.12.1)
- go.etcd.io/etcd/api/v3: v3.6.11 → v3.7.2
- go.etcd.io/etcd/client/pkg/v3: v3.6.11 → v3.7.2
- go.etcd.io/etcd/client/v3: v3.6.11 → v3.7.2
- go.opentelemetry.io/contrib/detectors/gcp: v1.42.0 → v1.44.0
- go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc: v0.69.0 → v0.71.0
- go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp: v0.69.0 → v0.71.0
- go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc: v1.44.0 → v1.46.0
- go.opentelemetry.io/otel/exporters/otlp/otlptrace: v1.44.0 → v1.46.0
- go.opentelemetry.io/otel/metric: v1.44.0 → v1.46.0
- go.opentelemetry.io/otel/sdk/metric: v1.44.0 → v1.46.0
- go.opentelemetry.io/otel/sdk: v1.44.0 → v1.46.0
- go.opentelemetry.io/otel/trace: v1.44.0 → v1.46.0
- go.opentelemetry.io/otel: v1.44.0 → v1.46.0
- go.opentelemetry.io/proto/otlp: v1.10.0 → v1.11.1
- go.yaml.in/yaml/v3: v3.0.4 → v3.0.5
- golang.org/x/crypto: v0.52.0 → v0.57.0
- golang.org/x/exp: 3dfff04 → 85c1c22
- golang.org/x/mod: v0.36.0 → v0.41.0
- golang.org/x/net: v0.55.0 → v0.59.0
- golang.org/x/oauth2: v0.36.0 → v0.37.0
- golang.org/x/sync: v0.20.0 → v0.23.0
- golang.org/x/sys: v0.45.0 → v0.48.0
- golang.org/x/telemetry: 42602be → 4bcc4b2
- golang.org/x/term: v0.43.0 → v0.46.0
- golang.org/x/text: v0.37.0 → v0.42.0
- golang.org/x/time: v0.15.0 → v0.16.0
- golang.org/x/tools: v0.45.0 → v0.50.0
- google.golang.org/genproto/googleapis/api: 3dc84a4 → 8a89bd6
- google.golang.org/genproto/googleapis/rpc: 3dc84a4 → 8a89bd6
- google.golang.org/grpc: v1.81.1 → v1.84.0
- google.golang.org/protobuf: f2248ac → v1.36.12
- k8s.io/kube-openapi: 43fb72c → be32def
- k8s.io/kubernetes: v1.36.1 → v1.36.3
- k8s.io/utils: b8788ab → cf1189d
- sigs.k8s.io/apiserver-network-proxy/konnectivity-client: v0.35.0 → v0.37.0
- sigs.k8s.io/controller-runtime: v0.24.1 → v0.25.2
- sigs.k8s.io/gateway-api: v1.5.1 → v1.6.2
- sigs.k8s.io/sig-storage-lib-external-provisioner/v13: v13.0.0 → v13.1.0
- sigs.k8s.io/structured-merge-diff/v6: v6.4.0 → v6.4.2

### Removed
- github.com/antihax/optional: [v1.0.0](https://github.com/antihax/optional/tree/v1.0.0)
- github.com/evanphx/json-patch: [v5.6.0+incompatible](https://github.com/evanphx/json-patch/tree/v5.6.0)
- github.com/golang/mock: [v1.6.0](https://github.com/golang/mock/tree/v1.6.0)
- github.com/grafana/regexp: [a468a5b](https://github.com/grafana/regexp/tree/a468a5b)
- github.com/kisielk/errcheck: [v1.5.0](https://github.com/kisielk/errcheck/tree/v1.5.0)
- github.com/kisielk/gotool: [v1.0.0](https://github.com/kisielk/gotool/tree/v1.0.0)
- github.com/pkg/errors: [v0.9.1](https://github.com/pkg/errors/tree/v0.9.1)
- golang.org/x/xerrors: 5ec99f8
- google.golang.org/appengine: v1.6.7
- sigs.k8s.io/structured-merge-diff/v4: v4.4.1
