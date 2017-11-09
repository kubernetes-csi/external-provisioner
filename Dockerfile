# Copyright 2015 bradley childs, All rights reserved.
#

FROM centos:7
MAINTAINER bradley childs, bchilds@gmail.com
# AWS CLI build deps
RUN yum update -y

# Provisioner build deps

# Copy the FLEX shell script to the provisioner container.
RUN mkdir -p  /opt/csi-flex/shell
COPY ./examples/flex-debug.sh /opt/csi-flex/shell/flex

# install the go kube piece of provisioner

COPY ./_output/flex-provisioner /opt/csi-flex/flex-provisioner


ENTRYPOINT ["/opt/csi-flex/flex-provision"]

