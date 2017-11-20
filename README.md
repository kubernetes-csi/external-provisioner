# Kubernetes CSI - Kubernetes external provisioner that works with CSI volumes.

This is an example external provisioner for Kubernetes which provisions using CSI Volume drivers..  It's under heavy development, so at this time README.md is notes for the developers coding.  Once complete this will change to something user friendly.


# Build

```bash
make provisioner
```

# Test

### Start Kubernetes

Run a local kubernetes cluster built from latest master code

## Run Storage Provider

### Use Flex drivers

Go to drivers and run:

```bash
_output/flexadapter --drivername mydriver --driverpath ./flexadapter/examples/simple-nfs-flexdriver/nfs --endpoint unix://tmp/csi.sock --nodeid foobar
```

### Start external provisioner

```bash
_output/csi-provisioner -kubeconfig /var/run/kubernetes/admin.kubeconfig -alsologtostderr -provisioner csi-flex
```

### Create Storage class and PVC

```bash
kubectl create -f examples/sc.yaml
kubectl create -f example/pvc1.yaml
kubectl describe pv
```

### Delete PVC
```bash
kubectl delete -f example/pvc1.yaml
```



