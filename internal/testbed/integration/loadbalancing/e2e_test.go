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

const k8sResolverConfig = `
      k8s:
        service: %s-receiver.default
        ports:
          - 4317`
const replacementK8sResolverConfig = `
      static:
        hostnames:
          - %s:%d
`
const replacementK8sMetricsResolverConfig = `
      static:
        hostnames:
          - %s:%d
          - %s:%d
`
const metricsPortGrpc1 = 4327
const metricsPortGrpc2 = 4328
const metricsPortHttp1 = 4329
const metricsPortHttp2 = 4330
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

	metricsConsumer1 := new(consumertest.MetricsSink)
	metricsConsumer2 := new(consumertest.MetricsSink)
	tracesConsumer := new(consumertest.TracesSink)
	logsConsumer := new(consumertest.LogsSink)
	shutdownSinks := oteltest.StartUpSinks(t, oteltest.ReceiverSinks{
		Metrics: []*oteltest.MetricSinkConfig{
			{
				Consumer: metricsConsumer1,
				Ports: &oteltest.ReceiverPorts{
					Grpc: metricsPortGrpc1,
					Http: metricsPortHttp1,
				},
			},
			{
				Consumer: metricsConsumer2,
				Ports: &oteltest.ReceiverPorts{
					Grpc: metricsPortGrpc2,
					Http: metricsPortHttp2,
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

	// create collector
	testID, err := testutil.GenerateRandomString(10)
	require.NoError(t, err)
	collectorConfigPath := path.Join(configExamplesDir, "load-balancing.yaml")
	host := otelk8stest.HostEndpoint(t)
	collectorConfig, err := k8stest.GetCollectorConfig(collectorConfigPath, k8stest.ConfigTemplate{
		Host: host,
		Templates: []string{
			fmt.Sprintf(k8sResolverConfig, "metrics"), fmt.Sprintf(replacementK8sMetricsResolverConfig, host, metricsPortGrpc1, host, metricsPortGrpc2),
			fmt.Sprintf(k8sResolverConfig, "traces"), fmt.Sprintf(replacementK8sResolverConfig, host, tracesPortGrpc),
			fmt.Sprintf(k8sResolverConfig, "logs"), fmt.Sprintf(replacementK8sResolverConfig, host, logsPortGrpc),
		},
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

	createTeleOpts := &otelk8stest.TelemetrygenCreateOpts{
		ManifestsDir: filepath.Join(testDir, "telemetrygen"),
		TestID:       testID,
		OtlpEndpoint: fmt.Sprintf("otelcol-%s.%s:4317", testID, testNs),
		DataTypes:    []string{""},
	}

	telemetryGenObjs, telemetryGenObjInfos := otelk8stest.CreateTelemetryGenObjects(t, k8sClient, createTeleOpts)

	defer func() {
		for _, obj := range append(collectorObjs, telemetryGenObjs...) {
			require.NoErrorf(t, otelk8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
		}
	}()

	for _, info := range telemetryGenObjInfos {
		otelk8stest.WaitForTelemetryGenToStart(t, k8sClient, info.Namespace, info.PodLabelSelectors, info.Workload, info.DataType)
	}

	customMetricName1 := "custom-metric-name1"
	customMetricName2 := "custom-metric-name2"
	customMetricName3 := "custom-metric-name3"
	customMetricName4 := "custom-metric-name4"

	oteltest.WaitForMetrics(t, 20, metricsConsumer1)
	oteltest.WaitForMetrics(t, 20, metricsConsumer2)

	for _, r := range metricsConsumer1.AllMetrics() {
		for i := 0; i < r.ResourceMetrics().Len(); i++ {
			datapoints := r.ResourceMetrics().At(i).ScopeMetrics().At(0).Metrics()
			for j := 0; j < datapoints.Len(); j++ {
				actual := datapoints.At(j).Name()
				require.Condition(t, func() bool {
					return actual == customMetricName2 || actual == customMetricName3
				}, "Expected metric name to be either %s or %s, but got: %s", customMetricName2, customMetricName3, actual)
			}
		}
	}

	for _, r := range metricsConsumer2.AllMetrics() {
		for i := 0; i < r.ResourceMetrics().Len(); i++ {
			datapoints := r.ResourceMetrics().At(i).ScopeMetrics().At(0).Metrics()
			for j := 0; j < datapoints.Len(); j++ {
				actual := datapoints.At(j).Name()
				require.Condition(t, func() bool {
					return actual == customMetricName1 || actual == customMetricName4
				}, "Expected metric name to be either %s or %s, but got: %s", customMetricName1, customMetricName4, actual)
			}
		}
	}

	oteltest.WaitForTraces(t, 20, tracesConsumer)
	oteltest.WaitForLogs(t, 20, logsConsumer)
}
