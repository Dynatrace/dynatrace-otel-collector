#!/usr/bin/env bash
# Deploy the OTLP-direct setup into a local kind cluster with a real Dynatrace tenant.
# No sink collector — all self-telemetry and scraped metrics go straight to DT.
#
# Reads DT_ENDPOINT and DT_API_TOKEN from ../../../credentials.yaml.
# DT_API_TOKEN is stored token-only (the configs already prepend "Api-Token ").
set -euo pipefail

NAMESPACE="${NAMESPACE:-otel-ta}"
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BASE="$(cd "$DIR/.." && pwd)"
CREDS="$(cd "$BASE/.." && pwd)/credentials.yaml"

TA_IMAGE_REPO="${TA_IMAGE_REPO:-localhost/dt-target-allocator}"
TA_IMAGE_TAG="${TA_IMAGE_TAG:-icp5695}"

if [[ ! -f "$CREDS" ]]; then
  echo "ERROR: credentials.yaml not found at $CREDS" >&2
  exit 1
fi

DT_ENDPOINT="$(grep -A1 'name: DT_ENDPOINT' "$CREDS" | grep 'value:' | sed 's/.*value: "\(.*\)"/\1/')"
DT_API_TOKEN="$(grep -A1 'name: DT_API_TOKEN' "$CREDS" | grep 'value:' | sed 's/.*value: "\(.*\)"/\1/' | sed 's/^Api-Token //')"

if [[ -z "$DT_ENDPOINT" || -z "$DT_API_TOKEN" ]]; then
  echo "ERROR: could not parse DT_ENDPOINT or DT_API_TOKEN from $CREDS" >&2
  exit 1
fi

echo "=== namespace ==="
kubectl create namespace "$NAMESPACE" --dry-run=client -o yaml | kubectl apply -f -

echo "=== Prometheus Operator CRDs ==="
CRD_BASE="https://raw.githubusercontent.com/prometheus-operator/prometheus-operator/v0.82.2/example/prometheus-operator-crd"
for crd in scrapeconfigs servicemonitors podmonitors probes; do
  kubectl apply --server-side -f "${CRD_BASE}/monitoring.coreos.com_${crd}.yaml"
done

echo "=== credentials secret (exporters -> Dynatrace: $DT_ENDPOINT) ==="
kubectl create secret generic dynatrace-otelcol-credentials \
  --namespace "$NAMESPACE" \
  --from-literal=DT_ENDPOINT="$DT_ENDPOINT" \
  --from-literal=DT_API_TOKEN="$DT_API_TOKEN" \
  --dry-run=client -o yaml | kubectl apply -f -

echo "=== rbac + scrapeconfig ==="
sed "s|\${NAMESPACE}|${NAMESPACE}|g" "$BASE/rbac.yaml" | kubectl apply -f -
sed "s|\${NAMESPACE}|${NAMESPACE}|g" "$BASE/scrapeconfig.yaml" | kubectl apply -f -

echo "=== helm repo ==="
helm repo add open-telemetry https://open-telemetry.github.io/opentelemetry-helm-charts >/dev/null 2>&1 || true
helm repo update >/dev/null

echo "=== Target Allocator ==="
helm upgrade --install otel-allocator open-telemetry/opentelemetry-target-allocator \
  --namespace "$NAMESPACE" -f "$BASE/allocator.values.yaml" \
  --set "targetAllocator.config.collector_namespace=${NAMESPACE}" \
  --set "targetAllocator.image.repository=${TA_IMAGE_REPO}" \
  --set "targetAllocator.image.tag=${TA_IMAGE_TAG}" \
  --set "targetAllocator.image.pullPolicy=Never" \
  --set "replicaCount=1" \
  --wait --timeout 180s

echo "=== Tier 2 Gateway (install before scraper so LB target exists) ==="
helm upgrade --install otel-gateway open-telemetry/opentelemetry-collector \
  --namespace "$NAMESPACE" -f "$BASE/tier2-gateway.values.yaml" \
  --set "autoscaling.enabled=false" --set "replicaCount=1" \
  --wait --timeout 180s

echo "=== Tier 1 Scraper ==="
helm upgrade --install otel-scraper open-telemetry/opentelemetry-collector \
  --namespace "$NAMESPACE" -f "$BASE/tier1-scraper.values.yaml" \
  --set "autoscaling.enabled=false" --set "replicaCount=1" \
  --wait --timeout 180s

echo "=== avalanche ==="
kubectl apply -f "$BASE/kind-test/avalanche.yaml"
kubectl rollout status deployment/avalanche -n avalanche --timeout=120s

echo "=== done ==="
kubectl get pods -n "$NAMESPACE"
