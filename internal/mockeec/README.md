# Mock EEC server

## How to use

1. Start the server in this directory: `go run .`
2. Run the Collector:

   ```text
    ./bin/dynatrace-otel-collector --config=eec://localhost:8000/config.yaml#insecure=true&refresh-interval=100ms
   ```

3. Update `config.yaml`
4. See the Collector restart the service.
5. Shut down the Collector and server.
