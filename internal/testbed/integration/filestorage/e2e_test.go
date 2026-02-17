//go:build e2e

package filestorage

import (
	"context"
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

// TestE2E_FileStorage tests the filestorage extension in a Kubernetes environment, verifying:
// 1. Filelog receiver checkpoint persistence across collector restarts (no data loss, no duplicates)
// 2. Exporter queue persistence when backend is unavailable (queued logs delivered after backend recovery)
func TestE2E_FileStorage(t *testing.T) {
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

	// Create collector
	testID, err := testutil.GenerateRandomString(10)
	require.NoError(t, err)
	collectorConfigPath := filepath.Join(configExamplesDir, "filestorage.yaml")
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
		filepath.Join(testDir, "collector"),
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

	missingLogs := findMissingLogs(allLogNumbers)
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

	// Phase 5: Test exporter queue persistence with filestorage
	t.Log("Phase 5: Testing exporter queue persistence...")

	// Get the highest log number collected so far
	allCollectedLogs := make(map[int]bool)
	for num := range firstBatchLogNumbers {
		allCollectedLogs[num] = true
	}
	for num := range secondBatchLogNumbers {
		allCollectedLogs[num] = true
	}

	maxCollected := 0
	for num := range allCollectedLogs {
		if num > maxCollected {
			maxCollected = num
		}
	}
	t.Logf("Highest log number collected so far: %d", maxCollected)

	// Shut down the sink to simulate backend unavailability
	t.Log("Shutting down sink to test queue persistence...")
	shutdownSinks()

	// Wait while logs accumulate in the queue
	t.Log("Waiting 10 seconds for logs to accumulate in exporter queue...")
	time.Sleep(10 * time.Second)

	// Start a new sink
	t.Log("Starting new sink to drain queued logs...")
	logsConsumer3 := new(consumertest.LogsSink)
	shutdownSinks3 := oteltest.StartUpSinks(t, oteltest.ReceiverSinks{
		Logs: []*oteltest.LogSinkConfig{
			{
				Consumer: logsConsumer3,
			},
		},
	})
	defer shutdownSinks3()

	// Wait for queued logs to be sent
	t.Log("Waiting for queued logs to be delivered...")
	time.Sleep(15 * time.Second)

	// Collect log numbers from third batch (queued logs)
	thirdBatchLogNumbers := extractLogNumbers(t, logsConsumer3)
	t.Logf("Third batch (from queue) collected %d unique log entries", len(thirdBatchLogNumbers))
	require.Greater(t, len(thirdBatchLogNumbers), 0, "Should have collected queued logs after sink restart")

	// Find the lowest log number in third batch
	minThirdBatch := maxCollected + 1000
	for num := range thirdBatchLogNumbers {
		if num < minThirdBatch {
			minThirdBatch = num
		}
	}
	t.Logf("Lowest log number in third batch: %d", minThirdBatch)

	// Merge all collected logs
	for num := range thirdBatchLogNumbers {
		allCollectedLogs[num] = true
	}

	// Check for gaps across all phases
	missingLogsAll := findMissingLogs(allCollectedLogs)
	if len(missingLogsAll) > 0 {
		t.Logf("WARNING: Missing log entries across all phases: %v (total: %d)", missingLogsAll, len(missingLogsAll))
		t.Logf("This indicates logs were lost during queue persistence")
	}

	// Assert no gaps - exporter queue should ensure no data loss
	require.Empty(t, missingLogsAll, "Found %d missing log entries %v - exporter queue persistence failed to prevent data loss", len(missingLogsAll), missingLogsAll)

	t.Log("FileStorage exporter queue persistence test completed successfully!")
	t.Logf("Final Summary: Collected %d unique logs across all phases (checkpoint + queue), 0 missing, 0 duplicates",
		len(allCollectedLogs))
}

// findMissingLogs checks for gaps in a sequence of log numbers and returns any missing entries
func findMissingLogs(logNumbers map[int]bool) []int {
	if len(logNumbers) == 0 {
		return []int{}
	}

	// Find min and max
	const unreachableHighValue = 1000000
	minLog := unreachableHighValue
	maxLog := 0
	for num := range logNumbers {
		if num < minLog {
			minLog = num
		}
		if num > maxLog {
			maxLog = num
		}
	}

	// Check for gaps
	missingLogs := []int{}
	for i := minLog; i <= maxLog; i++ {
		if !logNumbers[i] {
			missingLogs = append(missingLogs, i)
		}
	}

	return missingLogs
}

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
