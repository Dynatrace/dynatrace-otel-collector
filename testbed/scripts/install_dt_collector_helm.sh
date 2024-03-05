#!/bin/env bash
echo "Installing Dynatrace Collector using helm"
helm version

kubectl create namespace otel-collector
kubectl -n otel-collector create secret generic dynatrace-otelcol-dt-api-credentials --from-literal=DT_API_ENDPOINT=$DT_API_ENDPOINT/api/v2/otlp --from-literal=DT_API_TOKEN=$DT_API_TOKEN

helm repo add open-telemetry https://open-telemetry.github.io/opentelemetry-helm-charts
helm repo update

cd .github/actions/deploy-collector
helm upgrade -i --wait dynatrace open-telemetry/opentelemetry-collector \
    -f collector-helm-values.yaml \
    -n otel-collector \
    --timeout 5m

# show the collector logs
kubectl -n otel-collector logs -l app.kubernetes.io/name=opentelemetry-collector
