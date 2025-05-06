package kubeletstats

import (
	"os"
	"path"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/k8stest"
	"github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/oteltest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	otelk8stest "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/xk8stest"
)

func TestE2E_KubeletstatsReceiver(t *testing.T) {
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

	metricsConsumer := new(consumertest.MetricsSink)
	shutdownSinks := oteltest.StartUpSinks(t, oteltest.ReceiverSinks{
		Metrics: &oteltest.MetricSinkConfig{
			Consumer: metricsConsumer,
		},
	})
	defer shutdownSinks()

	// create collector
	testID := uuid.NewString()[:8]
	collectorConfigPath := path.Join(configExamplesDir, "kubeletstats.yaml")
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

	defer func() {
		for _, obj := range collectorObjs {
			require.NoErrorf(t, otelk8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
		}
	}()

	// time.Sleep(5 * time.Minute)
	// return

	oteltest.WaitForMetrics(t, 10, metricsConsumer)

	// the commented line below writes the received list of metrics to the expected.yaml
	// require.Nil(t, golden.WriteMetrics(t, expectedFile, metricsConsumer.AllMetrics()[len(metricsConsumer.AllMetrics())-1]))

	var expected pmetric.Metrics
	expected, err = golden.ReadMetrics(expectedFile)
	require.NoError(t, err)

	defaultOptions := []pmetrictest.CompareMetricsOption{
		pmetrictest.IgnoreTimestamp(),
		pmetrictest.IgnoreStartTimestamp(),
		pmetrictest.IgnoreMetricValues(
			"otelcol_processor_filter_datapoints.filtered",
			"otelcol_processor_filter_logs.filtered",
			"otelcol_processor_filter_spans.filtered",
			"otelcol_receiver_accepted_log_records",
			"otelcol_receiver_accepted_metric_points",
			"otelcol_receiver_accepted_spans",
			"otelcol_receiver_refused_log_records",
			"otelcol_receiver_refused_metric_points",
			"otelcol_receiver_refused_spans",
			"otelcol_process_cpu_seconds",
			"otelcol_process_memory_rss",
			"otelcol_process_runtime_heap_alloc_bytes",
			"otelcol_process_runtime_total_alloc_bytes",
			"otelcol_process_runtime_total_sys_memory_bytes",
			"otelcol_process_uptime",
			"http.client.request.size",
			"http.client.response.size",
			"http.client.duration",
			"otelcol_processor_batch_batch_send_size",
			"otelcol_processor_batch_batch_send_size_bytes",
			"otelcol_processor_batch_batch_size_trigger_send",
			"otelcol_processor_batch_metadata_cardinality",
			"otelcol_processor_batch_timeout_trigger_send",
			"otelcol_processor_incoming_items",
			"otelcol_processor_outgoing_items",
			"rpc.server.duration",
			"rpc.server.request.size",
			"rpc.server.response.size",
			"rpc.server.requests_per_rpc",
			"rpc.server.responses_per_rpc",
			"otelcol_exporter_queue_capacity",
			"otelcol_exporter_queue_size",
			"otelcol_exporter_send_failed_log_records",
			"otelcol_exporter_send_failed_metric_points",
			"otelcol_exporter_send_failed_spans",
			"otelcol_exporter_sent_log_records",
			"otelcol_exporter_sent_metric_points",
			"otelcol_exporter_sent_spans"),
		pmetrictest.IgnoreScopeVersion(),
		pmetrictest.IgnoreResourceMetricsOrder(),
		pmetrictest.IgnoreMetricsOrder(),
		pmetrictest.IgnoreScopeMetricsOrder(),
		pmetrictest.IgnoreMetricDataPointsOrder(),
		pmetrictest.IgnoreExemplarSlice(),
		pmetrictest.ChangeDatapointAttributeValue("net.peer.name", substituteWithStar),
		pmetrictest.ChangeResourceAttributeValue("k8s.node.name", substituteWithStar),
		pmetrictest.ChangeResourceAttributeValue("k8s.pod.name", substituteWithStar),
		pmetrictest.ChangeResourceAttributeValue("service.instance.id", substituteWithStar),
		pmetrictest.ChangeResourceAttributeValue("service.version", substituteWithStar),
	}

	require.EventuallyWithT(t, func(tt *assert.CollectT) {
		assert.NoError(tt, pmetrictest.CompareMetrics(expected, metricsConsumer.AllMetrics()[len(metricsConsumer.AllMetrics())-1],
			defaultOptions...,
		),
		)
	}, 3*time.Minute, 1*time.Second)
}

func substituteWithStar(_ string) string { return "*" }
