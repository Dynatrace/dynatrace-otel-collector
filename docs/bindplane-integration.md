# BindPlane Integration Guide — Dynatrace OTel Collector

## 1. Overview

This guide explains how to integrate the **Dynatrace OTel Collector** with **BindPlane** using the **Bring Your Own Collector (BYOC)** feature. BindPlane is an OpenTelemetry-native telemetry management platform that uses the **OpAMP** (Open Agent Management Protocol) to manage collector fleets.

### What does this enable?

- **Remote configuration management** — push collector configs from the BindPlane UI/API
- **Fleet management** — manage hundreds of Dynatrace collectors at scale
- **Component-aware config building** — BindPlane knows exactly which receivers/processors/exporters are available in the Dynatrace distribution and restricts configuration to valid components
- **Health monitoring** — collector health and status visible in BindPlane
- **Version syncing** — BindPlane can track releases from the GitHub repository

### What's NOT changing?

- The collector binary itself remains the same (plus one extension)
- All existing config examples, receivers, processors, exporters continue to work
- The `eec:` confmap provider is unaffected — it's an alternative management path
- No upstream components are removed

---

## 2. Architecture

The integration uses three components:

```
┌──────────────────────────┐
│     BindPlane Server     │◄─── UI (port 3001)
│  (OpAMP server)          │◄─── API
└─────────┬────────────────┘
          │ OpAMP (WebSocket)
          ▼
┌──────────────────────────┐
│    OpAMP Supervisor      │  ← separate process (sidecar in K8s)
│  - lifecycle management  │
│  - config relay          │
│  - health reporting      │
└─────────┬────────────────┘
          │ starts/configures
          ▼
┌──────────────────────────┐
│  Dynatrace OTel          │  ← your collector binary
│  Collector               │
│  (with opampextension)   │
└──────────────────────────┘
```

### Component Roles

| Component | What it does | Where it runs |
|-----------|-------------|---------------|
| **BindPlane Server** | OpAMP server, config management, fleet UI | Separate deployment (Helm/Docker/VM) |
| **OpAMP Supervisor** | Manages collector process, relays OpAMP messages | Sidecar container (K8s) or separate service (VM) |
| **OpAMP Extension** | In-process OpAMP agent, reports config/health/components | Compiled into the collector binary |
| **Dynatrace Collector** | Collects and exports telemetry | Your workload |

---

## 3. Changes Required in the Collector

### 3.1 Add `opampextension` to the build

One line added to `manifest.yaml` under `extensions:`:

```yaml
extensions:
  # ... existing extensions ...
  - gomod: github.com/open-telemetry/opentelemetry-collector-contrib/extension/opampextension v0.149.0
```

This is already done in this branch. The extension is compiled into the binary at build time by OCB.

### 3.2 Rebuild

```bash
make generate   # regenerates build/components.go with opampextension
make build       # compiles the binary
```

Or for Docker:
```bash
docker build -t dynatrace-otel-collector:bindplane-poc .
```

### 3.3 No runtime configuration required in collector config

The OpAMP extension is **not** configured in the collector's own YAML config. Instead, the **OpAMP Supervisor** injects the extension configuration automatically when it starts the collector. The supervisor templates an effective config that includes the opamp extension pointed at itself.

---

## 4. Deployment — Kubernetes

### Architecture in K8s

```
Pod (DaemonSet or Deployment)
├── initContainer: copy-collector
│   └── Copies collector binary to shared emptyDir volume
├── container: opamp-supervisor
│   ├── Mounts supervisor config (ConfigMap)
│   ├── Mounts shared volume with collector binary
│   ├── Connects to BindPlane via OPAMP_ENDPOINT env var
│   └── Starts/manages the collector as a child process
└── volumes:
    ├── supervisor-config (ConfigMap)
    ├── supervisor-storage (emptyDir — persistent state)
    └── collector-binary (emptyDir — shared binary)
```

### Manifests

All manifests are in [`deploy/bindplane-poc/collector/`](../deploy/bindplane-poc/collector/):

| File | Purpose |
|------|---------|
| `namespace.yaml` | Creates `dynatrace-collector` namespace |
| `serviceaccount.yaml` | ServiceAccount for the pods |
| `configmap.yaml` | Supervisor configuration |
| `daemonset.yaml` | DaemonSet with supervisor + collector |

### Key configuration points

**1. `OPAMP_ENDPOINT` in `daemonset.yaml`** — must point to your BindPlane server:
```yaml
- name: OPAMP_ENDPOINT
  value: "ws://bindplane.bindplane.svc.cluster.local:3001/v1/opamp"
```

**2. Collector image in `daemonset.yaml`** — the initContainer image must be your rebuilt collector:
```yaml
- name: copy-collector
  image: dynatrace-otel-collector:bindplane-poc  # ← change this
```

**3. Supervisor image version** — pinned to match the collector's upstream version:
```yaml
image: ghcr.io/open-telemetry/opentelemetry-collector-releases/opentelemetry-collector-opampsupervisor:0.149.0
```

---

## 5. Deployment — BindPlane Server

```bash
helm repo add bindplane https://observiq.github.io/bindplane-op-helm
helm repo update

helm upgrade --install bindplane bindplane/bindplane \
  --namespace=bindplane --create-namespace \
  --set config.license='YOUR_KEY' \
  --set config.username='admin' \
  --set config.password='admin' \
  --set config.sessions_secret="$(uuidgen)"
```

### Verify

```bash
kubectl -n bindplane port-forward service/bindplane 3001:3001
# Open http://localhost:3001
```

---

## 6. Agent Type Registration

When your collector first connects to BindPlane via OpAMP, BindPlane will auto-create a basic Agent Type. For full functionality (version syncing, install scripts), register via the API:

```bash
bindplane apply -f deploy/bindplane-poc/agent-type.yaml
```

See [`agent-type.yaml`](../deploy/bindplane-poc/agent-type.yaml) for the definition.

---

## 7. How Configuration Flows

1. **You create a configuration** in BindPlane UI/API, selecting sources, processors, and destinations from the components available in the Dynatrace distribution
2. **BindPlane pushes the config** to the OpAMP Supervisor via WebSocket
3. **The Supervisor writes the config** to a local file and starts/restarts the collector with it
4. **The collector reports back** its effective config and health via the OpAMP extension
5. **BindPlane shows status** in the Collectors view

### Config ownership

When using BindPlane, **BindPlane owns the collector configuration**. The collector does not read from a local file or the EEC provider — the supervisor provides the config. This means:

- Do **not** also pass `--config` flags to the collector manually
- The `eec:` provider is not used in this mode
- Config changes should be made in BindPlane, not locally

---

## 8. Available Components Reporting

With `reports_available_components: true` in the supervisor config, BindPlane will know exactly which components your collector was built with. This is used to:

- Restrict the configuration builder to valid components
- Show which receivers/processors/exporters are available
- Prevent invalid configurations

The OpAMP extension reads the component list from the collector's built-in component registry at startup.

---

## 9. Risks and Considerations

| Risk | Mitigation |
|------|-----------|
| **Invalid config from BindPlane could crash the collector** | Use BindPlane's config validation. The supervisor will report failure status. The collector's `validate` subcommand can be used. |
| **Network partition from BindPlane** | Supervisor keeps last-known-good config locally. Collector continues running. |
| **Conflict with EEC provider** | In BindPlane mode, don't use `eec:` configs. They are separate management paths. |
| **Supervisor is an additional moving part** | It's a lightweight process (~30MB). Well-tested upstream component. |
| **BindPlane Enterprise license required for BYOC** | Free tier exists but with limitations. Enterprise needed for full BYOC features. |
| **OpAMP Supervisor is alpha stability** | Pin versions. Test thoroughly. Monitor upstream issues. |

---

## 10. Testing the PoC

### Quick smoke test

```bash
# 1. Deploy BindPlane
helm upgrade --install bindplane bindplane/bindplane \
  --namespace=bindplane --create-namespace \
  --set config.license='YOUR_KEY' \
  --set config.username='admin' \
  --set config.password='admin' \
  --set config.sessions_secret="$(uuidgen)"

# 2. Wait for BindPlane to be ready
kubectl -n bindplane rollout status statefulset/bindplane

# 3. Deploy the collector with supervisor
kubectl apply -f deploy/bindplane-poc/collector/

# 4. Port-forward to BindPlane UI
kubectl -n bindplane port-forward service/bindplane 3001:3001 &

# 5. Open http://localhost:3001 → Collectors tab
# You should see the collector connected and reporting components.
```

### Verify OpAMP connection

```bash
# Supervisor logs should show successful connection
kubectl -n dynatrace-collector logs -l app.kubernetes.io/name=dynatrace-collector -c opamp-supervisor

# Look for:
#   "Connected to server"
#   "Effective config reported"
#   "Health reported"
```

---

## 11. Next Steps

1. **Get a BindPlane license** at https://bindplane.com/download
2. **Deploy BindPlane** to your K8s cluster
3. **Build the collector** with `opampextension` included
4. **Deploy the collector** with the supervisor sidecar
5. **Create a configuration** in BindPlane and push it to your collector
6. **Evaluate** whether BindPlane fleet management adds value for your use case
