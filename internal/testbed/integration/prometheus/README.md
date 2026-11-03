# Scrape data from Prometheus

This is the e2e test for the Collector use-case:
[Scrape data from Prometheus](https://docs.dynatrace.com/docs/shortlink/otel-collector-cases-prometheus).

## Requirements to run the tests

- The Collector binary: run `make build` in the repository root.

No Docker or Kubernetes cluster is needed. The test runs the Collector with
`config_examples/prometheus.yaml` as a local process. The scrape targets are an in-process
HTTP server serving a Prometheus registry with known values, and the Collector's own internal
metrics. The Collector exports to an in-process OTLP sink, where the test asserts the scraped
metrics, the `cumulative_to_delta` output and the start timestamps.

```sh
cd internal/testbed/integration/prometheus
go test -v
```
