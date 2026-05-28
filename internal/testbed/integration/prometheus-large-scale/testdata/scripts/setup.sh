#!/usr/bin/env bash
set -euo pipefail

NAMESPACE="${NAMESPACE:?NAMESPACE is required}"
HOST="${HOST:?HOST is required}"
CONTAINER_REGISTRY="${CONTAINER_REGISTRY:?CONTAINER_REGISTRY is required}"
CONFIG_DIR="${CONFIG_DIR:?CONFIG_DIR is required}"
SELFMON_HTTP_PORT="${SELFMON_HTTP_PORT:-4328}"

COLLECTOR_IMAGE="${CONTAINER_REGISTRY}dynatrace-otel-collector"
COLLECTOR_TAG="e2e-test"

echo "=== Setting up prometheus-large-scale test ==="
echo "  Namespace:       ${NAMESPACE}"
echo "  Host:            ${HOST}"
echo "  Config dir:      ${CONFIG_DIR}"
echo "  Collector image: ${COLLECTOR_IMAGE}:${COLLECTOR_TAG}"

# --- namespaces -----------------------------------------------------------
kubectl create namespace "${NAMESPACE}" --dry-run=client -o yaml | kubectl apply -f -

# --- Prometheus Operator CRDs (needed for ScrapeConfig / TA) ---------------
echo "Installing Prometheus Operator CRDs..."
CRD_BASE="https://raw.githubusercontent.com/prometheus-operator/prometheus-operator/v0.82.2/example/prometheus-operator-crd"
kubectl apply --server-side -f "${CRD_BASE}/monitoring.coreos.com_scrapeconfigs.yaml"
kubectl apply --server-side -f "${CRD_BASE}/monitoring.coreos.com_servicemonitors.yaml"
kubectl apply --server-side -f "${CRD_BASE}/monitoring.coreos.com_podmonitors.yaml"
kubectl apply --server-side -f "${CRD_BASE}/monitoring.coreos.com_probes.yaml"

# --- secrets ---------------------------------------------------------------
# Secret for tier1-scraper, tier2-gateway, allocator  → sink 1 (default OTLP ports)
kubectl create secret generic dynatrace-otelcol-credentials \
    --namespace "${NAMESPACE}" \
    --from-literal=DT_ENDPOINT="http://${HOST}:4318" \
    --from-literal=DT_API_TOKEN="" \
    --dry-run=client -o yaml | kubectl apply -f -

# Secret for selfmon-scraper → sink 2 (separate port)
kubectl create secret generic selfmon-credentials \
    --namespace "${NAMESPACE}" \
    --from-literal=DT_ENDPOINT="http://${HOST}:${SELFMON_HTTP_PORT}" \
    --from-literal=DT_API_TOKEN="" \
    --dry-run=client -o yaml | kubectl apply -f -

# --- RBAC ------------------------------------------------------------------
sed "s|\${NAMESPACE}|${NAMESPACE}|g" "${CONFIG_DIR}/rbac.yaml" | kubectl apply -f -

# --- ScrapeConfig ----------------------------------------------------------
sed "s|\${NAMESPACE}|${NAMESPACE}|g" "${CONFIG_DIR}/scrapeconfig.yaml" | kubectl apply -f -

# --- Helm repos ------------------------------------------------------------
helm repo add open-telemetry https://open-telemetry.github.io/opentelemetry-helm-charts
helm repo update

# --- Target Allocator ------------------------------------------------------
echo "Installing Target Allocator..."
helm upgrade --install otel-allocator open-telemetry/opentelemetry-target-allocator \
    --namespace "${NAMESPACE}" \
    -f "${CONFIG_DIR}/allocator.values.yaml" \
    --set "targetAllocator.config.collector_namespace=${NAMESPACE}" \
    --set "replicaCount=1" \
    --wait --timeout 180s

# --- Tier 2 Gateway (install before scraper so LB target exists) -----------
echo "Installing Tier 2 Gateway..."
helm upgrade --install otel-gateway open-telemetry/opentelemetry-collector \
    --namespace "${NAMESPACE}" \
    -f "${CONFIG_DIR}/tier2-gateway.values.yaml" \
    --set "image.repository=${COLLECTOR_IMAGE}" \
    --set "image.tag=${COLLECTOR_TAG}" \
    --set "autoscaling.enabled=false" \
    --set "replicaCount=1" \
    --wait --timeout 180s

# --- Tier 1 Scraper --------------------------------------------------------
echo "Installing Tier 1 Scraper..."
helm upgrade --install otel-scraper open-telemetry/opentelemetry-collector \
    --namespace "${NAMESPACE}" \
    -f "${CONFIG_DIR}/tier1-scraper.values.yaml" \
    --set "image.repository=${COLLECTOR_IMAGE}" \
    --set "image.tag=${COLLECTOR_TAG}" \
    --set "autoscaling.enabled=false" \
    --set "replicaCount=1" \
    --wait --timeout 180s

# --- Selfmon Scraper (override secret → selfmon sink) ----------------------
echo "Installing Selfmon Scraper..."
helm upgrade --install otel-selfmon open-telemetry/opentelemetry-collector \
    --namespace "${NAMESPACE}" \
    -f "${CONFIG_DIR}/selfmon-scraper.yaml" \
    --set "image.repository=${COLLECTOR_IMAGE}" \
    --set "image.tag=${COLLECTOR_TAG}" \
    --set "extraEnvsFrom[0].secretRef.name=selfmon-credentials" \
    --wait --timeout 180s

# --- avalanche (load generator) — deploy last so system is ready -----------
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
echo "Deploying avalanche..."
kubectl apply -f "${SCRIPT_DIR}/../avalanche.yaml"
kubectl rollout status deployment/avalanche -n avalanche --timeout=120s

echo "=== Setup complete ==="
