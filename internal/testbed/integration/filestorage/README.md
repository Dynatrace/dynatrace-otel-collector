# FileSto Integration Tests

This directory contains integration tests for the `filestorage` extension in the Dynatrace OpenTelemetry Collector distribution.

## Test Coverage

### Unit-Style Integration Tests (`filestorage_test.go`)

These tests run locally against a compiled collector binary and verify:

1. **TestFileStorageWithExporter** - Tests persistent queue functionality
   - Verifies the filestorage extension works with OTLP HTTP exporter's sending queue
   - Confirms files are created in the configured directory
   - Validates data transmission after queue persistence

2. **TestFileStorageWithFileLogReceiver** - Tests filelog receiver checkpointing
   - Verifies the filestorage extension works with the filelog receiver
   - Confirms checkpoint files are created and managed correctly
   - Validates log file position tracking across restarts

3. **TestFileStorageSecureLocation** - Tests secure file creation
   - Verifies the extension can write to directories with restricted permissions
   - Simulates installer-created secure directories (mode 0700)
   - Validates functionality in production-like environments

### E2E Kubernetes Tests (`e2e_test.go`)

These tests run in a Kubernetes cluster with the `//go:build e2e` tag and verify:

1. **TestE2E_FileStorage_PersistentQueue** - Tests persistent queue in K8s
   - Deploys collector with filestorage extension and persistent queue
   - Verifies data can be queued and transmitted
   - Validates volume mounts for file storage

2. **TestE2E_FileStorage_FileLogReceiver** - Tests filelog receiver in K8s
   - Deploys collector as DaemonSet with filelog receiver
   - Creates log-writing pods to generate test data
   - Verifies checkpoint persistence across pod restarts

3. **TestE2E_FileStorage_SecureLocation** - Tests secure deployment
   - Deploys collector with security context (fsGroup, runAsUser, runAsNonRoot)
   - Uses emptyDir volume with size limit
   - Verifies extension works with restricted security policies
   - Validates the collector can run as non-root user with secure storage

## Requirements Validated

✅ **Secure write location**: Tests verify the extension can write to secure directories with appropriate permissions

✅ **Persistent queue compatibility**: Tests verify the extension works with exporter sending queues for data persistence

✅ **Filelog receiver compatibility**: Tests verify the extension works with filelog receiver for checkpoint management

✅ **Kubernetes deployment**: Tests verify the extension works when deployed via Helm/installers in Kubernetes

## Running the Tests

### Unit-Style Integration Tests

```bash
# Build the collector first
make build

# Run integration tests
cd internal/testbed/integration/filestorage
go test -v .
```

### E2E Kubernetes Tests

```bash
# Ensure you have a Kubernetes cluster and kubectl configured
export KUBECONFIG=/path/to/kubeconfig
export CONTAINER_REGISTRY=your-registry/

# Build and push the collector image
make build
docker build -t ${CONTAINER_REGISTRY}dynatrace-otel-collector:latest .
docker push ${CONTAINER_REGISTRY}dynatrace-otel-collector:latest

# Run E2E tests
cd internal/testbed/integration/filestorage
go test -v -tags=e2e .
```

## Test Structure

```
filestorage/
├── filestorage_test.go          # Unit-style integration tests
├── e2e_test.go                   # Kubernetes E2E tests (requires e2e build tag)
├── README.md                     # This file
└── testdata/
    ├── namespace.yaml            # Test namespace for persistent queue tests
    ├── namespace-receiver.yaml   # Test namespace for receiver tests
    ├── namespace-secure.yaml     # Test namespace for secure storage tests
    ├── collector/                # Collector deployment manifests
    │   ├── deployment.yaml
    │   ├── configmap.yaml
    │   └── service.yaml
    ├── collector-receiver/       # Collector DaemonSet for filelog receiver
    │   ├── daemonset.yaml
    │   └── configmap.yaml
    ├── collector-secure/         # Collector deployment with security context
    │   ├── deployment.yaml
    │   ├── configmap.yaml
    │   └── service.yaml
    ├── config-overlays/          # Config template overlays
    │   ├── receiver-logpath.yaml # Log file path for filelog receiver
    │   └── secure-path.yaml      # Secure storage path
    └── testobjects/              # Test workloads
        ├── log-generator.yaml    # Telemetrygen for OTLP logs
        └── log-writer.yaml       # Busybox pod writing to files
```

## Configuration Examples Used

The tests use the following config examples from `config_examples/`:

- `filestorage-exporter.yaml` - Configuration with filestorage extension for exporter persistent queue
- `filestorage-receiver.yaml` - Configuration with filestorage extension for filelog receiver checkpointing

## Test Scenarios

### Scenario 1: Persistent Queue
1. Deploy collector with filestorage extension
2. Configure OTLP HTTP exporter with `storage: file_storage`
3. Send OTLP logs through the collector
4. Verify files are created in `/var/lib/otelcol/file_storage`
5. Verify data is successfully exported

### Scenario 2: Filelog Receiver Checkpointing
1. Deploy collector as DaemonSet with filestorage extension
2. Configure filelog receiver with `storage: file_storage`
3. Create pods writing logs to files
4. Verify checkpoint files are created
5. Verify logs are collected and exported

### Scenario 3: Secure Storage Location
1. Deploy collector with security context (non-root, read-only rootfs)
2. Mount emptyDir volume at `/var/lib/otelcol/file_storage`
3. Verify collector can write to the volume
4. Verify data transmission works correctly
5. Verify the setup matches installer deployment patterns

## Security Considerations

The tests verify that:

- The extension can create directories with `create_directory: true`
- Files are written with appropriate permissions
- The extension works with non-root users (UID 10001)
- The extension works with fsGroup security context
- Storage volumes can be restricted with size limits
- The extension respects Kubernetes security policies
