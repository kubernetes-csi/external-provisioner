FROM golang:alpine
LABEL maintainers="Kubernetes Authors"
LABEL description="CSI Provisioner"

RUN apk add --no-cache git
RUN go get github.com/kubernetes-csi/external-provisioner/cmd/csi-provisioner
