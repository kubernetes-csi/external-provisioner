# Release notes for v5.0.2

## Changes by Kind

### Bug or Regression

- Fixed removal of PV protection finalizer. PVs are no longer Terminating forever after PVC deletion. ([#1251](https://github.com/kubernetes-csi/external-provisioner/pull/1251), [@jsafrane](https://github.com/jsafrane))

### Uncategorized

- Updated go to 1.22.5 ([#1251](https://github.com/kubernetes-csi/external-provisioner/pull/1251), [@jsafrane](https://github.com/jsafrane))

## Dependencies

### Added
_Nothing has changed._

### Changed
- sigs.k8s.io/sig-storage-lib-external-provisioner/v10: v10.0.0 → v10.0.1

### Removed
_Nothing has changed._

# Release notes for v5.0.1

[Documentation](https://kubernetes-csi.github.io)

## Changes by Kind

### Bug or Regression

- Update csi-lib-utils to v0.18.1 ([#1224](https://github.com/kubernetes-csi/external-provisioner/pull/1224), [@rhrmo](https://github.com/rhrmo))

## Dependencies

### Added
_Nothing has changed._

### Changed
- github.com/kubernetes-csi/csi-lib-utils: [v0.18.0 → v0.18.1](https://github.com/kubernetes-csi/csi-lib-utils/compare/v0.18.0...v0.18.1)

### Removed
_Nothing has changed._

# Release notes for v5.0.0

[Documentation](https://kubernetes-csi.github.io)

# Changelog since v4.0.0

## Urgent Upgrade Notes

### (No, really, you MUST read this before you upgrade)

- Feature gate Topology is enabled by default. This feature gate is already GA and will be removed in a future release. CSI drivers are required to report capability `VOLUME_ACCESSIBILITY_CONSTRAINTS` correctly: when a CSI drivers reports the capability, topology is enabled in the external-provisioner. When the driver does not report it, external-provisioner disables topology support. ([#1167](https://github.com/kubernetes-csi/external-provisioner/pull/1167), [@huww98](https://github.com/huww98))
- The external-provisioner now needs permissions to patch persistentvolumes. Please update your RBACs appropriately. See the linked pull request for an example. ([#1155](https://github.com/kubernetes-csi/external-provisioner/pull/1155), [@carlory](https://github.com/carlory))

## Changes by Kind

### Feature

- Added support for PVC annotations in `csi.storage.k8s.io/provisioner-secret-name` StorageClass parameters. For example: `csi.storage.k8s.io/provisioner-secret-name: ${pvc.annotations['xyz']}`. ([#1196](https://github.com/kubernetes-csi/external-provisioner/pull/1196), [@hoyho](https://github.com/hoyho))
- Promote HonorPVReclaimPolicy to beta. ([#1209](https://github.com/kubernetes-csi/external-provisioner/pull/1209), [@carlory](https://github.com/carlory))
- Updated Kubernetes deps to v1.30 ([#1210](https://github.com/kubernetes-csi/external-provisioner/pull/1210), [@jsafrane](https://github.com/jsafrane))

### Bug or Regression

- Fix clone controller shallow copy bug ([#1190](https://github.com/kubernetes-csi/external-provisioner/pull/1190), [@1978629634](https://github.com/1978629634))
- `GetCapacityRequest.VolumeCapabilities` is now set to `nil` instead of some ambiguous default value when invoking `GetCapacity()`. ([#1193](https://github.com/kubernetes-csi/external-provisioner/pull/1193), [@zjx20](https://github.com/zjx20))

### Uncategorized

- Updates google.golang.org/protobuf to v1.33.0 to resolve CVE-2024-24786 ([#1169](https://github.com/kubernetes-csi/external-provisioner/pull/1169), [@humblec](https://github.com/humblec))

## Dependencies

### Added
- github.com/chromedp/cdproto: [3cf4e6d](https://github.com/chromedp/cdproto/tree/3cf4e6d)
- github.com/chromedp/chromedp: [v0.9.2](https://github.com/chromedp/chromedp/tree/v0.9.2)
- github.com/chromedp/sysutil: [v1.0.0](https://github.com/chromedp/sysutil/tree/v1.0.0)
- github.com/fxamacker/cbor/v2: [v2.6.0](https://github.com/fxamacker/cbor/v2/tree/v2.6.0)
- github.com/go-task/slim-sprig/v3: [v3.0.0](https://github.com/go-task/slim-sprig/v3/tree/v3.0.0)
- github.com/gobwas/httphead: [v0.1.0](https://github.com/gobwas/httphead/tree/v0.1.0)
- github.com/gobwas/pool: [v0.2.1](https://github.com/gobwas/pool/tree/v0.2.1)
- github.com/gobwas/ws: [v1.2.1](https://github.com/gobwas/ws/tree/v1.2.1)
- github.com/x448/float16: [v0.8.4](https://github.com/x448/float16/tree/v0.8.4)
- golang.org/x/telemetry: f48c80b
- google.golang.org/grpc/cmd/protoc-gen-go-grpc: v1.3.0
- k8s.io/gengo/v2: 51d4e06
- sigs.k8s.io/knftables: v0.0.14
- sigs.k8s.io/sig-storage-lib-external-provisioner/v10: v10.0.0

### Changed
- cloud.google.com/go/accessapproval: v1.7.3 → v1.7.5
- cloud.google.com/go/accesscontextmanager: v1.8.3 → v1.8.5
- cloud.google.com/go/aiplatform: v1.51.2 → v1.60.0
- cloud.google.com/go/analytics: v0.21.5 → v0.23.0
- cloud.google.com/go/apigateway: v1.6.3 → v1.6.5
- cloud.google.com/go/apigeeconnect: v1.6.3 → v1.6.5
- cloud.google.com/go/apigeeregistry: v0.8.1 → v0.8.3
- cloud.google.com/go/appengine: v1.8.3 → v1.8.5
- cloud.google.com/go/area120: v0.8.3 → v0.8.5
- cloud.google.com/go/artifactregistry: v1.14.4 → v1.14.7
- cloud.google.com/go/asset: v1.15.2 → v1.17.2
- cloud.google.com/go/assuredworkloads: v1.11.3 → v1.11.5
- cloud.google.com/go/automl: v1.13.3 → v1.13.5
- cloud.google.com/go/baremetalsolution: v1.2.2 → v1.2.4
- cloud.google.com/go/batch: v1.6.1 → v1.8.0
- cloud.google.com/go/beyondcorp: v1.0.2 → v1.0.4
- cloud.google.com/go/bigquery: v1.56.0 → v1.59.1
- cloud.google.com/go/billing: v1.17.3 → v1.18.2
- cloud.google.com/go/binaryauthorization: v1.7.2 → v1.8.1
- cloud.google.com/go/certificatemanager: v1.7.3 → v1.7.5
- cloud.google.com/go/channel: v1.17.2 → v1.17.5
- cloud.google.com/go/cloudbuild: v1.14.2 → v1.15.1
- cloud.google.com/go/clouddms: v1.7.2 → v1.7.4
- cloud.google.com/go/cloudtasks: v1.12.3 → v1.12.6
- cloud.google.com/go/compute/metadata: v0.2.3 → v0.3.0
- cloud.google.com/go/compute: v1.23.2 → v1.25.1
- cloud.google.com/go/contactcenterinsights: v1.11.2 → v1.13.0
- cloud.google.com/go/container: v1.26.2 → v1.31.0
- cloud.google.com/go/containeranalysis: v0.11.2 → v0.11.4
- cloud.google.com/go/datacatalog: v1.18.2 → v1.19.3
- cloud.google.com/go/dataflow: v0.9.3 → v0.9.5
- cloud.google.com/go/dataform: v0.8.3 → v0.9.2
- cloud.google.com/go/datafusion: v1.7.3 → v1.7.5
- cloud.google.com/go/datalabeling: v0.8.3 → v0.8.5
- cloud.google.com/go/dataplex: v1.10.2 → v1.14.2
- cloud.google.com/go/dataproc/v2: v2.2.2 → v2.4.0
- cloud.google.com/go/dataqna: v0.8.3 → v0.8.5
- cloud.google.com/go/datastream: v1.10.2 → v1.10.4
- cloud.google.com/go/deploy: v1.14.1 → v1.17.1
- cloud.google.com/go/dialogflow: v1.44.2 → v1.49.0
- cloud.google.com/go/dlp: v1.10.3 → v1.11.2
- cloud.google.com/go/documentai: v1.23.4 → v1.25.0
- cloud.google.com/go/domains: v0.9.3 → v0.9.5
- cloud.google.com/go/edgecontainer: v1.1.3 → v1.1.5
- cloud.google.com/go/essentialcontacts: v1.6.4 → v1.6.6
- cloud.google.com/go/eventarc: v1.13.2 → v1.13.4
- cloud.google.com/go/filestore: v1.7.3 → v1.8.1
- cloud.google.com/go/functions: v1.15.3 → v1.16.0
- cloud.google.com/go/gkebackup: v1.3.3 → v1.3.5
- cloud.google.com/go/gkeconnect: v0.8.3 → v0.8.5
- cloud.google.com/go/gkehub: v0.14.3 → v0.14.5
- cloud.google.com/go/gkemulticloud: v1.0.2 → v1.1.1
- cloud.google.com/go/gsuiteaddons: v1.6.3 → v1.6.5
- cloud.google.com/go/iam: v1.1.4 → v1.1.6
- cloud.google.com/go/iap: v1.9.2 → v1.9.4
- cloud.google.com/go/ids: v1.4.3 → v1.4.5
- cloud.google.com/go/iot: v1.7.3 → v1.7.5
- cloud.google.com/go/kms: v1.15.4 → v1.15.7
- cloud.google.com/go/language: v1.12.1 → v1.12.3
- cloud.google.com/go/lifesciences: v0.9.3 → v0.9.5
- cloud.google.com/go/logging: v1.8.1 → v1.9.0
- cloud.google.com/go/longrunning: v0.5.3 → v0.5.5
- cloud.google.com/go/managedidentities: v1.6.3 → v1.6.5
- cloud.google.com/go/maps: v1.5.1 → v1.6.4
- cloud.google.com/go/mediatranslation: v0.8.3 → v0.8.5
- cloud.google.com/go/memcache: v1.10.3 → v1.10.5
- cloud.google.com/go/metastore: v1.13.2 → v1.13.4
- cloud.google.com/go/monitoring: v1.16.2 → v1.18.0
- cloud.google.com/go/networkconnectivity: v1.14.2 → v1.14.4
- cloud.google.com/go/networkmanagement: v1.9.2 → v1.9.4
- cloud.google.com/go/networksecurity: v0.9.3 → v0.9.5
- cloud.google.com/go/notebooks: v1.11.1 → v1.11.3
- cloud.google.com/go/optimization: v1.6.1 → v1.6.3
- cloud.google.com/go/orchestration: v1.8.3 → v1.8.5
- cloud.google.com/go/orgpolicy: v1.11.3 → v1.12.1
- cloud.google.com/go/osconfig: v1.12.3 → v1.12.5
- cloud.google.com/go/oslogin: v1.12.1 → v1.13.1
- cloud.google.com/go/phishingprotection: v0.8.3 → v0.8.5
- cloud.google.com/go/policytroubleshooter: v1.10.1 → v1.10.3
- cloud.google.com/go/privatecatalog: v0.9.3 → v0.9.5
- cloud.google.com/go/pubsub: v1.33.0 → v1.36.1
- cloud.google.com/go/recaptchaenterprise/v2: v2.8.2 → v2.9.2
- cloud.google.com/go/recommendationengine: v0.8.3 → v0.8.5
- cloud.google.com/go/recommender: v1.11.2 → v1.12.1
- cloud.google.com/go/redis: v1.13.3 → v1.14.2
- cloud.google.com/go/resourcemanager: v1.9.3 → v1.9.5
- cloud.google.com/go/resourcesettings: v1.6.3 → v1.6.5
- cloud.google.com/go/retail: v1.14.3 → v1.16.0
- cloud.google.com/go/run: v1.3.2 → v1.3.4
- cloud.google.com/go/scheduler: v1.10.3 → v1.10.6
- cloud.google.com/go/secretmanager: v1.11.3 → v1.11.5
- cloud.google.com/go/security: v1.15.3 → v1.15.5
- cloud.google.com/go/securitycenter: v1.24.1 → v1.24.4
- cloud.google.com/go/servicedirectory: v1.11.2 → v1.11.4
- cloud.google.com/go/shell: v1.7.3 → v1.7.5
- cloud.google.com/go/spanner: v1.51.0 → v1.57.0
- cloud.google.com/go/speech: v1.19.2 → v1.21.1
- cloud.google.com/go/storagetransfer: v1.10.2 → v1.10.4
- cloud.google.com/go/talent: v1.6.4 → v1.6.6
- cloud.google.com/go/texttospeech: v1.7.3 → v1.7.5
- cloud.google.com/go/tpu: v1.6.3 → v1.6.5
- cloud.google.com/go/trace: v1.10.3 → v1.10.5
- cloud.google.com/go/translate: v1.9.2 → v1.10.1
- cloud.google.com/go/video: v1.20.2 → v1.20.4
- cloud.google.com/go/videointelligence: v1.11.3 → v1.11.5
- cloud.google.com/go/vision/v2: v2.7.4 → v2.8.0
- cloud.google.com/go/vmmigration: v1.7.3 → v1.7.5
- cloud.google.com/go/vmwareengine: v1.0.2 → v1.1.1
- cloud.google.com/go/vpcaccess: v1.7.3 → v1.7.5
- cloud.google.com/go/webrisk: v1.9.3 → v1.9.5
- cloud.google.com/go/websecurityscanner: v1.6.3 → v1.6.5
- cloud.google.com/go/workflows: v1.12.2 → v1.12.4
- cloud.google.com/go: v0.110.9 → v0.112.0
- github.com/alecthomas/kingpin/v2: [v2.3.2 → v2.4.0](https://github.com/alecthomas/kingpin/v2/compare/v2.3.2...v2.4.0)
- github.com/cenkalti/backoff/v4: [v4.2.1 → v4.3.0](https://github.com/cenkalti/backoff/v4/compare/v4.2.1...v4.3.0)
- github.com/cespare/xxhash/v2: [v2.2.0 → v2.3.0](https://github.com/cespare/xxhash/v2/compare/v2.2.0...v2.3.0)
- github.com/chzyer/readline: [2972be2 → v1.5.1](https://github.com/chzyer/readline/compare/2972be2...v1.5.1)
- github.com/cncf/xds/go: [e9ce688 → 8a4994d](https://github.com/cncf/xds/go/compare/e9ce688...8a4994d)
- github.com/distribution/reference: [v0.5.0 → v0.6.0](https://github.com/distribution/reference/compare/v0.5.0...v0.6.0)
- github.com/emicklei/go-restful/v3: [v3.11.0 → v3.12.0](https://github.com/emicklei/go-restful/v3/compare/v3.11.0...v3.12.0)
- github.com/envoyproxy/go-control-plane: [v0.11.1 → v0.12.0](https://github.com/envoyproxy/go-control-plane/compare/v0.11.1...v0.12.0)
- github.com/envoyproxy/protoc-gen-validate: [v1.0.2 → v1.0.4](https://github.com/envoyproxy/protoc-gen-validate/compare/v1.0.2...v1.0.4)
- github.com/evanphx/json-patch/v5: [v5.7.0 → v5.9.0](https://github.com/evanphx/json-patch/v5/compare/v5.7.0...v5.9.0)
- github.com/evanphx/json-patch: [v5.7.0+incompatible → v5.9.0+incompatible](https://github.com/evanphx/json-patch/compare/v5.7.0...v5.9.0)
- github.com/fatih/color: [v1.15.0 → v1.16.0](https://github.com/fatih/color/compare/v1.15.0...v1.16.0)
- github.com/go-logr/logr: [v1.3.0 → v1.4.1](https://github.com/go-logr/logr/compare/v1.3.0...v1.4.1)
- github.com/go-logr/zapr: [v1.2.4 → v1.3.0](https://github.com/go-logr/zapr/compare/v1.2.4...v1.3.0)
- github.com/go-openapi/jsonpointer: [v0.20.1 → v0.21.0](https://github.com/go-openapi/jsonpointer/compare/v0.20.1...v0.21.0)
- github.com/go-openapi/jsonreference: [v0.20.3 → v0.21.0](https://github.com/go-openapi/jsonreference/compare/v0.20.3...v0.21.0)
- github.com/go-openapi/swag: [v0.22.5 → v0.23.0](https://github.com/go-openapi/swag/compare/v0.22.5...v0.23.0)
- github.com/go-task/slim-sprig: [v2.20.0+incompatible → 52ccab3](https://github.com/go-task/slim-sprig/compare/v2.20.0...52ccab3)
- github.com/golang/glog: [v1.1.2 → v1.2.0](https://github.com/golang/glog/compare/v1.1.2...v1.2.0)
- github.com/golang/protobuf: [v1.5.3 → v1.5.4](https://github.com/golang/protobuf/compare/v1.5.3...v1.5.4)
- github.com/google/cadvisor: [v0.48.1 → v0.49.0](https://github.com/google/cadvisor/compare/v0.48.1...v0.49.0)
- github.com/google/cel-go: [v0.17.7 → v0.17.8](https://github.com/google/cel-go/compare/v0.17.7...v0.17.8)
- github.com/google/pprof: [4bb14d4 → a892ee0](https://github.com/google/pprof/compare/4bb14d4...a892ee0)
- github.com/google/uuid: [v1.5.0 → v1.6.0](https://github.com/google/uuid/compare/v1.5.0...v1.6.0)
- github.com/grpc-ecosystem/grpc-gateway/v2: [v2.18.1 → v2.20.0](https://github.com/grpc-ecosystem/grpc-gateway/v2/compare/v2.18.1...v2.20.0)
- github.com/ianlancetaylor/demangle: [28f6c0f → bd984b5](https://github.com/ianlancetaylor/demangle/compare/28f6c0f...bd984b5)
- github.com/kubernetes-csi/csi-lib-utils: [v0.17.0 → v0.18.0](https://github.com/kubernetes-csi/csi-lib-utils/compare/v0.17.0...v0.18.0)
- github.com/mattn/go-isatty: [v0.0.17 → v0.0.20](https://github.com/mattn/go-isatty/compare/v0.0.17...v0.0.20)
- github.com/miekg/dns: [v1.1.57 → v1.1.59](https://github.com/miekg/dns/compare/v1.1.57...v1.1.59)
- github.com/onsi/ginkgo/v2: [v2.13.2 → v2.17.3](https://github.com/onsi/ginkgo/v2/compare/v2.13.2...v2.17.3)
- github.com/onsi/gomega: [v1.30.0 → v1.33.1](https://github.com/onsi/gomega/compare/v1.30.0...v1.33.1)
- github.com/opencontainers/runc: [v1.1.10 → v1.1.12](https://github.com/opencontainers/runc/compare/v1.1.10...v1.1.12)
- github.com/prometheus/client_golang: [v1.17.0 → v1.18.0](https://github.com/prometheus/client_golang/compare/v1.17.0...v1.18.0)
- github.com/prometheus/client_model: [v0.5.0 → v0.6.1](https://github.com/prometheus/client_model/compare/v0.5.0...v0.6.1)
- github.com/prometheus/common: [v0.45.0 → v0.46.0](https://github.com/prometheus/common/compare/v0.45.0...v0.46.0)
- github.com/prometheus/procfs: [v0.12.0 → v0.15.0](https://github.com/prometheus/procfs/compare/v0.12.0...v0.15.0)
- github.com/stretchr/objx: [v0.5.0 → v0.5.2](https://github.com/stretchr/objx/compare/v0.5.0...v0.5.2)
- github.com/stretchr/testify: [v1.8.4 → v1.9.0](https://github.com/stretchr/testify/compare/v1.8.4...v1.9.0)
- go.etcd.io/etcd/api/v3: v3.5.11 → v3.5.13
- go.etcd.io/etcd/client/pkg/v3: v3.5.11 → v3.5.13
- go.etcd.io/etcd/client/v3: v3.5.11 → v3.5.13
- go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc: v0.46.1 → v0.51.0
- go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp: v0.46.1 → v0.51.0
- go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc: v1.21.0 → v1.26.0
- go.opentelemetry.io/otel/exporters/otlp/otlptrace: v1.21.0 → v1.26.0
- go.opentelemetry.io/otel/metric: v1.21.0 → v1.26.0
- go.opentelemetry.io/otel/sdk: v1.21.0 → v1.26.0
- go.opentelemetry.io/otel/trace: v1.21.0 → v1.26.0
- go.opentelemetry.io/otel: v1.21.0 → v1.26.0
- go.opentelemetry.io/proto/otlp: v1.0.0 → v1.2.0
- go.uber.org/atomic: v1.10.0 → v1.7.0
- go.uber.org/zap: v1.26.0 → v1.27.0
- golang.org/x/crypto: v0.17.0 → v0.23.0
- golang.org/x/exp: 7918f67 → fe59bbe
- golang.org/x/mod: v0.14.0 → v0.17.0
- golang.org/x/net: v0.19.0 → v0.25.0
- golang.org/x/oauth2: v0.15.0 → v0.20.0
- golang.org/x/sync: v0.5.0 → v0.7.0
- golang.org/x/sys: v0.15.0 → v0.20.0
- golang.org/x/term: v0.15.0 → v0.20.0
- golang.org/x/text: v0.14.0 → v0.15.0
- golang.org/x/tools: v0.16.1 → v0.21.0
- google.golang.org/genproto/googleapis/api: bbf56f3 → 0867130
- google.golang.org/genproto/googleapis/rpc: bbf56f3 → 0867130
- google.golang.org/genproto: d783a09 → 6ceb2ff
- google.golang.org/grpc: v1.60.0 → v1.64.0
- google.golang.org/protobuf: v1.31.0 → v1.34.1
- k8s.io/api: v0.29.0 → v0.30.0
- k8s.io/apiextensions-apiserver: v0.29.0 → v0.30.0
- k8s.io/apimachinery: v0.29.0 → v0.30.0
- k8s.io/apiserver: v0.29.0 → v0.30.0
- k8s.io/cli-runtime: v0.29.0 → v0.30.0
- k8s.io/client-go: v0.29.0 → v0.30.0
- k8s.io/cloud-provider: v0.29.0 → v0.30.0
- k8s.io/cluster-bootstrap: v0.29.0 → v0.30.0
- k8s.io/code-generator: v0.29.0 → v0.30.0
- k8s.io/component-base: v0.29.0 → v0.30.0
- k8s.io/component-helpers: v0.29.0 → v0.30.0
- k8s.io/controller-manager: v0.29.0 → v0.30.0
- k8s.io/cri-api: v0.29.0 → v0.30.0
- k8s.io/csi-translation-lib: v0.29.0 → v0.30.0
- k8s.io/dynamic-resource-allocation: v0.29.0 → v0.30.0
- k8s.io/endpointslice: v0.29.0 → v0.30.0
- k8s.io/klog/v2: v2.110.1 → v2.120.1
- k8s.io/kms: v0.29.0 → v0.30.0
- k8s.io/kube-aggregator: v0.29.0 → v0.30.0
- k8s.io/kube-controller-manager: v0.29.0 → v0.30.0
- k8s.io/kube-openapi: 2dd684a → 8948a66
- k8s.io/kube-proxy: v0.29.0 → v0.30.0
- k8s.io/kube-scheduler: v0.29.0 → v0.30.0
- k8s.io/kubectl: v0.29.0 → v0.30.0
- k8s.io/kubelet: v0.29.0 → v0.30.0
- k8s.io/kubernetes: v1.29.0 → v1.30.1
- k8s.io/legacy-cloud-providers: v0.29.0 → v0.30.0
- k8s.io/metrics: v0.29.0 → v0.30.0
- k8s.io/mount-utils: v0.29.0 → v0.30.0
- k8s.io/pod-security-admission: v0.29.0 → v0.30.0
- k8s.io/sample-apiserver: v0.29.0 → v0.30.0
- k8s.io/utils: 3b25d92 → 0849a56
- sigs.k8s.io/apiserver-network-proxy/konnectivity-client: v0.28.3 → v0.30.3
- sigs.k8s.io/controller-runtime: v0.16.3 → v0.18.2
- sigs.k8s.io/controller-tools: v0.13.0 → v0.15.0
- sigs.k8s.io/gateway-api: v1.0.0 → v1.1.0

### Removed
- github.com/Azure/azure-sdk-for-go: [v68.0.0+incompatible](https://github.com/Azure/azure-sdk-for-go/tree/v68.0.0)
- github.com/Azure/go-autorest/autorest/adal: [v0.9.23](https://github.com/Azure/go-autorest/autorest/adal/tree/v0.9.23)
- github.com/Azure/go-autorest/autorest/date: [v0.3.0](https://github.com/Azure/go-autorest/autorest/date/tree/v0.3.0)
- github.com/Azure/go-autorest/autorest/mocks: [v0.4.2](https://github.com/Azure/go-autorest/autorest/mocks/tree/v0.4.2)
- github.com/Azure/go-autorest/autorest/to: [v0.4.0](https://github.com/Azure/go-autorest/autorest/to/tree/v0.4.0)
- github.com/Azure/go-autorest/autorest/validation: [v0.3.1](https://github.com/Azure/go-autorest/autorest/validation/tree/v0.3.1)
- github.com/Azure/go-autorest/autorest: [v0.11.29](https://github.com/Azure/go-autorest/autorest/tree/v0.11.29)
- github.com/Azure/go-autorest/logger: [v0.2.1](https://github.com/Azure/go-autorest/logger/tree/v0.2.1)
- github.com/Azure/go-autorest/tracing: [v0.6.0](https://github.com/Azure/go-autorest/tracing/tree/v0.6.0)
- github.com/Azure/go-autorest: [v14.2.0+incompatible](https://github.com/Azure/go-autorest/tree/v14.2.0)
- github.com/Masterminds/goutils: [v1.1.1](https://github.com/Masterminds/goutils/tree/v1.1.1)
- github.com/Masterminds/semver: [v1.5.0](https://github.com/Masterminds/semver/tree/v1.5.0)
- github.com/chzyer/logex: [v1.1.10](https://github.com/chzyer/logex/tree/v1.1.10)
- github.com/chzyer/test: [a1ea475](https://github.com/chzyer/test/tree/a1ea475)
- github.com/cncf/udpa/go: [c52dc94](https://github.com/cncf/udpa/go/tree/c52dc94)
- github.com/danwinship/knftables: [v0.0.13](https://github.com/danwinship/knftables/tree/v0.0.13)
- github.com/gofrs/uuid: [v4.4.0+incompatible](https://github.com/gofrs/uuid/tree/v4.4.0)
- github.com/huandu/xstrings: [v1.4.0](https://github.com/huandu/xstrings/tree/v1.4.0)
- github.com/rubiojr/go-vhd: [02e2102](https://github.com/rubiojr/go-vhd/tree/02e2102)
- github.com/vmware/govmomi: [v0.30.6](https://github.com/vmware/govmomi/tree/v0.30.6)
- go.opentelemetry.io/otel/exporters/otlp/internal/retry: v1.10.0
- sigs.k8s.io/sig-storage-lib-external-provisioner/v9: v9.1.0-rc.0
