FROM gcr.io/distroless/base:latest
LABEL maintainers="Kubernetes Authors"
LABEL description="Volume Populator Controller"
ARG binary=./bin/populator-controller

COPY ${binary} populator-controller
ENTRYPOINT ["/populator-controller"]
