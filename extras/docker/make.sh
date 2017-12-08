#!/bin/sh

docker build -f Dockerfile.builder -t provisioner:builder .
docker run --privileged -v $PWD:/host provisioner:builder cp /go/bin/csi-provisioner /host/csi-provisioner
sudo chown $USER csi-provisioner
docker build -t docker.io/k8scsi/csi-provisioner .
rm -f csi-provisioner
