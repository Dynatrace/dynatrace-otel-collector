# Collector example configurations

Here you can find a collection of example configurations that can be used with the
Dynatrace distribution of the OpenTelemetry Collector.

> [!WARNING]
> The examples in this directory are for documentation purposes only and are not considered stable. Examples
> may change at any time and without notice.

> [!CAUTION]
> It is generally preferable to bind endpoints to localhost when all clients are local.
> As of [v0.9.0](https://github.com/Dynatrace/dynatrace-otel-collector/releases/tag/v0.9.0), that is also the default, but for convenience, our example 
> configurations use the “unspecified” address `0.0.0.0`.
> For details concerning either of these choices as endpoint configuration value, see [Safeguards against denial of service attacks](https://github.com/open-telemetry/opentelemetry-collector/blob/main/docs/security-best-practices.md#safeguards-against-denial-of-service-attacks).

## Samples

- [Jaeger Receiver](jaeger.yaml)
- [Tail sampling](tail_sampling.yaml)
- [Splitting `sum`/`count` from Histogram metrics](split_histogram.yaml)
- [Deriving request metrics from pre-sampled traces](spanmetrics.yaml)
- [StatsD Receiver](statsd.yaml)
- [Syslog Receiver](syslog.yaml)
- [Zipkin Receiver](zipkin.yaml)
- [Redaction Processor](redaction.yaml)
- [Host Metrics Receiver](host-metrics.yaml)

## Sending data to Dynatrace

In addition to the `debug` exporter, some samples are also configured with the `otlphttp` exporter
so you can also see the data in your Dynatrace environment.

Before running the samples, make sure you have the following environment variables set:

- `DT_ENDPOINT`: The OTLP HTTP endpoint of your Dynatrace environment.
  - Follow the guide: [Export to ActiveGate](https://docs.dynatrace.com/docs/shortlink/otel-getstarted-otlpexport#export-to-activegate)
    to see how to get the correct API URL for your environment
- `API_TOKEN`: The Dynatrace API access token. Follow the guide on [Authentication](https://docs.dynatrace.com/docs/shortlink/otel-getstarted-otlpexport#authentication-export-to-activegate) to see the scopes required for ingesting OTLP data.

## Trying out the configuration examples

You can try each configuration example by simply passing the file to the collector when starting up:

```shell
docker run --rm \
  --name dt-otelcol \
  --env DT_ENDPOINT=$DT_ENDPOINT \
  --env API_TOKEN=$API_TOKEN \
  -p 4317:4317 \
  -v $(pwd)/config_examples/tail_sampling.yaml:/collector.yaml \
  ghcr.io/dynatrace/dynatrace-otel-collector/dynatrace-otel-collector:latest \
  --config collector.yaml
```

Or try it out from the GitHub repo if you don't have it cloned:

```shell
docker run --rm \
  --name dt-otelcol \
  --env DT_ENDPOINT=$DT_ENDPOINT \
  --env API_TOKEN=$API_TOKEN \
  -p 4317:4317 \
  ghcr.io/dynatrace/dynatrace-otel-collector/dynatrace-otel-collector:latest \
  --config https://raw.githubusercontent.com/Dynatrace/dynatrace-otel-collector/main/config_examples/tail_sampling.yaml
```
