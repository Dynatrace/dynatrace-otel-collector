# Enrich from Kubernetes

This is the e2e test for the Collector use-case:
[Enrich from Kubernetes](https://docs.dynatrace.com/docs/shortlink/otel-collector-cases-k8s-enrich).

## Requirements to run the tests

- Docker
- Kind

The tests require a running Kind k8s cluster. During the tests,
a Dynatrace distribution of the OpenTelemetry Collector is deployed
on the k8s cluster with configurations as per the Dynatrace documentation page.

Traces are generated and sent to the Collector, which then
exports to the test where the k8s attributes are asserted on the
received traces.
