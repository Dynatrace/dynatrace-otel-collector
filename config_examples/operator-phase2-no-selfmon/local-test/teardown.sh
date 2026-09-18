#!/usr/bin/env bash
set -uo pipefail
NAMESPACE="${NAMESPACE:-otel-phase2}"
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
for r in dtp-allocator dtp-gateway dtp-scraper; do
  helm uninstall "$r" -n "$NAMESPACE" 2>/dev/null || true
done
kubectl delete -f "$DIR/avalanche.yaml" 2>/dev/null || true
kubectl delete namespace "$NAMESPACE" 2>/dev/null || true
for cr in dtp-otel-scraper dtp-otel-gateway dtp-otel-allocator; do
  kubectl delete clusterrole "$cr" 2>/dev/null || true
  kubectl delete clusterrolebinding "$cr" 2>/dev/null || true
done
echo "torn down"
