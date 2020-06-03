FROM alpine
LABEL maintainers="Kubernetes Authors"
LABEL description="CSI Provisioner"
ARG binary=csi-provisioner

COPY ${binary} /csi-provisioner
ENTRYPOINT ["/csi-provisioner"]
