## Running on command line

For debugging, it's possible to run the external-provisioner on command line:

```sh
csi-provisioner -kubeconfig ~/.kube/config -v 5 -csi-address /run/csi/socket
```

## Vendoring

We use [dep](https://github.com/golang/dep) for management of `vendor/`.

`vendor/k8s.io` is manually copied from `staging/` directory of work-in-progress API for CSI, namely <https://github.com/kubernetes/kubernetes/pull/54463>.
