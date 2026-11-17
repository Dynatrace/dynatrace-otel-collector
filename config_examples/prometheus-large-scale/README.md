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

Set `NAMESPACE` and apply RBAC + ScrapeConfig, then install Helm charts. The cluster name reported as `k8s.cluster.name` comes from `global.clusterName` in each values file; set it with `--set global.clusterName=<your-cluster>`:

`kubectl` and `helm` do not expand environment variables, so substitute `${NAMESPACE}` with `envsubst` first:

```sh
export NAMESPACE=<your-namespace>
kubectl create namespace "$NAMESPACE" --dry-run=client -o yaml | kubectl apply -f -

# ScrapeConfig CRD from the Prometheus Operator (skip if already installed)
kubectl apply --server-side -f https://raw.githubusercontent.com/prometheus-operator/prometheus-operator/v0.79.2/example/prometheus-operator-crd/monitoring.coreos.com_scrapeconfigs.yaml

envsubst '${NAMESPACE}' < rbac.yaml         | kubectl apply -f -
envsubst '${NAMESPACE}' < scrapeconfig.yaml | kubectl apply -f -

kubectl create secret generic dynatrace-otelcol-credentials -n "$NAMESPACE" \
  --from-literal=DT_ENDPOINT=https://<your-environment-id>.live.dynatrace.com/api/v2/otlp \
  --from-literal=DT_PLATFORM_TOKEN=<your-token>

helm repo add open-telemetry https://open-telemetry.github.io/opentelemetry-helm-charts
helm repo update

envsubst '${NAMESPACE}' < allocator.values.yaml | \
  helm install otel-allocator open-telemetry/opentelemetry-target-allocator -n "$NAMESPACE" -f -
envsubst '${NAMESPACE}' < tier1-scraper.values.yaml | \
  helm install otel-scraper open-telemetry/opentelemetry-collector -n "$NAMESPACE" -f -
envsubst '${NAMESPACE}' < tier2-gateway.values.yaml | \
  helm install otel-gateway open-telemetry/opentelemetry-collector -n "$NAMESPACE" -f -
envsubst '${NAMESPACE}' < selfmon-scraper.yaml | \
  helm install otel-selfmon open-telemetry/opentelemetry-collector -n "$NAMESPACE" -f -
```
