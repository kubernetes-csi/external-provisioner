FROM gcr.io/distroless/base:latest
LABEL maintainers="Kubernetes Authors"
LABEL description="Volume Populator Validation Webhook"
ARG binary=./bin/populator-validation-webhook

COPY ${binary} populator-validation-webhook
ENTRYPOINT ["/populator-validation-webhook"]
