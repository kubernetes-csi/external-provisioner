FROM alpine
LABEL maintainers="Kubernetes Authors"
LABEL description="CSI External Provisioner"

COPY ./bin/csi-provisioner csi-provisioner
ENTRYPOINT ["/csi-provisioner"]
