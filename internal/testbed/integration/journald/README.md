# Journald Receiver Integration Test

This directory contains resources for testing the OpenTelemetry Collector's journald receiver in a Kubernetes environment.

## Overview

The setup deploys an OpenTelemetry Collector configured to read logs from the host's systemd journal and output them via a debug exporter.

## Docker Image

### Image Building Process

The Docker image is built using a **multi-stage build** approach:

#### Stage 1: Builder
```dockerfile
FROM ubuntu:24.04 AS builder
```
- Uses Ubuntu 24.04 as the base
- Installs `curl` to download the collector binary
- Downloads `otelcol-contrib` from GitHub releases
- Extracts the binary to `/usr/local/bin/`

#### Stage 2: Runtime
```dockerfile
FROM ubuntu:24.04
```
- Fresh Ubuntu 24.04 base (without curl)
- Installs `systemd` package
- Copies the collector binary from the builder stage
- Verifies `journalctl` is available

### Why This Image is Minimal

The resulting image (~250-300MB) is already optimized for this use case:

| Component | Size | Why Needed |
|-----------|------|------------|
| Ubuntu 24.04 base | ~77MB | Provides glibc and basic system libraries required by both journalctl and the collector |
| systemd package | ~30-50MB | Contains `journalctl` binary and required libraries (libsystemd0, etc.) |
| otelcol-contrib | ~120-150MB | The collector binary with all contrib components |

### Why systemd Must Be Installed

**The journald receiver requires `journalctl`** to read journal files. Here's why:

1. **Journal File Format**: Systemd journal files are stored in a binary format, not plain text
2. **journalctl Tool**: The only reliable way to read journal files is through `journalctl`, which:
   - Understands the binary journal format
   - Handles journal file rotation
   - Supports filtering and querying
   - Manages journal integrity checking
3. **Container Environment**: Container images don't include systemd by default, so we must install it explicitly

#### Why Not Read Files Directly?

We initially tried using the `filelog` receiver with the `journald` operator to parse journal files directly, but encountered compatibility issues:
```
Journal file uses an unsupported feature, ignoring file
```

The journal file format created by the host's systemd version was incompatible with the version in the container. Using `journalctl` solves this because it's designed to handle different journal versions.

### Alternative: Using Dynatrace OTEL Collector

The Dockerfile includes commented-out instructions for using the Dynatrace OTEL Collector instead:

```dockerfile
# ARG COLLECTOR_VERSION=0.44.0
# RUN curl -L -o /tmp/dynatrace-otel-collector.tar.gz \
#     "https://github.com/dynatrace/dynatrace-otel-collector/releases/download/v${COLLECTOR_VERSION}/dynatrace-otel-collector_${COLLECTOR_VERSION}_Linux_x86_64.tar.gz" && \
#     tar -xzf /tmp/dynatrace-otel-collector.tar.gz -C /usr/local/bin && \
#     rm /tmp/dynatrace-otel-collector.tar.gz
```

Simply comment out the OpenTelemetry Collector section and uncomment this section, then update the `ENTRYPOINT` to use `dynatrace-otel-collector`.

## Kubernetes Deployment

### Volume Mounts Explained

The collector pod requires specific volume mounts to access the host's journal:

```yaml
volumeMounts:
  - name: config
    mountPath: /conf
    # REQUIRED: Contains the collector's configuration (receivers, processors, exporters)
  
  - name: run-journal
    mountPath: /run/log/journal
    readOnly: true
    # REQUIRED: The actual location of journal files on the host
    # In container environments (kind, minikube, etc.), journald stores logs in memory at /run/log/journal
    # not on persistent storage at /var/log/journal
```

### Why These Specific Mounts?

#### `/run/log/journal` - The Journal Files

**Why `/run/log/journal` and not `/var/log/journal`?**

- **Traditional systems**: Store persistent journal at `/var/log/journal`
- **Container environments** (kind, minikube): Store volatile journal in memory at `/run/log/journal`
- Our collector configuration points to `/run/log/journal` because that's where the journal actually exists in kind clusters

The directory structure looks like:
```
/run/log/journal/
└── e6c0b3019dc943369457f85746445c90/  # Machine ID
    └── system.journal                  # Binary journal file
```

#### Config Mount

The `config` volume mount provides the collector's configuration file, which specifies:
- Receiver configuration (journald with directory path)
- Processors (resource attributes, filtering)
- Exporters (debug output)

### Where Do the Logs Come From?

The collector reads logs from **the host's systemd journal**, which includes:

```
┌─────────────────────────────────────────┐
│           Kubernetes Node               │
│  ┌─────────────────────────────────┐   │
│  │     systemd (journald)          │   │
│  │                                 │   │
│  │  Collects logs from:            │   │
│  │  • kubelet.service              │   │
│  │  • containerd.service           │   │
│  │  • docker.service               │   │
│  │  • System services              │   │
│  │  • Container logs (stdout)      │   │
│  │                                 │   │
│  │  Writes to:                     │   │
│  │  /run/log/journal/              │   │
│  └─────────────────────────────────┘   │
│            ↓ (volume mount)             │
│  ┌─────────────────────────────────┐   │
│  │  OTEL Collector Pod             │   │
│  │                                 │   │
│  │  journald receiver              │   │
│  │    ↓ (via journalctl)           │   │
│  │  /run/log/journal/ (mounted)    │   │
│  │    ↓                             │   │
│  │  debug exporter                 │   │
│  │                                 │   │
│  └─────────────────────────────────┘   │
└─────────────────────────────────────────┘
```

**Log sources include:**
- **kubelet**: Kubernetes node agent operations
- **containerd/docker**: Container runtime events
- **System services**: Any systemd service running on the host
- **Container output**: Containers writing to stdout/stderr (forwarded to journal)

### Configuration Details

The collector is configured to read all journal entries:

```yaml
receivers:
  journald:
    directory: /run/log/journal  # Where to read journal files
    priority: info               # Minimum log level
    start_at: beginning          # Read from start (for testing)
```

**No unit filtering** means the collector captures logs from ALL systemd units. To filter specific services:

```yaml
receivers:
  journald:
    directory: /run/log/journal
    units:
      - kubelet.service
      - containerd.service
```

## Deployment Instructions

### 1. Build and Load Image

```bash
./build-and-load.sh
```

This script:
1. Builds the Docker image with journald support
2. Loads it into the kind cluster

### 2. Deploy to Kubernetes

```bash
kubectl apply -f collector.yaml
```

This creates:
- Namespace: `otel-journald-test`
- Deployment: Single replica of the collector
- ServiceAccount and RBAC
- ConfigMap: Collector configuration

### 3. View Logs

```bash
# Watch collector output (shows journal logs via debug exporter)
kubectl logs -n otel-journald-test -l app=otel-collector -f

# Check collector status
kubectl get pods -n otel-journald-test
```

## Test Resources

### failing-pod.yaml

A simple pod that continuously fails and restarts, generating logs in the journal:

```bash
kubectl apply -f failing-pod.yaml
```

This pod will:
- Print log messages
- Exit with code 1
- Enter CrashLoopBackOff
- Generate kubelet logs visible in the collector output

## Architecture Decisions

### Why Deployment Instead of DaemonSet?

Initially configured as a DaemonSet (one pod per node), we changed to a Deployment because:
- **Simpler for testing**: Single pod is easier to debug
- **journald is node-scoped**: Each node has its own journal
- **For production**: Use DaemonSet to collect logs from all nodes

To switch back to DaemonSet:
```yaml
apiVersion: apps/v1
kind: DaemonSet  # Change from Deployment
metadata:
  name: otel-collector
spec:
  # Remove replicas field
  selector:
    matchLabels:
      app: otel-collector
```

### Why hostPID?

```yaml
hostPID: true
```

This allows the collector to access the host's PID namespace, which can be useful for:
- Correlating journal entries with process IDs
- Accessing host-level system information

### Why privileged mode?

```yaml
securityContext:
  privileged: true
  runAsUser: 0
```

Required to:
- Mount and read from `/run/log/journal` (root-owned)
- Execute `journalctl` which needs specific capabilities
- Access system-level resources

## Security Considerations

### ⚠️ Important Security Notes

The journald receiver configuration requires **elevated privileges** that create security risks. Understand these implications before deploying to production:

**Quick Summary:**
- **Privileged mode** grants near-root host access - use specific capabilities (CAP_DAC_READ_SEARCH, SYS_PTRACE) instead
- **Sensitive data** in logs (credentials, tokens) - filter/redact using processors and limit units collected
- **Resource consumption** - set memory/CPU limits, use `start_at: end` in production
- **Network security** - enable TLS for exporters, implement network policies
- **Compliance** - review GDPR/PCI/HIPAA requirements before production deployment

#### 1. Privileged Container Access

```yaml
securityContext:
  privileged: true
  runAsUser: 0
```

**Risk**: Privileged containers have nearly unrestricted access to the host system.

**Implications**:
- Can access all host devices
- Can load kernel modules
- Can modify host system files
- If compromised, attacker gains root access to the node

**Mitigation**:
- Use only in trusted, isolated environments
- Consider using specific capabilities instead of `privileged: true`:
  ```yaml
  securityContext:
    runAsUser: 0
    capabilities:
      add:
        - DAC_READ_SEARCH  # Read any file
        - SYS_PTRACE       # For journalctl
  ```
- Implement Pod Security Standards/Policies to restrict privileged pods

#### 2. Sensitive Data Exposure

**Risk**: System journal contains sensitive information:
- Authentication attempts and credentials
- API keys and tokens from application logs
- Internal service communications
- System configuration details
- Container environment variables
- User activity and commands

**Mitigation**:
- **Filter logs**: Use processors to exclude sensitive fields
  ```yaml
  processors:
    attributes:
      actions:
        - action: delete
          key: password
        - action: delete
          pattern: ".*secret.*"
  ```
- **Redact patterns**: Use regex to mask sensitive data
- **Limit units**: Only collect logs from specific services
  ```yaml
  receivers:
    journald:
      units:
        - kubelet.service
        - containerd.service
  ```

#### 3. Host Access via Volume Mounts

**Risk**: Direct access to host filesystem paths:
- `/run/log/journal` - Contains all system logs
- If misconfigured, could expose additional host paths

**Mitigation**:
- Mount volumes as `readOnly: true` (already implemented)
- Use minimal, specific mount paths
- Avoid mounting entire `/var` or `/run` directories

#### 4. Network Exposure

**Risk**: If exporter sends logs to external systems:
- Logs transmitted over network may be intercepted
- Destination systems may be compromised

**Mitigation**:
- Use TLS/mTLS for exporters:
  ```yaml
  exporters:
    otlp:
      endpoint: collector.example.com:4317
      tls:
        insecure: false
        cert_file: /etc/certs/client.crt
        key_file: /etc/certs/client.key
  ```
- Implement network policies to restrict collector egress
- Use secure authentication for exporters

#### 5. Denial of Service

**Risk**: Reading unbounded logs can consume resources:
- High CPU usage from journalctl processing
- Memory exhaustion from buffering
- Network saturation from excessive log export

**Mitigation**:
- Set resource limits:
  ```yaml
  resources:
    limits:
      memory: 512Mi
      cpu: 500m
    requests:
      memory: 128Mi
      cpu: 100m
  ```
- Use `start_at: end` to avoid reading entire journal history
- Implement rate limiting in processors
- Filter verbose log levels

#### 6. Compliance and Audit

**Risk**: Log collection may violate:
- GDPR (personal data in logs)
- PCI DSS (payment card data)
- HIPAA (health information)
- Internal data policies

**Mitigation**:
- Document what data is collected and where it goes
- Implement data retention policies
- Add audit logs for collector access
- Ensure proper data classification and handling
- Get legal/compliance approval before production deployment

### Production Security Checklist

Before deploying to production, ensure:

- [ ] Pod Security Policy/Standards restrict privileged pods
- [ ] Network policies limit collector ingress/egress
- [ ] TLS enabled for all exporters
- [ ] Authentication configured for backend systems
- [ ] Sensitive data filtered/redacted
- [ ] Resource limits set appropriately
- [ ] RBAC permissions follow least-privilege principle
- [ ] Audit logging enabled
- [ ] Compliance requirements reviewed
- [ ] Incident response plan includes collector compromise scenario

### Recommendations by Environment

#### Development/Testing
- Current configuration is acceptable
- Focus on functionality over security hardening

#### Staging
- Remove `privileged: true`, use specific capabilities
- Enable TLS for exporters
- Implement basic filtering

#### Production
- **Full security hardening required**
- Consider alternative solutions like:
  - Cluster-level log aggregation (Fluentd, Fluent Bit)
  - Vendor-provided logging solutions
  - Sidecar pattern with limited scope
- If using journald receiver, implement all mitigations above
- Regular security audits and penetration testing

## Troubleshooting

### No logs appearing

1. **Check journal directory exists:**
   ```bash
   kubectl exec -n otel-journald-test <pod-name> -- ls -la /run/log/journal
   ```

2. **Verify journalctl works:**
   ```bash
   kubectl exec -n otel-journald-test <pod-name> -- journalctl -n 10
   ```

3. **Check collector logs for errors:**
   ```bash
   kubectl logs -n otel-journald-test <pod-name>
   ```

### Journal file compatibility errors

If you see:
```
Journal file uses an unsupported feature, ignoring file
```

This means the systemd version mismatch. Solution: Use Ubuntu 24.04 (newer systemd 255) instead of 22.04.

### Empty journal directory

If `/var/log/journal` is empty, the system is using `/run/log/journal` (volatile storage). Update the receiver configuration:

```yaml
directory: /run/log/journal  # Not /var/log/journal
```

## Further Optimization

To reduce image size further:
- Use `debian:12-slim` instead of Ubuntu (saves ~3MB)
- Extract only required systemd libraries (complex, minimal gain)
- Use distroless base with manual library copying (significant effort)

Current approach balances simplicity, maintainability, and size optimization.

## Documentation Enhancements

Detailed analysis of upstream documentation gaps and recommended improvements for the journald receiver can be found in [ENHANCEMENTS-DOCS.md](ENHANCEMENTS-DOCS.md).
