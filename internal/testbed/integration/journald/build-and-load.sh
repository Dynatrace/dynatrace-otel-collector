#!/bin/bash
set -e

echo "Building dynatrace-otel-collector with journald support..."
docker build -t localhost/otel-collector-journald:latest .

echo "Loading image into kind cluster..."
kind load docker-image localhost/otel-collector-journald:latest

echo "Image ready! Now you can apply the manifest:"
echo "  kubectl apply -f collector.yaml"
