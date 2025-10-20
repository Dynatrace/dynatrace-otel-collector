//go:build e2e

package kafka

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"testing"
	"time"

	"github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/k8stest"
	"github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/oteltest"
	"github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/ptracetest"
	otelk8stest "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/xk8stest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var (
	templateOrigin = `
  otlphttp:
    endpoint: ${env:DT_ENDPOINT}
    headers:
      Authorization: "Api-Token ${env:DT_API_TOKEN}"

service:
  extensions: [health_check]
  pipelines:
    traces:
      receivers: [otlp]
      processors: [memory_limiter, batch]
      exporters: [kafka]
    traces/receive:
      receivers: [kafka]
      exporters: [otlphttp]
    metrics:
      receivers: [otlp]
      processors: [memory_limiter, batch]
      exporters: [kafka]
    metrics/receive:
      receivers: [kafka]
      exporters: [otlphttp]
    logs:
      receivers: [otlp]
      processors: [memory_limiter, batch]
      exporters: [kafka]
    logs/receive:
      receivers: [kafka]
      exporters: [otlphttp]`
	templateNew = `
  otlphttp/traces:
    endpoint: http://%s:4321
  otlphttp/metrics:
    endpoint: http://%s:4320
  otlphttp/logs:
    endpoint: http://%s:4319

service:
  extensions: [health_check]
  pipelines:
    traces:
      receivers: [otlp]
      exporters: [kafka]
    traces/receive:
      receivers: [kafka]
      exporters: [otlphttp/traces]
    metrics:
      receivers: [otlp]
      exporters: [kafka]
    metrics/receive:
      receivers: [kafka]
      exporters: [otlphttp/metrics]
    logs:
      receivers: [otlp]
      exporters: [kafka]
    logs/receive:
      receivers: [kafka]
      exporters: [otlphttp/logs]`
)

func TestE2E_Kafka(t *testing.T) {
	testDir := filepath.Join("testdata")
	expectedLogsFile := testDir + "/e2e/expected-logs.yaml"
	expectedTracesFile := testDir + "/e2e/expected-traces.yaml"
	expectedMetricsFile := testDir + "/e2e/expected-metrics.yaml"
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
		Logs: []*oteltest.LogSinkConfig{
			{
				Consumer: logsConsumer,
				Ports: &oteltest.ReceiverPorts{
					Http: 4319,
				},
			},
		},
		Metrics: []*oteltest.MetricSinkConfig{
			{
				Consumer: metricsConsumer,
				Ports: &oteltest.ReceiverPorts{
					Http: 4320,
				},
			},
		},
		Traces: []*oteltest.TraceSinkConfig{
			{
				Consumer: tracesConsumer,
				Ports: &oteltest.ReceiverPorts{
					Http: 4321,
				},
			},
		},
	})

	defer func() {
		// give some more time to the collector to finish exporting before stopping the sinks
		// so we do not have any dropped data after the test is finished
		time.Sleep(10 * time.Second)
		shutdownSinks()
	}()

	// create collector
	testID, err := testutil.GenerateRandomString(10)
	require.NoError(t, err)
	host := otelk8stest.HostEndpoint(t)
	collectorConfigPath := path.Join(configExamplesDir, "kafka.yaml")
	collectorConfig, err := k8stest.GetCollectorConfig(collectorConfigPath, k8stest.ConfigTemplate{
		Host: host,
		Templates: []string{
			templateOrigin,
			fmt.Sprintf(templateNew, host, host, host),
		},
	})

	require.NoErrorf(t, err, "Failed to read collector config from file %s", collectorConfigPath)
	collectorObjs2 := otelk8stest.CreateCollectorObjects(
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
		for _, obj := range collectorObjs2 {
			require.NoErrorf(t, otelk8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
		}
	}()

	// create kafka server deployment
	deploymentFile := filepath.Join(testDir, "testobjects", "kafka-deployment.yaml")
	buf, err = os.ReadFile(deploymentFile)
	require.NoErrorf(t, err, "failed to read kafka object file %s", deploymentFile)
	kafkaDeploymentObj, err := otelk8stest.CreateObject(k8sClient, buf)
	require.NoErrorf(t, err, "failed to create kafka server from file %s", deploymentFile)

	// create kafka server service
	serviceFile := filepath.Join(testDir, "testobjects", "kafka-service.yaml")
	buf, err = os.ReadFile(serviceFile)
	require.NoErrorf(t, err, "failed to read kafka service file %s", serviceFile)
	kafkaServiceObj, err := otelk8stest.CreateObject(k8sClient, buf)
	require.NoErrorf(t, err, "failed to create kafka service from file %s", serviceFile)

	defer func() {
		for _, obj := range []unstructured.Unstructured{*kafkaDeploymentObj, *kafkaServiceObj} {
			require.NoErrorf(t, otelk8stest.DeleteObject(k8sClient, &obj), "failed to delete object %s", obj.GetName())
		}
	}()

	// create telemetrygen deployment
	deploymentFile = filepath.Join(testDir, "testobjects", "telemetrygen.yaml")
	buf, err = os.ReadFile(deploymentFile)
	require.NoErrorf(t, err, "failed to read deployment object file %s", deploymentFile)
	telemetrygenObj, err := otelk8stest.CreateObject(k8sClient, buf)
	require.NoErrorf(t, err, "failed to create k8s deployment from file %s", deploymentFile)

	defer func() {
		require.NoErrorf(t, otelk8stest.DeleteObject(k8sClient, telemetrygenObj), "failed to delete object %s", telemetrygenObj.GetName())
	}()

	t.Logf("Checking metrics...")

	oteltest.WaitForMetrics(t, 1, metricsConsumer)

	// the commented line below writes the received list of metrics to the expected.yaml
	// require.Nil(t, golden.WriteMetrics(t, expectedMetricsFile, metricsConsumer.AllMetrics()[len(metricsConsumer.AllMetrics())-1]))

	expectedMetrics, err := golden.ReadMetrics(expectedMetricsFile)
	require.NoError(t, err)

	metricsCompareOptions := []pmetrictest.CompareMetricsOption{
		pmetrictest.IgnoreMetricValues("gen"),
		pmetrictest.IgnoreTimestamp(),
		pmetrictest.IgnoreStartTimestamp(),
		pmetrictest.IgnoreScopeVersion(),
	}

	require.EventuallyWithT(t, func(tt *assert.CollectT) {
		assert.NoError(tt, pmetrictest.CompareMetrics(expectedMetrics, metricsConsumer.AllMetrics()[len(metricsConsumer.AllMetrics())-1], metricsCompareOptions...))
	}, 3*time.Minute, 3*time.Second)

	t.Logf("Metrics checked successfully")

	t.Logf("Checking logs...")

	oteltest.WaitForLogs(t, 1, logsConsumer)

	// the commented line below writes the received list of logs to the expected.yaml
	// require.Nil(t, golden.WriteLogs(t, expectedLogsFile, logsConsumer.AllLogs()[len(logsConsumer.AllLogs())-1]))

	expectedLogs, err := golden.ReadLogs(expectedLogsFile)
	require.NoError(t, err)

	logcompareOptions := []plogtest.CompareLogsOption{
		plogtest.IgnoreTimestamp(),
	}

	require.EventuallyWithT(t, func(tt *assert.CollectT) {
		assert.NoError(tt, plogtest.CompareLogs(expectedLogs, logsConsumer.AllLogs()[len(logsConsumer.AllLogs())-1], logcompareOptions...))
	}, 3*time.Minute, 3*time.Second)

	t.Logf("Logs checked successfully")

	t.Log("Checking traces...")

	oteltest.WaitForTraces(t, 1, tracesConsumer)

	// the commented line below writes the received list of metrics to the expected.yaml
	// require.Nil(t, golden.WriteTraces(t, expectedTracesFile, tracesConsumer.AllTraces()[len(tracesConsumer.AllTraces())-1]))

	traceCompareOptions := []ptracetest.CompareTracesOption{
		ptracetest.IgnoreStartTimestamp(),
		ptracetest.IgnoreEndTimestamp(),
		ptracetest.IgnoreTraceID(),
		ptracetest.IgnoreSpanID(),
		ptracetest.IgnoreEndTimestamp(),
		ptracetest.IgnoreSpansOrder(),
	}

	expectedTraces, err := golden.ReadTraces(expectedTracesFile)
	require.NoError(t, err)

	require.EventuallyWithT(t, func(tt *assert.CollectT) {
		gotTraces := tracesConsumer.AllTraces()[len(tracesConsumer.AllTraces())-1]
		testutil.MaskParentSpanID(expectedTraces)
		testutil.MaskParentSpanID(gotTraces)
		assert.NoError(tt,
			ptracetest.CompareTraces(
				expectedTraces,
				gotTraces,
				traceCompareOptions...,
			),
		)
	}, 3*time.Minute, 1*time.Second)

	t.Logf("Traces checked successfully")
}
