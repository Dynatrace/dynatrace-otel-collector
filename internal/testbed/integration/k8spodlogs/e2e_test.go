//go:build e2e

package k8sobjects

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/plog"
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"

	"github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/k8stest"
	"github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/oteltest"
	otelk8stest "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/xk8stest"
)

func TestE2E_K8PodLogs(t *testing.T) {
	testDir := filepath.Join("testdata")
	expectedFile := testDir + "/e2e/expected.yaml"
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
	testID := uuid.NewString()[:8]
	collectorConfigPath := path.Join(configExamplesDir, "k8s_pod_logs.yaml")
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

	// create deployment
	deploymentFile := filepath.Join(testDir, "testobjects", "deployment.yaml")
	buf, err = os.ReadFile(deploymentFile)
	require.NoErrorf(t, err, "failed to read deployment object file %s", deploymentFile)
	deploymentObj, err := otelk8stest.CreateObject(k8sClient, buf)
	require.NoErrorf(t, err, "failed to create k8s deployment from file %s", deploymentFile)

	defer func() {
		for _, obj := range append(collectorObjs, deploymentObj) {
			require.NoErrorf(t, otelk8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
		}
	}()

	oteltest.WaitForLogs(t, 1, logsConsumer)

	require.Nil(t, golden.WriteLogs(t, expectedFile, logsConsumer.AllLogs()[len(logsConsumer.AllLogs())-1]))

	var expected plog.Logs
	expected, err = golden.ReadLogs(expectedFile)
	require.NoError(t, err)

	assert.NoError(t, plogtest.CompareLogs(expected, logsConsumer.AllLogs()[len(logsConsumer.AllLogs())-1]))
}
