# Release notes for 5.1.0

[Documentation](https://kubernetes-csi.github.io)

# Changelog since 5.0.0

## Urgent Upgrade Notes

### (No, really, you MUST read this before you upgrade)

- Go module path was changed to github.com/kubernetes-csi/external-provisioner/v5, adjust accordingly when importing in go ([#1236](https://github.com/kubernetes-csi/external-provisioner/pull/1236), [@jakobmoellerdev](https://github.com/jakobmoellerdev))
- If VolumeAttributesClass feature gate is enabled, then this sidecar may only be used with Kubernetes v1.31 due to upgraded v1beta1 objects and listers ([#1253](https://github.com/kubernetes-csi/external-provisioner/pull/1253), [@AndrewSirenko](https://github.com/AndrewSirenko))

## Changes by Kind

### Feature

- Promote CSINodeExpandSecret to GA ([#1236](https://github.com/kubernetes-csi/external-provisioner/pull/1218), [@jakobmoellerdev](https://github.com/jakobmoellerdev))
- Promote VolumeAttributesClass feature gate to beta. If VolumeAttributesClass feature gate is enabled, then this sidecar may only be used with Kubernetes v1.31 due to upgraded v1beta1 objects and listers ([#1253](https://github.com/kubernetes-csi/external-provisioner/pull/1253), [@AndrewSirenko](https://github.com/AndrewSirenko))

### Bug or Regression

- Fixed removal of PV protection finalizer. PVs are no longer Terminating forever after PVC deletion ([#1250](https://github.com/kubernetes-csi/external-provisioner/pull/1250), [@jsafrane](https://github.com/jsafrane))

### Other (Cleanup or Flake)

- Updates Kubernetes dependencies to v1.31.0 ([#1256](https://github.com/kubernetes-csi/external-provisioner/pull/1256), [@dfajmon](https://github.com/dfajmon))

## Dependencies

### Added
- cel.dev/expr: v0.15.0
- github.com/antlr4-go/antlr/v4: [v4.13.1](https://github.com/antlr4-go/antlr/tree/v4.13.1)
- github.com/shurcooL/sanitized_anchor_name: [v1.0.0](https://github.com/shurcooL/sanitized_anchor_name/tree/v1.0.0)
- github.com/urfave/cli: [v1.22.1](https://github.com/urfave/cli/tree/v1.22.1)
- gopkg.in/evanphx/json-patch.v4: v4.12.0
- k8s.io/cri-client: v0.31.0

### Changed
- cloud.google.com/go/compute: v1.25.1 → v1.24.0
- github.com/Microsoft/hcsshim: [v0.8.25 → v0.8.26](https://github.com/Microsoft/hcsshim/compare/v0.8.25...v0.8.26)
- github.com/asaskevich/govalidator: [f61b66f → a9d515a](https://github.com/asaskevich/govalidator/compare/f61b66f...a9d515a)
- github.com/cncf/xds/go: [8a4994d → 555b57e](https://github.com/cncf/xds/compare/8a4994d...555b57e)
- github.com/cpuguy83/go-md2man/v2: [v2.0.3 → v2.0.4](https://github.com/cpuguy83/go-md2man/compare/v2.0.3...v2.0.4)
- github.com/davecgh/go-spew: [v1.1.1 → d8f796a](https://github.com/davecgh/go-spew/compare/v1.1.1...d8f796a)
- github.com/emicklei/go-restful/v3: [v3.12.0 → v3.12.1](https://github.com/emicklei/go-restful/compare/v3.12.0...v3.12.1)
- github.com/evanphx/json-patch: [v5.9.0+incompatible → v5.7.0+incompatible](https://github.com/evanphx/json-patch/compare/v5.9.0...v5.7.0)
- github.com/fxamacker/cbor/v2: [v2.6.0 → v2.7.0](https://github.com/fxamacker/cbor/compare/v2.6.0...v2.7.0)
- github.com/go-logr/logr: [v1.4.1 → v1.4.2](https://github.com/go-logr/logr/compare/v1.4.1...v1.4.2)
- github.com/golang/glog: [v1.2.0 → v1.2.1](https://github.com/golang/glog/compare/v1.2.0...v1.2.1)
- github.com/google/cel-go: [v0.17.8 → v0.20.1](https://github.com/google/cel-go/compare/v0.17.8...v0.20.1)
- github.com/google/pprof: [a892ee0 → 813a5fb](https://github.com/google/pprof/compare/a892ee0...813a5fb)
- github.com/gorilla/websocket: [v1.5.1 → v1.5.3](https://github.com/gorilla/websocket/compare/v1.5.1...v1.5.3)
- github.com/grpc-ecosystem/grpc-gateway/v2: [v2.20.0 → v2.22.0](https://github.com/grpc-ecosystem/grpc-gateway/compare/v2.20.0...v2.22.0)
- github.com/kubernetes-csi/csi-lib-utils: [v0.18.0 → v0.19.0](https://github.com/kubernetes-csi/csi-lib-utils/compare/v0.18.0...v0.19.0)
- github.com/matttproud/golang_protobuf_extensions: [v1.0.4 → v1.0.1](https://github.com/matttproud/golang_protobuf_extensions/compare/v1.0.4...v1.0.1)
- github.com/miekg/dns: [v1.1.59 → v1.1.62](https://github.com/miekg/dns/compare/v1.1.59...v1.1.62)
- github.com/moby/spdystream: [v0.2.0 → v0.5.0](https://github.com/moby/spdystream/compare/v0.2.0...v0.5.0)
- github.com/moby/sys/mountinfo: [v0.7.1 → v0.7.2](https://github.com/moby/sys/compare/mountinfo/v0.7.1...mountinfo/v0.7.2)
- github.com/moby/term: [1aeaba8 → v0.5.0](https://github.com/moby/term/compare/1aeaba8...v0.5.0)
- github.com/onsi/ginkgo/v2: [v2.17.3 → v2.20.0](https://github.com/onsi/ginkgo/compare/v2.17.3...v2.20.0)
- github.com/onsi/gomega: [v1.33.1 → v1.34.1](https://github.com/onsi/gomega/compare/v1.33.1...v1.34.1)
- github.com/opencontainers/runc: [v1.1.12 → v1.1.13](https://github.com/opencontainers/runc/compare/v1.1.12...v1.1.13)
- github.com/opencontainers/runtime-spec: [494a5a6 → v1.2.0](https://github.com/opencontainers/runtime-spec/compare/494a5a6...v1.2.0)
- github.com/pmezard/go-difflib: [v1.0.0 → 5d4384e](https://github.com/pmezard/go-difflib/compare/v1.0.0...5d4384e)
- github.com/prometheus/client_golang: [v1.18.0 → v1.19.1](https://github.com/prometheus/client_golang/compare/v1.18.0...v1.19.1)
- github.com/prometheus/common: [v0.46.0 → v0.55.0](https://github.com/prometheus/common/compare/v0.46.0...v0.55.0)
- github.com/prometheus/procfs: [v0.15.0 → v0.15.1](https://github.com/prometheus/procfs/compare/v0.15.0...v0.15.1)
- github.com/rogpeppe/go-internal: [v1.11.0 → v1.12.0](https://github.com/rogpeppe/go-internal/compare/v1.11.0...v1.12.0)
- github.com/sirupsen/logrus: [v1.9.0 → v1.9.3](https://github.com/sirupsen/logrus/compare/v1.9.0...v1.9.3)
- github.com/spf13/cobra: [v1.8.0 → v1.8.1](https://github.com/spf13/cobra/compare/v1.8.0...v1.8.1)
- go.etcd.io/bbolt: v1.3.8 → v1.3.9
- go.etcd.io/etcd/api/v3: v3.5.13 → v3.5.15
- go.etcd.io/etcd/client/pkg/v3: v3.5.13 → v3.5.15
- go.etcd.io/etcd/client/v2: v2.305.10 → v2.305.13
- go.etcd.io/etcd/client/v3: v3.5.13 → v3.5.15
- go.etcd.io/etcd/pkg/v3: v3.5.10 → v3.5.13
- go.etcd.io/etcd/raft/v3: v3.5.10 → v3.5.13
- go.etcd.io/etcd/server/v3: v3.5.10 → v3.5.13
- go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc: v0.51.0 → v0.53.0
- go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp: v0.51.0 → v0.53.0
- go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc: v1.26.0 → v1.28.0
- go.opentelemetry.io/otel/exporters/otlp/otlptrace: v1.26.0 → v1.28.0
- go.opentelemetry.io/otel/metric: v1.26.0 → v1.28.0
- go.opentelemetry.io/otel/sdk: v1.26.0 → v1.28.0
- go.opentelemetry.io/otel/trace: v1.26.0 → v1.28.0
- go.opentelemetry.io/otel: v1.26.0 → v1.28.0
- go.opentelemetry.io/proto/otlp: v1.2.0 → v1.3.1
- golang.org/x/crypto: v0.23.0 → v0.26.0
- golang.org/x/exp: fe59bbe → 8a7402a
- golang.org/x/mod: v0.17.0 → v0.20.0
- golang.org/x/net: v0.25.0 → v0.28.0
- golang.org/x/oauth2: v0.20.0 → v0.22.0
- golang.org/x/sync: v0.7.0 → v0.8.0
- golang.org/x/sys: v0.20.0 → v0.23.0
- golang.org/x/telemetry: f48c80b → bda5523
- golang.org/x/term: v0.20.0 → v0.23.0
- golang.org/x/text: v0.15.0 → v0.17.0
- golang.org/x/time: v0.5.0 → v0.6.0
- golang.org/x/tools: v0.21.0 → v0.24.0
- golang.org/x/xerrors: 04be3eb → 5ec99f8
- google.golang.org/appengine: v1.6.8 → v1.6.7
- google.golang.org/genproto/googleapis/api: 0867130 → ddb44da
- google.golang.org/genproto/googleapis/rpc: 0867130 → ddb44da
- google.golang.org/grpc: v1.64.0 → v1.65.0
- google.golang.org/protobuf: v1.34.1 → v1.34.2
- k8s.io/api: v0.30.0 → v0.31.0
- k8s.io/apiextensions-apiserver: v0.30.0 → v0.31.0
- k8s.io/apimachinery: v0.30.0 → v0.31.0
- k8s.io/apiserver: v0.30.0 → v0.31.0
- k8s.io/cli-runtime: v0.30.0 → v0.31.0
- k8s.io/client-go: v0.30.0 → v0.31.0
- k8s.io/cloud-provider: v0.30.0 → v0.31.0
- k8s.io/cluster-bootstrap: v0.30.0 → v0.31.0
- k8s.io/code-generator: v0.30.0 → v0.31.0
- k8s.io/component-base: v0.30.0 → v0.31.0
- k8s.io/component-helpers: v0.30.0 → v0.31.0
- k8s.io/controller-manager: v0.30.0 → v0.31.0
- k8s.io/cri-api: v0.30.0 → v0.31.0
- k8s.io/csi-translation-lib: v0.30.0 → v0.31.0
- k8s.io/dynamic-resource-allocation: v0.30.0 → v0.31.0
- k8s.io/endpointslice: v0.30.0 → v0.31.0
- k8s.io/klog/v2: v2.120.1 → v2.130.1
- k8s.io/kms: v0.30.0 → v0.31.0
- k8s.io/kube-aggregator: v0.30.0 → v0.31.0
- k8s.io/kube-controller-manager: v0.30.0 → v0.31.0
- k8s.io/kube-openapi: 8948a66 → 7a9a4e8
- k8s.io/kube-proxy: v0.30.0 → v0.31.0
- k8s.io/kube-scheduler: v0.30.0 → v0.31.0
- k8s.io/kubectl: v0.30.0 → v0.31.0
- k8s.io/kubelet: v0.30.0 → v0.31.0
- k8s.io/kubernetes: v1.30.1 → v1.31.0
- k8s.io/metrics: v0.30.0 → v0.31.0
- k8s.io/mount-utils: v0.30.0 → v0.31.0
- k8s.io/pod-security-admission: v0.30.0 → v0.31.0
- k8s.io/sample-apiserver: v0.30.0 → v0.31.0
- k8s.io/utils: 0849a56 → 18e509b
- sigs.k8s.io/controller-runtime: v0.18.2 → v0.19.0
- sigs.k8s.io/knftables: v0.0.14 → v0.0.17
- sigs.k8s.io/kustomize/api: 6ce0bf3 → v0.17.2
- sigs.k8s.io/kustomize/kustomize/v5: 6ce0bf3 → v5.4.2
- sigs.k8s.io/kustomize/kyaml: 6ce0bf3 → v0.17.1
- sigs.k8s.io/sig-storage-lib-external-provisioner/v10: v10.0.0 → v10.0.1

### Removed
- github.com/GoogleCloudPlatform/k8s-cloud-provider: [f118173](https://github.com/GoogleCloudPlatform/k8s-cloud-provider/tree/f118173)
- github.com/antlr/antlr4/runtime/Go/antlr/v4: [8188dc5](https://github.com/antlr/antlr4/tree/runtime/Go/antlr/v4/8188dc5)
- github.com/chromedp/cdproto: [3cf4e6d](https://github.com/chromedp/cdproto/tree/3cf4e6d)
- github.com/chromedp/chromedp: [v0.9.2](https://github.com/chromedp/chromedp/tree/v0.9.2)
- github.com/chromedp/sysutil: [v1.0.0](https://github.com/chromedp/sysutil/tree/v1.0.0)
- github.com/fvbommel/sortorder: [v1.1.0](https://github.com/fvbommel/sortorder/tree/v1.1.0)
- github.com/gobwas/httphead: [v0.1.0](https://github.com/gobwas/httphead/tree/v0.1.0)
- github.com/gobwas/pool: [v0.2.1](https://github.com/gobwas/pool/tree/v0.2.1)
- github.com/gobwas/ws: [v1.2.1](https://github.com/gobwas/ws/tree/v1.2.1)
- github.com/google/s2a-go: [v0.1.7](https://github.com/google/s2a-go/tree/v0.1.7)
- github.com/googleapis/enterprise-certificate-proxy: [v0.2.3](https://github.com/googleapis/enterprise-certificate-proxy/tree/v0.2.3)
- github.com/googleapis/gax-go/v2: [v2.11.0](https://github.com/googleapis/gax-go/tree/v2.11.0)
- github.com/matttproud/golang_protobuf_extensions/v2: [v2.0.0](https://github.com/matttproud/golang_protobuf_extensions/tree/v2.0.0)
- google.golang.org/api: v0.126.0
- gopkg.in/gcfg.v1: v1.2.3
- gopkg.in/warnings.v0: v0.1.2
- k8s.io/legacy-cloud-providers: v0.30.0
