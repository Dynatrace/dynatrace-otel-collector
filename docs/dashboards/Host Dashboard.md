# OpenTelemetry Host Dashboard

> [!WARNING]
> The dashboards shared in this repository are in an alpha state and can change significantly.
> They are provided as-is, with no support guarantees. 
> Newer versions of these dashboards could look significantly different from earlier versions and add or remove certain metrics.

This folder contains dashboards that can be used to monitor the health of deployed OpenTelemetry collectors. The dashboards are in JSON format and can be uploaded to your Dynatrace tenant by [following the steps in the Dynatrace documentation](https://docs.dynatrace.com/docs/shortlink/dashboards-use#dashboards-upload).

![A screenshot of the host dashboard providing an overview of the system resources](img/host-dashboard_1.png)

## Prerequisites

Dynatrace accepts metrics data with Delta temporality via OTLP/HTTP.
Collector and Collector Contrib versions 0.107.0 and above as well as Dynatrace collector versions 0.12.0 and above support exporting metrics data in that format.
Earlier versions ignore the `temporality_preference` flag and would, therefore, require additional processing (cumulative to delta conversion) before ingestion.
It is possible to do this conversion in a collector, but it would make the setup more complicated, so it is initially omitted in this document.


## Collector Configuration

Add the following receiver and processor configuration to your OpenTelemetry collector configuration file to enable the collection of host metrics with the required attributes, resource detection, and cumulative to delta conversion. Make sure to also add the receivers and processors to your collector pipeline.

```
receivers:
  hostmetrics:
    collection_interval: 10s 
    scrapers:
      paging:
        metrics:
          system.paging.utilization:
            enabled: true
      cpu:
        metrics:
          system.cpu.logical.count:
            enabled: true
          system.cpu.physical.count:
            enabled: true
          system.cpu.utilization:
            enabled: true
      disk:
      filesystem:
        metrics:
          system.filesystem.utilization:
            enabled: true
      load:
      memory:
        metrics:
          system.memory.limit:
            enabled: true
      network:
      processes:
      process:
        metrics:
          process.cpu.utilization:
            enabled: true
          process.memory.utilization:
            enabled: true
      system:

processors:
  cumulativetodelta:
  resourcedetection:
    detectors: ["system"]
    system:
      resource_attributes:
        host.arch:
          enabled: true
        host.ip:
          enabled: true

service:
  pipelines:
    metrics:
      receivers: [hostmetrics]
      processors: [resourcedetection, cumulativetodelta]
```

### Adding additional attributes to the allow list

The following attributes are not included in the default allow list of resource attributes in Dynatrace:s
- `host.arch`
- `host.ip`
- `host.mac`
- `host.name`
- `os.description`
- `os.type`
- `process.command_line`
- `process.executable.name`
- `process.name`
- `mountpoint`
- `device`
- `state`

To add it, follow [this guide](https://docs.dynatrace.com/docs/shortlink/metrics-configuration#allow-list) and add the obove listed attributes (case-sensitive) to the list.
This will ensure that this resource attribute is stored as a dimension on the metrics in Dynatrace.