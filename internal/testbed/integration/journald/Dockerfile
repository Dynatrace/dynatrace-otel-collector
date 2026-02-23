# Builder stage
FROM ubuntu:24.04 AS builder

RUN apt-get update && \
    apt-get install -y curl && \
    rm -rf /var/lib/apt/lists/*

# Download and install OpenTelemetry Collector
ARG OTEL_VERSION=0.146.0
RUN curl -L -o /tmp/otelcol.tar.gz \
    "https://github.com/open-telemetry/opentelemetry-collector-releases/releases/download/v${OTEL_VERSION}/otelcol-contrib_${OTEL_VERSION}_linux_amd64.tar.gz" && \
    tar -xzf /tmp/otelcol.tar.gz -C /usr/local/bin && \
    rm /tmp/otelcol.tar.gz

# Download and install Dynatrace OTEL Collector (alternative)
# ARG COLLECTOR_VERSION=0.44.0
# RUN curl -L -o /tmp/dynatrace-otel-collector.tar.gz \
#     "https://github.com/dynatrace/dynatrace-otel-collector/releases/download/v${COLLECTOR_VERSION}/dynatrace-otel-collector_${COLLECTOR_VERSION}_Linux_x86_64.tar.gz" && \
#     tar -xzf /tmp/dynatrace-otel-collector.tar.gz -C /usr/local/bin && \
#     rm /tmp/dynatrace-otel-collector.tar.gz

# Runtime stage
FROM ubuntu:24.04

# Install systemd for journalctl
RUN apt-get update && \
    apt-get install -y systemd && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

# Copy collector from builder
COPY --from=builder /usr/local/bin/otelcol-contrib /usr/local/bin/otelcol-contrib

# Verify journalctl is available
RUN journalctl --version

ENTRYPOINT ["/usr/local/bin/otelcol-contrib"]
CMD ["--config=/conf/config.yaml"]
