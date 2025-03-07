//go:build e2e

package netflow

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/k8stest"
	"github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/oteltest"
	otelk8stest "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/xk8stest"
)

func TestE2E_NetflowReceiver(t *testing.T) {
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
		Logs: &oteltest.LogSinkConfig{
			Consumer: logsConsumer,
		},
	})
	defer shutdownSinks()

	// create collector
	testID := uuid.NewString()[:8]
	collectorConfigPath := path.Join(configExamplesDir, "netflow.yaml")
	host := otelk8stest.HostEndpoint(t)
	collectorConfig, err := k8stest.GetCollectorConfig(collectorConfigPath, host)
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

	// create job
	jobFile := filepath.Join(testDir, "netflow", "job.yaml")
	buf, err = os.ReadFile(jobFile)
	require.NoErrorf(t, err, "failed to read job object file %s", jobFile)
	jobObj, err := otelk8stest.CreateObject(k8sClient, buf)
	require.NoErrorf(t, err, "failed to create k8s job from file %s", nsFile)

	defer func() {
		for _, obj := range append(collectorObjs, jobObj) {
			require.NoErrorf(t, otelk8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
		}
	}()

	oteltest.WaitForLogs(t, 32, logsConsumer)

	scanForServiceLogs(t, logsConsumer)
}

func scanForServiceLogs(t *testing.T, ms *consumertest.LogsSink) {
	for _, r := range ms.AllLogs() {
		for i := 0; i < r.ResourceLogs().Len(); i++ {
			sm := r.ResourceLogs().At(i).ScopeLogs().At(0).LogRecords()
			assert.NoError(t, assertExpectedLogs(sm))
		}
	}
}

func assertExpectedLogs(sm plog.LogRecordSlice) error {
	expectedFlowType := "netflow_v5"
	expectedAttributeKeys := []string{
		"source.address",
		"source.port",
		"destination.address",
		"destination.port",
		"network.transport",
		"network.type",
		"flow.io.bytes",
		"flow.io.packets",
		"flow.type",
		"flow.sequence_num",
		"flow.time_received",
		"flow.start",
		"flow.end",
		"flow.sampling_rate",
		"flow.sampler_address",
	}
	for i := 0; i < sm.Len(); i++ {
		attrs := sm.At(i).Attributes()
		if attrs.Len() != 15 {
			return fmt.Errorf("invalid lenght of attributes: %d", attrs.Len())
		}
		val, ok := attrs.Get("flow.type")
		if !ok {
			return fmt.Errorf("flow type not found")
		}
		if val.AsString() != expectedFlowType {
			return fmt.Errorf("invalid flow type: %s", val.AsString())
		}
		if !allItemsPresent(expectedAttributeKeys, attrs.AsRaw()) {
			return fmt.Errorf("invalid attributes: %v", attrs.AsRaw())
		}
	}
	return nil
}

func allItemsPresent(slice []string, m map[string]any) bool {
	for _, item := range slice {
		if _, exists := m[item]; !exists {
			return false
		}
	}
	return true
}
