# BindPlane + Dynatrace OTel Collector PoC

This directory contains everything needed to run a proof-of-concept integrating the **Dynatrace OTel Collector** with **BindPlane** using the **Bring Your Own Collector (BYOC)** feature and **OpAMP** protocol.

## Architecture Overview

```
┌─────────────────────────────────────────┐
│           BindPlane Server              │
│  (OpAMP server, UI on :3001)            │
│  ws://<bindplane>:3001/v1/opamp         │
└──────────────┬──────────────────────────┘
               │ OpAMP (WebSocket)
               ▼
┌─────────────────────────────────────────┐
│         OpAMP Supervisor                │
│  (sidecar container in K8s pod)         │
│  - manages collector lifecycle          │
│  - relays config from BindPlane         │
│  - reports health/status back           │
└──────────────┬──────────────────────────┘
               │ starts/stops/configures
               ▼
┌─────────────────────────────────────────┐
│     Dynatrace OTel Collector            │
│  (with opampextension compiled in)      │
│  - receives config from supervisor      │
│  - reports effective config             │
│  - reports available components         │
└─────────────────────────────────────────┘
```

## Prerequisites

1. **A Kubernetes cluster** (kind, minikube, EKS, GKE, AKS, etc.)
2. **kubectl** configured for the cluster
3. **Helm** (for BindPlane server deployment)
4. **A BindPlane license key** — get a free one at https://bindplane.com/download
5. **The Dynatrace OTel Collector image** rebuilt with `opampextension` (see below)

## Step 0: Rebuild the Collector with OpAMP Extension

The `opampextension` has been added to [`manifest.yaml`](../../manifest.yaml). You need to regenerate and rebuild.

### Build the collector binary

```bash
# From the repo root
make generate
make build
```

> **NOTE:** On Apple Silicon (M1/M2), `make build` produces an `arm64` binary that
> cannot run in a `kind` cluster (which uses `linux/amd64` nodes).
> Use `make build-all` instead, then copy the correct binary:
> ```bash
> make build-all
> cp dist/dynatrace-otel-collector_linux_amd64_v1/dynatrace-otel-collector bin/dynatrace-otel-collector
> ```

### Build the container image and load it into `kind`

From the `bin` directory, build the image and load it into your `kind` cluster.

**With `docker`:**
```bash
cd bin
docker buildx build -t dynatrace-otel-collector:bindplane-poc -f ../Dockerfile . --load
kind load docker-image dynatrace-otel-collector:bindplane-poc --name kind
cd ..
```

**With `podman`:**
```bash
cd bin
podman buildx build -t dynatrace-otel-collector:bindplane-poc -f ../Dockerfile . --load
podman save -o dynatrace-otel-collector.tar dynatrace-otel-collector:bindplane-poc
kind load image-archive dynatrace-otel-collector.tar --name kind
cd ..
```

> If using `podman`, set `CONTAINER_REGISTRY=localhost/` in subsequent steps,
> as the image will be prefixed with `localhost/` in the `kind` registry.

> **Important**: The collector image used in the K8s manifests defaults to
> `dynatrace-otel-collector:bindplane-poc`. Update the image reference in
> `collector/daemonset.yaml` if you used a different tag or a remote registry.

## Step 1: Deploy BindPlane Server

```bash
helm repo add bindplane https://observiq.github.io/bindplane-op-helm
helm repo update

cat > bindplane-values.yaml <<EOF
config:
  license: 'YOUR_LICENSE_KEY'
  username: 'admin'
  password: 'admin'
  sessions_secret: '$(uuidgen)'

backend:
  type: bbolt
  bbolt:
    volumeSize: '10Gi'
EOF

helm upgrade --install bindplane bindplane/bindplane \
  --namespace=bindplane \
  --create-namespace \
  --values=bindplane-values.yaml
```

### Verify

```bash
# Port-forward to access the UI
kubectl -n bindplane port-forward service/bindplane 3001:3001

# Open http://localhost:3001 — login with admin/admin
```

## Step 2: Deploy the Collector with OpAMP Supervisor

```bash
# Create the namespace
kubectl create namespace dynatrace-collector

# Update the OPAMP_ENDPOINT in collector/daemonset.yaml to point at your BindPlane server.
# For in-cluster BindPlane deployed via Helm:
#   ws://bindplane.bindplane.svc.cluster.local:3001/v1/opamp
# For external BindPlane:
#   ws://<BINDPLANE_HOST>:3001/v1/opamp

# Apply all manifests
kubectl apply -f collector/ -n dynatrace-collector
```

## Step 3: Verify in BindPlane

1. Open the BindPlane UI
2. Navigate to **Collectors** — you should see a new collector appear with type auto-detected from OpAMP
3. BindPlane will recognize the available components compiled into the Dynatrace collector

## Step 4 (Optional): Register Agent Type via API

For full BYOC functionality (version syncing, install scripts), create an Agent Type:

```bash
# Install bindplane CLI or use kubectl exec into the bindplane pod
bindplane apply -f agent-type.yaml
```

See [`agent-type.yaml`](agent-type.yaml) for the definition.

## How It Works

### OpAMP Extension (compiled into collector)
- Reports effective config to supervisor
- Reports available components (receivers, processors, exporters, extensions)
- Reports health status
- Accepts restart commands

### OpAMP Supervisor (sidecar container)
- Connects to BindPlane via WebSocket (OpAMP)
- Manages the collector process lifecycle
- Passes remote configuration from BindPlane to the collector
- Reports collector status back to BindPlane

### BindPlane
- Recognizes the custom distribution and its components
- Builds configurations using only components available in the distribution
- Pushes configuration changes to collectors via OpAMP
- Monitors collector health and status

## Key Differences from Standard Dynatrace OTel Collector Deployment

| Aspect | Standard | With BindPlane |
|--------|----------|----------------|
| Config source | File / EEC provider | BindPlane (via OpAMP) |
| Lifecycle management | systemd / K8s | OpAMP Supervisor |
| Remote config | EEC only | BindPlane UI/API |
| Fleet management | Not built-in | BindPlane fleets |
| Component visibility | Manual | Auto-reported via OpAMP |

## Troubleshooting

```bash
# Check supervisor logs
kubectl -n dynatrace-collector logs <pod-name> -c opamp-supervisor

# Check collector logs
kubectl -n dynatrace-collector logs <pod-name> -c collector

# Verify OpAMP connectivity
kubectl -n dynatrace-collector exec <pod-name> -c opamp-supervisor -- cat /etc/supervisor/supervisor.yaml
```

## Cleanup

```bash
kubectl delete namespace dynatrace-collector
kubectl delete namespace bindplane  # if deployed via Helm
```
