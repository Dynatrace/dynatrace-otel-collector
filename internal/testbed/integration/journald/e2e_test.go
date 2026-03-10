//go:build e2e

package journald

import (
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/k8stest"
	"github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/oteltest"
	"github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/testutil"
	otelk8stest "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/xk8stest"
)

func TestE2E_JournaldReceiver(t *testing.T) {
	testDir := filepath.Join("testdata")
	configExamplesDir := "../../../../config_examples"

	kubeconfigPath := k8stest.TestKubeConfig
	if kubeConfigFromEnv := os.Getenv(k8stest.KubeConfigEnvVar); kubeConfigFromEnv != "" {
		kubeconfigPath = kubeConfigFromEnv
	}

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

	// create collector
	testID, err := testutil.GenerateRandomString(10)
	require.NoError(t, err)
	collectorConfigPath := path.Join(configExamplesDir, "journald.yaml")
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

	oteltest.WaitForLogs(t, 1, logsConsumer)

	t.Logf("Received %d log record(s)", logsConsumer.LogRecordCount())

	// Assert that the operators in journald.yaml transformed the fields correctly:
	//   body._PID   -> body.pid
	//   body._EXE   -> attributes.process.executable.name
	//   body.MESSAGE -> body.message
	var foundMessage, foundProcessExec bool
	for _, logs := range logsConsumer.AllLogs() {
		for i := 0; i < logs.ResourceLogs().Len(); i++ {
			scopeLogs := logs.ResourceLogs().At(i).ScopeLogs()
			for j := 0; j < scopeLogs.Len(); j++ {
				logRecords := scopeLogs.At(j).LogRecords()
				for k := 0; k < logRecords.Len(); k++ {
					record := logRecords.At(k)

					// Only inspect records whose body is a map (all journald records are)
					if record.Body().Type() != pcommon.ValueTypeMap {
						continue
					}
					body := record.Body().Map()

					// Old field names must be absent – the move operators should have renamed them
					_, hasPID := body.Get("_PID")
					require.False(t, hasPID, "body._PID should have been moved to body.pid by the operator")
					_, hasEXE := body.Get("_EXE")
					require.False(t, hasEXE, "body._EXE should have been moved to attributes.process.executable.name by the operator")
					_, hasMESSAGE := body.Get("MESSAGE")
					require.False(t, hasMESSAGE, "body.MESSAGE should have been moved to body.message by the operator")

					// Track that the new field names appear at least once across all records
					if _, ok := body.Get("message"); ok {
						foundMessage = true
					}
					if _, ok := record.Attributes().Get("process.executable.name"); ok {
						foundProcessExec = true
					}
				}
			}
		}
	}
	require.True(t, foundMessage, "expected at least one log record with body.message (moved from body.MESSAGE by the operator)")
	require.True(t, foundProcessExec, "expected at least one log record with attributes.process.executable.name (moved from body._EXE by the operator)")
}
