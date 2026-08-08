# Prometheus Large-Scale — self-telemetry DIRECT to Dynatrace (no selfmon collector)

Second experimental variant for ICP-5695. This one **removes the dedicated self-monitoring
collector entirely**. The Target Allocator, tier1 scrapers and tier2 gateways push their
self-telemetry over OTLP **straight to Dynatrace**, and set every resource attribute the
self-monitoring dashboard needs themselves — via the **downward API** and **static env** — so
no `k8sattributes` enrichment step is required.

## Why this is possible

The `otelcol-prometheusScraping` dashboard references only four k8s attributes:

```
k8s.cluster.name   k8s.namespace.name   k8s.pod.name   k8s.workload.name
```

All four can be supplied by the source itself:

| Attribute | Source |
|---|---|
| `k8s.namespace.name` | downward API (`metadata.namespace`) |
| `k8s.pod.name` | downward API (`metadata.name`) |
| `k8s.pod.uid`, `k8s.node.name` | downward API (extra, not needed by the dashboard) |
| `k8s.cluster.name` | static env (deploy-time) |
| `k8s.workload.name` | static env (the Deployment/StatefulSet name, known at deploy-time) |

No API lookup ⇒ no `k8sattributes`, no `transform` workload-derivation, no sink.

## How it's wired

- **tier1 / tier2**: `service.telemetry.resource` sets the attributes above (downward API vars +
  a static `k8s.workload.name`); `service.telemetry.metrics.readers` exports OTLP (delta) to
  `${DT_ENDPOINT}` with an `Api-Token` header.
- **Target Allocator**: `OTEL_RESOURCE_ATTRIBUTES` (built from downward-API env via k8s `$(VAR)`
  substitution + static cluster/workload) sets the resource; `telemetry.metrics.otlp` exports to
  `${env:DT_ENDPOINT}` with the token from a Secret (`${env:DT_API_TOKEN}`) — never in the ConfigMap.
- **Customer path is unchanged**: tier1 → tier2 gateway, and the gateway still runs `k8sattributes`
  + `transform` for the *customer* metrics (arbitrary scraped workloads genuinely need the lookup).
  Only the *self-telemetry* path skips enrichment.

## Local kind test

There is no real Dynatrace tenant locally, so a debug **sink** collector stands in for Dynatrace
(`DT_ENDPOINT=http://sink:4318`). It does **no** enrichment — it only logs what it receives, which
is exactly the point: it shows whether the sources set the attributes correctly on their own.

```sh
NAMESPACE=otel-ta ./kind-test/setup.sh
# then read the sink logs and confirm each self-telemetry resource carries
# k8s.cluster.name / k8s.namespace.name / k8s.pod.name / k8s.workload.name
kubectl logs -n otel-ta -l app.kubernetes.io/instance=otel-sink --tail=4000 \
  | grep -oE 'k8s.workload.name: Str\([a-z-]+\)' | sort | uniq -c
./kind-test/teardown.sh
```

Prereq: the custom TA image loaded into kind (`localhost/dt-target-allocator:icp5695`), same as the
other variant.

## What to compare against the sink variant

- **Attributes present?** Confirm the 4 dashboard attributes are on every self-telemetry resource
  (TA, tier1, tier2) set purely from downward/static env.
- **What's missing vs. the enriched variant**: `k8s.cluster.uid`, `k8s.workload.uid`,
  `k8s.workload.kind`, `k8s.container.name` — the API-only attributes. The dashboard doesn't use
  them, but Dynatrace **k8s entity correlation** does, so self-telemetry won't map to k8s
  workload/cluster entities.
- **Ops trade-off**: every component now holds a Dynatrace token and opens its own egress; there is
  no single enrichment/batching funnel.

## Production notes

- Point `DT_ENDPOINT` at the real Dynatrace OTLP endpoint (e.g. `https://<env>/api/v2/otlp`) and set
  `insecure: false`. The reader appends `/v1/metrics`.
- `k8s.workload.name` is hard-coded per chart here; when the Operator manages these workloads it
  would inject the correct name.
- The `tiered-otel-sink` ServiceAccount/role in `rbac.yaml` is unused in this variant (kept for
  parity with the other manifest set); harmless.
