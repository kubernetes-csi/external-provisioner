# Release notes for v3.6.4

[Documentation](https://kubernetes-csi.github.io)



## Dependencies

### Added
_Nothing has changed._

### Changed
- github.com/golang/protobuf: [v1.5.3 → v1.5.4](https://github.com/golang/protobuf/compare/v1.5.3...v1.5.4)
- google.golang.org/protobuf: v1.31.0 → v1.33.0

### Removed
_Nothing has changed._

# Release notes for v3.6.3

[Documentation](https://kubernetes-csi.github.io)

## Changes by Kind

### Bug or Regression

- Fix: CVE-2023-48795 ([#1136](https://github.com/kubernetes-csi/external-provisioner/pull/1136), [@sunnylovestiramisu](https://github.com/sunnylovestiramisu))

## Dependencies

### Added
_Nothing has changed._

### Changed
- github.com/kubernetes-csi/csi-lib-utils: [v0.14.0 → v0.14.1](https://github.com/kubernetes-csi/csi-lib-utils/compare/v0.14.0...v0.14.1)
- golang.org/x/crypto: v0.14.0 → v0.17.0
- golang.org/x/sys: v0.13.0 → v0.15.0
- golang.org/x/term: v0.13.0 → v0.15.0
- golang.org/x/text: v0.13.0 → v0.14.0

### Removed
_Nothing has changed._

# Release notes for v3.6.2

[Documentation](https://kubernetes-csi.github.io)

# Changelog since v3.6.1

## Changes by Kind

### Bug or Regression

- Bump google.golang.org/grpc from v1.57.0 to v1.59.0 to fix CVE-2023-44487. ([#1096](https://github.com/kubernetes-csi/external-provisioner/pull/1096), [@songjiaxun](https://github.com/songjiaxun))

## Dependencies

### Added
_Nothing has changed._

### Changed
- cloud.google.com/go/firestore: v1.11.0 → v1.12.0
- cloud.google.com/go: v0.110.6 → v0.110.7
- github.com/envoyproxy/go-control-plane: [9239064 → v0.11.1](https://github.com/envoyproxy/go-control-plane/compare/9239064...v0.11.1)
- github.com/envoyproxy/protoc-gen-validate: [v0.10.1 → v1.0.2](https://github.com/envoyproxy/protoc-gen-validate/compare/v0.10.1...v1.0.2)
- github.com/golang/glog: [v1.1.0 → v1.1.2](https://github.com/golang/glog/compare/v1.1.0...v1.1.2)
- google.golang.org/genproto: f966b18 → b8732ec
- google.golang.org/grpc: v1.57.0 → v1.59.0

### Removed
_Nothing has changed._

# Release notes for v3.6.1

[Documentation](https://kubernetes-csi.github.io)

## Changes by Kind

### Bug or Regression

- CVE fixes: CVE-2023-44487, CVE-2023-39323, CVE-2023-3978 ([#1057](https://github.com/kubernetes-csi/external-provisioner/pull/1057), [@dannawang0221](https://github.com/dannawang0221))

## Dependencies

### Added
_Nothing has changed._

### Changed
- golang.org/x/crypto: v0.12.0 → v0.14.0
- golang.org/x/net: v0.14.0 → v0.17.0
- golang.org/x/sys: v0.11.0 → v0.13.0
- golang.org/x/term: v0.11.0 → v0.13.0
- golang.org/x/text: v0.12.0 → v0.13.0

### Removed
_Nothing has changed._

# Release notes for v3.6.0

[Documentation](https://kubernetes-csi.github.io)

# Changelog since v3.5.0

## Changes by Kind

### Uncategorized

- Update kubernetes dependencies to v1.28.0 (#999, @Sneha-at)

## Dependencies

### Added
- cloud.google.com/go/apigeeregistry: v0.7.1
- cloud.google.com/go/dataproc/v2: v2.0.1
- github.com/Masterminds/goutils: [v1.1.1](https://github.com/Masterminds/goutils/tree/v1.1.1)
- github.com/Masterminds/semver: [v1.5.0](https://github.com/Masterminds/semver/tree/v1.5.0)
- github.com/antlr/antlr4/runtime/Go/antlr/v4: [8188dc5](https://github.com/antlr/antlr4/runtime/Go/antlr/v4/tree/8188dc5)
- github.com/google/gnostic-models: [v0.6.8](https://github.com/google/gnostic-models/tree/v0.6.8)
- github.com/googleapis/enterprise-certificate-proxy: [v0.2.3](https://github.com/googleapis/enterprise-certificate-proxy/tree/v0.2.3)
- github.com/huandu/xstrings: [v1.4.0](https://github.com/huandu/xstrings/tree/v1.4.0)
- github.com/xhit/go-str2duration/v2: [v2.1.0](https://github.com/xhit/go-str2duration/v2/tree/v2.1.0)
- google.golang.org/genproto/googleapis/api: b8732ec
- google.golang.org/genproto/googleapis/rpc: b8732ec
- k8s.io/endpointslice: v0.28.0

### Changed
- cloud.google.com/go/accessapproval: v1.5.0 → v1.7.1
- cloud.google.com/go/accesscontextmanager: v1.4.0 → v1.8.1
- cloud.google.com/go/aiplatform: v1.27.0 → v1.48.0
- cloud.google.com/go/analytics: v0.12.0 → v0.21.3
- cloud.google.com/go/apigateway: v1.4.0 → v1.6.1
- cloud.google.com/go/apigeeconnect: v1.4.0 → v1.6.1
- cloud.google.com/go/appengine: v1.5.0 → v1.8.1
- cloud.google.com/go/area120: v0.6.0 → v0.8.1
- cloud.google.com/go/artifactregistry: v1.9.0 → v1.14.1
- cloud.google.com/go/asset: v1.10.0 → v1.14.1
- cloud.google.com/go/assuredworkloads: v1.9.0 → v1.11.1
- cloud.google.com/go/automl: v1.8.0 → v1.13.1
- cloud.google.com/go/baremetalsolution: v0.4.0 → v1.1.1
- cloud.google.com/go/batch: v0.4.0 → v1.3.1
- cloud.google.com/go/beyondcorp: v0.3.0 → v1.0.0
- cloud.google.com/go/bigquery: v1.44.0 → v1.53.0
- cloud.google.com/go/billing: v1.7.0 → v1.16.0
- cloud.google.com/go/binaryauthorization: v1.4.0 → v1.6.1
- cloud.google.com/go/certificatemanager: v1.4.0 → v1.7.1
- cloud.google.com/go/channel: v1.9.0 → v1.16.0
- cloud.google.com/go/cloudbuild: v1.4.0 → v1.13.0
- cloud.google.com/go/clouddms: v1.4.0 → v1.6.1
- cloud.google.com/go/cloudtasks: v1.8.0 → v1.12.1
- cloud.google.com/go/compute: v1.15.1 → v1.23.0
- cloud.google.com/go/contactcenterinsights: v1.4.0 → v1.10.0
- cloud.google.com/go/container: v1.7.0 → v1.24.0
- cloud.google.com/go/containeranalysis: v0.6.0 → v0.10.1
- cloud.google.com/go/datacatalog: v1.8.0 → v1.16.0
- cloud.google.com/go/dataflow: v0.7.0 → v0.9.1
- cloud.google.com/go/dataform: v0.5.0 → v0.8.1
- cloud.google.com/go/datafusion: v1.5.0 → v1.7.1
- cloud.google.com/go/datalabeling: v0.6.0 → v0.8.1
- cloud.google.com/go/dataplex: v1.4.0 → v1.9.0
- cloud.google.com/go/dataqna: v0.6.0 → v0.8.1
- cloud.google.com/go/datastore: v1.10.0 → v1.13.0
- cloud.google.com/go/datastream: v1.5.0 → v1.10.0
- cloud.google.com/go/deploy: v1.5.0 → v1.13.0
- cloud.google.com/go/dialogflow: v1.19.0 → v1.40.0
- cloud.google.com/go/dlp: v1.7.0 → v1.10.1
- cloud.google.com/go/documentai: v1.10.0 → v1.22.0
- cloud.google.com/go/domains: v0.7.0 → v0.9.1
- cloud.google.com/go/edgecontainer: v0.2.0 → v1.1.1
- cloud.google.com/go/essentialcontacts: v1.4.0 → v1.6.2
- cloud.google.com/go/eventarc: v1.8.0 → v1.13.0
- cloud.google.com/go/filestore: v1.4.0 → v1.7.1
- cloud.google.com/go/firestore: v1.9.0 → v1.11.0
- cloud.google.com/go/functions: v1.9.0 → v1.15.1
- cloud.google.com/go/gkebackup: v0.3.0 → v1.3.0
- cloud.google.com/go/gkeconnect: v0.6.0 → v0.8.1
- cloud.google.com/go/gkehub: v0.10.0 → v0.14.1
- cloud.google.com/go/gkemulticloud: v0.4.0 → v1.0.0
- cloud.google.com/go/gsuiteaddons: v1.4.0 → v1.6.1
- cloud.google.com/go/iam: v0.8.0 → v1.1.1
- cloud.google.com/go/iap: v1.5.0 → v1.8.1
- cloud.google.com/go/ids: v1.2.0 → v1.4.1
- cloud.google.com/go/iot: v1.4.0 → v1.7.1
- cloud.google.com/go/kms: v1.6.0 → v1.15.0
- cloud.google.com/go/language: v1.8.0 → v1.10.1
- cloud.google.com/go/lifesciences: v0.6.0 → v0.9.1
- cloud.google.com/go/logging: v1.6.1 → v1.7.0
- cloud.google.com/go/longrunning: v0.3.0 → v0.5.1
- cloud.google.com/go/managedidentities: v1.4.0 → v1.6.1
- cloud.google.com/go/maps: v0.1.0 → v1.4.0
- cloud.google.com/go/mediatranslation: v0.6.0 → v0.8.1
- cloud.google.com/go/memcache: v1.7.0 → v1.10.1
- cloud.google.com/go/metastore: v1.8.0 → v1.12.0
- cloud.google.com/go/monitoring: v1.8.0 → v1.15.1
- cloud.google.com/go/networkconnectivity: v1.7.0 → v1.12.1
- cloud.google.com/go/networkmanagement: v1.5.0 → v1.8.0
- cloud.google.com/go/networksecurity: v0.6.0 → v0.9.1
- cloud.google.com/go/notebooks: v1.5.0 → v1.9.1
- cloud.google.com/go/optimization: v1.2.0 → v1.4.1
- cloud.google.com/go/orchestration: v1.4.0 → v1.8.1
- cloud.google.com/go/orgpolicy: v1.5.0 → v1.11.1
- cloud.google.com/go/osconfig: v1.10.0 → v1.12.1
- cloud.google.com/go/oslogin: v1.7.0 → v1.10.1
- cloud.google.com/go/phishingprotection: v0.6.0 → v0.8.1
- cloud.google.com/go/policytroubleshooter: v1.4.0 → v1.8.0
- cloud.google.com/go/privatecatalog: v0.6.0 → v0.9.1
- cloud.google.com/go/pubsub: v1.27.1 → v1.33.0
- cloud.google.com/go/pubsublite: v1.5.0 → v1.8.1
- cloud.google.com/go/recaptchaenterprise/v2: v2.5.0 → v2.7.2
- cloud.google.com/go/recommendationengine: v0.6.0 → v0.8.1
- cloud.google.com/go/recommender: v1.8.0 → v1.10.1
- cloud.google.com/go/redis: v1.10.0 → v1.13.1
- cloud.google.com/go/resourcemanager: v1.4.0 → v1.9.1
- cloud.google.com/go/resourcesettings: v1.4.0 → v1.6.1
- cloud.google.com/go/retail: v1.11.0 → v1.14.1
- cloud.google.com/go/run: v0.3.0 → v1.2.0
- cloud.google.com/go/scheduler: v1.7.0 → v1.10.1
- cloud.google.com/go/secretmanager: v1.9.0 → v1.11.1
- cloud.google.com/go/security: v1.10.0 → v1.15.1
- cloud.google.com/go/securitycenter: v1.16.0 → v1.23.0
- cloud.google.com/go/servicedirectory: v1.7.0 → v1.11.0
- cloud.google.com/go/shell: v1.4.0 → v1.7.1
- cloud.google.com/go/spanner: v1.41.0 → v1.47.0
- cloud.google.com/go/speech: v1.9.0 → v1.19.0
- cloud.google.com/go/storagetransfer: v1.6.0 → v1.10.0
- cloud.google.com/go/talent: v1.4.0 → v1.6.2
- cloud.google.com/go/texttospeech: v1.5.0 → v1.7.1
- cloud.google.com/go/tpu: v1.4.0 → v1.6.1
- cloud.google.com/go/trace: v1.4.0 → v1.10.1
- cloud.google.com/go/translate: v1.4.0 → v1.8.2
- cloud.google.com/go/video: v1.9.0 → v1.19.0
- cloud.google.com/go/videointelligence: v1.9.0 → v1.11.1
- cloud.google.com/go/vision/v2: v2.5.0 → v2.7.2
- cloud.google.com/go/vmmigration: v1.3.0 → v1.7.1
- cloud.google.com/go/vmwareengine: v0.1.0 → v1.0.0
- cloud.google.com/go/vpcaccess: v1.5.0 → v1.7.1
- cloud.google.com/go/webrisk: v1.7.0 → v1.9.1
- cloud.google.com/go/websecurityscanner: v1.4.0 → v1.6.1
- cloud.google.com/go/workflows: v1.9.0 → v1.11.1
- cloud.google.com/go: v0.105.0 → v0.110.6
- github.com/Azure/azure-sdk-for-go: [v55.0.0+incompatible → v68.0.0+incompatible](https://github.com/Azure/azure-sdk-for-go/compare/v55.0.0...v68.0.0)
- github.com/Azure/go-autorest/autorest/adal: [v0.9.20 → v0.9.23](https://github.com/Azure/go-autorest/autorest/adal/compare/v0.9.20...v0.9.23)
- github.com/Azure/go-autorest/autorest/validation: [v0.1.0 → v0.3.1](https://github.com/Azure/go-autorest/autorest/validation/compare/v0.1.0...v0.3.1)
- github.com/Azure/go-autorest/autorest: [v0.11.27 → v0.11.29](https://github.com/Azure/go-autorest/autorest/compare/v0.11.27...v0.11.29)
- github.com/Microsoft/go-winio: [v0.4.17 → v0.6.0](https://github.com/Microsoft/go-winio/compare/v0.4.17...v0.6.0)
- github.com/alecthomas/kingpin/v2: [v2.3.1 → v2.3.2](https://github.com/alecthomas/kingpin/v2/compare/v2.3.1...v2.3.2)
- github.com/benbjohnson/clock: [v1.1.0 → v1.3.0](https://github.com/benbjohnson/clock/compare/v1.1.0...v1.3.0)
- github.com/cenkalti/backoff/v4: [v4.1.3 → v4.2.1](https://github.com/cenkalti/backoff/v4/compare/v4.1.3...v4.2.1)
- github.com/cilium/ebpf: [v0.7.0 → v0.9.1](https://github.com/cilium/ebpf/compare/v0.7.0...v0.9.1)
- github.com/cncf/xds/go: [06c439d → e9ce688](https://github.com/cncf/xds/go/compare/06c439d...e9ce688)
- github.com/containerd/cgroups: [v1.0.1 → v1.1.0](https://github.com/containerd/cgroups/compare/v1.0.1...v1.1.0)
- github.com/containerd/ttrpc: [v1.1.0 → v1.2.2](https://github.com/containerd/ttrpc/compare/v1.1.0...v1.2.2)
- github.com/coredns/caddy: [v1.1.0 → v1.1.1](https://github.com/coredns/caddy/compare/v1.1.0...v1.1.1)
- github.com/coreos/go-oidc: [v2.1.0+incompatible → v2.2.1+incompatible](https://github.com/coreos/go-oidc/compare/v2.1.0...v2.2.1)
- github.com/coreos/go-semver: [v0.3.0 → v0.3.1](https://github.com/coreos/go-semver/compare/v0.3.0...v0.3.1)
- github.com/coreos/go-systemd/v22: [v22.4.0 → v22.5.0](https://github.com/coreos/go-systemd/v22/compare/v22.4.0...v22.5.0)
- github.com/docker/distribution: [v2.8.1+incompatible → v2.8.2+incompatible](https://github.com/docker/distribution/compare/v2.8.1...v2.8.2)
- github.com/dustin/go-humanize: [v1.0.0 → v1.0.1](https://github.com/dustin/go-humanize/compare/v1.0.0...v1.0.1)
- github.com/emicklei/go-restful/v3: [v3.10.1 → v3.11.0](https://github.com/emicklei/go-restful/v3/compare/v3.10.1...v3.11.0)
- github.com/envoyproxy/go-control-plane: [v0.10.3 → 9239064](https://github.com/envoyproxy/go-control-plane/compare/v0.10.3...9239064)
- github.com/envoyproxy/protoc-gen-validate: [v0.9.1 → v0.10.1](https://github.com/envoyproxy/protoc-gen-validate/compare/v0.9.1...v0.10.1)
- github.com/fatih/color: [v1.12.0 → v1.13.0](https://github.com/fatih/color/compare/v1.12.0...v1.13.0)
- github.com/fvbommel/sortorder: [v1.0.1 → v1.1.0](https://github.com/fvbommel/sortorder/compare/v1.0.1...v1.1.0)
- github.com/go-logr/logr: [v1.2.3 → v1.2.4](https://github.com/go-logr/logr/compare/v1.2.3...v1.2.4)
- github.com/go-logr/zapr: [v1.2.3 → v1.2.4](https://github.com/go-logr/zapr/compare/v1.2.3...v1.2.4)
- github.com/go-openapi/jsonpointer: [v0.19.6 → v0.20.0](https://github.com/go-openapi/jsonpointer/compare/v0.19.6...v0.20.0)
- github.com/go-openapi/swag: [v0.22.3 → v0.22.4](https://github.com/go-openapi/swag/compare/v0.22.3...v0.22.4)
- github.com/go-task/slim-sprig: [52ccab3 → v2.20.0+incompatible](https://github.com/go-task/slim-sprig/compare/52ccab3...v2.20.0)
- github.com/gobuffalo/flect: [v0.2.3 → v0.3.0](https://github.com/gobuffalo/flect/compare/v0.2.3...v0.3.0)
- github.com/gofrs/uuid: [v4.0.0+incompatible → v4.4.0+incompatible](https://github.com/gofrs/uuid/compare/v4.0.0...v4.4.0)
- github.com/golang-jwt/jwt/v4: [v4.4.2 → v4.5.0](https://github.com/golang-jwt/jwt/v4/compare/v4.4.2...v4.5.0)
- github.com/golang/glog: [v1.0.0 → v1.1.0](https://github.com/golang/glog/compare/v1.0.0...v1.1.0)
- github.com/google/cadvisor: [v0.47.1 → v0.47.3](https://github.com/google/cadvisor/compare/v0.47.1...v0.47.3)
- github.com/google/cel-go: [v0.12.6 → v0.16.0](https://github.com/google/cel-go/compare/v0.12.6...v0.16.0)
- github.com/google/uuid: [v1.3.0 → v1.3.1](https://github.com/google/uuid/compare/v1.3.0...v1.3.1)
- github.com/googleapis/gax-go/v2: [v2.1.1 → v2.7.1](https://github.com/googleapis/gax-go/v2/compare/v2.1.1...v2.7.1)
- github.com/grpc-ecosystem/grpc-gateway/v2: [v2.7.0 → v2.17.1](https://github.com/grpc-ecosystem/grpc-gateway/v2/compare/v2.7.0...v2.17.1)
- github.com/inconshreveable/mousetrap: [v1.0.1 → v1.1.0](https://github.com/inconshreveable/mousetrap/compare/v1.0.1...v1.1.0)
- github.com/kubernetes-csi/csi-lib-utils: [v0.13.0 → v0.14.0](https://github.com/kubernetes-csi/csi-lib-utils/compare/v0.13.0...v0.14.0)
- github.com/mattn/go-colorable: [v0.1.8 → v0.1.9](https://github.com/mattn/go-colorable/compare/v0.1.8...v0.1.9)
- github.com/mattn/go-isatty: [v0.0.12 → v0.0.14](https://github.com/mattn/go-isatty/compare/v0.0.12...v0.0.14)
- github.com/miekg/dns: [v1.1.48 → v1.1.55](https://github.com/miekg/dns/compare/v1.1.48...v1.1.55)
- github.com/mitchellh/go-wordwrap: [v1.0.0 → v1.0.1](https://github.com/mitchellh/go-wordwrap/compare/v1.0.0...v1.0.1)
- github.com/onsi/ginkgo/v2: [v2.9.2 → v2.12.0](https://github.com/onsi/ginkgo/v2/compare/v2.9.2...v2.12.0)
- github.com/onsi/gomega: [v1.27.6 → v1.27.10](https://github.com/onsi/gomega/compare/v1.27.6...v1.27.10)
- github.com/opencontainers/runc: [v1.1.4 → v1.1.7](https://github.com/opencontainers/runc/compare/v1.1.4...v1.1.7)
- github.com/opencontainers/selinux: [v1.10.0 → v1.11.0](https://github.com/opencontainers/selinux/compare/v1.10.0...v1.11.0)
- github.com/prometheus/client_golang: [v1.15.0 → v1.16.0](https://github.com/prometheus/client_golang/compare/v1.15.0...v1.16.0)
- github.com/prometheus/client_model: [v0.3.0 → v0.4.0](https://github.com/prometheus/client_model/compare/v0.3.0...v0.4.0)
- github.com/prometheus/common: [v0.42.0 → v0.44.0](https://github.com/prometheus/common/compare/v0.42.0...v0.44.0)
- github.com/prometheus/procfs: [v0.9.0 → v0.11.1](https://github.com/prometheus/procfs/compare/v0.9.0...v0.11.1)
- github.com/seccomp/libseccomp-golang: [f33da4d → v0.10.0](https://github.com/seccomp/libseccomp-golang/compare/f33da4d...v0.10.0)
- github.com/spf13/cobra: [v1.6.0 → v1.7.0](https://github.com/spf13/cobra/compare/v1.6.0...v1.7.0)
- github.com/stoewer/go-strcase: [v1.2.0 → v1.3.0](https://github.com/stoewer/go-strcase/compare/v1.2.0...v1.3.0)
- github.com/stretchr/testify: [v1.8.2 → v1.8.4](https://github.com/stretchr/testify/compare/v1.8.2...v1.8.4)
- github.com/vishvananda/netns: [v0.0.2 → v0.0.4](https://github.com/vishvananda/netns/compare/v0.0.2...v0.0.4)
- github.com/xlab/treeprint: [v1.1.0 → v1.2.0](https://github.com/xlab/treeprint/compare/v1.1.0...v1.2.0)
- go.etcd.io/bbolt: v1.3.6 → v1.3.7
- go.etcd.io/etcd/api/v3: v3.5.7 → v3.5.9
- go.etcd.io/etcd/client/pkg/v3: v3.5.7 → v3.5.9
- go.etcd.io/etcd/client/v2: v2.305.7 → v2.305.9
- go.etcd.io/etcd/client/v3: v3.5.7 → v3.5.9
- go.etcd.io/etcd/pkg/v3: v3.5.7 → v3.5.9
- go.etcd.io/etcd/raft/v3: v3.5.7 → v3.5.9
- go.etcd.io/etcd/server/v3: v3.5.7 → v3.5.9
- go.opencensus.io: v0.23.0 → v0.24.0
- go.opentelemetry.io/otel/exporters/otlp/internal/retry: v1.10.0 → v1.17.0
- go.opentelemetry.io/proto/otlp: v0.19.0 → v1.0.0
- go.starlark.net: 8dd3e2e → a134d8f
- go.uber.org/atomic: v1.7.0 → v1.10.0
- go.uber.org/multierr: v1.6.0 → v1.11.0
- go.uber.org/zap: v1.24.0 → v1.25.0
- golang.org/x/crypto: v0.1.0 → v0.12.0
- golang.org/x/exp: 6cc2880 → a9213ee
- golang.org/x/lint: 738671d → d0100b6
- golang.org/x/mod: v0.9.0 → v0.12.0
- golang.org/x/net: v0.8.0 → v0.14.0
- golang.org/x/oauth2: v0.5.0 → v0.11.0
- golang.org/x/sync: v0.1.0 → v0.3.0
- golang.org/x/sys: v0.6.0 → v0.11.0
- golang.org/x/term: v0.6.0 → v0.11.0
- golang.org/x/text: v0.8.0 → v0.12.0
- golang.org/x/tools: v0.7.0 → v0.12.0
- gomodules.xyz/jsonpatch/v2: v2.2.0 → v2.3.0
- google.golang.org/api: v0.60.0 → v0.114.0
- google.golang.org/genproto: 76db087 → f966b18
- google.golang.org/grpc: v1.54.0 → v1.57.0
- google.golang.org/protobuf: v1.30.0 → v1.31.0
- gopkg.in/gcfg.v1: v1.2.0 → v1.2.3
- gopkg.in/natefinch/lumberjack.v2: v2.0.0 → v2.2.1
- gopkg.in/warnings.v0: v0.1.1 → v0.1.2
- honnef.co/go/tools: v0.0.1-2020.1.4 → ea95bdf
- k8s.io/api: v0.27.0 → v0.28.0
- k8s.io/apiextensions-apiserver: v0.27.0 → v0.28.0
- k8s.io/apimachinery: v0.27.0 → v0.28.0
- k8s.io/apiserver: v0.27.0 → v0.28.0
- k8s.io/cli-runtime: v0.27.0 → v0.28.0
- k8s.io/client-go: v0.27.0 → v0.28.0
- k8s.io/cloud-provider: v0.27.0 → v0.28.0
- k8s.io/cluster-bootstrap: v0.27.0 → v0.28.0
- k8s.io/code-generator: v0.27.0 → v0.28.0
- k8s.io/component-base: v0.27.0 → v0.28.0
- k8s.io/component-helpers: v0.27.0 → v0.28.0
- k8s.io/controller-manager: v0.27.0 → v0.28.0
- k8s.io/cri-api: v0.27.0 → v0.28.0
- k8s.io/csi-translation-lib: v0.27.0 → v0.28.0
- k8s.io/dynamic-resource-allocation: v0.27.0 → v0.28.0
- k8s.io/klog/v2: v2.90.1 → v2.100.1
- k8s.io/kms: v0.27.0 → v0.28.0
- k8s.io/kube-aggregator: v0.27.0 → v0.28.0
- k8s.io/kube-controller-manager: v0.27.0 → v0.28.0
- k8s.io/kube-openapi: 15aac26 → 2695361
- k8s.io/kube-proxy: v0.27.0 → v0.28.0
- k8s.io/kube-scheduler: v0.27.0 → v0.28.0
- k8s.io/kubectl: v0.27.0 → v0.28.0
- k8s.io/kubelet: v0.27.0 → v0.28.0
- k8s.io/kubernetes: v1.27.0 → v1.28.0
- k8s.io/legacy-cloud-providers: v0.27.0 → v0.28.0
- k8s.io/metrics: v0.27.0 → v0.28.0
- k8s.io/mount-utils: v0.27.0 → v0.28.0
- k8s.io/pod-security-admission: v0.27.0 → v0.28.0
- k8s.io/sample-apiserver: v0.27.0 → v0.28.0
- k8s.io/utils: a36077c → d93618c
- sigs.k8s.io/apiserver-network-proxy/konnectivity-client: v0.1.1 → v0.1.4
- sigs.k8s.io/controller-runtime: v0.14.6 → v0.15.1
- sigs.k8s.io/controller-tools: v0.7.0 → v0.11.4
- sigs.k8s.io/gateway-api: v0.6.2 → v0.7.1
- sigs.k8s.io/kustomize/api: v0.13.2 → 6ce0bf3
- sigs.k8s.io/kustomize/kustomize/v5: v5.0.1 → 6ce0bf3
- sigs.k8s.io/kustomize/kyaml: v0.14.1 → 6ce0bf3
- sigs.k8s.io/sig-storage-lib-external-provisioner/v9: v9.0.2 → v9.1.0-rc.0
- sigs.k8s.io/structured-merge-diff/v4: v4.2.3 → v4.3.0

### Removed
- cloud.google.com/go/dataproc: v1.8.0
- cloud.google.com/go/gaming: v1.8.0
- cloud.google.com/go/servicecontrol: v1.5.0
- cloud.google.com/go/servicemanagement: v1.5.0
- cloud.google.com/go/serviceusage: v1.4.0
- cloud.google.com/go/storage: v1.10.0
- dmitri.shuralyov.com/gpu/mtl: 666a987
- github.com/BurntSushi/xgb: [27f1227](https://github.com/BurntSushi/xgb/tree/27f1227)
- github.com/OneOfOne/xxhash: [v1.2.2](https://github.com/OneOfOne/xxhash/tree/v1.2.2)
- github.com/antlr/antlr4/runtime/Go/antlr: [v1.4.10](https://github.com/antlr/antlr4/runtime/Go/antlr/tree/v1.4.10)
- github.com/buger/jsonparser: [v1.1.1](https://github.com/buger/jsonparser/tree/v1.1.1)
- github.com/cespare/xxhash: [v1.1.0](https://github.com/cespare/xxhash/tree/v1.1.0)
- github.com/docopt/docopt-go: [ee0de3b](https://github.com/docopt/docopt-go/tree/ee0de3b)
- github.com/flowstack/go-jsonschema: [v0.1.1](https://github.com/flowstack/go-jsonschema/tree/v0.1.1)
- github.com/go-gl/glfw/v3.3/glfw: [6f7a984](https://github.com/go-gl/glfw/v3.3/glfw/tree/6f7a984)
- github.com/go-gl/glfw: [e6da0ac](https://github.com/go-gl/glfw/tree/e6da0ac)
- github.com/google/martian/v3: [v3.0.0](https://github.com/google/martian/v3/tree/v3.0.0)
- github.com/google/martian: [v2.1.0+incompatible](https://github.com/google/martian/tree/v2.1.0)
- github.com/google/renameio: [v0.1.0](https://github.com/google/renameio/tree/v0.1.0)
- github.com/hashicorp/golang-lru: [v0.5.1](https://github.com/hashicorp/golang-lru/tree/v0.5.1)
- github.com/jstemmer/go-junit-report: [v0.9.1](https://github.com/jstemmer/go-junit-report/tree/v0.9.1)
- github.com/mitchellh/mapstructure: [v1.4.1](https://github.com/mitchellh/mapstructure/tree/v1.4.1)
- github.com/spaolacci/murmur3: [f09979e](https://github.com/spaolacci/murmur3/tree/f09979e)
- github.com/xeipuuv/gojsonpointer: [4e3ac27](https://github.com/xeipuuv/gojsonpointer/tree/4e3ac27)
- github.com/xeipuuv/gojsonreference: [bd5ef7b](https://github.com/xeipuuv/gojsonreference/tree/bd5ef7b)
- github.com/xeipuuv/gojsonschema: [v1.2.0](https://github.com/xeipuuv/gojsonschema/tree/v1.2.0)
- github.com/xhit/go-str2duration: [v1.2.0](https://github.com/xhit/go-str2duration/tree/v1.2.0)
- golang.org/x/image: cff245a
- golang.org/x/mobile: d2bd2a2
- gopkg.in/errgo.v2: v2.1.0
- rsc.io/binaryregexp: v0.2.0
- rsc.io/quote/v3: v3.1.0
- rsc.io/sampler: v1.3.0
