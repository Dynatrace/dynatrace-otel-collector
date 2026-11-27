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
CLUSTER_NAME="${CLUSTER_NAME:-prometheus-large-scale-test}"

COLLECTOR_IMAGE="${CONTAINER_REGISTRY}dynatrace-otel-collector"
COLLECTOR_TAG="e2e-test"

echo "=== Setting up prometheus-large-scale test ==="
echo "  Namespace:       ${NAMESPACE}"
echo "  Host:            ${HOST}"
echo "  Config dir:      ${CONFIG_DIR}"
echo "  Cluster name:    ${CLUSTER_NAME}"
echo "  Collector image: ${COLLECTOR_IMAGE}:${COLLECTOR_TAG}"

# Private scratch dir for rendered override files. 0700 via mktemp -d,
# removed on exit including on error.
TMPDIR="$(mktemp -d)"
chmod 700 "${TMPDIR}"
trap 'rm -rf "${TMPDIR}"' EXIT

# Render HOST and port placeholders into override files
if [[ -n "${HELM_OVERRIDE_DIR}" ]]; then
    for tpl in tier1-scraper-override tier2-gateway-override allocator-override; do
        if [[ -f "${HELM_OVERRIDE_DIR}/${tpl}.yaml" ]]; then
            sed \
                -e "s|{{HOST}}|${HOST}|g" \
                -e "s|{{SELFMON_SCRAPER_HTTP_PORT}}|${SELFMON_SCRAPER_HTTP_PORT}|g" \
                -e "s|{{SELFMON_GATEWAY_HTTP_PORT}}|${SELFMON_GATEWAY_HTTP_PORT}|g" \
                -e "s|{{SELFMON_ALLOCATOR_HTTP_PORT}}|${SELFMON_ALLOCATOR_HTTP_PORT}|g" \
                "${HELM_OVERRIDE_DIR}/${tpl}.yaml" > "${TMPDIR}/${tpl}.yaml"
        fi
    done
fi

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
# Each component exports self-telemetry directly via OTLP. One secret covers
# both the scraped-data exporter (otlp_http → normal sink) and the
# self-telemetry readers (redirected per component via the helm overrides).
kubectl create secret generic dynatrace-otelcol-credentials \
    --namespace "${NAMESPACE}" \
    --from-literal=DT_ENDPOINT="http://${HOST}:4318" \
    --from-literal=DT_PLATFORM_TOKEN="" \
    --dry-run=client -o yaml | kubectl apply -f -

# Cluster name for the Target Allocator (its chart does not template extraEnvs).
kubectl create configmap dynatrace-otelcol-cluster-info \
    --namespace "${NAMESPACE}" \
    --from-literal=K8S_CLUSTER_NAME="${CLUSTER_NAME}" \
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
ALLOCATOR_HELM_ARGS=()
if [[ -f "${TMPDIR}/allocator-override.yaml" ]]; then
    ALLOCATOR_HELM_ARGS+=(-f "${TMPDIR}/allocator-override.yaml")
fi
helm upgrade --install dtp-allocator open-telemetry/opentelemetry-target-allocator \
    --namespace "${NAMESPACE}" \
    -f "${CONFIG_DIR}/allocator.values.yaml" \
    "${ALLOCATOR_HELM_ARGS[@]+"${ALLOCATOR_HELM_ARGS[@]}"}" \
    --set "targetAllocator.config.collector_namespace=${NAMESPACE}" \
    --set "replicaCount=1" \
    --wait --timeout 180s

# --- Tier 2 Gateway (install before scraper so LB target exists) -----------
echo "Installing Tier 2 Gateway..."
GATEWAY_HELM_ARGS=()
if [[ -f "${TMPDIR}/tier2-gateway-override.yaml" ]]; then
    GATEWAY_HELM_ARGS+=(-f "${TMPDIR}/tier2-gateway-override.yaml")
fi
helm upgrade --install dtp-gateway open-telemetry/opentelemetry-collector \
    --namespace "${NAMESPACE}" \
    -f "${CONFIG_DIR}/tier2-gateway.values.yaml" \
    "${GATEWAY_HELM_ARGS[@]+"${GATEWAY_HELM_ARGS[@]}"}" \
    --set "global.clusterName=${CLUSTER_NAME}" \
    --set "image.repository=${COLLECTOR_IMAGE}" \
    --set "image.tag=${COLLECTOR_TAG}" \
    --set "autoscaling.enabled=false" \
    --set "replicaCount=1" \
    --wait --timeout 180s

# --- Tier 1 Scraper --------------------------------------------------------
echo "Installing Tier 1 Scraper..."
SCRAPER_HELM_ARGS=()
if [[ -f "${TMPDIR}/tier1-scraper-override.yaml" ]]; then
    SCRAPER_HELM_ARGS+=(-f "${TMPDIR}/tier1-scraper-override.yaml")
fi
helm upgrade --install dtp-scraper open-telemetry/opentelemetry-collector \
    --namespace "${NAMESPACE}" \
    -f "${CONFIG_DIR}/tier1-scraper.values.yaml" \
    "${SCRAPER_HELM_ARGS[@]+"${SCRAPER_HELM_ARGS[@]}"}" \
    --set "global.clusterName=${CLUSTER_NAME}" \
    --set "image.repository=${COLLECTOR_IMAGE}" \
    --set "image.tag=${COLLECTOR_TAG}" \
    --set "autoscaling.enabled=false" \
    --set "replicaCount=1" \
    --wait --timeout 180s

# --- avalanche (load generator) — deploy last so system is ready -----------
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
echo "Deploying avalanche..."
kubectl apply -f "${SCRIPT_DIR}/../avalanche.yaml"
kubectl rollout status deployment/avalanche -n avalanche --timeout=120s

# --- Prime event-driven metrics to prevent flaky test assertions -----------
# Several self-monitoring metrics are delta monotonic counters that the OTEL SDK
# only emits when their value is non-zero in the collection window.  After the
# initial startup burst they stay at 0 and disappear from exports, causing the
# pmetricassert validation to fail intermittently.
#
# Fix: trigger the underlying events so the counters increment at least once
# while the test's 3-minute assertion retry window is open.
#
# 1. otelcol.k8s.watcher.namespace.added (gateway k8sattributesprocessor):
#    Create a temporary namespace — the k8s informer fires an Add event, which
#    increments the counter.
echo "Priming event-driven metrics: creating trigger namespace..."
kubectl create namespace "${NAMESPACE}-event-trigger" --dry-run=client -o yaml | kubectl apply -f -

# 2. otelcol_loadbalancer_num_resolutions + otelcol_loadbalancer_num_backend_updates
#    (scraper load_balancing exporter, k8s resolver):
#    Rolling-restart the gateway StatefulSet.  As pods cycle, the k8s endpoint
#    watch fires, the resolver re-resolves the backend list, and both counters
#    increment. "--wait=false" returns immediately; the pods finish in the
#    background while the Go test's assertion window is running.
echo "Priming event-driven metrics: restarting gateway to trigger LB re-resolution..."
kubectl rollout restart statefulset/dtp-gateway -n "${NAMESPACE}"

echo "=== Setup complete ==="
