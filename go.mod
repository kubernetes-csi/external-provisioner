module github.com/kubernetes-csi/external-provisioner/v5

go 1.22.5

require (
	github.com/container-storage-interface/spec v1.10.0
	github.com/golang/mock v1.6.0
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/imdario/mergo v0.3.16 // indirect
	github.com/kubernetes-csi/csi-lib-utils v0.19.0
	github.com/kubernetes-csi/csi-test/v5 v5.2.0
	github.com/kubernetes-csi/external-snapshotter/client/v6 v6.3.0
	github.com/miekg/dns v1.1.62 // indirect
	github.com/prometheus/client_golang v1.19.1
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.9.0
	google.golang.org/grpc v1.66.2
	google.golang.org/protobuf v1.34.2
	k8s.io/api v0.31.0
	k8s.io/apimachinery v0.31.0
	k8s.io/apiserver v0.31.0
	k8s.io/client-go v0.31.0
	k8s.io/component-base v0.31.0
	k8s.io/component-helpers v0.31.0
	k8s.io/csi-translation-lib v0.31.0
	k8s.io/klog/v2 v2.130.1
	sigs.k8s.io/controller-runtime v0.19.0
	sigs.k8s.io/gateway-api v1.1.0
	sigs.k8s.io/sig-storage-lib-external-provisioner/v10 v10.0.1
)

require (
	github.com/onsi/ginkgo/v2 v2.20.2
	github.com/onsi/gomega v1.34.2
	k8s.io/kubernetes v1.31.0
)

require (
	github.com/NYTimes/gziphandler v1.1.1 // indirect
	github.com/antlr4-go/antlr/v4 v4.13.1 // indirect
	github.com/asaskevich/govalidator v0.0.0-20230301143203-a9d515a09cc2 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/blang/semver/v4 v4.0.0 // indirect
	github.com/cenkalti/backoff/v4 v4.3.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/coreos/go-semver v0.3.1 // indirect
	github.com/coreos/go-systemd/v22 v22.5.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/distribution/reference v0.6.0 // indirect
	github.com/emicklei/go-restful/v3 v3.12.1 // indirect
	github.com/evanphx/json-patch/v5 v5.9.0 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/fsnotify/fsnotify v1.7.0 // indirect
	github.com/fxamacker/cbor/v2 v2.7.0 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-openapi/jsonpointer v0.21.0 // indirect
	github.com/go-openapi/jsonreference v0.21.0 // indirect
	github.com/go-openapi/swag v0.23.0 // indirect
	github.com/go-task/slim-sprig/v3 v3.0.0 // indirect
	github.com/godbus/dbus/v5 v5.1.0 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/google/cel-go v0.20.1 // indirect
	github.com/google/gnostic-models v0.6.8 // indirect
	github.com/google/go-cmp v0.6.0 // indirect
	github.com/google/pprof v0.0.0-20240827171923-fa2c70bbbfe5 // indirect
	github.com/gorilla/websocket v1.5.3 // indirect
	github.com/grpc-ecosystem/go-grpc-prometheus v1.2.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.22.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/moby/spdystream v0.5.0 // indirect
	github.com/moby/sys/mountinfo v0.7.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/mxk/go-flowrate v0.0.0-20140419014527-cca7078d478f // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/runc v1.1.14 // indirect
	github.com/opencontainers/runtime-spec v1.2.0 // indirect
	github.com/opencontainers/selinux v1.11.0 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/prometheus/client_model v0.6.1 // indirect
	github.com/prometheus/common v0.55.0 // indirect
	github.com/prometheus/procfs v0.15.1 // indirect
	github.com/sirupsen/logrus v1.9.3 // indirect
	github.com/spf13/cobra v1.8.1 // indirect
	github.com/stoewer/go-strcase v1.3.0 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	go.etcd.io/etcd/api/v3 v3.5.16 // indirect
	go.etcd.io/etcd/client/pkg/v3 v3.5.16 // indirect
	go.etcd.io/etcd/client/v3 v3.5.16 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.55.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.55.0 // indirect
	go.opentelemetry.io/otel v1.30.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.30.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.30.0 // indirect
	go.opentelemetry.io/otel/metric v1.30.0 // indirect
	go.opentelemetry.io/otel/sdk v1.30.0 // indirect
	go.opentelemetry.io/otel/trace v1.30.0 // indirect
	go.opentelemetry.io/proto/otlp v1.3.1 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.27.0 // indirect
	golang.org/x/crypto v0.27.0 // indirect
	golang.org/x/exp v0.0.0-20240719175910-8a7402abbf56 // indirect
	golang.org/x/mod v0.21.0 // indirect
	golang.org/x/net v0.29.0 // indirect
	golang.org/x/oauth2 v0.23.0 // indirect
	golang.org/x/sync v0.8.0 // indirect
	golang.org/x/sys v0.25.0 // indirect
	golang.org/x/term v0.24.0 // indirect
	golang.org/x/text v0.18.0 // indirect
	golang.org/x/time v0.6.0 // indirect
	golang.org/x/tools v0.25.0 // indirect
	google.golang.org/genproto v0.0.0-20240227224415-6ceb2ff114de // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20240903143218-8af14fe29dc1 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240903143218-8af14fe29dc1 // indirect
	gopkg.in/evanphx/json-patch.v4 v4.12.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/natefinch/lumberjack.v2 v2.2.1 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/apiextensions-apiserver v0.31.0 // indirect
	k8s.io/cloud-provider v0.30.0 // indirect
	k8s.io/controller-manager v0.31.0 // indirect
	k8s.io/kms v0.31.0 // indirect
	k8s.io/kube-openapi v0.0.0-20240730131305-7a9a4e85957e // indirect
	k8s.io/kubectl v0.31.0 // indirect
	k8s.io/kubelet v0.31.0 // indirect
	k8s.io/mount-utils v0.31.0 // indirect
	k8s.io/pod-security-admission v0.31.0 // indirect
	k8s.io/utils v0.0.0-20240711033017-18e509b52bc8 // indirect
	sigs.k8s.io/apiserver-network-proxy/konnectivity-client v0.30.3 // indirect
	sigs.k8s.io/json v0.0.0-20221116044647-bc3834ca7abd // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.4.1 // indirect
	sigs.k8s.io/yaml v1.4.0 // indirect
)

replace k8s.io/api => k8s.io/api v0.31.0

replace k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.31.0

replace k8s.io/apimachinery => k8s.io/apimachinery v0.31.0

replace k8s.io/apiserver => k8s.io/apiserver v0.31.0

replace k8s.io/cli-runtime => k8s.io/cli-runtime v0.31.0

replace k8s.io/client-go => k8s.io/client-go v0.31.0

replace k8s.io/cloud-provider => k8s.io/cloud-provider v0.31.0

replace k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.31.0

replace k8s.io/code-generator => k8s.io/code-generator v0.31.0

replace k8s.io/component-base => k8s.io/component-base v0.31.0

replace k8s.io/component-helpers => k8s.io/component-helpers v0.31.0

replace k8s.io/controller-manager => k8s.io/controller-manager v0.31.0

replace k8s.io/cri-api => k8s.io/cri-api v0.31.0

replace k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.31.0

replace k8s.io/dynamic-resource-allocation => k8s.io/dynamic-resource-allocation v0.31.0

replace k8s.io/endpointslice => k8s.io/endpointslice v0.31.0

replace k8s.io/kms => k8s.io/kms v0.31.0

replace k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.31.0

replace k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.31.0

replace k8s.io/kube-proxy => k8s.io/kube-proxy v0.31.0

replace k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.31.0

replace k8s.io/kubectl => k8s.io/kubectl v0.31.0

replace k8s.io/kubelet => k8s.io/kubelet v0.31.0

replace k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.31.0

replace k8s.io/metrics => k8s.io/metrics v0.31.0

replace k8s.io/mount-utils => k8s.io/mount-utils v0.31.0

replace k8s.io/pod-security-admission => k8s.io/pod-security-admission v0.31.0

replace k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.31.0

replace k8s.io/cri-client => k8s.io/cri-client v0.31.0
