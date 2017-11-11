# Copyright 2015 bradley childs, All rights reserved.
#

FROM centos:7
MAINTAINER bradley childs, bchilds@gmail.com
# AWS CLI build deps
RUN yum update -y

# Provisioner build deps

# Copy the FLEX shell script to the provisioner container.
RUN mkdir -p  /opt/csi-provisioner/
COPY ./examples/flex-debug.sh /opt/csi-provisioner/

# install the go kube piece of provisioner

COPY ./_output/csi-provisioner /opt/csi-provisioner/


ENTRYPOINT ["/opt/csi-flex/flex-provision"]

