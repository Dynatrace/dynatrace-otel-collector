# Collector example configurations

Here you can find a collection of example collector configurations.

## Samples

- [Drop](drop.yaml)
- [Effective](effective.yaml.yaml)
- [Health check](healthcheck.yaml)
- [Jaeget Receiver](jaeger.yaml)
- [Pipeline](pipeline.yaml)
- [Tail sampling](tail_sampling.yaml)

## Sending data to Dynatrace

In addition to the `debug` exporter, some samples are also configured with the `otlphttp` exporter
so you can also aend and see it in your Dynatrace environment.

Before running the samples, make sure you have the following environment variables set:

- `DT_OTLP_ENDPOINT`: The OTLP HTTP endpoint of your Dynatrace environment.
  - Follow the guide: [Export to ActiveGate](https://docs.dynatrace.com/docs/shortlink/otel-getstarted-otlpexport#export-to-activegate)
    to see how to get the correct API URL for your environment
- `API_TOKEN`: The Dynatrace API access token. Follow the guide on [Authentication](https://docs.dynatrace.com/docs/shortlink/otel-getstarted-otlpexport#authentication-export-to-activegate) to see the scopes required for ingesting OTLP data.

## Trying out the configuration examples

You can try each configuration example by simply passing it to the collector when starting up:

```shell
docker run --rm \
  -v $(pwd)/config_examples/tail_sampling.yaml:/collector.yaml \
  --name dt-otelcol ghcr.io/dynatrace/dynatrace-otel-collector/dynatrace-otel-collector:latest \
  --config collector.yaml
```
