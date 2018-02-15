FROM golang:alpine
LABEL maintainers="Kubernetes Authors"
LABEL description="CSI External Provisioner"

WORKDIR /go/src/github.com/kubernetes-csi/external-provisioner
COPY . .
RUN cd cmd/csi-provisioner && \
    CGO_ENABLED=0 GOOS=linux go install -a -ldflags '-extldflags "-static"' -o ./bin/csi-provisioner ./cmd/csi-provisioner
