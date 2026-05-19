# Release notes for 6.3.0

[Documentation](https://kubernetes-csi.github.io)

# Changelog since 6.2.0

## Changes by Kind

### Feature

- Add controller to clean up orphaned snapshot source-protection finalizers that can block snapshot deletion after provisioner crashes or PVC deletions. ([#1519](https://github.com/kubernetes-csi/external-provisioner/pull/1519), [@ConnorJC3](https://github.com/ConnorJC3))

### Bug or Regression

- A new flag --leader-election-namespace is introduced to allow the user to set where the leader election lock resource lives. ([#298](https://github.com/kubernetes-csi/external-provisioner/pull/298), [@verult](https://github.com/verult))
- Fixed possible duplicated CSIStorageCapacity and constantly failing update request. ([#1450](https://github.com/kubernetes-csi/external-provisioner/pull/1450), [@huww98](https://github.com/huww98))

### Other (Cleanup or Flake)

- Bump k8s dependencies to v1.36.1 ([#1509](https://github.com/kubernetes-csi/external-provisioner/pull/1509), [@dfajmon](https://github.com/dfajmon))

### Uncategorized

- Updated the package import path to github.com/kubernetes-csi/external-provisioner/v6 ([#1497](https://github.com/kubernetes-csi/external-provisioner/pull/1497), [@chimanjain](https://github.com/chimanjain))

## Dependencies

### Added
- buf.build/gen/go/bufbuild/protovalidate/protocolbuffers/go: 8976f5b
- buf.build/go/protovalidate: v0.12.0
- k8s.io/cri-streaming: v0.36.1
- k8s.io/streaming: v0.36.1

### Changed
- cel.dev/expr: v0.25.1 → v0.25.2
- cyphar.com/go-pathrs: v0.2.3 → v0.2.4
- github.com/GoogleCloudPlatform/opentelemetry-operations-go/detectors/gcp: [v1.30.0 → v1.31.0](https://github.com/GoogleCloudPlatform/opentelemetry-operations-go/compare/detectors/gcp/v1.30.0...detectors/gcp/v1.31.0)
- github.com/Masterminds/semver/v3: [v3.4.0 → v3.5.0](https://github.com/Masterminds/semver/compare/v3.4.0...v3.5.0)
- github.com/cncf/xds/go: [ee656c7 → dba9d58](https://github.com/cncf/xds/compare/ee656c7...dba9d58)
- github.com/containerd/containerd/api: [v1.9.0 → v1.10.0](https://github.com/containerd/containerd/compare/api/v1.9.0...api/v1.10.0)
- github.com/coredns/corefile-migration: [v1.0.29 → v1.0.31](https://github.com/coredns/corefile-migration/compare/v1.0.29...v1.0.31)
- github.com/coreos/go-oidc: [v2.3.0+incompatible → v2.5.0+incompatible](https://github.com/coreos/go-oidc/compare/v2.3.0...v2.5.0)
- github.com/envoyproxy/go-control-plane/envoy: [v1.36.0 → v1.37.0](https://github.com/envoyproxy/go-control-plane/compare/envoy/v1.36.0...envoy/v1.37.0)
- github.com/envoyproxy/protoc-gen-validate: [v1.3.0 → v1.3.3](https://github.com/envoyproxy/protoc-gen-validate/compare/v1.3.0...v1.3.3)
- github.com/fsnotify/fsnotify: [v1.9.0 → v1.10.1](https://github.com/fsnotify/fsnotify/compare/v1.9.0...v1.10.1)
- github.com/fxamacker/cbor/v2: [v2.9.0 → v2.9.2](https://github.com/fxamacker/cbor/compare/v2.9.0...v2.9.2)
- github.com/go-jose/go-jose/v4: [v4.1.3 → v4.1.4](https://github.com/go-jose/go-jose/compare/v4.1.3...v4.1.4)
- github.com/go-openapi/jsonpointer: [v0.22.4 → v0.23.1](https://github.com/go-openapi/jsonpointer/compare/v0.22.4...v0.23.1)
- github.com/go-openapi/jsonreference: [v0.21.4 → v0.21.6](https://github.com/go-openapi/jsonreference/compare/v0.21.4...v0.21.6)
- github.com/go-openapi/swag/cmdutils: [v0.25.4 → v0.26.0](https://github.com/go-openapi/swag/compare/cmdutils/v0.25.4...cmdutils/v0.26.0)
- github.com/go-openapi/swag/conv: [v0.25.4 → v0.26.0](https://github.com/go-openapi/swag/compare/conv/v0.25.4...conv/v0.26.0)
- github.com/go-openapi/swag/fileutils: [v0.25.4 → v0.26.0](https://github.com/go-openapi/swag/compare/fileutils/v0.25.4...fileutils/v0.26.0)
- github.com/go-openapi/swag/jsonname: [v0.25.4 → v0.26.0](https://github.com/go-openapi/swag/compare/jsonname/v0.25.4...jsonname/v0.26.0)
- github.com/go-openapi/swag/jsonutils/fixtures_test: [v0.25.4 → v0.26.0](https://github.com/go-openapi/swag/compare/jsonutils/fixtures_test/v0.25.4...jsonutils/fixtures_test/v0.26.0)
- github.com/go-openapi/swag/jsonutils: [v0.25.4 → v0.26.0](https://github.com/go-openapi/swag/compare/jsonutils/v0.25.4...jsonutils/v0.26.0)
- github.com/go-openapi/swag/loading: [v0.25.4 → v0.26.0](https://github.com/go-openapi/swag/compare/loading/v0.25.4...loading/v0.26.0)
- github.com/go-openapi/swag/mangling: [v0.25.4 → v0.26.0](https://github.com/go-openapi/swag/compare/mangling/v0.25.4...mangling/v0.26.0)
- github.com/go-openapi/swag/netutils: [v0.25.4 → v0.26.0](https://github.com/go-openapi/swag/compare/netutils/v0.25.4...netutils/v0.26.0)
- github.com/go-openapi/swag/stringutils: [v0.25.4 → v0.26.0](https://github.com/go-openapi/swag/compare/stringutils/v0.25.4...stringutils/v0.26.0)
- github.com/go-openapi/swag/typeutils: [v0.25.4 → v0.26.0](https://github.com/go-openapi/swag/compare/typeutils/v0.25.4...typeutils/v0.26.0)
- github.com/go-openapi/swag/yamlutils: [v0.25.4 → v0.26.0](https://github.com/go-openapi/swag/compare/yamlutils/v0.25.4...yamlutils/v0.26.0)
- github.com/go-openapi/swag: [v0.25.4 → v0.26.0](https://github.com/go-openapi/swag/compare/v0.25.4...v0.26.0)
- github.com/go-openapi/testify/enable/yaml/v2: [v2.0.2 → v2.4.2](https://github.com/go-openapi/testify/compare/enable/yaml/v2/v2.0.2...enable/yaml/v2/v2.4.2)
- github.com/go-openapi/testify/v2: [v2.0.2 → v2.5.1](https://github.com/go-openapi/testify/compare/v2.0.2...v2.5.1)
- github.com/godbus/dbus/v5: [v5.1.0 → v5.2.2](https://github.com/godbus/dbus/compare/v5.1.0...v5.2.2)
- github.com/golang-jwt/jwt/v5: [v5.3.0 → v5.3.1](https://github.com/golang-jwt/jwt/compare/v5.3.0...v5.3.1)
- github.com/google/cadvisor: [v0.53.0 → v0.56.2](https://github.com/google/cadvisor/compare/v0.53.0...v0.56.2)
- github.com/google/pprof: [cb029da → 545e8a4](https://github.com/google/pprof/compare/cb029da...545e8a4)
- github.com/grpc-ecosystem/go-grpc-middleware/providers/prometheus: [v1.0.1 → v1.1.0](https://github.com/grpc-ecosystem/go-grpc-middleware/compare/providers/prometheus/v1.0.1...providers/prometheus/v1.1.0)
- github.com/grpc-ecosystem/go-grpc-middleware/v2: [v2.3.0 → v2.3.3](https://github.com/grpc-ecosystem/go-grpc-middleware/compare/v2.3.0...v2.3.3)
- github.com/grpc-ecosystem/grpc-gateway/v2: [v2.28.0 → v2.29.0](https://github.com/grpc-ecosystem/grpc-gateway/compare/v2.28.0...v2.29.0)
- github.com/kubernetes-csi/csi-lib-utils: [v0.23.2 → v0.24.0](https://github.com/kubernetes-csi/csi-lib-utils/compare/v0.23.2...v0.24.0)
- github.com/moby/spdystream: [v0.5.0 → v0.5.1](https://github.com/moby/spdystream/compare/v0.5.0...v0.5.1)
- github.com/onsi/ginkgo/v2: [v2.28.1 → v2.29.0](https://github.com/onsi/ginkgo/compare/v2.28.1...v2.29.0)
- github.com/onsi/gomega: [v1.39.1 → v1.41.0](https://github.com/onsi/gomega/compare/v1.39.1...v1.41.0)
- github.com/opencontainers/cgroups: [v0.0.3 → v0.0.6](https://github.com/opencontainers/cgroups/compare/v0.0.3...v0.0.6)
- github.com/opencontainers/runtime-spec: [v1.2.1 → v1.3.0](https://github.com/opencontainers/runtime-spec/compare/v1.2.1...v1.3.0)
- github.com/opencontainers/selinux: [v1.13.1 → v1.15.0](https://github.com/opencontainers/selinux/compare/v1.13.1...v1.15.0)
- github.com/prometheus/common: [v0.67.5 → v0.68.0](https://github.com/prometheus/common/compare/v0.67.5...v0.68.0)
- github.com/prometheus/procfs: [v0.20.0 → v0.20.1](https://github.com/prometheus/procfs/compare/v0.20.0...v0.20.1)
- go.etcd.io/etcd/api/v3: v3.6.8 → v3.6.11
- go.etcd.io/etcd/client/pkg/v3: v3.6.8 → v3.6.11
- go.etcd.io/etcd/client/v3: v3.6.8 → v3.6.11
- go.etcd.io/etcd/pkg/v3: v3.6.5 → v3.6.8
- go.etcd.io/etcd/server/v3: v3.6.5 → v3.6.8
- go.opentelemetry.io/contrib/detectors/gcp: v1.39.0 → v1.42.0
- go.opentelemetry.io/contrib/instrumentation/github.com/emicklei/go-restful/otelrestful: v0.44.0 → v0.65.0
- go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc: v0.65.0 → v0.69.0
- go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp: v0.65.0 → v0.69.0
- go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc: v1.40.0 → v1.44.0
- go.opentelemetry.io/otel/exporters/otlp/otlptrace: v1.40.0 → v1.44.0
- go.opentelemetry.io/otel/metric: v1.40.0 → v1.44.0
- go.opentelemetry.io/otel/sdk/metric: v1.40.0 → v1.44.0
- go.opentelemetry.io/otel/sdk: v1.40.0 → v1.44.0
- go.opentelemetry.io/otel/trace: v1.40.0 → v1.44.0
- go.opentelemetry.io/otel: v1.40.0 → v1.44.0
- go.opentelemetry.io/proto/otlp: v1.9.0 → v1.10.0
- go.uber.org/zap: v1.27.1 → v1.28.0
- go.yaml.in/yaml/v2: v2.4.3 → v2.4.4
- golang.org/x/crypto: v0.48.0 → v0.52.0
- golang.org/x/mod: v0.33.0 → v0.36.0
- golang.org/x/net: v0.51.0 → v0.55.0
- golang.org/x/oauth2: v0.35.0 → v0.36.0
- golang.org/x/sync: v0.19.0 → v0.20.0
- golang.org/x/sys: v0.41.0 → v0.45.0
- golang.org/x/telemetry: e7419c6 → 42602be
- golang.org/x/term: v0.40.0 → v0.43.0
- golang.org/x/text: v0.34.0 → v0.37.0
- golang.org/x/time: v0.14.0 → v0.15.0
- golang.org/x/tools: v0.42.0 → v0.45.0
- gonum.org/v1/gonum: v0.16.0 → v0.17.0
- google.golang.org/genproto/googleapis/api: a57be14 → 3dc84a4
- google.golang.org/genproto/googleapis/rpc: a57be14 → 3dc84a4
- google.golang.org/grpc: v1.79.1 → v1.81.1
- google.golang.org/protobuf: v1.36.11 → f2248ac
- k8s.io/api: v0.35.2 → v0.36.1
- k8s.io/apiextensions-apiserver: v0.35.2 → v0.36.1
- k8s.io/apimachinery: v0.35.2 → v0.36.1
- k8s.io/apiserver: v0.35.2 → v0.36.1
- k8s.io/cli-runtime: v0.35.2 → v0.36.1
- k8s.io/client-go: v0.35.2 → v0.36.1
- k8s.io/cloud-provider: v0.35.2 → v0.36.1
- k8s.io/cluster-bootstrap: v0.35.2 → v0.36.1
- k8s.io/code-generator: v0.35.2 → v0.36.1
- k8s.io/component-base: v0.35.2 → v0.36.1
- k8s.io/component-helpers: v0.35.2 → v0.36.1
- k8s.io/controller-manager: v0.35.2 → v0.36.1
- k8s.io/cri-api: v0.35.2 → v0.36.1
- k8s.io/cri-client: v0.35.2 → v0.36.1
- k8s.io/csi-translation-lib: v0.35.2 → v0.36.1
- k8s.io/dynamic-resource-allocation: v0.35.2 → v0.36.1
- k8s.io/endpointslice: v0.35.2 → v0.36.1
- k8s.io/externaljwt: v0.35.2 → v0.36.1
- k8s.io/klog/v2: v2.130.1 → v2.140.0
- k8s.io/kms: v0.35.2 → v0.36.1
- k8s.io/kube-aggregator: v0.35.2 → v0.36.1
- k8s.io/kube-controller-manager: v0.35.2 → v0.36.1
- k8s.io/kube-openapi: a19766b → 43fb72c
- k8s.io/kube-proxy: v0.35.2 → v0.36.1
- k8s.io/kube-scheduler: v0.35.2 → v0.36.1
- k8s.io/kubectl: v0.35.2 → v0.36.1
- k8s.io/kubelet: v0.35.2 → v0.36.1
- k8s.io/kubernetes: v1.35.2 → v1.36.1
- k8s.io/metrics: v0.35.2 → v0.36.1
- k8s.io/mount-utils: v0.35.2 → v0.36.1
- k8s.io/pod-security-admission: v0.35.2 → v0.36.1
- k8s.io/sample-apiserver: v0.35.2 → v0.36.1
- sigs.k8s.io/apiserver-network-proxy/konnectivity-client: v0.34.0 → v0.35.0
- sigs.k8s.io/controller-runtime: v0.23.1 → v0.24.1
- sigs.k8s.io/gateway-api: v1.5.0 → v1.5.1
- sigs.k8s.io/knftables: v0.0.17 → v0.0.21
- sigs.k8s.io/kustomize/api: v0.20.1 → v0.21.1
- sigs.k8s.io/kustomize/kustomize/v5: v5.7.1 → v5.8.1
- sigs.k8s.io/kustomize/kyaml: v0.20.1 → v0.21.1
- sigs.k8s.io/structured-merge-diff/v6: v6.3.2 → v6.4.0

### Removed
- github.com/armon/circbuf: [5111143](https://github.com/armon/circbuf/tree/5111143)
- github.com/cenkalti/backoff/v4: [v4.3.0](https://github.com/cenkalti/backoff/tree/v4.3.0)
- github.com/gregjones/httpcache: [901d907](https://github.com/gregjones/httpcache/tree/901d907)
- github.com/grpc-ecosystem/go-grpc-prometheus: [v1.2.0](https://github.com/grpc-ecosystem/go-grpc-prometheus/tree/v1.2.0)
- github.com/karrick/godirwalk: [v1.17.0](https://github.com/karrick/godirwalk/tree/v1.17.0)
- github.com/libopenstorage/openstorage: [v1.0.0](https://github.com/libopenstorage/openstorage/tree/v1.0.0)
- github.com/mohae/deepcopy: [c48cc78](https://github.com/mohae/deepcopy/tree/c48cc78)
- github.com/mrunalp/fileutils: [v0.5.1](https://github.com/mrunalp/fileutils/tree/v0.5.1)
