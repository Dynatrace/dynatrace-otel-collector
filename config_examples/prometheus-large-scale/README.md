# Prometheus Large-Scale

Tiered OTel Collector setup for scraping Prometheus targets at scale and shipping them to Dynatrace.
Every component sets its own resource attributes and pushes its self-telemetry over OTLP directly
to Dynatrace. No separate self-monitoring collector is needed.

The workload names match the layout the `dynatrace-operator` produces for `DTPrometheus`.

## Architecture

- **Target Allocator** (`allocator.values.yaml`): distributes scrape targets across tier 1 replicas (consistent-hashing).
- **Tier 1 — Scraper** (`tier1-scraper.values.yaml`): scrapes targets assigned by the Target Allocator, load-balances OTLP to tier 2.
- **Tier 2 — Gateway** (`tier2-gateway.values.yaml`): enriches metrics, exports to Dynatrace.
- **ScrapeConfig** (`scrapeconfig.yaml`): example Prometheus Operator `ScrapeConfig` CR consumed by the Target Allocator.
- **RBAC** (`rbac.yaml`): ServiceAccounts + roles for scraper, gateway and allocator.

| Role | K8s object | Name | Operator name shape |
|---|---|---|---|
| Target Allocator | Deployment | `dtp-prometheus-allocator` | `<cr-name>-prometheus-allocator` |
| Scraper (tier 1) | Deployment | `dtp-scraper` | `<cr-name>-scraper` |
| Gateway (tier 2) | StatefulSet | `dtp-gateway` | `<cr-name>-gateway` |

The scraper, gateway and Target Allocator must run in the same namespace: the scraper resolves
`dtp-gateway.<own namespace>`, and the Target Allocator selects the scraper pods by the Helm
release name `dtp-scraper`.

## Deploy

```sh
export NAMESPACE=<namespace>
export CLUSTER_NAME=<your-cluster>   # reported as k8s.cluster.name

kubectl create namespace "${NAMESPACE}"
kubectl create secret generic dynatrace-otelcol-credentials --namespace "${NAMESPACE}" \
  --from-literal=DT_ENDPOINT="https://<env-id>.live.dynatrace.com/api/v2/otlp" \
  --from-literal=DT_PLATFORM_TOKEN="<your-token>"
kubectl create configmap dynatrace-otelcol-cluster-info --namespace "${NAMESPACE}" \
  --from-literal=K8S_CLUSTER_NAME="${CLUSTER_NAME}"

envsubst < rbac.yaml | kubectl apply -f -
envsubst < scrapeconfig.yaml | kubectl apply -f -

helm install dtp-allocator open-telemetry/opentelemetry-target-allocator --namespace "${NAMESPACE}" \
  -f allocator.values.yaml --set targetAllocator.config.collector_namespace="${NAMESPACE}"
helm install dtp-gateway open-telemetry/opentelemetry-collector --namespace "${NAMESPACE}" \
  -f tier2-gateway.values.yaml --set global.clusterName="${CLUSTER_NAME}"
helm install dtp-scraper open-telemetry/opentelemetry-collector --namespace "${NAMESPACE}" \
  -f tier1-scraper.values.yaml --set global.clusterName="${CLUSTER_NAME}"
```

## Self-telemetry resource attributes

Each component reads its resource attributes from these env vars. Four attributes are all the `otelcol-prometheusScraping` dashboard
uses: `k8s.cluster.name`, `k8s.namespace.name`, `k8s.pod.name`, `k8s.workload.name`. `k8s.node.name` and `k8s.pod.uid` are set too but unused by it.

| Env var | Source | Value |
|---|---|---|
| `K8S_POD_NAME` | downward API | `metadata.name` |
| `K8S_NAMESPACE_NAME` | downward API | `metadata.namespace` |
| `K8S_POD_UID` | downward API | `metadata.uid` |
| `K8S_NODE_NAME` | downward API | `spec.nodeName` |
| `K8S_CLUSTER_NAME` | static | cluster name (`global.clusterName` for the collectors, ConfigMap `dynatrace-otelcol-cluster-info` for the Target Allocator) |
| `K8S_WORKLOAD_NAME` | static | the Deployment/StatefulSet name the reconciler computes |
| `DT_ENDPOINT` | secret | base OTLP endpoint, e.g. `https://<env-id>.live.dynatrace.com/api/v2/otlp` |
| `DT_PLATFORM_TOKEN` | secret | bare token, no `Bearer ` prefix (the configs add it) |

Collectors additionally need `OTEL_EXPORTER_OTLP_METRICS_TEMPORALITY_PREFERENCE=delta`: this
collector build honors that SDK env var for internal telemetry, not the reader's
`temporality_preference`, and Dynatrace rejects cumulative `otelcol_*` counters
(`UNSUPPORTED_METRIC_TYPE_MONOTONIC_CUMULATIVE_SUM`).

The Target Allocator takes its attributes through `OTEL_SERVICE_NAME` +
`OTEL_RESOURCE_ATTRIBUTES` instead of a `resource` block. `resource.WithFromEnv()` is applied last
in the TA, so those env vars win on conflict. The TA does not report `service.version` itself, so
the example sets it in `OTEL_RESOURCE_ATTRIBUTES`. Renovate bumps it together with the image tag.

## Two endpoint conventions — do not mix them up

| Component | Config field | Value |
|---|---|---|
| Target Allocator | `telemetry.metrics.readers[].periodic.exporter.otlp_http.endpoint` | **base URL** — the TA appends `/v1/metrics` itself |
| Collectors | `service.telemetry.metrics.readers[].periodic.exporter.otlp.endpoint` | **full URL** — `${env:DT_ENDPOINT}/v1/metrics` |

TA headers are a **name/value list**; collector headers are a **map**.

## Target Allocator version requirement

OTLP self-telemetry landed upstream in **v0.157.0** ([open-telemetry/opentelemetry-operator#5294](https://github.com/open-telemetry/opentelemetry-operator/pull/5294)).

The example uses the Dynatrace Target Allocator image
(`ghcr.io/dynatrace/otel-target-allocator/target-allocator`, 0.11.0 or later). Any Target
Allocator built from upstream **v0.157.0** or later works.

## Limitations

No `k8sattributes` on the self-telemetry path means no `k8s.cluster.uid`, `k8s.workload.uid`,
`k8s.workload.kind`, `k8s.container.name`. The dashboard does not use them, but Dynatrace k8s
**entity correlation** does, so self-telemetry will not map to k8s workload/cluster entities.
Each component also holds its own token and opens its own egress.

The scraped-metrics data path still runs `k8sattributes` + `transform` in the gateway, because
arbitrary scraped workloads need the Kubernetes API lookup.
