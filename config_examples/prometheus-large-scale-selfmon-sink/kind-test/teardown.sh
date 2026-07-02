#!/usr/bin/env bash
set -uo pipefail
NAMESPACE="${NAMESPACE:-otel-ta}"
for r in otel-sink otel-allocator otel-gateway otel-scraper otel-selfmon; do
  helm uninstall "$r" -n "$NAMESPACE" 2>/dev/null || true
done
kubectl delete -f "$(dirname "${BASH_SOURCE[0]}")/avalanche.yaml" 2>/dev/null || true
kubectl delete namespace "$NAMESPACE" 2>/dev/null || true
for cr in tiered-otel-scraper tiered-otel-gateway tiered-otel-sink tiered-otel-allocator; do
  kubectl delete clusterrole "$cr" 2>/dev/null || true
  kubectl delete clusterrolebinding "$cr" 2>/dev/null || true
done
echo "torn down"
