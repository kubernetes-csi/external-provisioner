# Release notes for 6.0.0

[Documentation](https://kubernetes-csi.github.io)

# Changelog since 5.3.0

## Changes by Kind

### Feature

- Bump CSI spec to 1.12 ([#1428](https://github.com/kubernetes-csi/external-provisioner/pull/1428), [@sunnylovestiramisu](https://github.com/sunnylovestiramisu))
- Disable VolumeAttributesClass Feature Gate If V1 API Not Present ([#1423](https://github.com/kubernetes-csi/external-provisioner/pull/1423), [@sunnylovestiramisu](https://github.com/sunnylovestiramisu))
- Promote VolumeAttributesClass to GA ([#1400](https://github.com/kubernetes-csi/external-provisioner/pull/1400), [@carlory](https://github.com/carlory))
- Update gateway-api to v1.4.0 from rc ([#1420](https://github.com/kubernetes-csi/external-provisioner/pull/1420), [@sunnylovestiramisu](https://github.com/sunnylovestiramisu))
- Update kubernetes dependencies to v1.34.0 ([#1414](https://github.com/kubernetes-csi/external-provisioner/pull/1414), [@sunnylovestiramisu](https://github.com/sunnylovestiramisu))
- Update to sig-storage-lib-external-provisioner/v12 ([#1416](https://github.com/kubernetes-csi/external-provisioner/pull/1416), [@sunnylovestiramisu](https://github.com/sunnylovestiramisu))

### Bug or Regression

- Add In Memory Cache of Node/CSINode Info for Long Provisioning to Prevent Volume Leak ([#1413](https://github.com/kubernetes-csi/external-provisioner/pull/1413), [@sunnylovestiramisu](https://github.com/sunnylovestiramisu))
- Fix --automaxprocs flag not being recognized due to incorrect initialization order ([#1397](https://github.com/kubernetes-csi/external-provisioner/pull/1397), [@iPraveenParihar](https://github.com/iPraveenParihar))

### Other (Cleanup or Flake)

- Exclude explicitSet feature gate tuning ([#1425](https://github.com/kubernetes-csi/external-provisioner/pull/1425), [@sunnylovestiramisu](https://github.com/sunnylovestiramisu))

### Uncategorized

- Added Ability To Retry Provisions that Return InvalidArgument At retryIntervalMax ([#1369](https://github.com/kubernetes-csi/external-provisioner/pull/1369), [@mdzraf](https://github.com/mdzraf))

## Dependencies

### Added
- github.com/go-openapi/swag/cmdutils: [v0.24.0](https://github.com/go-openapi/swag/cmdutils/tree/v0.24.0)
- github.com/go-openapi/swag/conv: [v0.24.0](https://github.com/go-openapi/swag/conv/tree/v0.24.0)
- github.com/go-openapi/swag/fileutils: [v0.24.0](https://github.com/go-openapi/swag/fileutils/tree/v0.24.0)
- github.com/go-openapi/swag/jsonname: [v0.24.0](https://github.com/go-openapi/swag/jsonname/tree/v0.24.0)
- github.com/go-openapi/swag/jsonutils: [v0.24.0](https://github.com/go-openapi/swag/jsonutils/tree/v0.24.0)
- github.com/go-openapi/swag/loading: [v0.24.0](https://github.com/go-openapi/swag/loading/tree/v0.24.0)
- github.com/go-openapi/swag/mangling: [v0.24.0](https://github.com/go-openapi/swag/mangling/tree/v0.24.0)
- github.com/go-openapi/swag/netutils: [v0.24.0](https://github.com/go-openapi/swag/netutils/tree/v0.24.0)
- github.com/go-openapi/swag/stringutils: [v0.24.0](https://github.com/go-openapi/swag/stringutils/tree/v0.24.0)
- github.com/go-openapi/swag/typeutils: [v0.24.0](https://github.com/go-openapi/swag/typeutils/tree/v0.24.0)
- github.com/go-openapi/swag/yamlutils: [v0.24.0](https://github.com/go-openapi/swag/yamlutils/tree/v0.24.0)
- github.com/golang-jwt/jwt/v5: [v5.2.2](https://github.com/golang-jwt/jwt/v5/tree/v5.2.2)
- github.com/grafana/regexp: [a468a5b](https://github.com/grafana/regexp/tree/a468a5b)
- github.com/grpc-ecosystem/go-grpc-middleware/providers/prometheus: [v1.0.1](https://github.com/grpc-ecosystem/go-grpc-middleware/providers/prometheus/tree/v1.0.1)
- github.com/grpc-ecosystem/go-grpc-middleware/v2: [v2.3.0](https://github.com/grpc-ecosystem/go-grpc-middleware/v2/tree/v2.3.0)
- go.etcd.io/raft/v3: v3.6.0
- go.yaml.in/yaml/v2: v2.4.2
- go.yaml.in/yaml/v3: v3.0.4
- golang.org/x/tools/go/expect: v0.1.1-deprecated
- golang.org/x/tools/go/packages/packagestest: v0.1.1-deprecated
- gonum.org/v1/gonum: v0.16.0
- sigs.k8s.io/sig-storage-lib-external-provisioner/v13: v13.0.0
- sigs.k8s.io/structured-merge-diff/v6: v6.3.0

### Changed
- cloud.google.com/go/compute/metadata: v0.6.0 → v0.7.0
- github.com/GoogleCloudPlatform/opentelemetry-operations-go/detectors/gcp: [v1.26.0 → v1.29.0](https://github.com/GoogleCloudPlatform/opentelemetry-operations-go/detectors/gcp/compare/v1.26.0...v1.29.0)
- github.com/Microsoft/hnslib: [v0.0.8 → v0.1.1](https://github.com/Microsoft/hnslib/compare/v0.0.8...v0.1.1)
- github.com/cncf/xds/go: [2f00578 → 2ac532f](https://github.com/cncf/xds/go/compare/2f00578...2ac532f)
- github.com/container-storage-interface/spec: [v1.11.0 → v1.12.0](https://github.com/container-storage-interface/spec/compare/v1.11.0...v1.12.0)
- github.com/containerd/containerd/api: [v1.9.0 → v1.8.0](https://github.com/containerd/containerd/api/compare/v1.9.0...v1.8.0)
- github.com/containerd/ttrpc: [v1.2.7 → v1.2.6](https://github.com/containerd/ttrpc/compare/v1.2.7...v1.2.6)
- github.com/containerd/typeurl/v2: [v2.2.3 → v2.2.2](https://github.com/containerd/typeurl/v2/compare/v2.2.3...v2.2.2)
- github.com/coredns/corefile-migration: [v1.0.25 → v1.0.26](https://github.com/coredns/corefile-migration/compare/v1.0.25...v1.0.26)
- github.com/elastic/crd-ref-docs: [v0.1.0 → v0.2.0](https://github.com/elastic/crd-ref-docs/compare/v0.1.0...v0.2.0)
- github.com/emicklei/go-restful/v3: [v3.12.2 → v3.13.0](https://github.com/emicklei/go-restful/v3/compare/v3.12.2...v3.13.0)
- github.com/fxamacker/cbor/v2: [v2.8.0 → v2.9.0](https://github.com/fxamacker/cbor/v2/compare/v2.8.0...v2.9.0)
- github.com/go-jose/go-jose/v4: [v4.0.4 → v4.1.1](https://github.com/go-jose/go-jose/v4/compare/v4.0.4...v4.1.1)
- github.com/go-logr/logr: [v1.4.2 → v1.4.3](https://github.com/go-logr/logr/compare/v1.4.2...v1.4.3)
- github.com/go-openapi/jsonpointer: [v0.21.1 → v0.22.0](https://github.com/go-openapi/jsonpointer/compare/v0.21.1...v0.22.0)
- github.com/go-openapi/jsonreference: [v0.21.0 → v0.21.1](https://github.com/go-openapi/jsonreference/compare/v0.21.0...v0.21.1)
- github.com/go-openapi/swag: [v0.23.1 → v0.24.1](https://github.com/go-openapi/swag/compare/v0.23.1...v0.24.1)
- github.com/goccy/go-yaml: [v1.11.3 → v1.18.0](https://github.com/goccy/go-yaml/compare/v1.11.3...v1.18.0)
- github.com/golang/glog: [v1.2.4 → v1.2.5](https://github.com/golang/glog/compare/v1.2.4...v1.2.5)
- github.com/google/cel-go: [v0.23.2 → v0.26.0](https://github.com/google/cel-go/compare/v0.23.2...v0.26.0)
- github.com/google/gnostic-models: [v0.6.9 → v0.7.0](https://github.com/google/gnostic-models/compare/v0.6.9...v0.7.0)
- github.com/ishidawataru/sctp: [7ff4192 → ae8eb7f](https://github.com/ishidawataru/sctp/compare/7ff4192...ae8eb7f)
- github.com/jonboulle/clockwork: [v0.4.0 → v0.5.0](https://github.com/jonboulle/clockwork/compare/v0.4.0...v0.5.0)
- github.com/kubernetes-csi/csi-test/v5: [v5.3.1 → v5.4.0](https://github.com/kubernetes-csi/csi-test/v5/compare/v5.3.1...v5.4.0)
- github.com/miekg/dns: [v1.1.66 → v1.1.68](https://github.com/miekg/dns/compare/v1.1.66...v1.1.68)
- github.com/modern-go/reflect2: [v1.0.2 → 35a7c28](https://github.com/modern-go/reflect2/compare/v1.0.2...35a7c28)
- github.com/opencontainers/cgroups: [v0.0.2 → v0.0.1](https://github.com/opencontainers/cgroups/compare/v0.0.2...v0.0.1)
- github.com/opencontainers/runtime-spec: [v1.2.1 → v1.2.0](https://github.com/opencontainers/runtime-spec/compare/v1.2.1...v1.2.0)
- github.com/prometheus/client_golang: [v1.22.0 → v1.23.2](https://github.com/prometheus/client_golang/compare/v1.22.0...v1.23.2)
- github.com/prometheus/common: [v0.64.0 → v0.66.1](https://github.com/prometheus/common/compare/v0.64.0...v0.66.1)
- github.com/prometheus/procfs: [v0.16.1 → v0.17.0](https://github.com/prometheus/procfs/compare/v0.16.1...v0.17.0)
- github.com/spf13/pflag: [v1.0.6 → v1.0.7](https://github.com/spf13/pflag/compare/v1.0.6...v1.0.7)
- github.com/stretchr/testify: [v1.10.0 → v1.11.1](https://github.com/stretchr/testify/compare/v1.10.0...v1.11.1)
- github.com/vishvananda/netlink: [62fb240 → v1.3.1](https://github.com/vishvananda/netlink/compare/62fb240...v1.3.1)
- github.com/vishvananda/netns: [v0.0.4 → v0.0.5](https://github.com/vishvananda/netns/compare/v0.0.4...v0.0.5)
- go.etcd.io/bbolt: v1.3.11 → v1.4.2
- go.etcd.io/etcd/api/v3: v3.5.21 → v3.6.4
- go.etcd.io/etcd/client/pkg/v3: v3.5.21 → v3.6.4
- go.etcd.io/etcd/client/v3: v3.5.21 → v3.6.4
- go.etcd.io/etcd/pkg/v3: v3.5.21 → v3.6.4
- go.etcd.io/etcd/server/v3: v3.5.21 → v3.6.4
- go.opentelemetry.io/contrib/detectors/gcp: v1.34.0 → v1.36.0
- go.opentelemetry.io/contrib/instrumentation/github.com/emicklei/go-restful/otelrestful: v0.42.0 → v0.44.0
- go.opentelemetry.io/otel/metric: v1.35.0 → v1.37.0
- go.opentelemetry.io/otel/sdk/metric: v1.35.0 → v1.37.0
- go.opentelemetry.io/otel/sdk: v1.35.0 → v1.37.0
- go.opentelemetry.io/otel/trace: v1.35.0 → v1.37.0
- go.opentelemetry.io/otel: v1.35.0 → v1.37.0
- golang.org/x/crypto: v0.38.0 → v0.41.0
- golang.org/x/mod: v0.24.0 → v0.27.0
- golang.org/x/net: v0.40.0 → v0.43.0
- golang.org/x/sync: v0.14.0 → v0.16.0
- golang.org/x/sys: v0.33.0 → v0.35.0
- golang.org/x/telemetry: bda5523 → 1a19826
- golang.org/x/term: v0.32.0 → v0.34.0
- golang.org/x/text: v0.25.0 → v0.28.0
- golang.org/x/time: v0.11.0 → v0.12.0
- golang.org/x/tools: v0.33.0 → v0.36.0
- golang.org/x/xerrors: 104605a → 5ec99f8
- google.golang.org/genproto/googleapis/api: 10db94c → 8d1bb00
- google.golang.org/genproto/googleapis/rpc: 10db94c → ef028d9
- google.golang.org/grpc: v1.72.1 → v1.75.1
- google.golang.org/protobuf: v1.36.6 → v1.36.8
- gopkg.in/evanphx/json-patch.v4: v4.12.0 → v4.13.0
- k8s.io/api: v0.33.0 → v0.34.0
- k8s.io/apiextensions-apiserver: v0.33.0 → v0.34.0
- k8s.io/apimachinery: v0.33.0 → v0.34.1
- k8s.io/apiserver: v0.33.0 → v0.34.1
- k8s.io/cli-runtime: v0.33.0 → v0.34.0
- k8s.io/client-go: v0.33.0 → v0.34.1
- k8s.io/cloud-provider: v0.33.0 → v0.34.0
- k8s.io/cluster-bootstrap: v0.33.0 → v0.34.0
- k8s.io/code-generator: v0.33.0 → v0.34.0
- k8s.io/component-base: v0.33.0 → v0.34.0
- k8s.io/component-helpers: v0.33.0 → v0.34.0
- k8s.io/controller-manager: v0.33.0 → v0.34.0
- k8s.io/cri-api: v0.33.0 → v0.34.1
- k8s.io/cri-client: v0.33.0 → v0.34.0
- k8s.io/csi-translation-lib: v0.33.0 → v0.34.0
- k8s.io/dynamic-resource-allocation: v0.33.0 → v0.34.0
- k8s.io/endpointslice: v0.33.0 → v0.34.0
- k8s.io/externaljwt: v0.33.0 → v0.34.1
- k8s.io/gengo/v2: 1244d31 → c297c0c
- k8s.io/kms: v0.33.0 → v0.34.0
- k8s.io/kube-aggregator: v0.33.0 → v0.34.0
- k8s.io/kube-controller-manager: v0.33.0 → v0.34.0
- k8s.io/kube-openapi: c8a335a → 7fc2783
- k8s.io/kube-proxy: v0.33.0 → v0.34.0
- k8s.io/kube-scheduler: v0.33.0 → v0.34.0
- k8s.io/kubectl: v0.33.0 → v0.34.0
- k8s.io/kubelet: v0.33.0 → v0.34.0
- k8s.io/kubernetes: v1.33.1 → v1.34.1
- k8s.io/metrics: v0.33.0 → v0.34.0
- k8s.io/mount-utils: v0.33.0 → v0.34.1
- k8s.io/pod-security-admission: v0.33.0 → v0.34.0
- k8s.io/sample-apiserver: v0.33.0 → v0.34.0
- k8s.io/system-validators: v1.9.1 → v1.10.1
- k8s.io/utils: 24370be → 0af2bda
- sigs.k8s.io/apiserver-network-proxy/konnectivity-client: v0.32.1 → v0.33.0
- sigs.k8s.io/controller-runtime: v0.21.0 → v0.22.3
- sigs.k8s.io/controller-tools: v0.17.3 → v0.19.0
- sigs.k8s.io/gateway-api: v1.3.0 → v1.4.0
- sigs.k8s.io/json: cfa47c3 → 2d32026
- sigs.k8s.io/kustomize/api: v0.19.0 → v0.20.1
- sigs.k8s.io/kustomize/kustomize/v5: v5.6.0 → v5.7.1
- sigs.k8s.io/kustomize/kyaml: v0.19.0 → v0.20.1
- sigs.k8s.io/structured-merge-diff/v4: v4.7.0 → v4.6.0
- sigs.k8s.io/yaml: v1.4.0 → v1.6.0

### Removed
- github.com/aws/aws-sdk-go-v2/config: [v1.27.24](https://github.com/aws/aws-sdk-go-v2/config/tree/v1.27.24)
- github.com/aws/aws-sdk-go-v2/credentials: [v1.17.24](https://github.com/aws/aws-sdk-go-v2/credentials/tree/v1.17.24)
- github.com/aws/aws-sdk-go-v2/feature/ec2/imds: [v1.16.9](https://github.com/aws/aws-sdk-go-v2/feature/ec2/imds/tree/v1.16.9)
- github.com/aws/aws-sdk-go-v2/internal/configsources: [v1.3.13](https://github.com/aws/aws-sdk-go-v2/internal/configsources/tree/v1.3.13)
- github.com/aws/aws-sdk-go-v2/internal/endpoints/v2: [v2.6.13](https://github.com/aws/aws-sdk-go-v2/internal/endpoints/v2/tree/v2.6.13)
- github.com/aws/aws-sdk-go-v2/internal/ini: [v1.8.0](https://github.com/aws/aws-sdk-go-v2/internal/ini/tree/v1.8.0)
- github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding: [v1.11.3](https://github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding/tree/v1.11.3)
- github.com/aws/aws-sdk-go-v2/service/internal/presigned-url: [v1.11.15](https://github.com/aws/aws-sdk-go-v2/service/internal/presigned-url/tree/v1.11.15)
- github.com/aws/aws-sdk-go-v2/service/sso: [v1.22.1](https://github.com/aws/aws-sdk-go-v2/service/sso/tree/v1.22.1)
- github.com/aws/aws-sdk-go-v2/service/ssooidc: [v1.26.2](https://github.com/aws/aws-sdk-go-v2/service/ssooidc/tree/v1.26.2)
- github.com/aws/aws-sdk-go-v2/service/sts: [v1.30.1](https://github.com/aws/aws-sdk-go-v2/service/sts/tree/v1.30.1)
- github.com/aws/aws-sdk-go-v2: [v1.30.1](https://github.com/aws/aws-sdk-go-v2/tree/v1.30.1)
- github.com/aws/smithy-go: [v1.20.3](https://github.com/aws/smithy-go/tree/v1.20.3)
- github.com/cilium/ebpf: [v0.17.3](https://github.com/cilium/ebpf/tree/v0.17.3)
- github.com/docker/docker: [v26.1.4+incompatible](https://github.com/docker/docker/tree/v26.1.4)
- github.com/docker/go-connections: [v0.5.0](https://github.com/docker/go-connections/tree/v0.5.0)
- github.com/go-task/slim-sprig: [52ccab3](https://github.com/go-task/slim-sprig/tree/52ccab3)
- github.com/golang-jwt/jwt/v4: [v4.5.2](https://github.com/golang-jwt/jwt/v4/tree/v4.5.2)
- github.com/golang/groupcache: [41bb18b](https://github.com/golang/groupcache/tree/41bb18b)
- github.com/google/shlex: [e7afc7f](https://github.com/google/shlex/tree/e7afc7f)
- github.com/grpc-ecosystem/go-grpc-middleware: [v1.3.0](https://github.com/grpc-ecosystem/go-grpc-middleware/tree/v1.3.0)
- github.com/grpc-ecosystem/grpc-gateway: [v1.16.0](https://github.com/grpc-ecosystem/grpc-gateway/tree/v1.16.0)
- github.com/moby/docker-image-spec: [v1.3.1](https://github.com/moby/docker-image-spec/tree/v1.3.1)
- github.com/morikuni/aec: [v1.0.0](https://github.com/morikuni/aec/tree/v1.0.0)
- github.com/opencontainers/runc: [v1.2.5](https://github.com/opencontainers/runc/tree/v1.2.5)
- github.com/russross/blackfriday: [v1.6.0](https://github.com/russross/blackfriday/tree/v1.6.0)
- github.com/santhosh-tekuri/jsonschema/v5: [v5.3.1](https://github.com/santhosh-tekuri/jsonschema/v5/tree/v5.3.1)
- go.etcd.io/etcd/client/v2: v2.305.21
- go.etcd.io/etcd/raft/v3: v3.5.21
- go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp: v1.27.0
- google.golang.org/genproto: ef43131
- gotest.tools/v3: v3.0.2
- sigs.k8s.io/sig-storage-lib-external-provisioner/v11: v11.0.1
