FROM alpine
LABEL maintainers="Kubernetes Authors"
LABEL description="CSI External Provisioner"

COPY csi-provisioner csi-provisioner
ENTRYPOINT ["/csi-provisioner"]
