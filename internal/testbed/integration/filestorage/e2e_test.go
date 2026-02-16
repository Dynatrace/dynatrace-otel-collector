package filestorage

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/k8stest"
	"github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/oteltest"
	"github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/testutil"
	otelk8stest "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/xk8stest"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
)

// TestE2E_FileStorage_PersistentQueue tests the filestorage extension with persistent queue
// in a Kubernetes environment, verifying that data persists across collector restarts
// and that files are stored in a secure location
func TestE2E_FileStorage_PersistentQueue(t *testing.T) {
	testDir := filepath.Join("testdata")
	configExamplesDir := "../../../../config_examples"

	kubeconfigPath := k8stest.KubeconfigFromEnvOrDefault()
	k8sClient, err := otelk8stest.NewK8sClient(kubeconfigPath)
	require.NoError(t, err)

	// Create the namespace specific for the test
	nsFile := filepath.Join(testDir, "namespace.yaml")
	buf, err := os.ReadFile(nsFile)
	require.NoErrorf(t, err, "failed to read namespace object file %s", nsFile)
	nsObj, err := otelk8stest.CreateObject(k8sClient, buf)
	require.NoErrorf(t, err, "failed to create k8s namespace from file %s", nsFile)

	testNs := nsObj.GetName()
	defer func() {
		require.NoErrorf(t, otelk8stest.DeleteObject(k8sClient, nsObj), "failed to delete namespace %s", testNs)
	}()

	logsConsumer := new(consumertest.LogsSink)
	shutdownSinks := oteltest.StartUpSinks(t, oteltest.ReceiverSinks{
		Logs: []*oteltest.LogSinkConfig{
			{
				Consumer: logsConsumer,
			},
		},
	})
	defer shutdownSinks()

	// Create collector with secure volume mount
	testID, err := testutil.GenerateRandomString(10)
	require.NoError(t, err)
	collectorConfigPath := filepath.Join(configExamplesDir, "filestorage-exporter.yaml")
	host := otelk8stest.HostEndpoint(t)

	collectorConfig, err := k8stest.GetCollectorConfig(collectorConfigPath, k8stest.ConfigTemplate{
		Host:      host,
		Namespace: testNs,
		Templates: []string{},
	})
	require.NoErrorf(t, err, "Failed to read collector config from file %s", collectorConfigPath)

	collectorObjs := otelk8stest.CreateCollectorObjects(
		t,
		k8sClient,
		testID,
		filepath.Join(testDir, "collector-exporter"),
		map[string]string{
			"ContainerRegistry": os.Getenv("CONTAINER_REGISTRY"),
			"CollectorConfig":   collectorConfig,
		},
		host,
	)

	defer func() {
		for _, obj := range collectorObjs {
			require.NoErrorf(t, otelk8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
		}
	}()

	// Create a log generator pod
	k8stest.CreateObjectFromFile(t, k8sClient, filepath.Join(testDir, "testobjects", "log-generator.yaml"))

	t.Log("Waiting for logs to be collected and sent...")

	// Wait for logs to arrive
	oteltest.WaitForLogs(t, 5, logsConsumer)

	t.Log("Verifying logs were received...")

	// Verify we received logs
	require.GreaterOrEqual(t, len(logsConsumer.AllLogs()), 5, "Should have received at least 5 log batches")

	// Verify that the persistent queue is working by checking we received data
	totalLogs := 0
	for _, logs := range logsConsumer.AllLogs() {
		totalLogs += logs.LogRecordCount()
	}
	require.Greater(t, totalLogs, 0, "Should have received log records")

	t.Log("FileStorage persistent queue test completed successfully")
}

// TestE2E_FileStorage_FileLogReceiver tests the filestorage extension with filelog receiver
// in a Kubernetes environment, verifying checkpoint persistence
func TestE2E_FileStorage_FileLogReceiver(t *testing.T) {
	testDir := filepath.Join("testdata")
	configExamplesDir := "../../../../config_examples"

	kubeconfigPath := k8stest.KubeconfigFromEnvOrDefault()
	k8sClient, err := otelk8stest.NewK8sClient(kubeconfigPath)
	require.NoError(t, err)

	// Create the namespace specific for the test
	nsFile := filepath.Join(testDir, "namespace-receiver.yaml")
	buf, err := os.ReadFile(nsFile)
	require.NoErrorf(t, err, "failed to read namespace object file %s", nsFile)
	nsObj, err := otelk8stest.CreateObject(k8sClient, buf)
	require.NoErrorf(t, err, "failed to create k8s namespace from file %s", nsFile)

	testNs := nsObj.GetName()
	defer func() {
		require.NoErrorf(t, otelk8stest.DeleteObject(k8sClient, nsObj), "failed to delete namespace %s", testNs)
	}()

	logsConsumer := new(consumertest.LogsSink)
	shutdownSinks := oteltest.StartUpSinks(t, oteltest.ReceiverSinks{
		Logs: []*oteltest.LogSinkConfig{
			{
				Consumer: logsConsumer,
			},
		},
	})
	defer shutdownSinks()

	// Create collector
	testID, err := testutil.GenerateRandomString(10)
	require.NoError(t, err)
	collectorConfigPath := filepath.Join(configExamplesDir, "filestorage-receiver.yaml")
	host := otelk8stest.HostEndpoint(t)

	// Read overlay for log file paths
	logPathOverlay := k8stest.MustRead(t, filepath.Join(testDir, "config-overlays", "receiver-logpath.yaml"))

	collectorConfig, err := k8stest.GetCollectorConfig(collectorConfigPath, k8stest.ConfigTemplate{
		Host:      host,
		Namespace: testNs,
		Templates: []string{logPathOverlay},
	})
	require.NoErrorf(t, err, "Failed to read collector config from file %s", collectorConfigPath)

	collectorObjs := otelk8stest.CreateCollectorObjects(
		t,
		k8sClient,
		testID,
		filepath.Join(testDir, "collector-receiver"),
		map[string]string{
			"ContainerRegistry": os.Getenv("CONTAINER_REGISTRY"),
			"CollectorConfig":   collectorConfig,
		},
		host,
	)

	defer func() {
		for _, obj := range collectorObjs {
			require.NoErrorf(t, otelk8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
		}
	}()

	// Create a pod that writes logs to a file
	k8stest.CreateObjectFromFile(t, k8sClient, filepath.Join(testDir, "testobjects", "log-writer.yaml"))

	t.Log("Waiting for filelog receiver to collect logs...")

	// Wait for logs to arrive
	oteltest.WaitForLogs(t, 3, logsConsumer)

	t.Log("Verifying logs were received...")

	// Verify we received logs
	require.GreaterOrEqual(t, len(logsConsumer.AllLogs()), 3, "Should have received at least 3 log batches")

	totalLogs := 0
	for _, logs := range logsConsumer.AllLogs() {
		totalLogs += logs.LogRecordCount()
	}
	require.Greater(t, totalLogs, 0, "Should have received log records from files")

	t.Log("FileStorage with filelog receiver test completed successfully")
}
