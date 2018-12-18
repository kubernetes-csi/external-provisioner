[![Build Status](https://travis-ci.org/kubernetes-csi/external-provisioner.svg?branch=master)](https://travis-ci.org/kubernetes-csi/external-provisioner)
# Kubernetes external provisioner that works with CSI volumes.

This is an example external provisioner for Kubernetes which provisions using CSI Volume drivers..  It's under heavy development, so at this time README.md is notes for the developers coding.  Once complete this will change to something user friendly.


# Build

```bash
make csi-provisioner
```

# Test

### Start Kubernetes

Run a local kubernetes cluster built from latest master code

## Run Storage Provider

### Use HostPath drivers

Go to drivers and run:

```bash
bin/hostpathplugin --drivername mydriver  --endpoint unix://tmp/csi.sock --nodeid foobar -v=5
```

### Start external provisioner

```bash
bin/csi-provisioner --kubeconfig /var/run/kubernetes/admin.kubeconfig --alsologtostderr
```

### Create Storage class, PVC, and Pod

```bash
kubectl create -f examples/sc.yaml
kubectl create -f example/pvc2.yaml
kubectl create -f example/pod.yaml
```

### Delete PVC
```bash
kubectl delete -f example/pvc1.yaml
```

## Community, discussion, contribution, and support

Learn how to engage with the Kubernetes community on the [community page](http://kubernetes.io/community/).

You can reach the maintainers of this project at:

- [Slack channel](https://kubernetes.slack.com/messages/sig-storage)
- [Mailing list](https://groups.google.com/forum/#!forum/kubernetes-sig-storage)

### Code of conduct

Participation in the Kubernetes community is governed by the [Kubernetes Code of Conduct](code-of-conduct.md).
