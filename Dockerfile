FROM alpine
LABEL maintainers="Kubernetes Authors"
LABEL description="CSI External Provisioner"

COPY ./deploy/docker/csi-provisioner csi-provisioner
ENTRYPOINT ["/csi-provisioner"]
