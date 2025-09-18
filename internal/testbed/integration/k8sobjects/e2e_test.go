//go:build e2e

package k8sobjects

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
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

func TestE2E_K8sobjectsReceiver(t *testing.T) {
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
	testID := uuid.NewString()[:8]
	collectorConfigPath := path.Join(configExamplesDir, "k8sobjects.yaml")
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

	expected := map[string]bool{
		"Deployment": false,
		"Node":       false,
		"Namespace":  false,
		"Pod":        false,
		"Event":      false,
	}

	oteltest.WaitForLogs(t, 5, logsConsumer)

	for _, r := range logsConsumer.AllLogs() {
		for i := 0; i < r.ResourceLogs().Len(); i++ {
			sm := r.ResourceLogs().At(i).ScopeLogs().At(0).LogRecords()
			for j := 0; j < sm.Len(); j++ {
				switch sm.At(j).Body().Type() {
				case pcommon.ValueTypeStr:
					bodyStr := sm.At(j).Body().Str()
					// event log bodies received by the k8sevents receiver are of type strings
					_, ok := sm.At(j).Attributes().Get("k8s.event.name")
					if bodyStr != "" && ok {
						expected["Event"] = true
					}
				case pcommon.ValueTypeMap:
					// logs for other resources, received by the k8sobjects receiver are of type map
					bodyMap := sm.At(j).Body().Map()
					if kind, ok := bodyMap.Get("kind"); ok {
						if _, ok := bodyMap.Get("message"); ok {
							expected[kind.Str()] = true
						}
					}
				}
			}
		}
	}

	checkMatched(t, expected)
}

func checkMatched(t *testing.T, e map[string]bool) {
	for _, ok := range e {
		if !ok {
			require.True(t, ok, "Some resources were not found: %w", e)
		}
	}
}
