# Release notes for v2.0.1

[Documentation](https://kubernetes-csi.github.io)

# Changelog since v2.0.0

## Changes by Kind

### Bug or Regression

- CSIStorageCapacity objects were potentially incomplete when at least one storage class used "immediate binding" and topology changed while the controller was already running. ([#477](https://github.com/kubernetes-csi/external-provisioner/pull/477), [@pohly](https://github.com/pohly))

## Dependencies

### Added
_Nothing has changed._

### Changed
_Nothing has changed._

### Removed
_Nothing has changed._

# Release notes for v2.0.0

[Documentation](https://kubernetes-csi.github.io)

# Changelog since v1.6.0

## Urgent Upgrade Notes 

### (No, really, you MUST read this before you upgrade)

- Add VolumeAttachment Lister to  prevent calling DeleteVolume on the CSI plugin for volumes that are still attached to a kubernetes node. New RBAC rules for listing VolumeAttachment objects are required. ([#438](https://github.com/kubernetes-csi/external-provisioner/pull/438), [@RaunakShah](https://github.com/RaunakShah))
 - Deprecated arguments  "--connection-timeout" and "--provisioner" have been removed ([#458](https://github.com/kubernetes-csi/external-provisioner/pull/458), [@msau42](https://github.com/msau42))
 - The fstype on provisioned PVs no longer defaults to "ext4". A `defaultFStype` arg is added to the provisioner. Admins can also specify this `fstype` via storage class parameter. If `fstype` is set in storage class parameter, it will be used. The sidecar arg is only checked if fstype is not set in the SC param. ([#400](https://github.com/kubernetes-csi/external-provisioner/pull/400), [@humblec](https://github.com/humblec))
 - The topology feature is GA and requires K8s api server >= 1.17 and K8s nodes >= 1.15 ([#448](https://github.com/kubernetes-csi/external-provisioner/pull/448), [@msau42](https://github.com/msau42))
 - This introduces a new flag `leader-election` which is the same as in other sidecar containers and also removes the `enable-leader-election` and `leader-election-type` flag ([#402](https://github.com/kubernetes-csi/external-provisioner/pull/402), [@Madhu-1](https://github.com/Madhu-1))
 - Use v1 version of CSINode object. This removes support for v1beta1 and makes the minimum Kubernetes apiserver version 1.17. ([#383](https://github.com/kubernetes-csi/external-provisioner/pull/383), [@bertinatto](https://github.com/bertinatto))
 
## Changes by Kind

### Feature

- Optionally publish storage capacity information (alpha feature) ([#450](https://github.com/kubernetes-csi/external-provisioner/pull/450), [@pohly](https://github.com/pohly))

### Bug or Regression

- Don't create volume if the DataSource is not VolumeSnapshot and PVC. ([#442](https://github.com/kubernetes-csi/external-provisioner/pull/442), [@xing-yang](https://github.com/xing-yang))
- Use a separate client for leader election go routine and add kube-api-qps and kube-api-burst configurable parameters for the provisioner's kubernetes client. ([#447](https://github.com/kubernetes-csi/external-provisioner/pull/447), [@RaunakShah](https://github.com/RaunakShah))

### Other (Cleanup or Flake)

- Reduced API server load by using cache of provisioned PVs. ([#460](https://github.com/kubernetes-csi/external-provisioner/pull/460), [@jsafrane](https://github.com/jsafrane))

### Uncategorized

- Build with Go 1.15 ([#464](https://github.com/kubernetes-csi/external-provisioner/pull/464), [@pohly](https://github.com/pohly))
- Publishing of images on k8s.gcr.io ([#439](https://github.com/kubernetes-csi/external-provisioner/pull/439), [@pohly](https://github.com/pohly))

## Dependencies

### Added
- bitbucket.org/bertimus9/systemstat: 0eeff89
- cloud.google.com/go/bigquery: v1.0.1
- cloud.google.com/go/datastore: v1.0.0
- cloud.google.com/go/pubsub: v1.0.1
- cloud.google.com/go/storage: v1.0.0
- dmitri.shuralyov.com/gpu/mtl: 666a987
- github.com/Azure/azure-sdk-for-go: [v43.0.0+incompatible](https://github.com/Azure/azure-sdk-for-go/tree/v43.0.0)
- github.com/Azure/go-autorest/autorest/to: [v0.2.0](https://github.com/Azure/go-autorest/autorest/to/tree/v0.2.0)
- github.com/Azure/go-autorest/autorest/validation: [v0.1.0](https://github.com/Azure/go-autorest/autorest/validation/tree/v0.1.0)
- github.com/GoogleCloudPlatform/k8s-cloud-provider: [7901bc8](https://github.com/GoogleCloudPlatform/k8s-cloud-provider/tree/7901bc8)
- github.com/JeffAshton/win_pdh: [76bb4ee](https://github.com/JeffAshton/win_pdh/tree/76bb4ee)
- github.com/MakeNowJust/heredoc: [bb23615](https://github.com/MakeNowJust/heredoc/tree/bb23615)
- github.com/Microsoft/go-winio: [fc70bd9](https://github.com/Microsoft/go-winio/tree/fc70bd9)
- github.com/Microsoft/hcsshim: [v0.8.9](https://github.com/Microsoft/hcsshim/tree/v0.8.9)
- github.com/OneOfOne/xxhash: [v1.2.2](https://github.com/OneOfOne/xxhash/tree/v1.2.2)
- github.com/agnivade/levenshtein: [v1.0.1](https://github.com/agnivade/levenshtein/tree/v1.0.1)
- github.com/ajstarks/svgo: [644b8db](https://github.com/ajstarks/svgo/tree/644b8db)
- github.com/andreyvit/diff: [c7f18ee](https://github.com/andreyvit/diff/tree/c7f18ee)
- github.com/armon/circbuf: [bbbad09](https://github.com/armon/circbuf/tree/bbbad09)
- github.com/armon/consul-api: [eb2c6b5](https://github.com/armon/consul-api/tree/eb2c6b5)
- github.com/asaskevich/govalidator: [f61b66f](https://github.com/asaskevich/govalidator/tree/f61b66f)
- github.com/auth0/go-jwt-middleware: [5493cab](https://github.com/auth0/go-jwt-middleware/tree/5493cab)
- github.com/aws/aws-sdk-go: [v1.28.2](https://github.com/aws/aws-sdk-go/tree/v1.28.2)
- github.com/bifurcation/mint: [93c51c6](https://github.com/bifurcation/mint/tree/93c51c6)
- github.com/boltdb/bolt: [v1.3.1](https://github.com/boltdb/bolt/tree/v1.3.1)
- github.com/caddyserver/caddy: [v1.0.3](https://github.com/caddyserver/caddy/tree/v1.0.3)
- github.com/cenkalti/backoff: [v2.1.1+incompatible](https://github.com/cenkalti/backoff/tree/v2.1.1)
- github.com/cespare/xxhash: [v1.1.0](https://github.com/cespare/xxhash/tree/v1.1.0)
- github.com/chai2010/gettext-go: [c6fed77](https://github.com/chai2010/gettext-go/tree/c6fed77)
- github.com/checkpoint-restore/go-criu/v4: [v4.0.2](https://github.com/checkpoint-restore/go-criu/v4/tree/v4.0.2)
- github.com/cheekybits/genny: [9127e81](https://github.com/cheekybits/genny/tree/9127e81)
- github.com/chzyer/logex: [v1.1.10](https://github.com/chzyer/logex/tree/v1.1.10)
- github.com/chzyer/readline: [2972be2](https://github.com/chzyer/readline/tree/2972be2)
- github.com/chzyer/test: [a1ea475](https://github.com/chzyer/test/tree/a1ea475)
- github.com/cilium/ebpf: [1c8d4c9](https://github.com/cilium/ebpf/tree/1c8d4c9)
- github.com/clusterhq/flocker-go: [2b8b725](https://github.com/clusterhq/flocker-go/tree/2b8b725)
- github.com/cncf/udpa/go: [269d4d4](https://github.com/cncf/udpa/go/tree/269d4d4)
- github.com/codegangsta/negroni: [v1.0.0](https://github.com/codegangsta/negroni/tree/v1.0.0)
- github.com/containerd/cgroups: [bf292b2](https://github.com/containerd/cgroups/tree/bf292b2)
- github.com/containerd/console: [v1.0.0](https://github.com/containerd/console/tree/v1.0.0)
- github.com/containerd/containerd: [v1.3.3](https://github.com/containerd/containerd/tree/v1.3.3)
- github.com/containerd/continuity: [aaeac12](https://github.com/containerd/continuity/tree/aaeac12)
- github.com/containerd/fifo: [a9fb20d](https://github.com/containerd/fifo/tree/a9fb20d)
- github.com/containerd/go-runc: [5a6d9f3](https://github.com/containerd/go-runc/tree/5a6d9f3)
- github.com/containerd/ttrpc: [v1.0.0](https://github.com/containerd/ttrpc/tree/v1.0.0)
- github.com/containerd/typeurl: [v1.0.0](https://github.com/containerd/typeurl/tree/v1.0.0)
- github.com/containernetworking/cni: [v0.8.0](https://github.com/containernetworking/cni/tree/v0.8.0)
- github.com/coredns/corefile-migration: [v1.0.10](https://github.com/coredns/corefile-migration/tree/v1.0.10)
- github.com/coreos/bbolt: [v1.3.2](https://github.com/coreos/bbolt/tree/v1.3.2)
- github.com/coreos/etcd: [v3.3.10+incompatible](https://github.com/coreos/etcd/tree/v3.3.10)
- github.com/coreos/go-systemd/v22: [v22.1.0](https://github.com/coreos/go-systemd/v22/tree/v22.1.0)
- github.com/cpuguy83/go-md2man/v2: [v2.0.0](https://github.com/cpuguy83/go-md2man/v2/tree/v2.0.0)
- github.com/cyphar/filepath-securejoin: [v0.2.2](https://github.com/cyphar/filepath-securejoin/tree/v0.2.2)
- github.com/daviddengcn/go-colortext: [511bcaf](https://github.com/daviddengcn/go-colortext/tree/511bcaf)
- github.com/dgryski/go-sip13: [e10d5fe](https://github.com/dgryski/go-sip13/tree/e10d5fe)
- github.com/dnaeon/go-vcr: [v1.0.1](https://github.com/dnaeon/go-vcr/tree/v1.0.1)
- github.com/docker/distribution: [v2.7.1+incompatible](https://github.com/docker/distribution/tree/v2.7.1)
- github.com/docker/go-connections: [v0.4.0](https://github.com/docker/go-connections/tree/v0.4.0)
- github.com/docker/go-units: [v0.4.0](https://github.com/docker/go-units/tree/v0.4.0)
- github.com/docopt/docopt-go: [ee0de3b](https://github.com/docopt/docopt-go/tree/ee0de3b)
- github.com/euank/go-kmsg-parser: [v2.0.0+incompatible](https://github.com/euank/go-kmsg-parser/tree/v2.0.0)
- github.com/exponent-io/jsonpath: [d6023ce](https://github.com/exponent-io/jsonpath/tree/d6023ce)
- github.com/fatih/camelcase: [v1.0.0](https://github.com/fatih/camelcase/tree/v1.0.0)
- github.com/flynn/go-shlex: [3f9db97](https://github.com/flynn/go-shlex/tree/3f9db97)
- github.com/fogleman/gg: [0403632](https://github.com/fogleman/gg/tree/0403632)
- github.com/globalsign/mgo: [eeefdec](https://github.com/globalsign/mgo/tree/eeefdec)
- github.com/go-acme/lego: [v2.5.0+incompatible](https://github.com/go-acme/lego/tree/v2.5.0)
- github.com/go-bindata/go-bindata: [v3.1.1+incompatible](https://github.com/go-bindata/go-bindata/tree/v3.1.1)
- github.com/go-gl/glfw/v3.3/glfw: [12ad95a](https://github.com/go-gl/glfw/v3.3/glfw/tree/12ad95a)
- github.com/go-ini/ini: [v1.9.0](https://github.com/go-ini/ini/tree/v1.9.0)
- github.com/go-logr/zapr: [v0.1.0](https://github.com/go-logr/zapr/tree/v0.1.0)
- github.com/go-openapi/analysis: [v0.19.5](https://github.com/go-openapi/analysis/tree/v0.19.5)
- github.com/go-openapi/errors: [v0.19.2](https://github.com/go-openapi/errors/tree/v0.19.2)
- github.com/go-openapi/loads: [v0.19.4](https://github.com/go-openapi/loads/tree/v0.19.4)
- github.com/go-openapi/runtime: [v0.19.4](https://github.com/go-openapi/runtime/tree/v0.19.4)
- github.com/go-openapi/strfmt: [v0.19.3](https://github.com/go-openapi/strfmt/tree/v0.19.3)
- github.com/go-openapi/validate: [v0.19.5](https://github.com/go-openapi/validate/tree/v0.19.5)
- github.com/go-ozzo/ozzo-validation: [v3.5.0+incompatible](https://github.com/go-ozzo/ozzo-validation/tree/v3.5.0)
- github.com/godbus/dbus/v5: [v5.0.3](https://github.com/godbus/dbus/v5/tree/v5.0.3)
- github.com/godbus/dbus: [ade71ed](https://github.com/godbus/dbus/tree/ade71ed)
- github.com/golang/freetype: [e2365df](https://github.com/golang/freetype/tree/e2365df)
- github.com/golangplus/bytes: [45c989f](https://github.com/golangplus/bytes/tree/45c989f)
- github.com/golangplus/fmt: [2a5d6d7](https://github.com/golangplus/fmt/tree/2a5d6d7)
- github.com/golangplus/testing: [af21d9c](https://github.com/golangplus/testing/tree/af21d9c)
- github.com/google/cadvisor: [v0.37.0](https://github.com/google/cadvisor/tree/v0.37.0)
- github.com/google/renameio: [v0.1.0](https://github.com/google/renameio/tree/v0.1.0)
- github.com/gopherjs/gopherjs: [0766667](https://github.com/gopherjs/gopherjs/tree/0766667)
- github.com/gorilla/context: [v1.1.1](https://github.com/gorilla/context/tree/v1.1.1)
- github.com/gorilla/mux: [v1.7.3](https://github.com/gorilla/mux/tree/v1.7.3)
- github.com/hashicorp/go-syslog: [v1.0.0](https://github.com/hashicorp/go-syslog/tree/v1.0.0)
- github.com/hashicorp/hcl: [v1.0.0](https://github.com/hashicorp/hcl/tree/v1.0.0)
- github.com/heketi/heketi: [c2e2a4a](https://github.com/heketi/heketi/tree/c2e2a4a)
- github.com/heketi/tests: [f3775cb](https://github.com/heketi/tests/tree/f3775cb)
- github.com/ianlancetaylor/demangle: [5e5cf60](https://github.com/ianlancetaylor/demangle/tree/5e5cf60)
- github.com/ishidawataru/sctp: [7c296d4](https://github.com/ishidawataru/sctp/tree/7c296d4)
- github.com/jimstudt/http-authentication: [3eca13d](https://github.com/jimstudt/http-authentication/tree/3eca13d)
- github.com/jmespath/go-jmespath: [c2b33e8](https://github.com/jmespath/go-jmespath/tree/c2b33e8)
- github.com/jtolds/gls: [v4.20.0+incompatible](https://github.com/jtolds/gls/tree/v4.20.0)
- github.com/jung-kurt/gofpdf: [24315ac](https://github.com/jung-kurt/gofpdf/tree/24315ac)
- github.com/karrick/godirwalk: [v1.7.5](https://github.com/karrick/godirwalk/tree/v1.7.5)
- github.com/klauspost/cpuid: [v1.2.0](https://github.com/klauspost/cpuid/tree/v1.2.0)
- github.com/kubernetes-csi/csi-test/v3: [v3.1.1](https://github.com/kubernetes-csi/csi-test/v3/tree/v3.1.1)
- github.com/kubernetes-csi/external-snapshotter/v2: [v2.2.0-rc2](https://github.com/kubernetes-csi/external-snapshotter/v2/tree/v2.2.0-rc2)
- github.com/kylelemons/godebug: [d65d576](https://github.com/kylelemons/godebug/tree/d65d576)
- github.com/libopenstorage/openstorage: [v1.0.0](https://github.com/libopenstorage/openstorage/tree/v1.0.0)
- github.com/liggitt/tabwriter: [89fcab3](https://github.com/liggitt/tabwriter/tree/89fcab3)
- github.com/lithammer/dedent: [v1.1.0](https://github.com/lithammer/dedent/tree/v1.1.0)
- github.com/lpabon/godbc: [v0.1.1](https://github.com/lpabon/godbc/tree/v0.1.1)
- github.com/lucas-clemente/aes12: [cd47fb3](https://github.com/lucas-clemente/aes12/tree/cd47fb3)
- github.com/lucas-clemente/quic-clients: [v0.1.0](https://github.com/lucas-clemente/quic-clients/tree/v0.1.0)
- github.com/lucas-clemente/quic-go-certificates: [d2f8652](https://github.com/lucas-clemente/quic-go-certificates/tree/d2f8652)
- github.com/lucas-clemente/quic-go: [v0.10.2](https://github.com/lucas-clemente/quic-go/tree/v0.10.2)
- github.com/magiconair/properties: [v1.8.1](https://github.com/magiconair/properties/tree/v1.8.1)
- github.com/marten-seemann/qtls: [v0.2.3](https://github.com/marten-seemann/qtls/tree/v0.2.3)
- github.com/mholt/certmagic: [6a42ef9](https://github.com/mholt/certmagic/tree/6a42ef9)
- github.com/mindprince/gonvml: [9ebdce4](https://github.com/mindprince/gonvml/tree/9ebdce4)
- github.com/mistifyio/go-zfs: [f784269](https://github.com/mistifyio/go-zfs/tree/f784269)
- github.com/mitchellh/go-homedir: [v1.1.0](https://github.com/mitchellh/go-homedir/tree/v1.1.0)
- github.com/mitchellh/go-wordwrap: [v1.0.0](https://github.com/mitchellh/go-wordwrap/tree/v1.0.0)
- github.com/mitchellh/mapstructure: [v1.1.2](https://github.com/mitchellh/mapstructure/tree/v1.1.2)
- github.com/moby/ipvs: [v1.0.1](https://github.com/moby/ipvs/tree/v1.0.1)
- github.com/moby/sys/mountinfo: [v0.1.3](https://github.com/moby/sys/mountinfo/tree/v0.1.3)
- github.com/moby/term: [672ec06](https://github.com/moby/term/tree/672ec06)
- github.com/mohae/deepcopy: [491d360](https://github.com/mohae/deepcopy/tree/491d360)
- github.com/morikuni/aec: [v1.0.0](https://github.com/morikuni/aec/tree/v1.0.0)
- github.com/mrunalp/fileutils: [abd8a0e](https://github.com/mrunalp/fileutils/tree/abd8a0e)
- github.com/mvdan/xurls: [v1.1.0](https://github.com/mvdan/xurls/tree/v1.1.0)
- github.com/naoina/go-stringutil: [v0.1.0](https://github.com/naoina/go-stringutil/tree/v0.1.0)
- github.com/naoina/toml: [v0.1.1](https://github.com/naoina/toml/tree/v0.1.1)
- github.com/nxadm/tail: [v1.4.4](https://github.com/nxadm/tail/tree/v1.4.4)
- github.com/oklog/ulid: [v1.3.1](https://github.com/oklog/ulid/tree/v1.3.1)
- github.com/opencontainers/go-digest: [v1.0.0-rc1](https://github.com/opencontainers/go-digest/tree/v1.0.0-rc1)
- github.com/opencontainers/image-spec: [v1.0.1](https://github.com/opencontainers/image-spec/tree/v1.0.1)
- github.com/opencontainers/runc: [819fcc6](https://github.com/opencontainers/runc/tree/819fcc6)
- github.com/opencontainers/runtime-spec: [237cc4f](https://github.com/opencontainers/runtime-spec/tree/237cc4f)
- github.com/opencontainers/selinux: [v1.5.2](https://github.com/opencontainers/selinux/tree/v1.5.2)
- github.com/pborman/uuid: [v1.2.0](https://github.com/pborman/uuid/tree/v1.2.0)
- github.com/pelletier/go-toml: [v1.2.0](https://github.com/pelletier/go-toml/tree/v1.2.0)
- github.com/prometheus/tsdb: [v0.7.1](https://github.com/prometheus/tsdb/tree/v0.7.1)
- github.com/quobyte/api: [v0.1.2](https://github.com/quobyte/api/tree/v0.1.2)
- github.com/robertkrimen/otto: [c382bd3](https://github.com/robertkrimen/otto/tree/c382bd3)
- github.com/robfig/cron: [v1.1.0](https://github.com/robfig/cron/tree/v1.1.0)
- github.com/rogpeppe/go-internal: [v1.3.0](https://github.com/rogpeppe/go-internal/tree/v1.3.0)
- github.com/rubiojr/go-vhd: [02e2102](https://github.com/rubiojr/go-vhd/tree/02e2102)
- github.com/russross/blackfriday/v2: [v2.0.1](https://github.com/russross/blackfriday/v2/tree/v2.0.1)
- github.com/russross/blackfriday: [v1.5.2](https://github.com/russross/blackfriday/tree/v1.5.2)
- github.com/satori/go.uuid: [v1.2.0](https://github.com/satori/go.uuid/tree/v1.2.0)
- github.com/seccomp/libseccomp-golang: [v0.9.1](https://github.com/seccomp/libseccomp-golang/tree/v0.9.1)
- github.com/sergi/go-diff: [v1.0.0](https://github.com/sergi/go-diff/tree/v1.0.0)
- github.com/shurcooL/sanitized_anchor_name: [v1.0.0](https://github.com/shurcooL/sanitized_anchor_name/tree/v1.0.0)
- github.com/smartystreets/assertions: [b2de0cb](https://github.com/smartystreets/assertions/tree/b2de0cb)
- github.com/smartystreets/goconvey: [v1.6.4](https://github.com/smartystreets/goconvey/tree/v1.6.4)
- github.com/spaolacci/murmur3: [f09979e](https://github.com/spaolacci/murmur3/tree/f09979e)
- github.com/spf13/cast: [v1.3.0](https://github.com/spf13/cast/tree/v1.3.0)
- github.com/spf13/jwalterweatherman: [v1.1.0](https://github.com/spf13/jwalterweatherman/tree/v1.1.0)
- github.com/spf13/viper: [v1.4.0](https://github.com/spf13/viper/tree/v1.4.0)
- github.com/storageos/go-api: [343b3ef](https://github.com/storageos/go-api/tree/343b3ef)
- github.com/syndtr/gocapability: [d983527](https://github.com/syndtr/gocapability/tree/d983527)
- github.com/thecodeteam/goscaleio: [v0.1.0](https://github.com/thecodeteam/goscaleio/tree/v0.1.0)
- github.com/tidwall/pretty: [v1.0.0](https://github.com/tidwall/pretty/tree/v1.0.0)
- github.com/ugorji/go: [v1.1.4](https://github.com/ugorji/go/tree/v1.1.4)
- github.com/urfave/negroni: [v1.0.0](https://github.com/urfave/negroni/tree/v1.0.0)
- github.com/vektah/gqlparser: [v1.1.2](https://github.com/vektah/gqlparser/tree/v1.1.2)
- github.com/vishvananda/netlink: [v1.1.0](https://github.com/vishvananda/netlink/tree/v1.1.0)
- github.com/vishvananda/netns: [52d707b](https://github.com/vishvananda/netns/tree/52d707b)
- github.com/vmware/govmomi: [v0.20.3](https://github.com/vmware/govmomi/tree/v0.20.3)
- github.com/xlab/handysort: [fb3537e](https://github.com/xlab/handysort/tree/fb3537e)
- github.com/xordataexchange/crypt: [b2862e3](https://github.com/xordataexchange/crypt/tree/b2862e3)
- go.mongodb.org/mongo-driver: v1.1.2
- gomodules.xyz/jsonpatch/v2: v2.0.1
- gonum.org/v1/plot: e2840ee
- google.golang.org/protobuf: v1.24.0
- gopkg.in/errgo.v2: v2.1.0
- gopkg.in/gcfg.v1: v1.2.0
- gopkg.in/mcuadros/go-syslog.v2: v2.2.1
- gopkg.in/sourcemap.v1: v1.0.5
- gopkg.in/warnings.v0: v0.1.1
- gotest.tools/v3: v3.0.2
- k8s.io/apiextensions-apiserver: v0.19.0-rc.2
- k8s.io/cli-runtime: v0.19.0-rc.2
- k8s.io/cluster-bootstrap: v0.19.0-rc.2
- k8s.io/cri-api: v0.19.0-rc.2
- k8s.io/heapster: v1.2.0-beta.1
- k8s.io/klog/v2: v2.2.0
- k8s.io/kube-aggregator: v0.19.0-rc.2
- k8s.io/kube-controller-manager: v0.19.0-rc.2
- k8s.io/kube-proxy: v0.19.0-rc.2
- k8s.io/kube-scheduler: v0.19.0-rc.2
- k8s.io/kubectl: v0.19.0-rc.2
- k8s.io/kubelet: v0.19.0-rc.2
- k8s.io/legacy-cloud-providers: v0.19.0-rc.2
- k8s.io/metrics: v0.19.0-rc.2
- k8s.io/sample-apiserver: v0.19.0-rc.2
- k8s.io/system-validators: v1.1.2
- rsc.io/binaryregexp: v0.2.0
- rsc.io/pdf: v0.1.1
- rsc.io/quote/v3: v3.1.0
- rsc.io/sampler: v1.3.0
- sigs.k8s.io/apiserver-network-proxy/konnectivity-client: v0.0.9
- sigs.k8s.io/controller-runtime: v0.6.2
- sigs.k8s.io/kustomize: v2.0.3+incompatible
- sigs.k8s.io/sig-storage-lib-external-provisioner/v6: v6.1.0-rc1
- sigs.k8s.io/structured-merge-diff/v3: 43c19bb
- vbom.ml/util: db5cfe1

### Changed
- cloud.google.com/go: v0.38.0 → v0.51.0
- github.com/Azure/go-autorest/autorest/adal: [v0.5.0 → v0.8.2](https://github.com/Azure/go-autorest/autorest/adal/compare/v0.5.0...v0.8.2)
- github.com/Azure/go-autorest/autorest/date: [v0.1.0 → v0.2.0](https://github.com/Azure/go-autorest/autorest/date/compare/v0.1.0...v0.2.0)
- github.com/Azure/go-autorest/autorest/mocks: [v0.2.0 → v0.3.0](https://github.com/Azure/go-autorest/autorest/mocks/compare/v0.2.0...v0.3.0)
- github.com/Azure/go-autorest/autorest: [v0.9.0 → v0.9.6](https://github.com/Azure/go-autorest/autorest/compare/v0.9.0...v0.9.6)
- github.com/container-storage-interface/spec: [v1.1.0 → v1.2.0](https://github.com/container-storage-interface/spec/compare/v1.1.0...v1.2.0)
- github.com/coreos/pkg: [97fdf19 → 399ea9e](https://github.com/coreos/pkg/compare/97fdf19...399ea9e)
- github.com/docker/docker: [be7ac8b → aa6a989](https://github.com/docker/docker/compare/be7ac8b...aa6a989)
- github.com/elazarl/goproxy: [c4fc265 → 947c36d](https://github.com/elazarl/goproxy/compare/c4fc265...947c36d)
- github.com/envoyproxy/go-control-plane: [5f8ba28 → v0.9.4](https://github.com/envoyproxy/go-control-plane/compare/5f8ba28...v0.9.4)
- github.com/fsnotify/fsnotify: [v1.4.7 → v1.4.9](https://github.com/fsnotify/fsnotify/compare/v1.4.7...v1.4.9)
- github.com/go-logr/logr: [v0.1.0 → v0.2.0](https://github.com/go-logr/logr/compare/v0.1.0...v0.2.0)
- github.com/gogo/protobuf: [65acae2 → v1.3.1](https://github.com/gogo/protobuf/compare/65acae2...v1.3.1)
- github.com/golang/groupcache: [5b532d6 → 215e871](https://github.com/golang/groupcache/compare/5b532d6...215e871)
- github.com/golang/mock: [v1.2.0 → v1.4.3](https://github.com/golang/mock/compare/v1.2.0...v1.4.3)
- github.com/golang/protobuf: [v1.3.2 → v1.4.2](https://github.com/golang/protobuf/compare/v1.3.2...v1.4.2)
- github.com/google/gofuzz: [v1.0.0 → v1.1.0](https://github.com/google/gofuzz/compare/v1.0.0...v1.1.0)
- github.com/google/pprof: [3ea8567 → d4f498a](https://github.com/google/pprof/compare/3ea8567...d4f498a)
- github.com/googleapis/gax-go/v2: [v2.0.4 → v2.0.5](https://github.com/googleapis/gax-go/v2/compare/v2.0.4...v2.0.5)
- github.com/googleapis/gnostic: [v0.2.0 → v0.4.1](https://github.com/googleapis/gnostic/compare/v0.2.0...v0.4.1)
- github.com/hashicorp/golang-lru: [v0.5.1 → v0.5.4](https://github.com/hashicorp/golang-lru/compare/v0.5.1...v0.5.4)
- github.com/imdario/mergo: [v0.3.7 → v0.3.9](https://github.com/imdario/mergo/compare/v0.3.7...v0.3.9)
- github.com/json-iterator/go: [v1.1.9 → v1.1.10](https://github.com/json-iterator/go/compare/v1.1.9...v1.1.10)
- github.com/jstemmer/go-junit-report: [af01ea7 → v0.9.1](https://github.com/jstemmer/go-junit-report/compare/af01ea7...v0.9.1)
- github.com/konsorten/go-windows-terminal-sequences: [v1.0.1 → v1.0.3](https://github.com/konsorten/go-windows-terminal-sequences/compare/v1.0.1...v1.0.3)
- github.com/kr/pretty: [v0.1.0 → v0.2.0](https://github.com/kr/pretty/compare/v0.1.0...v0.2.0)
- github.com/matttproud/golang_protobuf_extensions: [v1.0.1 → c182aff](https://github.com/matttproud/golang_protobuf_extensions/compare/v1.0.1...c182aff)
- github.com/miekg/dns: [v1.1.27 → v1.1.29](https://github.com/miekg/dns/compare/v1.1.27...v1.1.29)
- github.com/onsi/ginkgo: [v1.12.0 → v1.12.1](https://github.com/onsi/ginkgo/compare/v1.12.0...v1.12.1)
- github.com/onsi/gomega: [v1.9.0 → v1.10.1](https://github.com/onsi/gomega/compare/v1.9.0...v1.10.1)
- github.com/pkg/errors: [v0.8.1 → v0.9.1](https://github.com/pkg/errors/compare/v0.8.1...v0.9.1)
- github.com/prometheus/client_golang: [v1.4.1 → v1.7.1](https://github.com/prometheus/client_golang/compare/v1.4.1...v1.7.1)
- github.com/prometheus/common: [v0.9.1 → v0.10.0](https://github.com/prometheus/common/compare/v0.9.1...v0.10.0)
- github.com/prometheus/procfs: [v0.0.8 → v0.1.3](https://github.com/prometheus/procfs/compare/v0.0.8...v0.1.3)
- github.com/sirupsen/logrus: [v1.4.2 → v1.6.0](https://github.com/sirupsen/logrus/compare/v1.4.2...v1.6.0)
- github.com/spf13/cobra: [v0.0.3 → v1.0.0](https://github.com/spf13/cobra/compare/v0.0.3...v1.0.0)
- github.com/tmc/grpc-websocket-proxy: [89b8d40 → 0ad062e](https://github.com/tmc/grpc-websocket-proxy/compare/89b8d40...0ad062e)
- github.com/urfave/cli: [v1.20.0 → v1.22.1](https://github.com/urfave/cli/compare/v1.20.0...v1.22.1)
- go.etcd.io/bbolt: v1.3.3 → v1.3.5
- go.etcd.io/etcd: 3cf2f69 → 18dfb9c
- go.opencensus.io: v0.21.0 → v0.22.2
- go.uber.org/atomic: v1.3.2 → v1.4.0
- golang.org/x/crypto: 87dc89f → bac4c82
- golang.org/x/exp: 4b39c73 → da58074
- golang.org/x/image: 0694c2d → cff245a
- golang.org/x/lint: d0100b6 → fdd1cda
- golang.org/x/mobile: d3739f8 → d2bd2a2
- golang.org/x/net: c0dbc17 → 59133d7
- golang.org/x/sys: e047566 → ed371f2
- golang.org/x/text: v0.3.2 → v0.3.3
- golang.org/x/tools: 49a3e74 → c00d67e
- gonum.org/v1/gonum: 3d26580 → v0.6.2
- google.golang.org/api: v0.4.0 → v0.15.1
- google.golang.org/appengine: v1.5.0 → v1.6.5
- google.golang.org/genproto: 5c49e3e → cb27e3a
- google.golang.org/grpc: v1.26.0 → v1.29.1
- gopkg.in/yaml.v2: v2.2.8 → v2.3.0
- honnef.co/go/tools: ea95bdf → v0.0.1-2019.2.3
- k8s.io/api: v0.17.3 → v0.19.0-rc.2
- k8s.io/apimachinery: v0.17.3 → v0.19.0-rc.2
- k8s.io/apiserver: v0.17.0 → v0.19.0-rc.2
- k8s.io/client-go: v0.17.0 → v0.19.0-rc.2
- k8s.io/cloud-provider: v0.17.0 → v0.19.0-rc.2
- k8s.io/code-generator: v0.17.1-beta.0 → v0.19.0-rc.2
- k8s.io/component-base: v0.17.0 → v0.19.0-rc.2
- k8s.io/csi-translation-lib: v0.17.0 → v0.19.0-rc.2
- k8s.io/gengo: 26a6646 → 8167cfd
- k8s.io/kube-openapi: 30be4d1 → 656914f
- k8s.io/kubernetes: v1.14.0 → v1.19.0-rc.2
- k8s.io/utils: 8619460 → 0bdb4ca
- sigs.k8s.io/yaml: v1.1.0 → v1.2.0

### Removed
- github.com/kubernetes-csi/external-snapshotter: [bba3584](https://github.com/kubernetes-csi/external-snapshotter/tree/bba3584)
- sigs.k8s.io/sig-storage-lib-external-provisioner/v5: v5.0.0
- sigs.k8s.io/sig-storage-lib-external-provisioner: v4.1.0+incompatible
- sigs.k8s.io/structured-merge-diff: b1b620d
