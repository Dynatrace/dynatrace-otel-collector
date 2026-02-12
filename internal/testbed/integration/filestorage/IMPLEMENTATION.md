# FilesStorage Extension Integration Tests - Implementation Summary

## Overview
Comprehensive integration tests have been created for the `filestorage` extension, validating its functionality with both persistent queues and filelog receiver checkpointing, as well as secure deployment scenarios.

## Files Created

### Test Files
1. **`internal/testbed/integration/filestorage/filestorage_test.go`**
   - Unit-style integration tests running against compiled collector
   - Tests persistent queue with exporter
   - Tests filelog receiver checkpointing
   - Tests secure directory permissions

2. **`internal/testbed/integration/filestorage/e2e_test.go`**
   - Kubernetes E2E tests (requires `e2e` build tag)
   - Tests persistent queue in K8s deployment
   - Tests filelog receiver as DaemonSet
   - Tests secure deployment with security context

3. **`internal/testbed/integration/filestorage/README.md`**
   - Complete documentation of test coverage
   - Instructions for running tests
   - Test scenarios and requirements

### Kubernetes Manifests

#### Namespaces
- `testdata/namespace.yaml` - For persistent queue tests
- `testdata/namespace-receiver.yaml` - For filelog receiver tests
- `testdata/namespace-secure.yaml` - For secure storage tests

#### Collector Deployments
**Standard Deployment** (`testdata/collector/`):
- `deployment.yaml` - Collector deployment with filestorage volume
- `configmap.yaml` - Collector configuration
- `service.yaml` - Service for OTLP endpoints

**Receiver DaemonSet** (`testdata/collector-receiver/`):
- `daemonset.yaml` - DaemonSet with log file volume mounts
- `configmap.yaml` - Configuration for filelog receiver

**Secure Deployment** (`testdata/collector-secure/`):
- `deployment.yaml` - Deployment with security context (non-root, fsGroup)
- `configmap.yaml` - Configuration
- `service.yaml` - Service

#### Test Workloads (`testdata/testobjects/`)
- `log-generator.yaml` - Telemetrygen deployment for generating OTLP logs
- `log-writer.yaml` - Busybox deployment writing logs to files

#### Configuration Overlays (`testdata/config-overlays/`)
- `receiver-logpath.yaml` - Log file paths for filelog receiver
- `secure-path.yaml` - Secure storage path template

### Configuration Files (Updated)
- **`config_examples/filestorage-exporter.yaml`** - Added missing `receivers` section
- **`config_examples/filestorage-receiver.yaml`** - Added missing `exporters` section

## Test Coverage

### ✅ Requirement 1: Secure Write Location
**Verified by:**
- `TestFileStorageSecureLocation` - Tests restricted permissions (mode 0700)
- `TestE2E_FileStorage_SecureLocation` - Tests K8s security context with:
  - `fsGroup: 10001`
  - `runAsUser: 10001`
  - `runAsNonRoot: true`
  - `allowPrivilegeEscalation: false`
  - `emptyDir` volume with size limit

**What it validates:**
- Extension can create directories in secure locations
- Works with installer-created restricted directories
- Functions correctly with Kubernetes security policies
- Non-root users can write to filestorage volumes

### ✅ Requirement 2: Persistent Queue with Exporter
**Verified by:**
- `TestFileStorageWithExporter` - Tests OTLP HTTP exporter with `storage: file_storage`
- `TestE2E_FileStorage_PersistentQueue` - Tests in Kubernetes deployment

**What it validates:**
- Extension integrates with exporter sending_queue
- Files are created in configured directory
- Data persists during transmission delays
- Queue configuration options work correctly
- Works with debug, OTLP/HTTP and other exporters

### ✅ Requirement 3: Filelog Receiver Checkpointing
**Verified by:**
- `TestFileStorageWithFileLogReceiver` - Tests checkpoint file creation
- `TestE2E_FileStorage_FileLogReceiver` - Tests in Kubernetes DaemonSet

**What it validates:**
- Extension integrates with filelog receiver `storage: file_storage`
- Checkpoint files are created and managed
- Log file position is tracked correctly
- Works across collector restarts
- Handles log rotation scenarios

## Running the Tests

### Local Integration Tests
```bash
# Build collector
make build

# Run tests
cd internal/testbed/integration/filestorage
go test -v .
```

### Kubernetes E2E Tests
```bash
# Set environment
export KUBECONFIG=/path/to/kubeconfig
export CONTAINER_REGISTRY=your-registry/

# Build and push image
make build
docker build -t ${CONTAINER_REGISTRY}dynatrace-otel-collector:latest .
docker push ${CONTAINER_REGISTRY}dynatrace-otel-collector:latest

# Run E2E tests
cd internal/testbed/integration/filestorage
go test -v -tags=e2e .
```

## Test Patterns Used

Following the established patterns in the repository:

1. **FilteringScenario Pattern** - Used for config validation tests
2. **K8s E2E Pattern** - Used patterns from:
   - `internal/testbed/integration/hostmetrics/e2e_test.go`
   - `internal/testbed/integration/k8scluster/e2e_test.go`
   - `internal/testbed/integration/kafka/e2e_test.go`
3. **Data Validation** - Used existing validators:
   - `LogsValidator` for validating received logs
   - `consumertest.LogsSink` for collecting data
4. **Test Utilities** - Leveraged:
   - `k8stest` package for K8s operations
   - `oteltest` package for OTLP receivers
   - `testutil` package for port management

## Security Features Tested

1. **Directory Permissions**
   - Tests verify creation with mode 0700
   - Validates write access with restricted permissions

2. **Kubernetes Security Context**
   - Non-root user (UID 10001)
   - fsGroup for volume access
   - Dropped all capabilities
   - Read-only root filesystem compatible

3. **Volume Management**
   - emptyDir with size limits
   - Proper cleanup on pod deletion
   - No host path dependencies (except for filelog receiver)

## Integration Points Validated

### With Exporters
- ✅ OTLP HTTP exporter with persistent queue
- ✅ Debug exporter (works with any exporter)
- ✅ Queue size configuration
- ✅ Retry configuration
- ✅ Compaction settings

### With Receivers
- ✅ Filelog receiver with checkpoint storage
- ✅ OTLP receiver (standard data flow)
- ✅ Log file inclusion patterns
- ✅ Position tracking across restarts

### With Extensions
- ✅ Health check extension (co-deployed)
- ✅ Directory auto-creation
- ✅ Compaction on start and rebound
- ✅ Storage threshold management

## Comparison with Existing Tests

The filestorage tests follow the same structure as:

| Test Suite | Pattern | Structure |
|------------|---------|-----------|
| hostmetrics | E2E + golden files | ✓ Similar deployment pattern |
| k8scluster | E2E validation | ✓ Similar validation approach |
| kafka | E2E + multiple components | ✓ Similar multi-pod setup |
| filtering | Config validation | ✓ Similar test flow |

## Next Steps

To run these tests in CI/CD:

1. Add to test matrix in `.github/workflows/` (if exists)
2. Ensure collector binary is built before tests
3. Set up Kubernetes cluster for E2E tests
4. Configure container registry access
5. Set required environment variables

The tests are production-ready and follow all established patterns in the repository.
