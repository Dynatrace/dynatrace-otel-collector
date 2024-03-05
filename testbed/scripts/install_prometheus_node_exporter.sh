#!/bin/env bash

helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update
kubectl create namespace prometheus-node-exporter
helm install prometheus-node-exporter prometheus-community/prometheus-node-exporter --namespace prometheus-node-exporter

# Wait until the node exporter is up and running
kubectl rollout --timeout 120s status daemonset/prometheus-node-exporter -n prometheus-node-exporter