# BindPlane + Dynatrace OTel Collector PoC (BYOC)

Proof-of-concept integrating the **Dynatrace OTel Collector** with **BindPlane Cloud**
using the **Bring Your Own Collector (BYOC)** pattern and the **OpAMP** protocol.

BindPlane remotely manages the collector's configuration — pushing pipeline configs,
monitoring health, and viewing available components — all through the OpAMP protocol.

## Architecture

```
┌──────────────────────────────────────────┐
│          BindPlane Cloud                 │
│   wss://app.bindplane.com/v1/opamp      │
│                                          │
│   Fleet: odubaj-poc-fleet                │
│   Config: odubaj-poc-config              │
│   Agent type: dynatrace-otel-collector   │
└──────────────┬───────────────────────────┘
               │ OpAMP (WebSocket)
               │ Auth: Authorization: Secret-Key <key>
               ▼
┌──────────────────────────────────────────┐
│  K8s Pod (Deployment)                    │
│                                          │
│  ┌────────────────────────────────────┐  │
│  │ initContainer: copy-collector     │  │
│  │   alpine image with DT collector  │  │
│  │   → cp binary to shared volume    │  │
│  └────────────────────────────────────┘  │
│                                          │
│  ┌────────────────────────────────────┐  │
│  │ container: opamp-supervisor       │  │
│  │   upstream OTel OpAMP Supervisor   │  │
│  │   connects to BindPlane Cloud      │  │
│  │   manages collector as child proc  │  │
│  └──────────────┬─────────────────────┘  │
│                 │ starts/stops/configures │
│                 ▼                         │
│  ┌────────────────────────────────────┐  │
│  │ child process: DT OTel Collector  │  │
│  │   with opampextension +           │  │
│  │   bindplaneextension +            │  │
│  │   bearertokenauthextension +      │  │
│  │   snapshotprocessor +             │  │
│  │   throughputmeasurement +         │  │
│  │   metricstransformprocessor       │  │
│  └────────────────────────────────────┘  │
└──────────────────────────────────────────┘
```

## What Was Changed in the Dynatrace OTel Collector

The following components were added to `manifest.yaml` to enable BindPlane BYOC:

### Required for OpAMP communication

```yaml
# manifest.yaml — extensions section:
- gomod: github.com/open-telemetry/opentelemetry-collector-contrib/extension/opampextension v0.149.0
```

### Required by BindPlane destinations (e.g., Dynatrace exporter)

```yaml
# manifest.yaml — extensions section:
- gomod: github.com/open-telemetry/opentelemetry-collector-contrib/extension/bearertokenauthextension v0.149.0
```

The Dynatrace exporter destination in BindPlane uses `bearertokenauthextension` to attach
API tokens to outgoing requests. Without it, adding a Dynatrace destination will fail with
an "Unavailable Components" error.

### Required by BindPlane-generated configurations

BindPlane automatically injects these components into every config it generates.
Without them, BindPlane will reject the config rollout with:
`"Unavailable Components: extensions: [bindplane], processors: [snapshotprocessor, throughputmeasurement, metricstransform]"`

```yaml
# manifest.yaml — extensions section:
- gomod: github.com/observiq/bindplane-otel-contrib/extension/bindplaneextension v1.3.0

# manifest.yaml — processors section:
- gomod: github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricstransformprocessor v0.149.0
- gomod: github.com/observiq/bindplane-otel-contrib/processor/snapshotprocessor v1.3.0
- gomod: github.com/observiq/bindplane-otel-contrib/processor/throughputmeasurementprocessor v1.3.0
```

These components enable:
- `opampextension` — reports effective config, available components, and health to the supervisor
- `bindplaneextension` — BindPlane-specific measurements, topology reporting, and custom OpAMP messages
- `bearertokenauthextension` — attaches bearer tokens (e.g., Dynatrace API tokens) to outgoing HTTP requests
- `snapshotprocessor` — allows BindPlane to request telemetry snapshots via OpAMP custom messages
- `throughputmeasurementprocessor` — measures pipeline throughput (data sizes, record counts)
- `metricstransformprocessor` — metric renaming/aggregation used by BindPlane's internal health pipeline

> **Important:** The `bindplane-otel-contrib` components are open source (Apache 2.0)
> and maintained by observIQ at https://github.com/observiq/bindplane-otel-contrib.

## Files

| File | Purpose |
|------|---------|
| `dynatrace-collector.yaml` | Single-file K8s manifest (Namespace, RBAC, ConfigMap, Deployment) |
| `collector-config.yaml` | Sample collector pipeline config (OTLP receiver + debug exporter) — reference only |
| `telemetrygen.yaml` | Telemetrygen deployment + Service — generates test traces/metrics/logs |
| `README.md` | This documentation |

## Prerequisites

1. **A Kubernetes cluster** (kind, minikube, EKS, GKE, AKS, etc.)
2. **kubectl** configured for the cluster
3. **A BindPlane Cloud account** — sign up at https://app.bindplane.com
4. **A BindPlane Secret Key** — from BindPlane Cloud Settings
5. **The Dynatrace OTel Collector** rebuilt with the components listed above

## Step 1: Build the Collector

```bash
# From the repo root — regenerate build files after manifest.yaml changes
make generate

# Resolve dependencies
cd build && go mod tidy && cd ..
```

### Cross-compile for linux/amd64 (required when building on Apple Silicon for kind)

```bash
cd build
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o ../bin/dynatrace-otel-collector .
cd ..
```

> `make build` produces a native binary (arm64 on Apple Silicon).
> For kind clusters (linux/amd64 nodes), cross-compile as shown above.

### Build the container image

The collector image uses `alpine` (not `scratch`) because the initContainer
needs `cp` to copy the binary to a shared volume:

```bash
# Build with the Dockerfile in bin/
podman build -t localhost/dynatrace-otel-collector:bindplane-poc -f bin/Dockerfile.poc bin/

# Or with docker:
docker build -t dynatrace-otel-collector:bindplane-poc -f bin/Dockerfile.poc bin/
```

The `Dockerfile.poc` should contain:
```dockerfile
FROM alpine:3.23
COPY --chmod=755 dynatrace-otel-collector /dynatrace-otel-collector
ENTRYPOINT ["/dynatrace-otel-collector"]
CMD ["--config", "/etc/otelcol/config.yaml"]
```

### Load into kind

```bash
# Podman:
podman save localhost/dynatrace-otel-collector:bindplane-poc | kind load image-archive /dev/stdin --name kind

# Docker:
kind load docker-image dynatrace-otel-collector:bindplane-poc --name kind
```

## Step 2: Configure and Deploy

Edit `dynatrace-collector.yaml` — update the supervisor ConfigMap:

```yaml
# In the ConfigMap supervisor.yaml:
server:
  endpoint: wss://app.bindplane.com/v1/opamp
  headers:
    Authorization: "Secret-Key YOUR_SECRET_KEY"   # hardcode the key directly
```

> **Important:** Hardcode the secret key directly in the supervisor config.
> Do NOT use `${OPAMP_SECRET_KEY}` env var substitution — the supervisor image
> may not expand environment variables reliably, leading to 401 errors.

Deploy:

```bash
kubectl apply -f dynatrace-collector.yaml
```

Verify:

```bash
# Pod should be 1/1 Running
kubectl get pods -n bindplane-agent

# Check supervisor connected
kubectl logs -n bindplane-agent deploy/bindplane-cluster-agent -c opamp-supervisor
# Look for: "Connected to the server."
# And: "No config present, not starting agent." (expected — config comes from BindPlane)
```

## Step 3: Configure BindPlane Cloud

This is the critical step. BindPlane has specific requirements for BYOC agents.

### 3a. Create a Configuration

1. In BindPlane UI, go to **Configurations** → **Create Configuration**
2. **Platform**: Select **Kubernetes — Gateway** (not Cluster or Node)
3. **Agent Type**: Must match the collector's `service.name` which is `dynatrace-otel-collector`
   - BindPlane auto-detects this from the agent's OpAMP registration
   - If creating the config before the agent connects, you need to specify this type manually
4. Add your desired sources (e.g., OTLP) and destinations (e.g., debug exporter)

### 3b. Create a Fleet

1. Go to **Fleets** → **Create Fleet** - same parameters as for configuration above^^
2. Assign the configuration to the fleet

### 3c. Assign Agent to Fleet

The upstream OpAMP Supervisor sends labels via `non_identifying_attributes` in the
supervisor config. However, **BindPlane does not auto-assign agents to fleets based
on these attributes** — auto-assignment only works with the native BindPlane agent
which uses the proprietary `OPAMP_LABELS` env var.

**You must manually assign the agent to the fleet in the BindPlane UI:**

1. Go to **Agents** — find the connected agent and assign it to the fleet you created
2. The agent should now appear in the fleet and receive the configuration

### 3d. Roll Out Configuration

Once the agent is in the fleet:

1. Go to the Fleet page
2. Click **Roll out** on the configuration
3. The supervisor will receive the config, start the collector, and report "Everything is ready"

## Step 4: Verify the Collector is Running

The collector runs as a child process of the supervisor — its logs go to `agent.log`,
not to the container's stdout:

```bash
# Supervisor logs (connection status only — 4 lines):
kubectl logs -n bindplane-agent deploy/bindplane-cluster-agent -c opamp-supervisor

# Collector logs (the actual pipeline output):
kubectl exec -n bindplane-agent deploy/bindplane-cluster-agent -c opamp-supervisor \
  -- cat /var/lib/otelcol/supervisor/agent.log

# Look for: "Starting dynatrace-otel-collector..."
# And: "Everything is ready. Begin running and processing data."

# Effective config (the full config the collector is running):
kubectl exec -n bindplane-agent deploy/bindplane-cluster-agent -c opamp-supervisor \
  -- cat /var/lib/otelcol/supervisor/effective.yaml

# Verify both processes are running:
kubectl exec -n bindplane-agent deploy/bindplane-cluster-agent -c opamp-supervisor -- ps aux
```

## Step 5: Deploy Telemetrygen (Test Data)

Once the collector is running with a config that has an OTLP receiver:

```bash
kubectl apply -f telemetrygen.yaml
```

This creates:

- A **Service** (`bindplane-cluster-agent`) exposing ports 4317/4318
- A **Deployment** with 3 containers generating traces, metrics, and logs at 1/sec each

Verify data is flowing:

```bash
# Check collector agent.log for received telemetry
kubectl exec -n bindplane-agent deploy/bindplane-cluster-agent -c opamp-supervisor \
  -- tail -50 /var/lib/otelcol/supervisor/agent.log
```

You can see the flowing telemetry in the agent details page in BindPlane Cloud.

## How It Works

1. **initContainer** copies the Dynatrace collector binary from the alpine image
   to a shared `emptyDir` volume
2. **OpAMP Supervisor** starts, reads its config from the ConfigMap, and connects
   to BindPlane Cloud via WebSocket with `Authorization: Secret-Key <key>` header
3. **Supervisor reports** the collector's available components to BindPlane via
   the `reports_available_components` capability
4. **Supervisor waits** for BindPlane to push a pipeline configuration
5. Once config is received, the supervisor **starts the collector** as a child
   process with that config
6. The collector's **opampextension** reports effective config, available components,
   and health back to the supervisor via a local OpAMP connection
7. **Supervisor relays** everything upstream to BindPlane — the agent appears in
   the UI with its full component list, health status, and effective config
8. When you **update the config** in BindPlane, it pushes the new config via OpAMP,
   the supervisor writes it to disk, and restarts the collector

## Cleanup

```bash
kubectl delete -f telemetrygen.yaml
kubectl delete -f dynatrace-collector.yaml
```
