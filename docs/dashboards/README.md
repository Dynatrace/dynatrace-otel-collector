# OpenTelemetry collector self-monitoring dashboards

> [!WARNING]
> The dashboards shared in this repository are in an alpha state and can change significantly
> They are provided as-is, with no support guarantees. 
> Newer versions of these dashboards could look significantly different to earlier versions and add or remove certain metrics.

This folder contains dashboards that can be used to monitor the health of deployed OpenTelemetry collectors. The dashboards are in json format and can be uploaded to your Dynatrace tenant by [following the steps in the Dynatrace documentation](https://docs.dynatrace.com/docs/observe-and-explore/dashboards-and-notebooks/dashboards-new/get-started/dashboards-manage#dashboards-upload).

For collectors deployed in Kubernetes, two dashboards exist:

- [collector_selfmon_kubernetes_all.json](collector_selfmon_kubernetes_all.json): Shows aggregated data for all collectors sending data.
- [collector_selfmon_kubernetes_single.json](collector_selfmon_kubernetes_single.json): Allows to drill down into a single collector based on the collectors service name and pod name.

If you are running your collectors outside of Kubernetes, or you can't add `k8s.pod.name` to your pods for any reason, you can use these dashboards:
- [collector_selfmon_instance-id_all.json](collector_selfmon_instance-id_all.json): Shows aggregated data for all collectors sending data.
- [collector_selfmon_instance-id_single.json](collector_selfmon_instance-id_single.json): Allows to drill down into a single collector based on the collectors service instance ID. 

To use the `service.instance.id` based dashboards, you only need to [allow-list `service.instance.id`](#adding-serviceinstanceid-to-the-allow-list).

The dashboards rely on metrics from the collectors' [internal telelemetry](https://opentelemetry.io/docs/collector/internal-telemetry/). See the [list of internal metrics](https://opentelemetry.io/docs/collector/internal-telemetry/#lists-of-internal-metrics) for an overview of which metrics are available.

## Prerequisites
The dashboards rely on the selfmonitoring capabilities of the OTel collector as well as certain attributes on the exported metrics data.
Required attributes are: 
- `service.name` (automatically added by the Collector)
- `service.instance.id` (automatically added by the collector, needs to be added to the Dynatrace attribute allow list, see "[Adding `service.instance.id` to the allow list](#adding-serviceinstanceid-to-the-allow-list)")
- `k8s.pod.name` (needs to be added to the telemetry data, see the [Kubernetes section](#kubernetes) below)

### Adding `service.instance.id` to the allow list
`service.name` and `k8s.pod.name` are on the Dynatrace OTLP metrics ingest allow list by default, `service.instance.id` is not. In order to add it, follow [this guide](https://docs.dynatrace.com/docs/shortlink/metrics-configuration#allow-list) and add `service.instance.id` to the list.
This will ensure that this resource attribute is stored as a dimension on the metrics in Dynatrace. 

## Architecture
Every OpenTelemetry collector has selfmonitoring capabilities, but they need to be activated.
Selfmonitoring data can be exported from the collector via the OTLP protocol.
The suggested way of exporting selfmonitoring data is to run one collector dedicated for collecting and exporting the selfmonitoring data for the other running collectors, and forwarding that data to Dynatrace.
Below, you can see a configuration example for a selfmonitoring collector.

```yaml
# receive selfmonitoring data via gRPC from OTel collector instances.
receivers:
  otlp/selfmon:
    protocols:
      grpc: 
        endpoint: 0.0.0.0:4317

processors:
  # transform cumulative values to deltas. 
  cumulativetodelta/selfmon: {}
  # (kubernetes only) retrieves kubernetes attributes for other collectors sending to this collector. See Kubernetes prerequisites below.
  k8sattributes/selfmon: {}

  # prepend 'sfm.otelcol' to all selfmon metrics - the charts in the {dashboard_name}.json file expects this prefix.
  transform/selfmon:
    error_mode: ignore
    metric_statements:
      - context: metric
        statements:
          - set(name, Concat(["sfm.otelcol", name], "."))

exporters:
  # Inject DT_ENDPOINT and DT_API_TOKEN as environment variables. This should be the environment where the selfmonitoring data will go.
  # See <https://docs.dynatrace.com/docs/shortlink/otel-getstarted-otlpexport> for instructions on which endpoint and token scope to use.
  otlphttp/selfmon:
    endpoint: "${DT_ENDPOINT}/api/v2/otlp"
    headers:
      Authorization: "Api-Token ${DT_API_TOKEN}"

  # (optional) logs how many elements were exported to the Dynatrace backend.
  debug:
    verbosity: basic

service:
  # turn on the selfmonitoring for the selfmonitoring collector itself.
  telemetry:
    # (kubernetes only) the k8sattributesprocessor does not add attributes for the selfmonitoring collector itself. This is a known limitation of the processor.
    # These environment variables need to be injected, see the Kubernetes prerequisite section below.
    resource:
      k8s.namespace.name: "${env:K8S_POD_NAMESPACE}"
      k8s.pod.name: "${env:K8S_POD_NAME}"
      k8s.node.name: "${env:K8S_NODE_NAME}"

    metrics:
      level: detailed
      # export data via OTLP OTLP/grpc
      readers:
        - periodic:
            interval: 20000
            exporter:
              otlp:
                # the endpoint of the selfmonitoring collector. In this case, it is assumed that there is a service called `selfmon-collector` that exposes port 4317.
                endpoint: selfmon-collector:4317
                protocol: grpc/protobuf
                temporality_preference: delta

  extensions: []
  pipelines:
    metrics/selfmon:
      # receive OTLP/grpc from OTel collectors
      receivers: [otlp/selfmon]
      # process selfmonitoring data for the dashboard
      processors: [cumulativetodelta/selfmon, transform/selfmon, k8sattributes/selfmon]
      # export OTLP/http to Dynatrace
      exporters: [otlphttp/selfmon, debug]
```

For all other collectors, the following snippet in the `service` section of the OTel collector config should be enough to start exporting data to the selfmonitoring collector:
```yaml
# receiver, exporter, processor definitions, etc
# ...
service:
  telemetry:
    metrics:
      level: detailed
      readers:
        - periodic:
            interval: 20000
            exporter:
              otlp:
                # location of the selfmonitoring collector
                endpoint: selfmon-collector:4317
                protocol: grpc/protobuf
                temporality_preference: delta

  # ... extensions, pipelines, etc.
```

## Dashboards
### Kubernetes

In Kubernetes, there are multiple ways of getting the `k8s.pod.name` onto the selfmonitoring data:
1. Using the [Kubernetes Attributes Processor](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/processor/k8sattributesprocessor/README.md): This processor will check where the incoming telemetry is coming from, retrieve data about the telemetry producer from the Kubernetes API, and add it to the telemetry. 
   1. The Kubernetes attributes processor needs access to the Kubernetes API. Therefore, a service account is required. [Instructions are available on the `k8sattributesprocessor` GitHub page](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/processor/k8sattributesprocessor/README.md#cluster-scoped-rbac).
   2. The Kubernetes attributes processor [will not work](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/processor/k8sattributesprocessor/README.md#as-a-sidecar) for the telemetry data about the selfmonitoring collector itself, i.e. the data being sent to the selfmonitoring collector by the selfmonitoring collector. If you desire selfmonitoring data about the selfmonitoring collector, please follow the section below about injecting environment variables.
2. Using the Kubernetes [downward API](https://kubernetes.io/docs/concepts/workloads/pods/downward-api/) to inject information into the pod, and attach that information to the exported telemetry data.
   1. Use the downward API to inject information as environment variables, e.g. in the collector deployment: 
   ```yaml
    apiVersion: apps/v1
    kind: Deployment
    metadata:
      name: selfmon-collector
    spec:
      # other properties omitted for brevity
      template:
        spec:
          containers:
            - env:
                - name: K8S_NODE_NAME
                  valueFrom:
                    fieldRef:
                      fieldPath: spec.nodeName
                - name: K8S_POD_NAME
                  valueFrom:
                    fieldRef:
                      fieldPath: metadata.name
                - name: K8S_POD_NAMESPACE
                  valueFrom:
                    fieldRef:
                      fieldPath: metadata.namespace
   ```
   2. Read the environment variables and add them to the telemetry resource attributes by specifying them in the collector config file: 
   ```yaml
    service:
      telemetry:
        resource:
          k8s.namespace.name: "${env:K8S_POD_NAMESPACE}"
          k8s.pod.name: "${env:K8S_POD_NAME}"
          k8s.node.name: "${env:K8S_NODE_NAME}"
        # ... other selfmon settings, pipelines, etc. 
   ```
   If you don't want to use the k8sattributeprocessor, you will have to add the env vars and read them back for every collector. If you use the processor, setting and reading will only be required for the selfmon collector itself. If omitted, the selfmon collector will show up as `null` in the dashboard (as the data is missing).
