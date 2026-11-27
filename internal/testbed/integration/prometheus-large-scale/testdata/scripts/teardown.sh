#!/usr/bin/env bash
# Copyright Dynatrace LLC
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail

NAMESPACE="${NAMESPACE:-otel-ta}"

echo "=== Tearing down prometheus-large-scale test ==="

# Uninstall Helm releases (ignore errors if already removed)
helm uninstall dtp-scraper   --namespace "${NAMESPACE}" 2>/dev/null || true
helm uninstall dtp-gateway   --namespace "${NAMESPACE}" 2>/dev/null || true
helm uninstall dtp-allocator --namespace "${NAMESPACE}" 2>/dev/null || true

# Delete avalanche namespace (removes all avalanche resources)
kubectl delete namespace avalanche --ignore-not-found --wait=false 2>/dev/null || true

# Delete the event-trigger namespace created during setup to prime delta counters
kubectl delete namespace "${NAMESPACE}-event-trigger" --ignore-not-found --wait=false 2>/dev/null || true

# Delete test namespace (removes secrets, service accounts, etc.)
kubectl delete namespace "${NAMESPACE}" --ignore-not-found --wait=false 2>/dev/null || true

# Clean up cluster-scoped resources created by rbac.yaml
for name in dtp-otel-scraper dtp-otel-gateway dtp-otel-allocator; do
    kubectl delete clusterrolebinding "${name}" --ignore-not-found 2>/dev/null || true
    kubectl delete clusterrole "${name}" --ignore-not-found 2>/dev/null || true
done

echo "=== Teardown complete ==="
