# Collector example configurations

Here you can find a collection of example configurations that can be used with the
Dynatrace OpenTelemetry Collector Distribution.

> [!WARNING]
> The examples in this directory are for documentation purposes only and are not considered stable. Examples
> may change at any time and without notice.

## Samples

- [Jaeger Receiver](jaeger.yaml)
- [Tail sampling](tail_sampling.yaml)

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
