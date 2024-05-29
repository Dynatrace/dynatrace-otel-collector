FROM alpine:3.20 as certs
RUN apk --update add ca-certificates

FROM scratch

USER 10001

COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --chmod=755 dynatrace-otel-collector /dynatrace-otel-collector
ENTRYPOINT ["/dynatrace-otel-collector"]
CMD ["--config", "/etc/otelcol/config.yaml"]
