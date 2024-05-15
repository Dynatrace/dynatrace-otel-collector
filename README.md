# Dynatrace distribution of the OpenTelemetry Collector

The Dynatrace distribution of the [OpenTelemetry Collector] allows collecting observability data from a
variety of sources for sending to Dynatrace. It includes a set of Collector
components that have been verified to work well for common Dynatrace use cases.

[OpenTelemetry Collector]: https://github.com/open-telemetry/opentelemetry-collector

## Installation

For deployment instructions, please see [Dynatrace's documentation].

For configuration examples, please see [`config_examples`].

[Dynatrace's documentation]: https://docs.dynatrace.com/docs/extend-dynatrace/opentelemetry/collector/deployment
[`config_examples`]: ./config_examples/README.md

## Troubleshooting

For help troubleshooting issues, please see the [OpenTelemetry documentation]
and the Collector's [troubleshooting guide].

[OpenTelemetry documentation]: https://opentelemetry.io/docs/collector/troubleshooting/
[troubleshooting guide]: https://github.com/open-telemetry/opentelemetry-collector/blob/main/docs/troubleshooting.md

## OpenTelemetry Sampling Considerations

### Mixed-Mode Sampling

[OpenTelemetry tail-based sampling](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/tailsamplingprocessor) and [Dynatrace OneAgent Adaptive Traffic Management](https://docs.dynatrace.com/docs/observe-and-explore/purepath-distributed-traces/adaptive-traffic-management-saas) use different approaches to sampling.
If a distributed trace, which may span multiple applications and services, only partially utilizes either method, it is likely to result in inconsistent results and incomplete distributed traces.
Each distributed trace should be sampled by only one of either method to ensure each sampled trace is captured in its entirety.

### Trace-Derived Service Metrics

Dynatrace trace-derived metrics are calculated from trace data after it is ingested to Dynatrace.
If OpenTelemetry traces are sampled, the trace-derived metrics are calculated only from the sampled subset of trace data.
This means that some trace-derived metrics may be biased or incorrect.
For example, a probabilistic sampler which saves 5% of traffic will result in a throughput metric that shows 5% of the actual throughput.
If you use OpenTelemetry tail-based sampling to also capture 100% of slow or error traces, your service metrics will not only show incorrect throughput, but will also incorrectly bias error rates and response times.

To mitigate this, if you wish to sample OpenTelemetry traces, you should calculate service metrics before sampling and use those metrics rather than the trace-derived metrics calculated by Dynatrace.
If you are using the collector for sampling, trace-derived metrics should be calculated in the collector before applying sampling, or by the SDK.
An example for deriving those metrics using the _Span Metrics Connector_ in the collector can be found at [config_examples/spanmetrics.yaml](config_examples/spanmetrics.yaml).

## Components

### Receivers

* [filelogreceiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/filelogreceiver)
* [fluentforwardreceiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/fluentforwardreceiver)
* [hostmetricsreceiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/hostmetricsreceiver)
* [jaegereceiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/jaegerreceiver)
* [otlpreceiver](https://github.com/open-telemetry/opentelemetry-collector/tree/main/receiver/otlpreceiver)
* [prometheusreceiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/prometheusreceiver)
* [syslogreceiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/syslogreceiver)

### Processors

* [attributesprocessor](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/attributesprocessor)
* [batchprocessor](https://github.com/open-telemetry/opentelemetry-collector/tree/main/processor/batchprocessor)
* [cumulativetodeltaprocessor](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/cumulativetodeltaprocessor)
* [filterprocessor](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/filterprocessor)
* [k8sattributesprocessor](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/k8sattributesprocessor)
* [memorylimiterprocessor](https://github.com/open-telemetry/opentelemetry-collector/tree/main/processor/memorylimiterprocessor)
* [probabilisticsamplerprocessor](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/probabilisticsamplerprocessor)
* [resourcedetectionprocessor](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/resourcedetectionprocessor)
* [resourceprocessor](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/resourceprocessor)
* [tailsamplingprocessor](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/tailsamplingprocessor)
* [transformprocessor](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/transformprocessor)

### Exporters

* [debugexporter](https://github.com/open-telemetry/opentelemetry-collector/tree/main/exporter/debugexporter)
* [otlpexporter](https://github.com/open-telemetry/opentelemetry-collector/tree/main/exporter/otlpexporter)
* [otlphttpexporter](https://github.com/open-telemetry/opentelemetry-collector/tree/main/exporter/otlphttpexporter)

### Extensions

* [healthcheckextension](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/extension/healthcheckextension)
* [zpagesextension](https://github.com/open-telemetry/opentelemetry-collector/tree/main/extension/zpagesextension)

### Connectors

* [forwardconnector](https://github.com/open-telemetry/opentelemetry-collector/tree/main/connector/forwardconnector)
* [spanmetricsconnector](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/connector/spanmetricsconnector)

## Support

This distribution is supported by the Dynatrace Support team, as described on the Dynatrace [support page].
For issues reported via GitHub, support contracts and SLAs do not apply.
Please reach out via our official support channels for full coverage.

Each minor version is supported for three months.
Fixes are provided either as a patch release for the most recent minor version, or in a new minor version release.

This distribution depends on components provided upstream by the OpenTelemetry community.
We plan to release a new version of the distribution with updated upstream components at least on a monthly cadence.
If the OpenTelemetry community decides to make a breaking change, it will be pulled into this distribution
as we upgrade to newer versions of these upstream components.
For the complete list of changes, please refer to the changelogs provided in the [opentelemetry-collector releases] and [opentelemetry-collector-contrib releases] pages.
Further information on the stability guarantees provided upstream can be found in the definitions for the [OpenTelemetry Collector stability levels].

[support page]: https://support.dynatrace.com/
[opentelemetry-collector releases]: https://github.com/open-telemetry/opentelemetry-collector/releases
[opentelemetry-collector-contrib releases]: https://github.com/open-telemetry/opentelemetry-collector-contrib/releases
[OpenTelemetry Collector stability levels]: https://github.com/open-telemetry/opentelemetry-collector#stability-levels
