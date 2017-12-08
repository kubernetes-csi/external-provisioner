# Copyright 2017 Kubernetes, All rights reserved.
#

FROM centos:7
MAINTAINER sig-storage, bchilds@redhat.com

# Provisioner build deps

# Copy the FLEX shell script to the provisioner container.
RUN mkdir -p  /opt/csi-provisioner/
COPY ./examples/flex-debug.sh /opt/csi-provisioner/

# install the go kube piece of provisioner

COPY ./_output/csi-provisioner /opt/csi-provisioner/


ENTRYPOINT ["/opt/csi-provisioner/csi-provisioner"]

