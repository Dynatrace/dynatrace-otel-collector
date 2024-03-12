This Action Requires the following environment variables

- `DT_API_ENDPOINT`: The URL of the Dynatrace OTLP endpoint. Example: `https://<your-environment-id>.live.dynatrace.com`

- `DT_API_TOKEN`: The API token for the Dynatrace environment. Required scopes: 
    - Ingest logs
    - Read logs
    - Ingest metrics
    - Read metrics
    - Ingest OpenTelemetry traces