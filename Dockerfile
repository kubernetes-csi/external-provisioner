FROM gcr.io/distroless/static:latest
LABEL maintainers="Kubernetes Authors"
LABEL description="CSI External Provisioner"
ARG binary=./bin/csi-provisioner
WORKDIR /go/src/sigs.k8s.io/external-provisioner

COPY ${binary} csi-provisioner
ENTRYPOINT ["/csi-provisioner"]
