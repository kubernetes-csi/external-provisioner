module github.com/kubernetes-csi/external-provisioner

go 1.16

require (
	github.com/container-storage-interface/spec v1.5.0
	github.com/golang/mock v1.4.4
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/uuid v1.2.0 // indirect
	github.com/googleapis/gnostic v0.5.4 // indirect
	github.com/imdario/mergo v0.3.12 // indirect
	github.com/kubernetes-csi/csi-lib-utils v0.9.1
	github.com/kubernetes-csi/csi-test/v4 v4.0.2
	github.com/kubernetes-csi/external-snapshotter/client/v3 v3.0.0
	github.com/miekg/dns v1.1.40 // indirect
	github.com/prometheus/client_golang v1.9.0
	github.com/prometheus/common v0.19.0 // indirect
	github.com/prometheus/procfs v0.6.0 // indirect
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.7.0
	golang.org/x/crypto v0.0.0-20210317152858-513c2a44f670 // indirect
	golang.org/x/oauth2 v0.0.0-20210313182246-cd4f82c27b84 // indirect
	golang.org/x/term v0.0.0-20210317153231-de623e64d2a6 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/genproto v0.0.0-20210317182105-75c7a8546eb9 // indirect
	google.golang.org/grpc v1.38.0
	google.golang.org/protobuf v1.26.0
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b // indirect
	k8s.io/api v0.22.0-beta.1
	k8s.io/apimachinery v0.22.0-beta.1
	k8s.io/apiserver v0.21.0
	k8s.io/client-go v0.22.0-beta.1
	k8s.io/component-base v0.22.0-beta.1
	k8s.io/component-helpers v0.21.0
	k8s.io/csi-translation-lib v0.21.0
	k8s.io/klog/v2 v2.9.0
	k8s.io/kube-openapi v0.0.0-20210305164622-f622666832c1 // indirect
	k8s.io/utils v0.0.0-20210305010621-2afb4311ab10 // indirect
	sigs.k8s.io/controller-runtime v0.8.3
	sigs.k8s.io/sig-storage-lib-external-provisioner/v7 v7.0.1
)

replace k8s.io/api => k8s.io/api v0.21.0

replace k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.21.0

replace k8s.io/apimachinery => k8s.io/apimachinery v0.21.0

replace k8s.io/apiserver => k8s.io/apiserver v0.21.0

replace k8s.io/client-go => k8s.io/client-go v0.21.0

replace k8s.io/code-generator => k8s.io/code-generator v0.21.0

replace k8s.io/component-base => k8s.io/component-base v0.21.0

replace k8s.io/component-helpers => k8s.io/component-helpers v0.21.0

replace k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.21.0

replace github.com/kubernetes-csi/csi-lib-utils => ../csi-lib-utils
