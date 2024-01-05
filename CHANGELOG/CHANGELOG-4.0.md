# Release notes for v4.0.0

[Documentation](https://kubernetes-csi.github.io)

# Changelog since v3.6.0

## Urgent Upgrade Notes 

### (No, really, you MUST read this before you upgrade)

- Enable prevent-volume-mode-conversion feature flag by default.
  
  Volume mode change will be rejected when creating a PVC from a VolumeSnapshot unless the AllowVolumeModeChange annotation has been set to true. Applications relying on volume mode change when creating a PVC from VolumeSnapshot need to be updated accordingly. ([#1126](https://github.com/kubernetes-csi/external-provisioner/pull/1126), [@akalenyu](https://github.com/akalenyu))
 
## Changes by Kind

### Feature

- Pass VolumeAttributesClass Parameters as MutableParameters to CreateVolume ([#1068](https://github.com/kubernetes-csi/external-provisioner/pull/1068), [@ConnorJC3](https://github.com/ConnorJC3))

### Bug or Regression

- CVE fixes: CVE-2023-44487 ([#1051](https://github.com/kubernetes-csi/external-provisioner/pull/1051), [@dannawang0221](https://github.com/dannawang0221))

### Uncategorized

- Fix: CVE-2023-48795 ([#1132](https://github.com/kubernetes-csi/external-provisioner/pull/1132), [@dobsonj](https://github.com/dobsonj))
- Update kubernetes dependencies to v1.29.0 ([#1133](https://github.com/kubernetes-csi/external-provisioner/pull/1133), [@sunnylovestiramisu](https://github.com/sunnylovestiramisu))

## Dependencies

### Added
- github.com/danwinship/knftables: [v0.0.13](https://github.com/danwinship/knftables/tree/v0.0.13)
- github.com/distribution/reference: [v0.5.0](https://github.com/distribution/reference/tree/v0.5.0)
- github.com/google/s2a-go: [v0.1.7](https://github.com/google/s2a-go/tree/v0.1.7)
- github.com/matttproud/golang_protobuf_extensions/v2: [v2.0.0](https://github.com/matttproud/golang_protobuf_extensions/v2/tree/v2.0.0)

### Changed
- cloud.google.com/go/accessapproval: v1.7.1 → v1.7.3
- cloud.google.com/go/accesscontextmanager: v1.8.1 → v1.8.3
- cloud.google.com/go/aiplatform: v1.48.0 → v1.51.2
- cloud.google.com/go/analytics: v0.21.3 → v0.21.5
- cloud.google.com/go/apigateway: v1.6.1 → v1.6.3
- cloud.google.com/go/apigeeconnect: v1.6.1 → v1.6.3
- cloud.google.com/go/apigeeregistry: v0.7.1 → v0.8.1
- cloud.google.com/go/appengine: v1.8.1 → v1.8.3
- cloud.google.com/go/area120: v0.8.1 → v0.8.3
- cloud.google.com/go/artifactregistry: v1.14.1 → v1.14.4
- cloud.google.com/go/asset: v1.14.1 → v1.15.2
- cloud.google.com/go/assuredworkloads: v1.11.1 → v1.11.3
- cloud.google.com/go/automl: v1.13.1 → v1.13.3
- cloud.google.com/go/baremetalsolution: v1.1.1 → v1.2.2
- cloud.google.com/go/batch: v1.3.1 → v1.6.1
- cloud.google.com/go/beyondcorp: v1.0.0 → v1.0.2
- cloud.google.com/go/bigquery: v1.53.0 → v1.56.0
- cloud.google.com/go/billing: v1.16.0 → v1.17.3
- cloud.google.com/go/binaryauthorization: v1.6.1 → v1.7.2
- cloud.google.com/go/certificatemanager: v1.7.1 → v1.7.3
- cloud.google.com/go/channel: v1.16.0 → v1.17.2
- cloud.google.com/go/cloudbuild: v1.13.0 → v1.14.2
- cloud.google.com/go/clouddms: v1.6.1 → v1.7.2
- cloud.google.com/go/cloudtasks: v1.12.1 → v1.12.3
- cloud.google.com/go/compute: v1.23.0 → v1.23.2
- cloud.google.com/go/contactcenterinsights: v1.10.0 → v1.11.2
- cloud.google.com/go/container: v1.24.0 → v1.26.2
- cloud.google.com/go/containeranalysis: v0.10.1 → v0.11.2
- cloud.google.com/go/datacatalog: v1.16.0 → v1.18.2
- cloud.google.com/go/dataflow: v0.9.1 → v0.9.3
- cloud.google.com/go/dataform: v0.8.1 → v0.8.3
- cloud.google.com/go/datafusion: v1.7.1 → v1.7.3
- cloud.google.com/go/datalabeling: v0.8.1 → v0.8.3
- cloud.google.com/go/dataplex: v1.9.0 → v1.10.2
- cloud.google.com/go/dataproc/v2: v2.0.1 → v2.2.2
- cloud.google.com/go/dataqna: v0.8.1 → v0.8.3
- cloud.google.com/go/datastore: v1.13.0 → v1.15.0
- cloud.google.com/go/datastream: v1.10.0 → v1.10.2
- cloud.google.com/go/deploy: v1.13.0 → v1.14.1
- cloud.google.com/go/dialogflow: v1.40.0 → v1.44.2
- cloud.google.com/go/dlp: v1.10.1 → v1.10.3
- cloud.google.com/go/documentai: v1.22.0 → v1.23.4
- cloud.google.com/go/domains: v0.9.1 → v0.9.3
- cloud.google.com/go/edgecontainer: v1.1.1 → v1.1.3
- cloud.google.com/go/essentialcontacts: v1.6.2 → v1.6.4
- cloud.google.com/go/eventarc: v1.13.0 → v1.13.2
- cloud.google.com/go/filestore: v1.7.1 → v1.7.3
- cloud.google.com/go/firestore: v1.11.0 → v1.14.0
- cloud.google.com/go/functions: v1.15.1 → v1.15.3
- cloud.google.com/go/gkebackup: v1.3.0 → v1.3.3
- cloud.google.com/go/gkeconnect: v0.8.1 → v0.8.3
- cloud.google.com/go/gkehub: v0.14.1 → v0.14.3
- cloud.google.com/go/gkemulticloud: v1.0.0 → v1.0.2
- cloud.google.com/go/gsuiteaddons: v1.6.1 → v1.6.3
- cloud.google.com/go/iam: v1.1.1 → v1.1.4
- cloud.google.com/go/iap: v1.8.1 → v1.9.2
- cloud.google.com/go/ids: v1.4.1 → v1.4.3
- cloud.google.com/go/iot: v1.7.1 → v1.7.3
- cloud.google.com/go/kms: v1.15.0 → v1.15.4
- cloud.google.com/go/language: v1.10.1 → v1.12.1
- cloud.google.com/go/lifesciences: v0.9.1 → v0.9.3
- cloud.google.com/go/logging: v1.7.0 → v1.8.1
- cloud.google.com/go/longrunning: v0.5.1 → v0.5.3
- cloud.google.com/go/managedidentities: v1.6.1 → v1.6.3
- cloud.google.com/go/maps: v1.4.0 → v1.5.1
- cloud.google.com/go/mediatranslation: v0.8.1 → v0.8.3
- cloud.google.com/go/memcache: v1.10.1 → v1.10.3
- cloud.google.com/go/metastore: v1.12.0 → v1.13.2
- cloud.google.com/go/monitoring: v1.15.1 → v1.16.2
- cloud.google.com/go/networkconnectivity: v1.12.1 → v1.14.2
- cloud.google.com/go/networkmanagement: v1.8.0 → v1.9.2
- cloud.google.com/go/networksecurity: v0.9.1 → v0.9.3
- cloud.google.com/go/notebooks: v1.9.1 → v1.11.1
- cloud.google.com/go/optimization: v1.4.1 → v1.6.1
- cloud.google.com/go/orchestration: v1.8.1 → v1.8.3
- cloud.google.com/go/orgpolicy: v1.11.1 → v1.11.3
- cloud.google.com/go/osconfig: v1.12.1 → v1.12.3
- cloud.google.com/go/oslogin: v1.10.1 → v1.12.1
- cloud.google.com/go/phishingprotection: v0.8.1 → v0.8.3
- cloud.google.com/go/policytroubleshooter: v1.8.0 → v1.10.1
- cloud.google.com/go/privatecatalog: v0.9.1 → v0.9.3
- cloud.google.com/go/recaptchaenterprise/v2: v2.7.2 → v2.8.2
- cloud.google.com/go/recommendationengine: v0.8.1 → v0.8.3
- cloud.google.com/go/recommender: v1.10.1 → v1.11.2
- cloud.google.com/go/redis: v1.13.1 → v1.13.3
- cloud.google.com/go/resourcemanager: v1.9.1 → v1.9.3
- cloud.google.com/go/resourcesettings: v1.6.1 → v1.6.3
- cloud.google.com/go/retail: v1.14.1 → v1.14.3
- cloud.google.com/go/run: v1.2.0 → v1.3.2
- cloud.google.com/go/scheduler: v1.10.1 → v1.10.3
- cloud.google.com/go/secretmanager: v1.11.1 → v1.11.3
- cloud.google.com/go/security: v1.15.1 → v1.15.3
- cloud.google.com/go/securitycenter: v1.23.0 → v1.24.1
- cloud.google.com/go/servicedirectory: v1.11.0 → v1.11.2
- cloud.google.com/go/shell: v1.7.1 → v1.7.3
- cloud.google.com/go/spanner: v1.47.0 → v1.51.0
- cloud.google.com/go/speech: v1.19.0 → v1.19.2
- cloud.google.com/go/storagetransfer: v1.10.0 → v1.10.2
- cloud.google.com/go/talent: v1.6.2 → v1.6.4
- cloud.google.com/go/texttospeech: v1.7.1 → v1.7.3
- cloud.google.com/go/tpu: v1.6.1 → v1.6.3
- cloud.google.com/go/trace: v1.10.1 → v1.10.3
- cloud.google.com/go/translate: v1.8.2 → v1.9.2
- cloud.google.com/go/video: v1.19.0 → v1.20.2
- cloud.google.com/go/videointelligence: v1.11.1 → v1.11.3
- cloud.google.com/go/vision/v2: v2.7.2 → v2.7.4
- cloud.google.com/go/vmmigration: v1.7.1 → v1.7.3
- cloud.google.com/go/vmwareengine: v1.0.0 → v1.0.2
- cloud.google.com/go/vpcaccess: v1.7.1 → v1.7.3
- cloud.google.com/go/webrisk: v1.9.1 → v1.9.3
- cloud.google.com/go/websecurityscanner: v1.6.1 → v1.6.3
- cloud.google.com/go/workflows: v1.11.1 → v1.12.2
- cloud.google.com/go: v0.110.6 → v0.110.9
- github.com/container-storage-interface/spec: [v1.8.0 → v1.9.0](https://github.com/container-storage-interface/spec/compare/v1.8.0...v1.9.0)
- github.com/coredns/corefile-migration: [v1.0.20 → v1.0.21](https://github.com/coredns/corefile-migration/compare/v1.0.20...v1.0.21)
- github.com/cpuguy83/go-md2man/v2: [v2.0.2 → v2.0.3](https://github.com/cpuguy83/go-md2man/v2/compare/v2.0.2...v2.0.3)
- github.com/cyphar/filepath-securejoin: [v0.2.3 → v0.2.4](https://github.com/cyphar/filepath-securejoin/compare/v0.2.3...v0.2.4)
- github.com/envoyproxy/go-control-plane: [9239064 → v0.11.1](https://github.com/envoyproxy/go-control-plane/compare/9239064...v0.11.1)
- github.com/envoyproxy/protoc-gen-validate: [v0.10.1 → v1.0.2](https://github.com/envoyproxy/protoc-gen-validate/compare/v0.10.1...v1.0.2)
- github.com/evanphx/json-patch/v5: [v5.6.0 → v5.7.0](https://github.com/evanphx/json-patch/v5/compare/v5.6.0...v5.7.0)
- github.com/evanphx/json-patch: [v5.6.0+incompatible → v5.7.0+incompatible](https://github.com/evanphx/json-patch/compare/v5.6.0...v5.7.0)
- github.com/fatih/color: [v1.13.0 → v1.15.0](https://github.com/fatih/color/compare/v1.13.0...v1.15.0)
- github.com/felixge/httpsnoop: [v1.0.3 → v1.0.4](https://github.com/felixge/httpsnoop/compare/v1.0.3...v1.0.4)
- github.com/fsnotify/fsnotify: [v1.6.0 → v1.7.0](https://github.com/fsnotify/fsnotify/compare/v1.6.0...v1.7.0)
- github.com/go-logr/logr: [v1.2.4 → v1.3.0](https://github.com/go-logr/logr/compare/v1.2.4...v1.3.0)
- github.com/go-openapi/jsonpointer: [v0.20.0 → v0.20.1](https://github.com/go-openapi/jsonpointer/compare/v0.20.0...v0.20.1)
- github.com/go-openapi/jsonreference: [v0.20.2 → v0.20.3](https://github.com/go-openapi/jsonreference/compare/v0.20.2...v0.20.3)
- github.com/go-openapi/swag: [v0.22.4 → v0.22.5](https://github.com/go-openapi/swag/compare/v0.22.4...v0.22.5)
- github.com/gobuffalo/flect: [v0.3.0 → v1.0.2](https://github.com/gobuffalo/flect/compare/v0.3.0...v1.0.2)
- github.com/godbus/dbus/v5: [v5.0.6 → v5.1.0](https://github.com/godbus/dbus/v5/compare/v5.0.6...v5.1.0)
- github.com/golang/glog: [v1.1.0 → v1.1.2](https://github.com/golang/glog/compare/v1.1.0...v1.1.2)
- github.com/google/cadvisor: [v0.47.3 → v0.48.1](https://github.com/google/cadvisor/compare/v0.47.3...v0.48.1)
- github.com/google/cel-go: [v0.16.0 → v0.17.7](https://github.com/google/cel-go/compare/v0.16.0...v0.17.7)
- github.com/google/go-cmp: [v0.5.9 → v0.6.0](https://github.com/google/go-cmp/compare/v0.5.9...v0.6.0)
- github.com/google/uuid: [v1.3.1 → v1.5.0](https://github.com/google/uuid/compare/v1.3.1...v1.5.0)
- github.com/googleapis/gax-go/v2: [v2.7.1 → v2.11.0](https://github.com/googleapis/gax-go/v2/compare/v2.7.1...v2.11.0)
- github.com/gorilla/websocket: [v1.4.2 → v1.5.1](https://github.com/gorilla/websocket/compare/v1.4.2...v1.5.1)
- github.com/grpc-ecosystem/grpc-gateway/v2: [v2.17.1 → v2.18.1](https://github.com/grpc-ecosystem/grpc-gateway/v2/compare/v2.17.1...v2.18.1)
- github.com/imdario/mergo: [v0.3.13 → v0.3.16](https://github.com/imdario/mergo/compare/v0.3.13...v0.3.16)
- github.com/ishidawataru/sctp: [7c296d4 → 7ff4192](https://github.com/ishidawataru/sctp/compare/7c296d4...7ff4192)
- github.com/kubernetes-csi/csi-lib-utils: [v0.14.0 → v0.17.0](https://github.com/kubernetes-csi/csi-lib-utils/compare/v0.14.0...v0.17.0)
- github.com/kubernetes-csi/csi-test/v5: [v5.0.0 → v5.2.0](https://github.com/kubernetes-csi/csi-test/v5/compare/v5.0.0...v5.2.0)
- github.com/kubernetes-csi/external-snapshotter/client/v6: [v6.2.0 → v6.3.0](https://github.com/kubernetes-csi/external-snapshotter/client/v6/compare/v6.2.0...v6.3.0)
- github.com/mattn/go-colorable: [v0.1.9 → v0.1.13](https://github.com/mattn/go-colorable/compare/v0.1.9...v0.1.13)
- github.com/mattn/go-isatty: [v0.0.14 → v0.0.17](https://github.com/mattn/go-isatty/compare/v0.0.14...v0.0.17)
- github.com/miekg/dns: [v1.1.55 → v1.1.57](https://github.com/miekg/dns/compare/v1.1.55...v1.1.57)
- github.com/moby/sys/mountinfo: [v0.6.2 → v0.7.1](https://github.com/moby/sys/mountinfo/compare/v0.6.2...v0.7.1)
- github.com/mrunalp/fileutils: [v0.5.0 → v0.5.1](https://github.com/mrunalp/fileutils/compare/v0.5.0...v0.5.1)
- github.com/onsi/ginkgo/v2: [v2.12.0 → v2.13.2](https://github.com/onsi/ginkgo/v2/compare/v2.12.0...v2.13.2)
- github.com/onsi/gomega: [v1.27.10 → v1.30.0](https://github.com/onsi/gomega/compare/v1.27.10...v1.30.0)
- github.com/opencontainers/runc: [v1.1.7 → v1.1.10](https://github.com/opencontainers/runc/compare/v1.1.7...v1.1.10)
- github.com/prometheus/client_golang: [v1.16.0 → v1.17.0](https://github.com/prometheus/client_golang/compare/v1.16.0...v1.17.0)
- github.com/prometheus/client_model: [v0.4.0 → v0.5.0](https://github.com/prometheus/client_model/compare/v0.4.0...v0.5.0)
- github.com/prometheus/common: [v0.44.0 → v0.45.0](https://github.com/prometheus/common/compare/v0.44.0...v0.45.0)
- github.com/prometheus/procfs: [v0.11.1 → v0.12.0](https://github.com/prometheus/procfs/compare/v0.11.1...v0.12.0)
- github.com/rogpeppe/go-internal: [v1.10.0 → v1.11.0](https://github.com/rogpeppe/go-internal/compare/v1.10.0...v1.11.0)
- github.com/spf13/cobra: [v1.7.0 → v1.8.0](https://github.com/spf13/cobra/compare/v1.7.0...v1.8.0)
- github.com/vmware/govmomi: [v0.30.0 → v0.30.6](https://github.com/vmware/govmomi/compare/v0.30.0...v0.30.6)
- go.etcd.io/bbolt: v1.3.7 → v1.3.8
- go.etcd.io/etcd/api/v3: v3.5.9 → v3.5.11
- go.etcd.io/etcd/client/pkg/v3: v3.5.9 → v3.5.11
- go.etcd.io/etcd/client/v2: v2.305.9 → v2.305.10
- go.etcd.io/etcd/client/v3: v3.5.9 → v3.5.11
- go.etcd.io/etcd/pkg/v3: v3.5.9 → v3.5.10
- go.etcd.io/etcd/raft/v3: v3.5.9 → v3.5.10
- go.etcd.io/etcd/server/v3: v3.5.9 → v3.5.10
- go.opentelemetry.io/contrib/instrumentation/github.com/emicklei/go-restful/otelrestful: v0.35.0 → v0.42.0
- go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc: v0.35.0 → v0.46.1
- go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp: v0.35.1 → v0.46.1
- go.opentelemetry.io/otel/exporters/otlp/internal/retry: v1.17.0 → v1.10.0
- go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc: v1.10.0 → v1.21.0
- go.opentelemetry.io/otel/exporters/otlp/otlptrace: v1.10.0 → v1.21.0
- go.opentelemetry.io/otel/metric: v0.31.0 → v1.21.0
- go.opentelemetry.io/otel/sdk: v1.10.0 → v1.21.0
- go.opentelemetry.io/otel/trace: v1.10.0 → v1.21.0
- go.opentelemetry.io/otel: v1.10.0 → v1.21.0
- go.uber.org/goleak: v1.2.1 → v1.3.0
- go.uber.org/zap: v1.25.0 → v1.26.0
- golang.org/x/crypto: v0.12.0 → v0.17.0
- golang.org/x/exp: a9213ee → 7918f67
- golang.org/x/mod: v0.12.0 → v0.14.0
- golang.org/x/net: v0.14.0 → v0.19.0
- golang.org/x/oauth2: v0.11.0 → v0.15.0
- golang.org/x/sync: v0.3.0 → v0.5.0
- golang.org/x/sys: v0.11.0 → v0.15.0
- golang.org/x/term: v0.11.0 → v0.15.0
- golang.org/x/text: v0.12.0 → v0.14.0
- golang.org/x/time: v0.3.0 → v0.5.0
- golang.org/x/tools: v0.12.0 → v0.16.1
- gomodules.xyz/jsonpatch/v2: v2.3.0 → v2.4.0
- google.golang.org/api: v0.114.0 → v0.126.0
- google.golang.org/appengine: v1.6.7 → v1.6.8
- google.golang.org/genproto/googleapis/api: b8732ec → bbf56f3
- google.golang.org/genproto/googleapis/rpc: b8732ec → bbf56f3
- google.golang.org/genproto: f966b18 → d783a09
- google.golang.org/grpc: v1.57.0 → v1.60.0
- k8s.io/api: v0.28.0 → v0.29.0
- k8s.io/apiextensions-apiserver: v0.28.0 → v0.29.0
- k8s.io/apimachinery: v0.28.0 → v0.29.0
- k8s.io/apiserver: v0.28.0 → v0.29.0
- k8s.io/cli-runtime: v0.28.0 → v0.29.0
- k8s.io/client-go: v0.28.0 → v0.29.0
- k8s.io/cloud-provider: v0.28.0 → v0.29.0
- k8s.io/cluster-bootstrap: v0.28.0 → v0.29.0
- k8s.io/code-generator: v0.28.0 → v0.29.0
- k8s.io/component-base: v0.28.0 → v0.29.0
- k8s.io/component-helpers: v0.28.0 → v0.29.0
- k8s.io/controller-manager: v0.28.0 → v0.29.0
- k8s.io/cri-api: v0.28.0 → v0.29.0
- k8s.io/csi-translation-lib: v0.28.0 → v0.29.0
- k8s.io/dynamic-resource-allocation: v0.28.0 → v0.29.0
- k8s.io/endpointslice: v0.28.0 → v0.29.0
- k8s.io/gengo: 3913671 → 9cce18d
- k8s.io/klog/v2: v2.100.1 → v2.110.1
- k8s.io/kms: v0.28.0 → v0.29.0
- k8s.io/kube-aggregator: v0.28.0 → v0.29.0
- k8s.io/kube-controller-manager: v0.28.0 → v0.29.0
- k8s.io/kube-openapi: 2695361 → 2dd684a
- k8s.io/kube-proxy: v0.28.0 → v0.29.0
- k8s.io/kube-scheduler: v0.28.0 → v0.29.0
- k8s.io/kubectl: v0.28.0 → v0.29.0
- k8s.io/kubelet: v0.28.0 → v0.29.0
- k8s.io/kubernetes: v1.28.0 → v1.29.0
- k8s.io/legacy-cloud-providers: v0.28.0 → v0.29.0
- k8s.io/metrics: v0.28.0 → v0.29.0
- k8s.io/mount-utils: v0.28.0 → v0.29.0
- k8s.io/pod-security-admission: v0.28.0 → v0.29.0
- k8s.io/sample-apiserver: v0.28.0 → v0.29.0
- k8s.io/utils: d93618c → 3b25d92
- sigs.k8s.io/apiserver-network-proxy/konnectivity-client: v0.1.4 → v0.28.3
- sigs.k8s.io/controller-runtime: v0.15.1 → v0.16.3
- sigs.k8s.io/controller-tools: v0.11.4 → v0.13.0
- sigs.k8s.io/gateway-api: v0.7.1 → v1.0.0
- sigs.k8s.io/structured-merge-diff/v4: v4.3.0 → v4.4.1
- sigs.k8s.io/yaml: v1.3.0 → v1.4.0

### Removed
- github.com/BurntSushi/toml: [v0.3.1](https://github.com/BurntSushi/toml/tree/v0.3.1)
- github.com/benbjohnson/clock: [v1.3.0](https://github.com/benbjohnson/clock/tree/v1.3.0)
- github.com/client9/misspell: [v0.3.4](https://github.com/client9/misspell/tree/v0.3.4)
- github.com/creack/pty: [v1.1.9](https://github.com/creack/pty/tree/v1.1.9)
- github.com/docker/distribution: [v2.8.2+incompatible](https://github.com/docker/distribution/tree/v2.8.2)
- github.com/ghodss/yaml: [v1.0.0](https://github.com/ghodss/yaml/tree/v1.0.0)
- github.com/hpcloud/tail: [v1.0.0](https://github.com/hpcloud/tail/tree/v1.0.0)
- github.com/kr/pty: [v1.1.1](https://github.com/kr/pty/tree/v1.1.1)
- github.com/nxadm/tail: [v1.4.8](https://github.com/nxadm/tail/tree/v1.4.8)
- github.com/onsi/ginkgo: [v1.16.4](https://github.com/onsi/ginkgo/tree/v1.16.4)
- golang.org/x/lint: d0100b6
- gopkg.in/fsnotify.v1: v1.4.7
- gopkg.in/tomb.v1: dd63297
- honnef.co/go/tools: ea95bdf
