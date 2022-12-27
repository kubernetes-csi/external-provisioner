# Release notes for v3.4.0

[Documentation](https://kubernetes-csi.github.io)

# Changelog since v3.3.0

## Changes by Kind

### Feature

- Add support for cross-namespace data sources alpha feature ([#805](https://github.com/kubernetes-csi/external-provisioner/pull/805), [@ttakahashi21]
- Register metrics exposed by sig-storage-lib ([#792](https://github.com/kubernetes-csi/external-provisioner/pull/792), [@RaunakShah](https://github.com/RaunakShah))
- Update the annotation that needs to be applies to VolumeSnapshotContents from snapshot.storage.kubernetes.io/allowVolumeModeChange to snapshot.storage.kubernetes.io/allow-volume-mode-change ([#791](https://github.com/kubernetes-csi/external-provisioner/pull/791), [@RaunakShah](https://github.com/RaunakShah))

### Bug or Regression

- Fix string pointer comparison for source volume mode conversion ([#793](https://github.com/kubernetes-csi/external-provisioner/pull/793), [@RaunakShah](https://github.com/RaunakShah))
- Fix nil pointer crash for PV without ClaimRef ([#796](https://github.com/kubernetes-csi/external-provisioner/pull/796), [@zezaeoh](https://github.com/zezaeoh))

### Uncategorized

- Update go to 1.19 and dependencies for k8s v1.26.0 ([#834](https://github.com/kubernetes-csi/external-provisioner/pull/834), [@sunnylovestiramisu](https://github.com/sunnylovestiramisu))

## Dependencies

### Added
- github.com/ahmetb/gen-crd-api-reference-docs: [v0.3.0](https://github.com/ahmetb/gen-crd-api-reference-docs/tree/v0.3.0)
- github.com/antlr/antlr4/runtime/Go/antlr: [v1.4.10](https://github.com/antlr/antlr4/runtime/Go/antlr/tree/v1.4.10)
- github.com/cenkalti/backoff/v4: [v4.1.3](https://github.com/cenkalti/backoff/v4/tree/v4.1.3)
- github.com/dgrijalva/jwt-go: [v3.2.0+incompatible](https://github.com/dgrijalva/jwt-go/tree/v3.2.0)
- github.com/docker/spdystream: [449fdfc](https://github.com/docker/spdystream/tree/449fdfc)
- github.com/emicklei/go-restful: [ff4f55a](https://github.com/emicklei/go-restful/tree/ff4f55a)
- github.com/fatih/color: [v1.12.0](https://github.com/fatih/color/tree/v1.12.0)
- github.com/go-logr/stdr: [v1.2.2](https://github.com/go-logr/stdr/tree/v1.2.2)
- github.com/go-openapi/spec: [6aced65](https://github.com/go-openapi/spec/tree/6aced65)
- github.com/gobuffalo/flect: [v0.2.3](https://github.com/gobuffalo/flect/tree/v0.2.3)
- github.com/google/cel-go: [v0.12.5](https://github.com/google/cel-go/tree/v0.12.5)
- github.com/googleapis/gnostic: [v0.4.1](https://github.com/googleapis/gnostic/tree/v0.4.1)
- github.com/grpc-ecosystem/grpc-gateway/v2: [v2.7.0](https://github.com/grpc-ecosystem/grpc-gateway/v2/tree/v2.7.0)
- github.com/lithammer/dedent: [v1.1.0](https://github.com/lithammer/dedent/tree/v1.1.0)
- github.com/mattn/go-colorable: [v0.1.8](https://github.com/mattn/go-colorable/tree/v0.1.8)
- github.com/mattn/go-isatty: [v0.0.12](https://github.com/mattn/go-isatty/tree/v0.0.12)
- go.opentelemetry.io/otel/exporters/otlp/internal/retry: v1.10.0
- go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc: v1.10.0
- go.opentelemetry.io/otel/exporters/otlp/otlptrace: v1.10.0
- k8s.io/klog: v0.2.0
- k8s.io/kms: v0.26.0
- sigs.k8s.io/controller-tools: v0.7.0
- sigs.k8s.io/gateway-api: v0.6.0

### Changed
- github.com/Azure/go-autorest/autorest/adal: [v0.9.20 → v0.8.2](https://github.com/Azure/go-autorest/autorest/adal/compare/v0.9.20...v0.8.2)
- github.com/Azure/go-autorest/autorest/date: [v0.3.0 → v0.2.0](https://github.com/Azure/go-autorest/autorest/date/compare/v0.3.0...v0.2.0)
- github.com/Azure/go-autorest/autorest/mocks: [v0.4.2 → v0.3.0](https://github.com/Azure/go-autorest/autorest/mocks/compare/v0.4.2...v0.3.0)
- github.com/Azure/go-autorest/autorest: [v0.11.27 → v0.9.6](https://github.com/Azure/go-autorest/autorest/compare/v0.11.27...v0.9.6)
- github.com/Azure/go-autorest/logger: [v0.2.1 → v0.1.0](https://github.com/Azure/go-autorest/logger/compare/v0.2.1...v0.1.0)
- github.com/Azure/go-autorest/tracing: [v0.6.0 → v0.5.0](https://github.com/Azure/go-autorest/tracing/compare/v0.6.0...v0.5.0)
- github.com/container-storage-interface/spec: [v1.6.0 → v1.7.0](https://github.com/container-storage-interface/spec/compare/v1.6.0...v1.7.0)
- github.com/cpuguy83/go-md2man/v2: [v2.0.1 → v2.0.2](https://github.com/cpuguy83/go-md2man/v2/compare/v2.0.1...v2.0.2)
- github.com/felixge/httpsnoop: [v1.0.1 → v1.0.3](https://github.com/felixge/httpsnoop/compare/v1.0.1...v1.0.3)
- github.com/fsnotify/fsnotify: [v1.5.4 → v1.6.0](https://github.com/fsnotify/fsnotify/compare/v1.5.4...v1.6.0)
- github.com/google/go-cmp: [v0.5.8 → v0.5.9](https://github.com/google/go-cmp/compare/v0.5.8...v0.5.9)
- github.com/google/martian/v3: [v3.2.1 → v3.0.0](https://github.com/google/martian/v3/compare/v3.2.1...v3.0.0)
- github.com/google/pprof: [4bb14d4 → 94a9f03](https://github.com/google/pprof/compare/4bb14d4...94a9f03)
- github.com/googleapis/gax-go/v2: [v2.1.0 → v2.0.5](https://github.com/googleapis/gax-go/v2/compare/v2.1.0...v2.0.5)
- github.com/inconshreveable/mousetrap: [v1.0.0 → v1.0.1](https://github.com/inconshreveable/mousetrap/compare/v1.0.0...v1.0.1)
- github.com/kubernetes-csi/csi-lib-utils: [v0.11.0 → v0.12.0](https://github.com/kubernetes-csi/csi-lib-utils/compare/v0.11.0...v0.12.0)
- github.com/matttproud/golang_protobuf_extensions: [c182aff → v1.0.2](https://github.com/matttproud/golang_protobuf_extensions/compare/c182aff...v1.0.2)
- github.com/moby/term: [3f7ff69 → 39b0c02](https://github.com/moby/term/compare/3f7ff69...39b0c02)
- github.com/onsi/ginkgo/v2: [v2.1.6 → v2.6.0](https://github.com/onsi/ginkgo/v2/compare/v2.1.6...v2.6.0)
- github.com/onsi/ginkgo: [v1.16.5 → v1.16.4](https://github.com/onsi/ginkgo/compare/v1.16.5...v1.16.4)
- github.com/onsi/gomega: [v1.20.1 → v1.24.1](https://github.com/onsi/gomega/compare/v1.20.1...v1.24.1)
- github.com/prometheus/client_golang: [v1.13.0 → v1.14.0](https://github.com/prometheus/client_golang/compare/v1.13.0...v1.14.0)
- github.com/prometheus/client_model: [v0.2.0 → v0.3.0](https://github.com/prometheus/client_model/compare/v0.2.0...v0.3.0)
- github.com/spf13/afero: [v1.6.0 → v1.2.2](https://github.com/spf13/afero/compare/v1.6.0...v1.2.2)
- github.com/spf13/cobra: [v1.4.0 → v1.6.0](https://github.com/spf13/cobra/compare/v1.4.0...v1.6.0)
- github.com/stretchr/objx: [v0.4.0 → v0.5.0](https://github.com/stretchr/objx/compare/v0.4.0...v0.5.0)
- github.com/stretchr/testify: [v1.8.0 → v1.8.1](https://github.com/stretchr/testify/compare/v1.8.0...v1.8.1)
- go.etcd.io/etcd/api/v3: v3.5.4 → v3.5.5
- go.etcd.io/etcd/client/pkg/v3: v3.5.4 → v3.5.5
- go.etcd.io/etcd/client/v2: v2.305.4 → v2.305.5
- go.etcd.io/etcd/client/v3: v3.5.4 → v3.5.5
- go.etcd.io/etcd/pkg/v3: v3.5.4 → v3.5.5
- go.etcd.io/etcd/raft/v3: v3.5.4 → v3.5.5
- go.etcd.io/etcd/server/v3: v3.5.4 → v3.5.5
- go.opencensus.io: v0.23.0 → v0.22.4
- go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc: v0.20.0 → v0.35.0
- go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp: v0.20.0 → v0.35.0
- go.opentelemetry.io/otel/metric: v0.20.0 → v0.31.0
- go.opentelemetry.io/otel/sdk: v0.20.0 → v1.10.0
- go.opentelemetry.io/otel/trace: v0.20.0 → v1.10.0
- go.opentelemetry.io/otel: v0.20.0 → v1.10.0
- go.opentelemetry.io/proto/otlp: v0.7.0 → v0.19.0
- go.uber.org/goleak: v1.1.12 → v1.2.0
- go.uber.org/zap: v1.21.0 → v1.24.0
- golang.org/x/crypto: 3147a52 → v0.1.0
- golang.org/x/lint: 6edffad → 738671d
- golang.org/x/mod: 86c51ed → v0.6.0
- golang.org/x/net: bea034e → v0.4.0
- golang.org/x/sys: fb04ddd → v0.3.0
- golang.org/x/term: 03fcf44 → v0.3.0
- golang.org/x/text: v0.3.7 → v0.5.0
- golang.org/x/time: 579cf78 → v0.3.0
- golang.org/x/tools: v0.1.12 → v0.2.0
- google.golang.org/api: v0.57.0 → v0.30.0
- google.golang.org/grpc: v1.49.0 → v1.51.0
- k8s.io/api: v0.25.2 → v0.26.0
- k8s.io/apiextensions-apiserver: v0.25.2 → v0.26.0
- k8s.io/apimachinery: v0.25.2 → v0.26.0
- k8s.io/apiserver: v0.25.2 → v0.26.0
- k8s.io/client-go: v0.25.2 → v0.26.0
- k8s.io/code-generator: v0.25.2 → v0.26.0
- k8s.io/component-base: v0.25.2 → v0.26.0
- k8s.io/component-helpers: v0.25.2 → v0.26.0
- k8s.io/csi-translation-lib: v0.25.2 → v0.26.0
- k8s.io/kube-openapi: a70c9af → 172d655
- k8s.io/utils: ee6ede2 → 99ec85e
- sigs.k8s.io/apiserver-network-proxy/konnectivity-client: v0.0.32 → v0.0.33
- sigs.k8s.io/controller-runtime: v0.13.0 → v0.14.1

### Removed
- github.com/Azure/go-autorest: [v14.2.0+incompatible](https://github.com/Azure/go-autorest/tree/v14.2.0)
- github.com/benbjohnson/clock: [v1.1.0](https://github.com/benbjohnson/clock/tree/v1.1.0)
- github.com/creack/pty: [v1.1.11](https://github.com/creack/pty/tree/v1.1.11)
- github.com/getkin/kin-openapi: [v0.76.0](https://github.com/getkin/kin-openapi/tree/v0.76.0)
- github.com/golang-jwt/jwt/v4: [v4.2.0](https://github.com/golang-jwt/jwt/v4/tree/v4.2.0)
- github.com/golang/snappy: [v0.0.3](https://github.com/golang/snappy/tree/v0.0.3)
- github.com/gorilla/mux: [v1.8.0](https://github.com/gorilla/mux/tree/v1.8.0)
- go.opentelemetry.io/contrib: v0.20.0
- go.opentelemetry.io/otel/exporters/otlp: v0.20.0
- go.opentelemetry.io/otel/oteltest: v0.20.0
- go.opentelemetry.io/otel/sdk/export/metric: v0.20.0
- go.opentelemetry.io/otel/sdk/metric: v0.20.0
- google.golang.org/grpc/cmd/protoc-gen-go-grpc: v1.1.0
