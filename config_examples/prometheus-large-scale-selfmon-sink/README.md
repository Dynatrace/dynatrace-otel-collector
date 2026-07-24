# Prometheus Large-Scale — OTLP self-monitoring (no scraping selfmon)

Variant of the upstream `prometheus-large-scale` example (ICP-5695) that removes the
Prometheus-scraping self-monitoring collector. The Target Allocator, tier1 scrapers and
tier2 gateways now **push** their self-telemetry via OTLP to a dedicated, stateless
`selfmon` **sink**, which enriches it and exports to Dynatrace.

## Why

- The old `selfmon-scraper` did two jobs: an OTLP sink for collector self-telemetry AND a
  Prometheus scraper of the Target Allocator (the only component that couldn't push OTLP).
  The scrape forced it to be a **single replica** (double-scrape otherwise) → a SPOF.
- With upstream TA OTLP self-telemetry (opentelemetry-operator #5068 / issue #5047), the TA
  pushes its own metrics. Nothing scrapes it anymore.
- Every source now exports **delta** (tier1/tier2 `service.telemetry` readers
  `temporality_preference: delta`; TA `telemetry.metrics.otlp.temporality: delta`). The sink
  therefore needs **no `cumulativetodelta`** → it holds no per-series state → it is safe to
  run **HA with an HPA**. SPOF gone.
- Self-telemetry stays on its **own** collector (not merged into the customer gateway), so it
  is isolated from customer-data load and never dropped when the gateway is under pressure.

## Topology

```
                    ┌───────────────┐
  ServiceMonitors → │ Target        │  self-telemetry (OTLP, delta) ─┐
  PodMonitors     → │ Allocator     │                                │
  ScrapeConfigs   → └───────┬───────┘                                │
                            │ assigns targets                        ▼
                    ┌───────┴───────┐  self-telemetry (OTLP)   ┌──────────────┐   OTLP    ┌────────────┐
  customer targets →│ tier1 scraper │ ────────────────────────▶│  selfmon     │──────────▶│ Dynatrace  │
                    └───────┬───────┘                          │  SINK        │  (enrich) └────────────┘
                            │ load-balanced OTLP (customer)    │  (HA + HPA,  │
                    ┌───────▼───────┐  self-telemetry (OTLP)   │  stateless)  │
                    │ tier2 gateway │ ────────────────────────▶└──────────────┘
                    └───────┬───────┘
                            │ customer metrics (enriched)
                            ▼
                        Dynatrace
```

- **Customer** metrics: tier1 → tier2 gateway (enrich, cumulative→delta) → Dynatrace. Unchanged.
- **Self-telemetry**: TA + tier1 + tier2 → `selfmon` sink (enrich) → Dynatrace.

## Files

| File | Role | Changes vs. upstream example |
|------|------|------------------------------|
| `selfmon.values.yaml` | OTLP self-mon **sink** | Was `selfmon-scraper.yaml`. Prometheus receiver removed; `cumulativetodelta`/`metric_start_time`/rename removed; `replicaCount: 2` + `autoscaling`. |
| `allocator.values.yaml` | Target Allocator | Added `config.telemetry.metrics.otlp` → `selfmon:4318` (delta); scrape annotation `false`; `env` for `OTEL_RESOURCE_ATTRIBUTES`. **Needs a TA image with OTLP support.** |
| `tier1-scraper.values.yaml` | tier1 scraper | Self-telemetry reader endpoint → `http://selfmon:4318`. |
| `tier2-gateway.values.yaml` | tier2 gateway | Self-telemetry reader endpoint → `http://selfmon:4318`. |
| `rbac.yaml` | SAs + roles | `tiered-otel-sink` role broadened for `k8s_attributes` workload lookups. |
| `scrapeconfig.yaml` | example customer targets | Unchanged. |

## Prerequisite — Target Allocator image

`allocator.values.yaml` uses `config.telemetry.metrics.otlp`, which requires a Target
Allocator built from the OTLP self-telemetry work (opentelemetry-operator PR #5068). The
stock `target-allocator:0.2.0` image does **not** support it yet. Build the TA image from
that branch and set `targetAllocator.image.tag` before deploying.

## Deploy

```sh
export NAMESPACE=otel-ta
kubectl create namespace "$NAMESPACE"

# Dynatrace credentials consumed by selfmon + tier2 (DT_ENDPOINT, DT_API_TOKEN)
kubectl -n "$NAMESPACE" create secret generic dynatrace-otelcol-credentials \
  --from-literal=DT_ENDPOINT="https://<env>.live.dynatrace.com/api/v2/otlp" \
  --from-literal=DT_API_TOKEN="<token-with-metrics.ingest>"

envsubst < rbac.yaml | kubectl apply -f -
envsubst < scrapeconfig.yaml | kubectl apply -f -

helm install otel-selfmon   open-telemetry/opentelemetry-collector          -n "$NAMESPACE" -f selfmon.values.yaml
helm install otel-allocator open-telemetry/opentelemetry-target-allocator   -n "$NAMESPACE" -f allocator.values.yaml
helm install otel-scraper   open-telemetry/opentelemetry-collector          -n "$NAMESPACE" -f tier1-scraper.values.yaml
helm install otel-gateway   open-telemetry/opentelemetry-collector          -n "$NAMESPACE" -f tier2-gateway.values.yaml
```

## Test plan (to run later)

Goal: confirm self-telemetry from all three sources arrives in Dynatrace **properly enriched**,
matching the selfmon dashboard, with no dedicated scraper.

1. **All three sources land in DT.** Query each source's marker metric and confirm presence:
   - TA: `opentelemetry_allocator_*` (e.g. `opentelemetry_allocator_targets_per_collector`).
   - tier1/tier2: `otelcol_*` (e.g. `otelcol_exporter_sent_metric_points`, `otelcol_process_*`).
2. **SD-metrics parity (the bridge).** Confirm the TA's Prometheus-registry metrics reach DT
   via OTLP, not just `/metrics`: `otelcol_...`/`go_*`/`process_*` and Prometheus SD internals
   from the TA pod. Compare the TA `/metrics` endpoint against what arrives in DT — sets match.
3. **Enrichment.** Every self-mon series carries the dashboard's resource attributes:
   `k8s.cluster.name`, `k8s.namespace.name`, `k8s.pod.name`, `k8s.workload.name` (+ kind/uid),
   `k8s.node.name`, `k8s.container.name`, `service.name`. Verify for a TA series specifically
   (`k8s.workload.name = tiered-allocator`, `k8s.workload.kind = ...`).
4. **Dashboard renders.** Load `otelcol-prometheusScraping.dashboard.json` and confirm tiles
   populate for allocator/scraper/gateway.
5. **Delta correctness.** Counters (e.g. `otelcol_exporter_sent_metric_points`) show sane
   rates — no doubling (validates the bridge doesn't double-count SDK metrics) and no gaps.
6. **HA / HPA.** Scale `selfmon` to ≥2 replicas; delete one pod mid-flow → no SFM gap. Drive
   load and confirm the HPA scales `selfmon` and metrics stay continuous.
7. **No scraper dependency.** Confirm nothing scrapes the TA (`metrics.dynatrace.com/scrape`
   is `false`) and self-telemetry still flows.
