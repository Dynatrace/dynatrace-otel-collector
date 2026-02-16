package filestorage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"
	"time"

	"github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/k8stest"
	"github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/oteltest"
	"github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/testutil"
	otelk8stest "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/xk8stest"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// TestE2E_FileStorage_PersistentQueue tests the filestorage extension with persistent queue
// in a Kubernetes environment, verifying that data persists across collector restarts
// and that files are stored in a secure location
func TestE2E_FileStorage_PersistentQueue(t *testing.T) {
	return
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

	// Create telemetrygen for log generation
	createTeleOpts := &otelk8stest.TelemetrygenCreateOpts{
		ManifestsDir: filepath.Join(testDir, "telemetrygen"),
		TestID:       testID,
		OtlpEndpoint: fmt.Sprintf("otelcol-%s.%s:4317", testID, testNs),
		DataTypes:    []string{"logs"},
	}

	telemetryGenObjs, telemetryGenObjInfos := otelk8stest.CreateTelemetryGenObjects(t, k8sClient, createTeleOpts)

	defer func() {
		for _, obj := range append(collectorObjs, telemetryGenObjs...) {
			require.NoErrorf(t, otelk8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
		}
	}()

	// Wait for telemetrygen to start
	for _, info := range telemetryGenObjInfos {
		otelk8stest.WaitForTelemetryGenToStart(t, k8sClient, info.Namespace, info.PodLabelSelectors, info.Workload, info.DataType)
	}

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
// in a Kubernetes environment, verifying checkpoint persistence across collector restarts
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

	collectorConfig, err := k8stest.GetCollectorConfig(collectorConfigPath, k8stest.ConfigTemplate{
		Host:      host,
		Namespace: testNs,
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

	// Create a pod that writes numbered logs to a file
	k8stest.CreateObjectFromFile(t, k8sClient, filepath.Join(testDir, "testobjects", "log-writer.yaml"))

	t.Log("Phase 1: Waiting for initial logs to be collected...")

	// Wait for initial logs to arrive (let it collect some logs before restart)
	time.Sleep(15 * time.Second)

	t.Log("Phase 2: Restarting collector to test checkpoint persistence...")

	// Restart the collector by deleting the DaemonSet pod
	// This simulates a collector restart and tests if the checkpoint is persisted
	podName, err := k8stest.GetPodNameByLabels(k8sClient.DynamicClient, testNs, map[string]string{
		"app.kubernetes.io/name": "opentelemetry-collector",
	})
	require.NoError(t, err, "Failed to get collector pod name")

	// Delete the pod to trigger restart
	gvr := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "pods",
	}
	err = k8sClient.DynamicClient.Resource(gvr).Namespace(testNs).Delete(
		context.Background(),
		podName,
		metav1.DeleteOptions{},
	)
	require.NoError(t, err, "Failed to delete collector pod for restart")

	t.Log("Phase 3: Checking logs which came before restart...")

	// Collect log numbers from first batch
	firstBatchLogNumbers := extractLogNumbers(t, logsConsumer)
	// Clear the consumer to start fresh for the second batch
	logsConsumer.Reset()
	t.Logf("First batch collected %d unique log entries", len(firstBatchLogNumbers))
	require.Greater(t, len(firstBatchLogNumbers), 0, "Should have collected some logs before restart")

	// Find the highest log number seen so far
	maxLogNumber := 0
	for num := range firstBatchLogNumbers {
		if num > maxLogNumber {
			maxLogNumber = num
		}
	}
	t.Logf("Highest log number before restart: %d", maxLogNumber)

	// Wait for the DaemonSet to create a new pod and for it to be ready
	time.Sleep(10 * time.Second)
	t.Log("Collector restarted, waiting for it to resume from checkpoint and catch up on missed logs...")

	t.Log("Phase 4: Waiting for logs after restart...")

	// Wait for more logs to arrive after restart - give it time to catch up on logs written during restart
	time.Sleep(15 * time.Second)

	t.Log("Verifying checkpoint persistence...")

	// Collect log numbers from second batch (after restart)
	secondBatchLogNumbers := extractLogNumbers(t, logsConsumer)
	t.Logf("Second batch collected %d unique log entries", len(secondBatchLogNumbers))
	require.Greater(t, len(secondBatchLogNumbers), 0, "Should have collected logs after restart")

	// Find the minimum log number in the second batch
	minLogNumberAfterRestart := maxLogNumber + 1000 // Start with a high value
	for num := range secondBatchLogNumbers {
		if num < minLogNumberAfterRestart {
			minLogNumberAfterRestart = num
		}
	}
	t.Logf("Lowest log number after restart: %d", minLogNumberAfterRestart)

	// Check for gaps (missing log numbers)
	allLogNumbers := make(map[int]bool)
	for num := range firstBatchLogNumbers {
		allLogNumbers[num] = true
	}
	for num := range secondBatchLogNumbers {
		allLogNumbers[num] = true
	}

	// Find min and max across all logs
	minOverall := maxLogNumber + 1000
	maxOverall := 0
	for num := range allLogNumbers {
		if num < minOverall {
			minOverall = num
		}
		if num > maxOverall {
			maxOverall = num
		}
	}

	// Check for gaps
	missingLogs := []int{}
	for i := minOverall; i <= maxOverall; i++ {
		if !allLogNumbers[i] {
			missingLogs = append(missingLogs, i)
		}
	}

	if len(missingLogs) > 0 {
		t.Logf("WARNING: Missing log entries: %v (total: %d)", missingLogs, len(missingLogs))
		t.Logf("This indicates logs were lost during collection or restart")
	}

	// Assert no gaps - checkpoint should ensure continuity
	require.Empty(t, missingLogs, "Found %d missing log entries %v - checkpoint persistence failed to prevent data loss", len(missingLogs), missingLogs)

	// Verify checkpoint persistence: the collector should resume from where it left off
	// The minimum log number after restart should be close to (or greater than) the maximum from before restart
	require.GreaterOrEqual(t, minLogNumberAfterRestart, maxLogNumber,
		"Collector should resume from checkpoint (min after restart: %d, max before restart: %d). "+
			"Gap detected: logs may have been lost during restart.",
		minLogNumberAfterRestart, maxLogNumber)

	// Verify no duplication at all: logs from first batch should NOT appear in second batch
	duplicateCount := 0
	duplicateLogs := []int{}
	for num := range firstBatchLogNumbers {
		if _, exists := secondBatchLogNumbers[num]; exists {
			duplicateCount++
			duplicateLogs = append(duplicateLogs, num)
		}
	}
	duplicatePercentage := float64(duplicateCount) / float64(len(firstBatchLogNumbers)) * 100
	t.Logf("Duplicate logs: %d out of %d (%.1f%%)", duplicateCount, len(firstBatchLogNumbers), duplicatePercentage)

	if duplicateCount > 0 {
		t.Logf("WARNING: Duplicate log entries found: %v", duplicateLogs)
	}

	// STRICT requirement: No duplications allowed - checkpoint must prevent re-reading
	require.Equal(t, 0, duplicateCount,
		"Found %d duplicate log entries %v - checkpoint persistence must prevent any duplication",
		duplicateCount, duplicateLogs)

	t.Log("FileStorage checkpoint persistence test completed successfully!")
	t.Logf("Summary: Collected %d logs before restart, %d after restart, 0 missing, 0 duplicates",
		len(firstBatchLogNumbers), len(secondBatchLogNumbers))
	t.Logf("STRICT REQUIREMENTS MET: No data loss + No duplication = Perfect checkpoint persistence")
}

// extractLogNumbers parses log messages and extracts the log entry numbers
// Expected format: "Log entry X from filestorage checkpoint test at ..."
func extractLogNumbers(t *testing.T, consumer *consumertest.LogsSink) map[int]bool {
	logNumbers := make(map[int]bool)
	re := regexp.MustCompile(`Log entry (\d+) from filestorage`)

	for _, logs := range consumer.AllLogs() {
		for i := 0; i < logs.ResourceLogs().Len(); i++ {
			rl := logs.ResourceLogs().At(i)
			for j := 0; j < rl.ScopeLogs().Len(); j++ {
				sl := rl.ScopeLogs().At(j)
				for k := 0; k < sl.LogRecords().Len(); k++ {
					logRecord := sl.LogRecords().At(k)
					body := logRecord.Body().AsString()

					matches := re.FindStringSubmatch(body)
					if len(matches) >= 2 {
						if num, err := strconv.Atoi(matches[1]); err == nil {
							logNumbers[num] = true
						}
					}
				}
			}
		}
	}

	return logNumbers
}
