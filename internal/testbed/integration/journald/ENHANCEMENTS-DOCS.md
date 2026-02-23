# Upstream Documentation Gap Analysis

## Overview

The [official OpenTelemetry journald receiver documentation](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/journaldreceiver) provides basic configuration options but lacks crucial details for real-world Kubernetes deployments. This section identifies gaps and provides recommendations for improvement.

## Critical Missing Information

### 1. Container Environment Differences

**What's Missing:**
- No explanation of `/run/log/journal` vs `/var/log/journal` in containerized environments
- No mention that container distributions (kind, minikube, k3s) use volatile journal storage
- No guidance on journal location discovery

**Impact:** Users waste hours debugging "empty directory" errors when mounting `/var/log/journal` which is empty in most container environments.

**Recommended Addition:**
```markdown
### Journal Location in Container Environments

**Traditional Systems:**
- Persistent journal: `/var/log/journal/`
- Requires Storage=persistent in journald.conf

**Container Platforms (kind, minikube, k3s):**
- Volatile journal: `/run/log/journal/`
- Default Storage=volatile or Storage=auto with missing /var/log/journal

**Discovery Command:**
journalctl --header | grep "File path"
# Or check both locations:
ls -la /var/log/journal /run/log/journal
```

### 2. journalctl Dependency Not Documented

**What's Missing:**
- No clear statement that `journalctl` binary is **required**
- No explanation of why direct file reading doesn't work
- No guidance on systemd version compatibility

**Impact:** Users attempt to use minimal base images (alpine, distroless) and hit "executable not found" errors. They try to mount host journalctl binary and face library dependency issues.

**Recommended Addition:**
```markdown
### journalctl Requirement

The journald receiver **requires the journalctl binary** in the container.

**Why journalctl is required:**
1. Journal files use a binary format that changes between systemd versions
2. Direct file parsing requires exact systemd version matching between host and container
3. journalctl handles format compatibility, rotation, integrity checking

**Container Image Requirements:**
- Install systemd package (contains journalctl)
- Recommended minimum: systemd 245+
- Use Ubuntu 24.04 (systemd 255) or Debian 12 (systemd 252)

**Common Errors:**
- "journalctl: executable file not found" → systemd not installed
- "Journal file uses unsupported feature" → systemd version too old
```

### 3. Kubernetes Security Requirements Unclear

**What's Missing:**
- No explicit documentation of required permissions
- No explanation of why privileged mode is typically needed
- No guidance on capability-based alternatives
- No security implications discussed

**Impact:** Users deploy with insufficient permissions and get cryptic "permission denied" errors. Or they use privileged mode without understanding the security risks.

**Recommended Addition:**
```markdown
### Kubernetes Permission Requirements

**Minimum Requirements:**
- Run as root (runAsUser: 0) - journal files are root-owned
- Read access to /run/log/journal (root permission level)
- Capability requirements:
  - CAP_DAC_READ_SEARCH: Read any file regardless of permissions
  - CAP_SYS_PTRACE: Required by journalctl for some operations

**Option 1: Privileged Mode (Easiest, Least Secure)**
```yaml
securityContext:
  privileged: true
  runAsUser: 0
```

**Option 2: Specific Capabilities (Recommended)**
```yaml
securityContext:
  runAsUser: 0
  capabilities:
    add:
      - DAC_READ_SEARCH
      - SYS_PTRACE
```

**Option 3: Security Context Constraints (OpenShift)**
Create custom SCC with required capabilities and bind to service account.
```

### 4. Volume Mount Patterns Not Included

**What's Missing:**
- No example Kubernetes manifests
- No guidance on volume mount configuration
- No discussion of read-only vs read-write mounts

**Impact:** Users experiment with various mount patterns, often mounting entire `/var` or `/run` directories unnecessarily.

**Recommended Addition:**
```markdown
### Kubernetes Volume Mount Configuration

**Minimal Required Mount:**
```yaml
volumeMounts:
  - name: run-journal
    mountPath: /run/log/journal
    readOnly: true

volumes:
  - name: run-journal
    hostPath:
      path: /run/log/journal
      # Note: Do not specify type: Directory
      # Kind clusters use symlinks which fail strict type checking
```

**What NOT to mount:**
- ❌ Entire `/run` directory - overly permissive
- ❌ Entire `/var` directory - unnecessary access
- ❌ `/etc/machine-id` - not required if directory is explicit
- ❌ Host `/usr/bin/journalctl` - library dependency nightmare
```

### 5. Performance and Resource Impacts Undocumented

**What's Missing:**
- No guidance on resource consumption
- No discussion of start_at: beginning vs end
- No mention of CPU/memory implications of unfiltered collection

**Impact:** Users deploy with `start_at: beginning` in production, causing massive CPU spikes as the collector processes entire journal history.

**Recommended Addition:**
```markdown
### Performance Considerations

**start_at Setting:**
- `beginning`: Reads entire journal history (potentially GBs of logs)
  - Use only for testing or initial backfill
  - Can cause 10-60 second startup delay
  - High CPU usage during catchup
- `end`: Reads only new entries after collector start
  - Recommended for production
  - Minimal startup impact

**Resource Guidelines:**
```yaml
# Testing/Development
resources:
  requests:
    memory: 128Mi
    cpu: 100m
  limits:
    memory: 512Mi
    cpu: 500m

# Production (unfiltered)
resources:
  requests:
    memory: 256Mi
    cpu: 200m
  limits:
    memory: 1Gi
    cpu: 1000m
```

**Unit Filtering Impact:**
- Unfiltered: Reads all systemd units (kubelet, containerd, every pod)
  - 100-1000+ entries/second in busy clusters
- Filtered: Reads only specified units
  - 10-100 entries/second typically
```

### 6. No DaemonSet vs Deployment Guidance

**What's Missing:**
- No architecture pattern recommendations
- No discussion of when to use DaemonSet vs single Deployment

**Impact:** Users deploy single Deployment in multi-node clusters and wonder why they only see logs from one node.

**Recommended Addition:**
```markdown
### Deployment Architecture

**DaemonSet Pattern (Recommended for Production):**
- Deploys one collector pod per node
- Each pod reads that node's journal
- Required for complete cluster-wide log collection
- journald is node-scoped, not cluster-scoped

```yaml
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: otel-collector
spec:
  selector:
    matchLabels:
      app: otel-collector
  template:
    # ... pod spec
```

**Deployment Pattern (Development/Testing):**
- Single pod
- Only collects logs from one node
- Simpler for debugging
- Not suitable for multi-node production

**When to Use Each:**
- **Single-node clusters** (kind, minikube, Docker Desktop): Deployment is sufficient
- **Multi-node clusters** (production): DaemonSet required
- **Testing/debugging**: Start with Deployment, move to DaemonSet
```

### 7. Common Errors and Troubleshooting Missing

**What's Missing:**
- No troubleshooting section
- No common error messages documented
- No debugging commands provided

**Impact:** Users encounter errors and have no starting point for diagnosis.

**Recommended Addition:**
```markdown
### Troubleshooting Guide

| Error Message | Cause | Solution |
|---------------|-------|----------|
| `journalctl: executable file not found` | systemd not installed in container | Install systemd package |
| `Journal file uses an unsupported feature` | systemd version mismatch | Use Ubuntu 24.04 or Debian 12 |
| `permission denied` reading journal | Insufficient permissions | Add CAP_DAC_READ_SEARCH capability |
| `directory "/var/log/journal" is empty` | Wrong journal path | Change to `/run/log/journal` |
| No logs appearing | Various | See diagnostic commands below |

**Diagnostic Commands:**
```bash
# 1. Check journal location
kubectl exec -it <pod> -- sh -c "ls -la /var/log/journal /run/log/journal"

# 2. Verify journalctl works in container
kubectl exec -it <pod> -- journalctl -n 20

# 3. Check journalctl version
kubectl exec -it <pod> -- journalctl --version

# 4. Test receiver configuration manually
kubectl exec -it <pod> -- journalctl -D /run/log/journal --priority=info -n 10

# 5. Check collector logs
kubectl logs <pod> | grep -i "journald\|error"
```
```

### 8. No Discussion of Alternatives

**What's Missing:**
- No comparison with other log collection approaches
- No guidance on when to use journald receiver vs alternatives

**Impact:** Users implement journald receiver when fluentd/fluent-bit would be simpler and more secure.

**Recommended Addition:**
```markdown
### When to Use journald Receiver

**Good Use Cases:**
- Collecting systemd service logs specifically
- Need for systemd metadata enrichment (_SYSTEMD_UNIT, _PID, etc.)
- Already using OTel Collector for metrics/traces
- Want unified collection architecture

**Consider Alternatives:**
- **Container logs only**: Use filelogreceiver with /var/log/pods
- **General cluster logging**: Fluentd/Fluent Bit with less privilege
- **Cloud environments**: Native cloud logging (CloudWatch, Stackdriver)
- **Security-sensitive**: Sidecar pattern over node-level collection

**Comparison:**
| Aspect | journald receiver | Fluent Bit | filelogreceiver |
|--------|------------------|------------|-----------------|
| Privileges | Requires root + capabilities | Configurable | Typically less |
| Scope | System-wide | System-wide or pod | Per-directory |
| Metadata | Rich systemd metadata | Kubernetes metadata | Basic file metadata |
| Complexity | Moderate (needs journalctl) | Low | Low |
```

## Summary of Recommended Improvements

The upstream documentation should add:

1. **Container-specific section** explaining journal storage differences
2. **Prerequisite checklist** (journalctl, systemd version, permissions)
3. **Complete Kubernetes example manifests** (DaemonSet + RBAC)
4. **Security considerations** with capability examples
5. **Performance best practices** (start_at, resource limits, filtering)
6. **Comprehensive troubleshooting** section
7. **Architecture decision guide** (DaemonSet vs Deployment)
8. **Comparison with alternatives** to help users choose

## Contributing Back

These improvements should be contributed to the upstream project:
- Repository: https://github.com/open-telemetry/opentelemetry-collector-contrib
- File: `receiver/journaldreceiver/README.md`
- Consider opening an issue first to discuss scope
- Reference real-world deployment challenges from this testing setup
