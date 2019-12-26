ARG BASE_IMAGE=gcr.io/distroless/static:latest
FROM $BASE_IMAGE
LABEL maintainers="Kubernetes Authors"
LABEL description="CSI External Provisioner"

COPY ./bin/csi-provisioner csi-provisioner
ENTRYPOINT ["/csi-provisioner"]
