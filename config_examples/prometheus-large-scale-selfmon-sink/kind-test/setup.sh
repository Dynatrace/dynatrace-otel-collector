#!/usr/bin/env bash
# Deploy the OTLP-selfmon setup into the local kind cluster and point all exporters
# at an in-cluster sink (no real Dynatrace tenant needed).
set -euo pipefail

NAMESPACE="${NAMESPACE:-otel-ta}"
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BASE="$(cd "$DIR/.." && pwd)"
TA_IMAGE_REPO="${TA_IMAGE_REPO:-localhost/dt-target-allocator}"
TA_IMAGE_TAG="${TA_IMAGE_TAG:-icp5695}"

echo "=== namespace ==="
kubectl create namespace "$NAMESPACE" --dry-run=client -o yaml | kubectl apply -f -

echo "=== Prometheus Operator CRDs ==="
CRD_BASE="https://raw.githubusercontent.com/prometheus-operator/prometheus-operator/v0.82.2/example/prometheus-operator-crd"
for crd in scrapeconfigs servicemonitors podmonitors probes; do
  kubectl apply --server-side -f "${CRD_BASE}/monitoring.coreos.com_${crd}.yaml"
done

echo "=== credentials secret (exporters -> in-cluster sink) ==="
kubectl create secret generic dynatrace-otelcol-credentials \
  --namespace "$NAMESPACE" \
  --from-literal=DT_ENDPOINT="http://sink:4318" \
  --from-literal=DT_API_TOKEN="" \
  --dry-run=client -o yaml | kubectl apply -f -

echo "=== rbac + scrapeconfig ==="
sed "s|\${NAMESPACE}|${NAMESPACE}|g" "$BASE/rbac.yaml" | kubectl apply -f -
sed "s|\${NAMESPACE}|${NAMESPACE}|g" "$BASE/scrapeconfig.yaml" | kubectl apply -f -

echo "=== helm repo ==="
helm repo add open-telemetry https://open-telemetry.github.io/opentelemetry-helm-charts >/dev/null 2>&1 || true
helm repo update >/dev/null

echo "=== sink ==="
helm upgrade --install otel-sink open-telemetry/opentelemetry-collector \
  --namespace "$NAMESPACE" -f "$DIR/sink.values.yaml" \
  --wait --timeout 180s

echo "=== Target Allocator (custom OTLP-self-telemetry image) ==="
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

echo "=== Selfmon sink collector ==="
helm upgrade --install otel-selfmon open-telemetry/opentelemetry-collector \
  --namespace "$NAMESPACE" -f "$BASE/selfmon.values.yaml" \
  --set "autoscaling.enabled=false" --set "replicaCount=1" \
  --wait --timeout 180s

echo "=== avalanche ==="
kubectl apply -f "$DIR/avalanche.yaml"
kubectl rollout status deployment/avalanche -n avalanche --timeout=120s

echo "=== done ==="
kubectl get pods -n "$NAMESPACE"
