# Phase 2 (Operator layout) — self-telemetry direct to Dynatrace, no self-monitoring collector

Deployable Helm equivalent of the workload layout the `dynatrace-operator` will produce for
`DTPrometheus`. Every component sets its own resource attributes and pushes self-telemetry over
OTLP straight to Dynatrace. There is **no self-monitoring collector** — neither an OTLP sink nor a
Prometheus scraper of the other components.

Purpose: prove the `otelcol-prometheusScraping` dashboard fills correctly under the Operator
workload layout, and pin down exactly which env vars the Operator has to inject.

## Workload layout

| Role | K8s object | Name here | Operator name shape |
|---|---|---|---|
| Target Allocator | Deployment | `dtp-prometheus-allocator` | `<cr-name>-prometheus-allocator` |
| Scraper (tier 1) | Deployment | `dtp-scraper` | `<cr-name>-scraper` |
| Gateway (tier 2) | StatefulSet | `dtp-gateway` | `<cr-name>-gateway` |

Names deliberately differ from the Phase-1 Helm names (`tiered-*`). The dashboard discovers each
role by the metric it emits, not by name, so a successful run here also proves name independence.

## The contract — what the Operator must inject

Four attributes are all the dashboard uses: `k8s.cluster.name`, `k8s.namespace.name`,
`k8s.pod.name`, `k8s.workload.name`. `k8s.node.name` and `k8s.pod.uid` are set too but unused by it.

| Env var | Source | Value |
|---|---|---|
| `K8S_POD_NAME` | downward API | `metadata.name` |
| `K8S_NAMESPACE_NAME` | downward API | `metadata.namespace` |
| `K8S_POD_UID` | downward API | `metadata.uid` |
| `K8S_NODE_NAME` | downward API | `spec.nodeName` |
| `K8S_CLUSTER_NAME` | static | `DynaKube.Status.KubernetesClusterName` |
| `K8S_WORKLOAD_NAME` | static | the Deployment/StatefulSet name the reconciler computes |
| `DT_ENDPOINT` | secret | base OTLP endpoint, e.g. `https://<env-id>.live.dynatrace.com/api/v2/otlp` |
| `DT_API_TOKEN` | secret | bare token, no `Api-Token ` prefix (the configs add it) |

Collectors additionally need `OTEL_EXPORTER_OTLP_METRICS_TEMPORALITY_PREFERENCE=delta`: this
collector build honors that SDK env var for internal telemetry, not the reader's
`temporality_preference`, and Dynatrace rejects cumulative `otelcol_*` counters
(`UNSUPPORTED_METRIC_TYPE_MONOTONIC_CUMULATIVE_SUM`).

The Target Allocator takes its attributes through `OTEL_SERVICE_NAME` +
`OTEL_RESOURCE_ATTRIBUTES` instead of a `resource` block. `resource.WithFromEnv()` is applied last
in the TA, so those env vars win on conflict.

## Two endpoint conventions — do not mix them up

| Component | Config field | Value |
|---|---|---|
| Target Allocator | `telemetry.metrics.readers[].periodic.exporter.otlp_http.endpoint` | **base URL** — the TA appends `/v1/metrics` itself |
| Collectors | `service.telemetry.metrics.readers[].periodic.exporter.otlp.endpoint` | **full URL** — `${env:DT_ENDPOINT}/v1/metrics` |

TA headers are a **name/value list**; collector headers are a **map**.

## Target Allocator version requirement

OTLP self-telemetry landed upstream in **v0.157.0** (PR #5294).

The chart pin is `opentelemetry-target-allocator` **0.158.0** with the chart's default image
(`ghcr.io/open-telemetry/opentelemetry-operator/target-allocator:0.158.0`).

## Run it

```sh
export DT_ENDPOINT="https://<env-id>.live.dynatrace.com/api/v2/otlp"
export DT_API_TOKEN="<your-api-token>"   # metrics.ingest scope, bare token, no "Api-Token " prefix
export CLUSTER_NAME="dtp-phase2-test"    # becomes k8s.cluster.name
./local-test/setup.sh
```

## What this setup gives up

No `k8sattributes` on the self-telemetry path means no `k8s.cluster.uid`, `k8s.workload.uid`,
`k8s.workload.kind`, `k8s.container.name`. The dashboard does not use them, but Dynatrace k8s
**entity correlation** does, so self-telemetry will not map to k8s workload/cluster entities.
Each component also holds its own token and opens its own egress.

The **customer** data path is unchanged: the gateway still runs `k8sattributes` + `transform`,
because arbitrary scraped workloads genuinely need the API lookup.
