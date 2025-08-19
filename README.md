# Dynatrace distribution of the OpenTelemetry Collector

The Dynatrace distribution of the [OpenTelemetry Collector] allows collecting observability data from a
variety of sources for sending to Dynatrace. It includes a set of Collector
components that have been verified to work well for common Dynatrace use cases.

[OpenTelemetry Collector]: https://github.com/open-telemetry/opentelemetry-collector

## Installation

For deployment instructions, please see the [Collector deployment page in the Dynatrace documentation].

Configuration suggestions can be found under [Collector use cases in the Dynatrace documentation] and in the [`config_examples`] folder.

[Collector deployment page in the Dynatrace documentation]: https://docs.dynatrace.com/docs/shortlink/otel-collector-deploy
[Collector use cases in the Dynatrace documentation]: https://docs.dynatrace.com/docs/ingest-from/opentelemetry/collector/use-cases
[`config_examples`]: ./config_examples/README.md

### Container images

Container images for the Dynatrace distribution of the OpenTelemetry Collector are available in:

- [GitHub Container Registry (GHCR)](https://github.com/Dynatrace/dynatrace-otel-collector/pkgs/container/dynatrace-otel-collector%2Fdynatrace-otel-collector)
- [Amazon Elastic Container Registry (Amazon ECR)](https://gallery.ecr.aws/dynatrace/dynatrace-otel-collector)
- [Docker Hub Container Registry](https://hub.docker.com/r/dynatrace/dynatrace-otel-collector)

## Troubleshooting

For help troubleshooting issues, please see the OpenTelemetry documentation on [troubleshooting the Collector].

[troubleshooting the Collector]: https://opentelemetry.io/docs/collector/troubleshooting/

## Components

The following components are included in the distribution:

### Receivers

* [filelogreceiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/filelogreceiver)
* [fluentforwardreceiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/fluentforwardreceiver)
* [hostmetricsreceiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/hostmetricsreceiver)
* [jaegereceiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/jaegerreceiver)
* [netflowreceiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/netflowreceiver)
* [otlpreceiver](https://github.com/open-telemetry/opentelemetry-collector/tree/main/receiver/otlpreceiver)
* [prometheusreceiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/prometheusreceiver)
* [statsdreceiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/statsdreceiver)
* [syslogreceiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/syslogreceiver)
* [zipkinreceiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/zipkinreceiver)
* [k8sobjectsreceiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/k8sobjectsreceiver)
* [kubeletstatsreceiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/kubeletstatsreceiver)
* [k8sclusterreceiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/k8sclusterreceiver)

### Processors

* [attributesprocessor](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/attributesprocessor)
* [batchprocessor](https://github.com/open-telemetry/opentelemetry-collector/tree/main/processor/batchprocessor)
* [cumulativetodeltaprocessor](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/cumulativetodeltaprocessor)
* [filterprocessor](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/filterprocessor)
* [k8sattributesprocessor](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/k8sattributesprocessor)
* [memorylimiterprocessor](https://github.com/open-telemetry/opentelemetry-collector/tree/main/processor/memorylimiterprocessor)
* [probabilisticsamplerprocessor](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/probabilisticsamplerprocessor)
* [redactionprocessor](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/redactionprocessor)
* [resourcedetectionprocessor](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/resourcedetectionprocessor)
* [resourceprocessor](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/resourceprocessor)
* [tailsamplingprocessor](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/tailsamplingprocessor)
* [transformprocessor](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/transformprocessor)

### Exporters

* [debugexporter](https://github.com/open-telemetry/opentelemetry-collector/tree/main/exporter/debugexporter)
* [loadbalancingexporter](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/exporter/loadbalancingexporter)
* [otlpexporter](https://github.com/open-telemetry/opentelemetry-collector/tree/main/exporter/otlpexporter)
* [otlphttpexporter](https://github.com/open-telemetry/opentelemetry-collector/tree/main/exporter/otlphttpexporter)
* [loadbalancingexporter](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/exporter/loadbalancingexporter) [1]

[1]: Load balancing exporter [**in development**](https://github.com/open-telemetry/opentelemetry-collector/blob/main/docs/component-stability.md#development) for the metrics signal. There may be bugs or performance issues and production use is discouraged. Bugs, performance issues, and feature requests related to metrics should be reported to the upstream repository.

### Extensions

* [healthcheckextension](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/extension/healthcheckextension)
* [zpagesextension](https://github.com/open-telemetry/opentelemetry-collector/tree/main/extension/zpagesextension)

### Connectors

* [forwardconnector](https://github.com/open-telemetry/opentelemetry-collector/tree/main/connector/forwardconnector)
* [spanmetricsconnector](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/connector/spanmetricsconnector)

## Support

The x86-64 and ARM64 builds of this distribution are supported by the Dynatrace Support team, as described on the Dynatrace [support page].
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
