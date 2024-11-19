#!/bin/env bash
echo "Installing Dynatrace Collector using helm"
helm version

kubectl create secret generic dynatrace-otelcol-dt-api-credentials --from-literal=DT_ENDPOINT="endpoint" --from-literal=DT_API_TOKEN="token"

helm repo add open-telemetry https://open-telemetry.github.io/opentelemetry-helm-charts
helm repo update

envsubst < config_examples/collector-helm-values.yaml > tmp.yaml

echo "Using config:"
echo "-------------------------"
cat tmp.yaml
echo "-------------------------"

helm upgrade -i --wait dynatrace-collector open-telemetry/opentelemetry-collector \
    -f tmp.yaml \
    --timeout 5m

# show the collector logs
kubectl logs -l app.kubernetes.io/name=opentelemetry-collector | grep "Everything is ready. Begin running and processing data." || exit 1

rm tmp.yaml
