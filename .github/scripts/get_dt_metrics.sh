#!/bin/env bash

# This script will query the Dynatrace Metrics v2 API for metrics from the last 5 minutes with a specific selector which is provided as command line argument.
# The script will then print the metrics to the console.

# Required environment variables:
# - DT_API_ENDPOINT: The URL of the Dynatrace environment
# - DT_API_TOKEN: The API token for the Dynatrace environment

# Required command line arguments:
# - metricsSelector: The log message to search for

# Example usage:
# ./get_dt_metrics.sh "query?metricSelector=(node_dmi_info:filter(and(or(eq(\"otel.scope.name\",\"otelcol/prometheusreceiver\")),or(eq(product_name,kind)))):splitBy(product_name):sort(value(auto,descending)):limit(20)):limit(100):names"

# Example query?metricSelector=(node_dmi_info:filter(and(or(eq(\"otel.scope.name\",\"otelcol/prometheusreceiver\")),or(eq(product_name,kind)))):splitBy(product_name):sort(value(auto,descending)):limit(20)):limit(100):names&from=-6h&to=now


if [ -z "$1" ]; then
  echo "Please provide a metric selector as command line argument"
  exit 1
fi

echo "Query DT Tenant $DT_API_ENDPOINT/api/v2/metrics/"

for i in {1..10}
do
    echo "$i attempt to query metrics with selector: $1"...
    # Query the Dynatrace Metrics v2 API for logs from the last 5 minutes with the provided metric selector
    METRICS=$(curl -X GET "$DT_API_ENDPOINT/api/v2/metrics/query?from=now-5m&metricSelector=$1" -H "accept: application/json; charset=utf-8" -H "Authorization: Api-Token $DT_API_TOKEN" | jq -r '.result[].data[]')
    #curl -X GET "$DT_API_ENDPOINT/api/v2/metrics/query?from=now-5m&metricSelector=$1" -H "accept: application/json; charset=utf-8" -H "Authorization: Api-Token $DT_API_TOKEN" | jq -r '.result[].data[]'

    # Check if METRICS is empty
    if [ -z "$METRICS" ]; then
        echo "No metrics found with given selector: $1"
        sleep 30
    else
        echo "Metrics found with given selector: $1"
        echo "$METRICS"
        exit 0
    fi
done

exit 1