# Enrich from Kubernetes

This is the e2e test for the Collector use-case:
[Scrape data from Prometheus](https://docs.dynatrace.com/docs/shortlink/otel-collector-cases-prometheus).

## Requirements to run the tests

- Docker
- Kind

The tests require a running Kind k8s cluster. During the tests,
a Dynatrace distribution of the OpenTelemetry Collector along with a
Prometheus node exporter are deployed on the k8s cluster with
configurations as per the Dynatrace documentation page.

The Prometheus receiver scrapes the metrics which then are exported by the collector
to the test where a few metrics are asserted.
