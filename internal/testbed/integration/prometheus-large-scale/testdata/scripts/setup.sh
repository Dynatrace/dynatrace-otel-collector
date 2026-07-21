#!/usr/bin/env bash
# Copyright Dynatrace LLC
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail

NAMESPACE="${NAMESPACE:-otel-ta}"
HOST="${HOST:-localhost}"
CONTAINER_REGISTRY="${CONTAINER_REGISTRY:-}"
CONFIG_DIR="${CONFIG_DIR:-$(cd "$(dirname "${BASH_SOURCE[0]}")/../../../.." && pwd)/config_examples/prometheus-large-scale}"
HELM_OVERRIDE_DIR="${HELM_OVERRIDE_DIR:-}"
SELFMON_SCRAPER_HTTP_PORT="${SELFMON_SCRAPER_HTTP_PORT:-4328}"
SELFMON_GATEWAY_HTTP_PORT="${SELFMON_GATEWAY_HTTP_PORT:-4330}"
SELFMON_ALLOCATOR_HTTP_PORT="${SELFMON_ALLOCATOR_HTTP_PORT:-4332}"

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

# Secret for selfmon-scraper with 3 separate endpoints (one per source)
# DT_ENDPOINT is kept for the base otlphttp/dynatrace exporter (unused but validated)
kubectl create secret generic selfmon-credentials \
    --namespace "${NAMESPACE}" \
    --from-literal=DT_ENDPOINT="http://${HOST}:${SELFMON_SCRAPER_HTTP_PORT}" \
    --from-literal=DT_ENDPOINT_SCRAPER="http://${HOST}:${SELFMON_SCRAPER_HTTP_PORT}" \
    --from-literal=DT_ENDPOINT_GATEWAY="http://${HOST}:${SELFMON_GATEWAY_HTTP_PORT}" \
    --from-literal=DT_ENDPOINT_ALLOCATOR="http://${HOST}:${SELFMON_ALLOCATOR_HTTP_PORT}" \
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
GATEWAY_HELM_ARGS=()
if [[ -n "${HELM_OVERRIDE_DIR}" && -f "${HELM_OVERRIDE_DIR}/tier2-gateway-override.yaml" ]]; then
    GATEWAY_HELM_ARGS+=(-f "${HELM_OVERRIDE_DIR}/tier2-gateway-override.yaml")
fi
helm upgrade --install otel-gateway open-telemetry/opentelemetry-collector \
    --namespace "${NAMESPACE}" \
    -f "${CONFIG_DIR}/tier2-gateway.values.yaml" \
    "${GATEWAY_HELM_ARGS[@]+"${GATEWAY_HELM_ARGS[@]}"}" \
    --set "image.repository=${COLLECTOR_IMAGE}" \
    --set "image.tag=${COLLECTOR_TAG}" \
    --set "autoscaling.enabled=false" \
    --set "replicaCount=1" \
    --wait --timeout 180s

# --- Tier 1 Scraper --------------------------------------------------------
echo "Installing Tier 1 Scraper..."
SCRAPER_HELM_ARGS=()
if [[ -n "${HELM_OVERRIDE_DIR}" && -f "${HELM_OVERRIDE_DIR}/tier1-scraper-override.yaml" ]]; then
    SCRAPER_HELM_ARGS+=(-f "${HELM_OVERRIDE_DIR}/tier1-scraper-override.yaml")
fi
helm upgrade --install otel-scraper open-telemetry/opentelemetry-collector \
    --namespace "${NAMESPACE}" \
    -f "${CONFIG_DIR}/tier1-scraper.values.yaml" \
    "${SCRAPER_HELM_ARGS[@]+"${SCRAPER_HELM_ARGS[@]}"}" \
    --set "image.repository=${COLLECTOR_IMAGE}" \
    --set "image.tag=${COLLECTOR_TAG}" \
    --set "autoscaling.enabled=false" \
    --set "replicaCount=1" \
    --wait --timeout 180s

# --- Selfmon Scraper (override secret → selfmon sink) ----------------------
echo "Installing Selfmon Scraper..."
SELFMON_VALUES="${CONFIG_DIR}/selfmon-scraper.yaml"
if [[ -n "${HELM_OVERRIDE_DIR}" && -f "${HELM_OVERRIDE_DIR}/selfmon-scraper-override.yaml" ]]; then
    SELFMON_VALUES="${HELM_OVERRIDE_DIR}/selfmon-scraper-override.yaml"
fi
helm upgrade --install otel-selfmon open-telemetry/opentelemetry-collector \
    --namespace "${NAMESPACE}" \
    -f "${CONFIG_DIR}/selfmon-scraper.yaml" \
    -f "${SELFMON_VALUES}" \
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
