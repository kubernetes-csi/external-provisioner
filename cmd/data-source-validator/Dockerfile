FROM gcr.io/distroless/base:latest
LABEL maintainers="Kubernetes Authors"
LABEL description="Data Source Validator Controller"
ARG binary=./bin/data-source-validator

COPY ${binary} data-source-validator
ENTRYPOINT ["/data-source-validator"]
