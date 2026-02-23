//go:build e2e

package loadbalancing

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"

	"github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/k8stest"
	"github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/oteltest"
	"github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/testutil"
	otelk8stest "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/xk8stest"
)

const OTLPConfig = `
      otlp:`

const replacementOTLPConfig = `
      otlp:
        tls:
          insecure: true`

const metricsPortGrpc = 4327
const metricsPortHttp = 4328
const tracesPortGrpc = 4337
const tracesPortHttp = 4438
const logsPortGrpc = 4347
const logsPortHttp = 4448

func TestE2E_LoadBalancing(t *testing.T) {
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

	metricsConsumer := new(consumertest.MetricsSink)
	tracesConsumer := new(consumertest.TracesSink)
	logsConsumer := new(consumertest.LogsSink)
	shutdownSinks := oteltest.StartUpSinks(t, oteltest.ReceiverSinks{
		Metrics: []*oteltest.MetricSinkConfig{
			{
				Consumer: metricsConsumer,
				Ports: &oteltest.ReceiverPorts{
					Grpc: metricsPortGrpc,
					Http: metricsPortHttp,
				},
			},
		},
		Traces: []*oteltest.TraceSinkConfig{
			{
				Consumer: tracesConsumer,
				Ports: &oteltest.ReceiverPorts{
					Grpc: tracesPortGrpc,
					Http: tracesPortHttp,
				},
			},
		},
		Logs: []*oteltest.LogSinkConfig{
			{
				Consumer: logsConsumer,
				Ports: &oteltest.ReceiverPorts{
					Grpc: logsPortGrpc,
					Http: logsPortHttp,
				},
			},
		},
	})
	defer shutdownSinks()

	// create collector for passbying data
	host := otelk8stest.HostEndpoint(t)
	testID2, err := testutil.GenerateRandomString(10)
	require.NoError(t, err)
	collectorObjs2 := k8stest.CreateCollectorObjects(
		t,
		k8sClient,
		testID2,
		filepath.Join(testDir, "otlp-receiver"),
		map[string]string{
			"ContainerRegistry": os.Getenv("CONTAINER_REGISTRY"),
		},
		host,
		testNs,
	)

	// create collector
	testID, err := testutil.GenerateRandomString(10)
	require.NoError(t, err)
	collectorConfigPath := path.Join(configExamplesDir, "load-balancing.yaml")
	collectorConfig, err := k8stest.GetCollectorConfig(collectorConfigPath, k8stest.ConfigTemplate{
		Host: host,
		Templates: []string{
			OTLPConfig, replacementOTLPConfig,
		},
	})
	require.NoErrorf(t, err, "Failed to read collector config from file %s", collectorConfigPath)
	collectorObjs := k8stest.CreateCollectorObjects(
		t,
		k8sClient,
		testID,
		filepath.Join(testDir, "collector"),
		map[string]string{
			"ContainerRegistry": os.Getenv("CONTAINER_REGISTRY"),
			"CollectorConfig":   collectorConfig,
		},
		host,
		testNs,
	)

	// create telemetrygen
	createTeleOpts := &otelk8stest.TelemetrygenCreateOpts{
		ManifestsDir: filepath.Join(testDir, "telemetrygen"),
		TestID:       testID,
		OtlpEndpoint: fmt.Sprintf("otelcol-%s.%s:4317", testID, testNs),
		DataTypes:    []string{""},
	}

	telemetryGenObjs, telemetryGenObjInfos := otelk8stest.CreateTelemetryGenObjects(t, k8sClient, createTeleOpts)

	collectorObjs = append(collectorObjs, collectorObjs2...)
	defer func() {
		for _, obj := range append(collectorObjs, telemetryGenObjs...) {
			require.NoErrorf(t, otelk8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
		}
	}()

	for _, info := range telemetryGenObjInfos {
		otelk8stest.WaitForTelemetryGenToStart(t, k8sClient, info.Namespace, info.PodLabelSelectors, info.Workload, info.DataType)
	}

	oteltest.WaitForMetrics(t, 20, metricsConsumer)
	oteltest.WaitForTraces(t, 20, tracesConsumer)
	oteltest.WaitForLogs(t, 20, logsConsumer)
}
