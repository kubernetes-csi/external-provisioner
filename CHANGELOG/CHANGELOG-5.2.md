# Release notes for 5.2.0

[Documentation](https://kubernetes-csi.github.io)

# Changelog since 5.1.0

## Changes by Kind

### Feature

- Add support for -logging-format=json
  Remove klog specific flags according to KEP-2845 ([#1318](https://github.com/kubernetes-csi/external-provisioner/pull/1318), [@huww98](https://github.com/huww98))

### Other (Cleanup or Flake)

- Updated CSI spec to 1.10.0 ([#1262](https://github.com/kubernetes-csi/external-provisioner/pull/1262), [@jsafrane](https://github.com/jsafrane))

### Uncategorized

- Fix CVE-2024-45337 ([#1314](https://github.com/kubernetes-csi/external-provisioner/pull/1314), [@shannon](https://github.com/shannon))
- Update build to Go 1.12.4 ([#267](https://github.com/kubernetes-csi/external-provisioner/pull/267), [@pohly](https://github.com/pohly))

## Dependencies

### Added
- github.com/GoogleCloudPlatform/opentelemetry-operations-go/detectors/gcp: [v1.24.2](https://github.com/GoogleCloudPlatform/opentelemetry-operations-go/tree/detectors/gcp/v1.24.2)
- github.com/Microsoft/hnslib: [v0.0.8](https://github.com/Microsoft/hnslib/tree/v0.0.8)
- github.com/aws/aws-sdk-go-v2/config: [v1.27.24](https://github.com/aws/aws-sdk-go-v2/tree/config/v1.27.24)
- github.com/aws/aws-sdk-go-v2/credentials: [v1.17.24](https://github.com/aws/aws-sdk-go-v2/tree/credentials/v1.17.24)
- github.com/aws/aws-sdk-go-v2/feature/ec2/imds: [v1.16.9](https://github.com/aws/aws-sdk-go-v2/tree/feature/ec2/imds/v1.16.9)
- github.com/aws/aws-sdk-go-v2/internal/configsources: [v1.3.13](https://github.com/aws/aws-sdk-go-v2/tree/internal/configsources/v1.3.13)
- github.com/aws/aws-sdk-go-v2/internal/endpoints/v2: [v2.6.13](https://github.com/aws/aws-sdk-go-v2/tree/internal/endpoints/v2/v2.6.13)
- github.com/aws/aws-sdk-go-v2/internal/ini: [v1.8.0](https://github.com/aws/aws-sdk-go-v2/tree/internal/ini/v1.8.0)
- github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding: [v1.11.3](https://github.com/aws/aws-sdk-go-v2/tree/service/internal/accept-encoding/v1.11.3)
- github.com/aws/aws-sdk-go-v2/service/internal/presigned-url: [v1.11.15](https://github.com/aws/aws-sdk-go-v2/tree/service/internal/presigned-url/v1.11.15)
- github.com/aws/aws-sdk-go-v2/service/sso: [v1.22.1](https://github.com/aws/aws-sdk-go-v2/tree/service/sso/v1.22.1)
- github.com/aws/aws-sdk-go-v2/service/ssooidc: [v1.26.2](https://github.com/aws/aws-sdk-go-v2/tree/service/ssooidc/v1.26.2)
- github.com/aws/aws-sdk-go-v2/service/sts: [v1.30.1](https://github.com/aws/aws-sdk-go-v2/tree/service/sts/v1.30.1)
- github.com/aws/aws-sdk-go-v2: [v1.30.1](https://github.com/aws/aws-sdk-go-v2/tree/v1.30.1)
- github.com/aws/smithy-go: [v1.20.3](https://github.com/aws/smithy-go/tree/v1.20.3)
- github.com/checkpoint-restore/go-criu/v6: [v6.3.0](https://github.com/checkpoint-restore/go-criu/tree/v6.3.0)
- github.com/containerd/containerd/api: [v1.8.0](https://github.com/containerd/containerd/tree/api/v1.8.0)
- github.com/containerd/errdefs: [v0.1.0](https://github.com/containerd/errdefs/tree/v0.1.0)
- github.com/containerd/log: [v0.1.0](https://github.com/containerd/log/tree/v0.1.0)
- github.com/containerd/typeurl/v2: [v2.2.0](https://github.com/containerd/typeurl/tree/v2.2.0)
- github.com/docker/docker: [v26.1.4+incompatible](https://github.com/docker/docker/tree/v26.1.4)
- github.com/docker/go-connections: [v0.5.0](https://github.com/docker/go-connections/tree/v0.5.0)
- github.com/klauspost/compress: [v1.17.11](https://github.com/klauspost/compress/tree/v1.17.11)
- github.com/kubernetes-csi/external-snapshotter/client/v8: [v8.2.0](https://github.com/kubernetes-csi/external-snapshotter/tree/client/v8/v8.2.0)
- github.com/kylelemons/godebug: [v1.1.0](https://github.com/kylelemons/godebug/tree/v1.1.0)
- github.com/moby/docker-image-spec: [v1.3.1](https://github.com/moby/docker-image-spec/tree/v1.3.1)
- github.com/moby/sys/user: [v0.3.0](https://github.com/moby/sys/tree/user/v0.3.0)
- github.com/moby/sys/userns: [v0.1.0](https://github.com/moby/sys/tree/userns/v0.1.0)
- github.com/morikuni/aec: [v1.0.0](https://github.com/morikuni/aec/tree/v1.0.0)
- github.com/opencontainers/image-spec: [v1.1.0](https://github.com/opencontainers/image-spec/tree/v1.1.0)
- github.com/planetscale/vtprotobuf: [0393e58](https://github.com/planetscale/vtprotobuf/tree/0393e58)
- github.com/russross/blackfriday: [v1.6.0](https://github.com/russross/blackfriday/tree/v1.6.0)
- github.com/xeipuuv/gojsonpointer: [4e3ac27](https://github.com/xeipuuv/gojsonpointer/tree/4e3ac27)
- github.com/xeipuuv/gojsonreference: [bd5ef7b](https://github.com/xeipuuv/gojsonreference/tree/bd5ef7b)
- github.com/xeipuuv/gojsonschema: [v1.2.0](https://github.com/xeipuuv/gojsonschema/tree/v1.2.0)
- go.opentelemetry.io/auto/sdk: v1.1.0
- go.opentelemetry.io/contrib/detectors/gcp: v1.31.0
- go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp: v1.27.0
- go.opentelemetry.io/otel/sdk/metric: v1.31.0
- gotest.tools/v3: v3.0.2
- k8s.io/externaljwt: v0.32.0
- sigs.k8s.io/sig-storage-lib-external-provisioner/v11: v11.0.1

### Changed
- cel.dev/expr: v0.15.0 → v0.19.1
- cloud.google.com/go/compute/metadata: v0.3.0 → v0.5.2
- cloud.google.com/go/compute: v1.24.0 → v1.25.1
- github.com/Azure/go-ansiterm: [d185dfc → 306776e](https://github.com/Azure/go-ansiterm/compare/d185dfc...306776e)
- github.com/Microsoft/go-winio: [v0.6.0 → v0.6.2](https://github.com/Microsoft/go-winio/compare/v0.6.0...v0.6.2)
- github.com/armon/circbuf: [bbbad09 → 5111143](https://github.com/armon/circbuf/compare/bbbad09...5111143)
- github.com/cilium/ebpf: [v0.9.1 → v0.16.0](https://github.com/cilium/ebpf/compare/v0.9.1...v0.16.0)
- github.com/cncf/xds/go: [555b57e → b4127c9](https://github.com/cncf/xds/compare/555b57e...b4127c9)
- github.com/container-storage-interface/spec: [v1.9.0 → v1.11.0](https://github.com/container-storage-interface/spec/compare/v1.9.0...v1.11.0)
- github.com/containerd/console: [v1.0.3 → v1.0.4](https://github.com/containerd/console/compare/v1.0.3...v1.0.4)
- github.com/containerd/ttrpc: [v1.2.2 → v1.2.7](https://github.com/containerd/ttrpc/compare/v1.2.2...v1.2.7)
- github.com/coredns/corefile-migration: [v1.0.21 → v1.0.24](https://github.com/coredns/corefile-migration/compare/v1.0.21...v1.0.24)
- github.com/cyphar/filepath-securejoin: [v0.2.4 → v0.3.6](https://github.com/cyphar/filepath-securejoin/compare/v0.2.4...v0.3.6)
- github.com/envoyproxy/go-control-plane: [v0.12.0 → v0.13.1](https://github.com/envoyproxy/go-control-plane/compare/v0.12.0...v0.13.1)
- github.com/envoyproxy/protoc-gen-validate: [v1.0.4 → v1.1.0](https://github.com/envoyproxy/protoc-gen-validate/compare/v1.0.4...v1.1.0)
- github.com/exponent-io/jsonpath: [d6023ce → 1de76d7](https://github.com/exponent-io/jsonpath/compare/d6023ce...1de76d7)
- github.com/fatih/color: [v1.16.0 → v1.17.0](https://github.com/fatih/color/compare/v1.16.0...v1.17.0)
- github.com/fsnotify/fsnotify: [v1.7.0 → v1.8.0](https://github.com/fsnotify/fsnotify/compare/v1.7.0...v1.8.0)
- github.com/golang/glog: [v1.2.1 → v1.2.2](https://github.com/golang/glog/compare/v1.2.1...v1.2.2)
- github.com/google/btree: [v1.0.1 → v1.1.3](https://github.com/google/btree/compare/v1.0.1...v1.1.3)
- github.com/google/cadvisor: [v0.49.0 → v0.51.0](https://github.com/google/cadvisor/compare/v0.49.0...v0.51.0)
- github.com/google/cel-go: [v0.20.1 → v0.22.1](https://github.com/google/cel-go/compare/v0.20.1...v0.22.1)
- github.com/google/gnostic-models: [v0.6.8 → v0.6.9](https://github.com/google/gnostic-models/compare/v0.6.8...v0.6.9)
- github.com/google/pprof: [813a5fb → 40e02aa](https://github.com/google/pprof/compare/813a5fb...40e02aa)
- github.com/gregjones/httpcache: [9cad4c3 → 901d907](https://github.com/gregjones/httpcache/compare/9cad4c3...901d907)
- github.com/grpc-ecosystem/grpc-gateway/v2: [v2.22.0 → v2.25.1](https://github.com/grpc-ecosystem/grpc-gateway/compare/v2.22.0...v2.25.1)
- github.com/jonboulle/clockwork: [v0.2.2 → v0.4.0](https://github.com/jonboulle/clockwork/compare/v0.2.2...v0.4.0)
- github.com/kubernetes-csi/csi-lib-utils: [v0.19.0 → v0.20.0](https://github.com/kubernetes-csi/csi-lib-utils/compare/v0.19.0...v0.20.0)
- github.com/kubernetes-csi/csi-test/v5: [v5.2.0 → v5.3.1](https://github.com/kubernetes-csi/csi-test/compare/v5.2.0...v5.3.1)
- github.com/mailru/easyjson: [v0.7.7 → v0.9.0](https://github.com/mailru/easyjson/compare/v0.7.7...v0.9.0)
- github.com/matttproud/golang_protobuf_extensions: [v1.0.1 → v1.0.2](https://github.com/matttproud/golang_protobuf_extensions/compare/v1.0.1...v1.0.2)
- github.com/mohae/deepcopy: [491d360 → c48cc78](https://github.com/mohae/deepcopy/compare/491d360...c48cc78)
- github.com/onsi/ginkgo/v2: [v2.20.0 → v2.22.2](https://github.com/onsi/ginkgo/compare/v2.20.0...v2.22.2)
- github.com/onsi/gomega: [v1.34.1 → v1.36.2](https://github.com/onsi/gomega/compare/v1.34.1...v1.36.2)
- github.com/opencontainers/runc: [v1.1.13 → v1.2.4](https://github.com/opencontainers/runc/compare/v1.1.13...v1.2.4)
- github.com/opencontainers/selinux: [v1.11.0 → v1.11.1](https://github.com/opencontainers/selinux/compare/v1.11.0...v1.11.1)
- github.com/prometheus/client_golang: [v1.19.1 → v1.20.5](https://github.com/prometheus/client_golang/compare/v1.19.1...v1.20.5)
- github.com/prometheus/common: [v0.55.0 → v0.61.0](https://github.com/prometheus/common/compare/v0.55.0...v0.61.0)
- github.com/rogpeppe/go-internal: [v1.12.0 → v1.13.1](https://github.com/rogpeppe/go-internal/compare/v1.12.0...v1.13.1)
- github.com/stretchr/testify: [v1.9.0 → v1.10.0](https://github.com/stretchr/testify/compare/v1.9.0...v1.10.0)
- github.com/urfave/cli: [v1.22.1 → v1.22.14](https://github.com/urfave/cli/compare/v1.22.1...v1.22.14)
- github.com/vishvananda/netlink: [v1.1.0 → b1ce50c](https://github.com/vishvananda/netlink/compare/v1.1.0...b1ce50c)
- github.com/xiang90/probing: [43a291a → a49e3df](https://github.com/xiang90/probing/compare/43a291a...a49e3df)
- go.etcd.io/bbolt: v1.3.9 → v1.3.11
- go.etcd.io/etcd/api/v3: v3.5.15 → v3.5.17
- go.etcd.io/etcd/client/pkg/v3: v3.5.15 → v3.5.17
- go.etcd.io/etcd/client/v2: v2.305.13 → v2.305.16
- go.etcd.io/etcd/client/v3: v3.5.15 → v3.5.17
- go.etcd.io/etcd/pkg/v3: v3.5.13 → v3.5.16
- go.etcd.io/etcd/raft/v3: v3.5.13 → v3.5.16
- go.etcd.io/etcd/server/v3: v3.5.13 → v3.5.16
- go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc: v0.53.0 → v0.58.0
- go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp: v0.53.0 → v0.58.0
- go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc: v1.28.0 → v1.33.0
- go.opentelemetry.io/otel/exporters/otlp/otlptrace: v1.28.0 → v1.33.0
- go.opentelemetry.io/otel/metric: v1.28.0 → v1.33.0
- go.opentelemetry.io/otel/sdk: v1.28.0 → v1.33.0
- go.opentelemetry.io/otel/trace: v1.28.0 → v1.33.0
- go.opentelemetry.io/otel: v1.28.0 → v1.33.0
- go.opentelemetry.io/proto/otlp: v1.3.1 → v1.5.0
- golang.org/x/crypto: v0.26.0 → v0.32.0
- golang.org/x/exp: 8a7402a → 7588d65
- golang.org/x/mod: v0.20.0 → v0.22.0
- golang.org/x/net: v0.28.0 → v0.34.0
- golang.org/x/oauth2: v0.22.0 → v0.25.0
- golang.org/x/sync: v0.8.0 → v0.10.0
- golang.org/x/sys: v0.23.0 → v0.29.0
- golang.org/x/term: v0.23.0 → v0.28.0
- golang.org/x/text: v0.17.0 → v0.21.0
- golang.org/x/time: v0.6.0 → v0.9.0
- golang.org/x/tools: v0.24.0 → v0.29.0
- google.golang.org/genproto/googleapis/api: ddb44da → 5f5ef82
- google.golang.org/genproto/googleapis/rpc: ddb44da → 5f5ef82
- google.golang.org/grpc/cmd/protoc-gen-go-grpc: v1.3.0 → v1.5.1
- google.golang.org/grpc: v1.65.0 → v1.69.2
- google.golang.org/protobuf: v1.34.2 → v1.36.2
- k8s.io/api: v0.31.0 → v0.32.0
- k8s.io/apiextensions-apiserver: v0.31.0 → v0.32.0
- k8s.io/apimachinery: v0.31.0 → v0.32.0
- k8s.io/apiserver: v0.31.0 → v0.32.0
- k8s.io/cli-runtime: v0.31.0 → v0.32.0
- k8s.io/client-go: v0.31.0 → v0.32.0
- k8s.io/cloud-provider: v0.31.0 → v0.32.0
- k8s.io/cluster-bootstrap: v0.31.0 → v0.32.0
- k8s.io/code-generator: v0.31.0 → v0.32.0
- k8s.io/component-base: v0.31.0 → v0.32.0
- k8s.io/component-helpers: v0.31.0 → v0.32.0
- k8s.io/controller-manager: v0.31.0 → v0.32.0
- k8s.io/cri-api: v0.31.0 → v0.32.0
- k8s.io/cri-client: v0.31.0 → v0.32.0
- k8s.io/csi-translation-lib: v0.31.0 → v0.32.0
- k8s.io/dynamic-resource-allocation: v0.31.0 → v0.32.0
- k8s.io/endpointslice: v0.31.0 → v0.32.0
- k8s.io/gengo/v2: 51d4e06 → 2b36238
- k8s.io/kms: v0.31.0 → v0.32.0
- k8s.io/kube-aggregator: v0.31.0 → v0.32.0
- k8s.io/kube-controller-manager: v0.31.0 → v0.32.0
- k8s.io/kube-openapi: 7a9a4e8 → 2c72e55
- k8s.io/kube-proxy: v0.31.0 → v0.32.0
- k8s.io/kube-scheduler: v0.31.0 → v0.32.0
- k8s.io/kubectl: v0.31.0 → v0.32.0
- k8s.io/kubelet: v0.31.0 → v0.32.0
- k8s.io/kubernetes: v1.31.0 → v1.32.0
- k8s.io/metrics: v0.31.0 → v0.32.0
- k8s.io/mount-utils: v0.31.0 → v0.32.0
- k8s.io/pod-security-admission: v0.31.0 → v0.32.0
- k8s.io/sample-apiserver: v0.31.0 → v0.32.0
- k8s.io/system-validators: v1.8.0 → v1.9.1
- k8s.io/utils: 18e509b → 24370be
- sigs.k8s.io/apiserver-network-proxy/konnectivity-client: v0.30.3 → v0.31.1
- sigs.k8s.io/controller-runtime: v0.19.0 → v0.19.4
- sigs.k8s.io/controller-tools: v0.15.0 → v0.16.3
- sigs.k8s.io/gateway-api: v1.1.0 → v1.2.1
- sigs.k8s.io/json: bc3834c → cfa47c3
- sigs.k8s.io/kustomize/api: v0.17.2 → v0.18.0
- sigs.k8s.io/kustomize/kustomize/v5: v5.4.2 → v5.5.0
- sigs.k8s.io/kustomize/kyaml: v0.17.1 → v0.18.1
- sigs.k8s.io/structured-merge-diff/v4: v4.4.1 → v4.5.0

### Removed
- github.com/Microsoft/hcsshim: [v0.8.26](https://github.com/Microsoft/hcsshim/tree/v0.8.26)
- github.com/checkpoint-restore/go-criu/v5: [v5.3.0](https://github.com/checkpoint-restore/go-criu/tree/v5.3.0)
- github.com/containerd/cgroups: [v1.1.0](https://github.com/containerd/cgroups/tree/v1.1.0)
- github.com/daviddengcn/go-colortext: [v1.0.0](https://github.com/daviddengcn/go-colortext/tree/v1.0.0)
- github.com/google/gnostic: [v0.6.9](https://github.com/google/gnostic/tree/v0.6.9)
- github.com/kubernetes-csi/external-snapshotter/client/v6: [v6.3.0](https://github.com/kubernetes-csi/external-snapshotter/tree/client/v6/v6.3.0)
- github.com/shurcooL/sanitized_anchor_name: [v1.0.0](https://github.com/shurcooL/sanitized_anchor_name/tree/v1.0.0)
- go.opencensus.io: v0.24.0
- go.starlark.net: a134d8f
- sigs.k8s.io/sig-storage-lib-external-provisioner/v10: v10.0.1
