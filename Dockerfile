FROM alpine:3.23.4@sha256:5b10f432ef3da1b8d4c7eb6c487f2f5a8f096bc91145e68878dd4a5019afde11 AS certs
RUN apk --update add ca-certificates

FROM scratch

USER 10001

COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --chmod=755 dynatrace-otel-collector /dynatrace-otel-collector
ENTRYPOINT ["/dynatrace-otel-collector"]
CMD ["--config", "/etc/otelcol/config.yaml"]
