FROM golang:alpine
LABEL maintainers="Kubernetes Authors"
LABEL description="CSI External Provisioner"

WORKDIR /go/src/github.com/kubernetes-csi/external-provisioner
COPY . .
RUN cd cmd/csi-provisioner && \
    go install
