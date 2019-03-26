FROM gcr.io/distroless/static:latest
LABEL maintainers="Kubernetes Authors"
LABEL description="CSI External Provisioner"

COPY ./bin/csi-provisioner csi-provisioner
ENTRYPOINT ["/csi-provisioner"]
