#!/usr/bin/env bash

set -euo pipefail

kind create cluster --config=kind-config.yaml
kubectl create namespace kafka

kubectl create -f 'https://strimzi.io/install/latest?namespace=kafka' -n kafka

echo "Waiting for Strimzi operator to be ready..."
kubectl wait deployment/strimzi-cluster-operator --for=condition=Available --timeout=300s -n kafka
echo "Strimzi operator is ready."

kubectl apply -f https://strimzi.io/examples/latest/kafka/kafka-single-node.yaml -n kafka 

echo "Waiting for Kafka cluster to be ready..."
kubectl wait kafka/my-cluster --for=condition=Ready --timeout=300s -n kafka
echo "Kafka cluster is ready."

helm repo add open-telemetry https://open-telemetry.github.io/opentelemetry-helm-charts
helm repo update

helm install my-opentelemetry-collector open-telemetry/opentelemetry-collector --values=otelcol-values.yaml
