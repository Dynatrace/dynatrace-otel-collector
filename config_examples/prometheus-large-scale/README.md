# Prometheus Large-Scale

Tiered OTel Collector setup for scraping Prometheus targets at scale and shipping to Dynatrace.

## Architecture

- **Tier 1 — Scraper** (`tier1-scraper.values.yaml`): scrapes targets assigned by Target Allocator, load-balances OTLP to tier 2.
- **Tier 2 — Gateway** (`tier2-gateway.values.yaml`): enriches metrics, exports to Dynatrace.
- **Target Allocator** (`allocator.values.yaml`): distributes scrape targets across tier 1 replicas (consistent-hashing).
- **Selfmon Scraper** (`selfmon-scraper.yaml`): scrapes collector/allocator self-metrics direct to Dynatrace.
- **ScrapeConfig** (`scrapeconfig.yaml`): example Prometheus Operator `ScrapeConfig` CR consumed by TA.
- **RBAC** (`rbac.yaml`): ServiceAccounts + roles for scraper, gateway, sink, allocator.

## Deploy

Set `NAMESPACE` and apply RBAC + ScrapeConfig, then install Helm charts:

```sh
kubectl apply -f rbac.yaml
kubectl apply -f scrapeconfig.yaml

helm install otel-allocator open-telemetry/opentelemetry-target-allocator -f allocator.values.yaml
helm install otel-scraper   open-telemetry/opentelemetry-collector       -f tier1-scraper.values.yaml
helm install otel-gateway   open-telemetry/opentelemetry-collector       -f tier2-gateway.values.yaml
helm install otel-selfmon   open-telemetry/opentelemetry-collector       -f selfmon-scraper.yaml
```
