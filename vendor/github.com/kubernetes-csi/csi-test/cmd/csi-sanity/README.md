# Sanity Test Command Line Program
This is the command line program that tests a CSI driver using the [`sanity`](https://github.com/kubernetes-csi/csi-test/tree/master/pkg/sanity) package test suite.

Example:

```
$ csi-sanity --csi.endpoint=<your csi driver endpoint>
```

If you want to specify a mount point:

```
$ csi-sanity --csi.endpoint=<your csi driver endpoint> --csi.mountpoint=/mnt
```

For verbose type:

```
$ csi-sanity --ginkgo.v --csi.endpoint=<your csi driver endpoint>
```

### Help
The full Ginkgo and golang unit test parameters are available. Type

```
$ csi-sanity -h
```

to get more information

### Download

Please see the [Releases](https://github.com/kubernetes-csi/csi-test/releases) page
to download the latest version of `csi-sanity`
