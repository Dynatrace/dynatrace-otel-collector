#!/bin/env bash

# This script will query the Dynatrace Logs v2 API for logs from the last 5 minutes with a specific log message which is provided as command line argument.
# The script will then print the logs to the console.

# Required environment variables:
# - DT_API_ENDPOINT: The URL of the Dynatrace environment
# - DT_API_TOKEN: The API token for the Dynatrace environment

# Required command line arguments:
# - logMessage: The log message to search for

# Example usage:
# ./get_dt_logs.sh "Error"

# Check if the log message is provided as command line argument
if [ -z "$1" ]; then
  echo "Please provide a log message as command line argument"
  exit 1
fi

echo "Query DT Tenant $DT_API_ENDPOINT/api/v2/logs/"

for i in {1..10}
do
    echo "$i attempt to query logs with the message: $1"...
    # Query the Dynatrace Logs v2 API for logs from the last 5 minutes with the provided log message
    LOGRESULT=$(curl -X GET "$DT_API_ENDPOINT/api/v2/logs/search?from=now-5m&query=$1" -H "accept: application/json; charset=utf-8" -H "Authorization: Api-Token $DT_API_TOKEN" | jq -r '.results[]')

    # Check if LOGRESULT is empty
    if [ -z "$LOGRESULT" ]; then
        echo "No logs found with the message: $1"
        sleep 10
    else
        echo "Logs found with the message: $1"
        echo "$LOGRESULT"
        exit 0
    fi
done

exit 1