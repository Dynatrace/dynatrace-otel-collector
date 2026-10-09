#!/usr/bin/env bash
# Deploy the Phase-2 (Operator-layout) no-selfmon setup against a real Dynatrace tenant.
# Every component pushes its own self-telemetry over OTLP straight to Dynatrace.
#
# Required env:
#   DT_ENDPOINT      base OTLP endpoint, e.g. https://<env-id>.live.dynatrace.com/api/v2/otlp
#   DT_API_TOKEN     token value only, WITHOUT the "Api-Token " prefix
#                    scopes: metrics.ingest (openTelemetryTrace.ingest not needed)
# Optional env:
#   NAMESPACE        default otel-phase2
#   CLUSTER_NAME     default dtp-phase2-test  (becomes k8s.cluster.name)
set -euo pipefail

NAMESPACE="${NAMESPACE:-otel-phase2}"
CLUSTER_NAME="${CLUSTER_NAME:-dtp-phase2-test}"
TA_CHART_VERSION="${TA_CHART_VERSION:-0.158.0}"
COLLECTOR_CHART_VERSION="${COLLECTOR_CHART_VERSION:-0.170.0}"
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BASE="$(cd "$DIR/.." && pwd)"

: "${DT_ENDPOINT:?set DT_ENDPOINT, e.g. https://abc12345.live.dynatrace.com/api/v2/otlp}"
: "${DT_API_TOKEN:?set DT_API_TOKEN (token value only, no \"Api-Token \" prefix)}"

if [[ "$DT_API_TOKEN" == Api-Token\ * ]]; then
  echo "ERROR: DT_API_TOKEN must be the bare token; the configs add the \"Api-Token \" prefix." >&2
  exit 1
fi

# Private scratch dir for rendered values and the secret env-file. 0700 via mktemp -d,
# removed on exit including on error.
RENDER="$(mktemp -d)"
chmod 700 "$RENDER"
trap 'rm -rf "$RENDER"' EXIT

echo "=== namespace ==="
kubectl create namespace "$NAMESPACE" --dry-run=client -o yaml | kubectl apply -f -

echo "=== Prometheus Operator CRDs ==="
CRD_BASE="https://raw.githubusercontent.com/prometheus-operator/prometheus-operator/v0.82.2/example/prometheus-operator-crd"
for crd in scrapeconfigs servicemonitors podmonitors probes; do
  kubectl apply --server-side -f "${CRD_BASE}/monitoring.coreos.com_${crd}.yaml"
done

echo "=== credentials secret (-> $DT_ENDPOINT) ==="
# --from-env-file, not --from-literal: a literal would put the token in the process
# argument list, readable via `ps` by any local user.
ENV_FILE="$RENDER/dt-credentials.env"
( umask 077; printf 'DT_ENDPOINT=%s\nDT_API_TOKEN=%s\n' "$DT_ENDPOINT" "$DT_API_TOKEN" > "$ENV_FILE" )
kubectl create secret generic dynatrace-otelcol-credentials \
  --namespace "$NAMESPACE" \
  --from-env-file="$ENV_FILE" \
  --dry-run=client -o yaml | kubectl apply -f -

echo "=== rbac + scrapeconfig ==="
sed "s|\${NAMESPACE}|${NAMESPACE}|g" "$BASE/rbac.yaml"         | kubectl apply -f -
sed "s|\${NAMESPACE}|${NAMESPACE}|g" "$BASE/scrapeconfig.yaml" | kubectl apply -f -

echo "=== render values (substitute cluster name) ==="
for f in allocator.values.yaml tier1-scraper.values.yaml tier2-gateway.values.yaml; do
  sed "s|REPLACE_ME_CLUSTER_NAME|${CLUSTER_NAME}|g" "$BASE/$f" > "$RENDER/$f"
done

echo "=== helm repo ==="
helm repo add open-telemetry https://open-telemetry.github.io/opentelemetry-helm-charts >/dev/null 2>&1 || true
helm repo update >/dev/null

echo "=== Target Allocator (upstream chart ${TA_CHART_VERSION}, upstream image) ==="
helm upgrade --install dtp-allocator open-telemetry/opentelemetry-target-allocator \
  --version "$TA_CHART_VERSION" \
  --namespace "$NAMESPACE" -f "$RENDER/allocator.values.yaml" \
  --set "targetAllocator.config.collector_namespace=${NAMESPACE}" \
  --wait --timeout 180s

echo "=== Tier 2 Gateway (before scraper, so the LB target exists) ==="
helm upgrade --install dtp-gateway open-telemetry/opentelemetry-collector \
  --version "$COLLECTOR_CHART_VERSION" \
  --namespace "$NAMESPACE" -f "$RENDER/tier2-gateway.values.yaml" \
  --set "autoscaling.enabled=false" --set "replicaCount=1" \
  --wait --timeout 180s

echo "=== Tier 1 Scraper ==="
helm upgrade --install dtp-scraper open-telemetry/opentelemetry-collector \
  --version "$COLLECTOR_CHART_VERSION" \
  --namespace "$NAMESPACE" -f "$RENDER/tier1-scraper.values.yaml" \
  --set "autoscaling.enabled=false" --set "replicaCount=1" \
  --wait --timeout 180s

echo "=== avalanche (scrape target for the customer path) ==="
kubectl apply -f "$DIR/avalanche.yaml"
kubectl rollout status deployment/avalanche -n avalanche --timeout=120s

echo "=== done ==="
kubectl get pods -n "$NAMESPACE"
echo
echo "k8s.cluster.name = ${CLUSTER_NAME}"
echo "Check the dashboard variables resolve to: dtp-prometheus-allocator / dtp-scraper / dtp-gateway"
