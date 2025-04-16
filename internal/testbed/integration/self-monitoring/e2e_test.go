//go:build e2e

package selfmonitoring

import (
	"fmt"
	"github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/k8stest"
	oteltest "github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/oteltest"
	"github.com/google/uuid"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	otelk8stest "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/xk8stest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"os"
	"path"
	"path/filepath"
	"testing"
	"time"
)

func Test_Selfmonitoring_withK8sEnrichment(t *testing.T) {
	testNs := "e2eselfmonitoringk8senrich"
	expectedk8sEnrichResourceAttributes := map[string]oteltest.ExpectedValue{
		"k8s.pod.name":                oteltest.NewExpectedValue(oteltest.AttributeMatchTypeRegex, "otelcol-.*"),
		"k8s.pod.uid":                 oteltest.NewExpectedValue(oteltest.AttributeMatchTypeRegex, oteltest.UidRe),
		"k8s.namespace.name":          oteltest.NewExpectedValue(oteltest.AttributeMatchTypeEqual, testNs),
		"k8s.node.name":               oteltest.NewExpectedValue(oteltest.AttributeMatchTypeExist, ""),
		"k8s.cluster.uid":             oteltest.NewExpectedValue(oteltest.AttributeMatchTypeRegex, oteltest.UidRe),
		"k8s.pod.ip":                  oteltest.NewExpectedValue(oteltest.AttributeMatchTypeRegex, oteltest.IPRe),
		"k8s.workload.kind":           oteltest.NewExpectedValue(oteltest.AttributeMatchTypeEqual, "deployment"),
		"k8s.workload.name":           oteltest.NewExpectedValue(oteltest.AttributeMatchTypeRegex, "otelcol-.*"),
		oteltest.ServiceNameAttribute: oteltest.NewExpectedValue(oteltest.AttributeMatchTypeEqual, "dynatrace-otel-collector"),
		"service.instance.id":         oteltest.NewExpectedValue(oteltest.AttributeMatchTypeRegex, oteltest.UidRe),
		"service.version":             oteltest.NewExpectedValue(oteltest.AttributeMatchTypeExist, ""),
	}
	selfMonitoring_general(t, "self-monitoring-k8s-enrich.yaml", expectedk8sEnrichResourceAttributes, testNs)
}

func Test_Selfmonitoring(t *testing.T) {
	testNs := "e2eselfmonitoring"
	expectedResourceAttributes := map[string]oteltest.ExpectedValue{
		"k8s.pod.name":                oteltest.NewExpectedValue(oteltest.AttributeMatchTypeRegex, "otelcol-.*"),
		"k8s.namespace.name":          oteltest.NewExpectedValue(oteltest.AttributeMatchTypeEqual, testNs),
		"k8s.node.name":               oteltest.NewExpectedValue(oteltest.AttributeMatchTypeExist, ""),
		oteltest.ServiceNameAttribute: oteltest.NewExpectedValue(oteltest.AttributeMatchTypeEqual, "dynatrace-otel-collector"),
		"service.instance.id":         oteltest.NewExpectedValue(oteltest.AttributeMatchTypeRegex, oteltest.UidRe),
		"service.version":             oteltest.NewExpectedValue(oteltest.AttributeMatchTypeExist, ""),
	}
	selfMonitoring_general(t, "self-monitoring.yaml", expectedResourceAttributes, testNs)
}

func selfMonitoring_general(t *testing.T, configPath string, expectedAttributes map[string]oteltest.ExpectedValue, testNs string) {
	testDir := filepath.Join("testdata")
	configExamplesDir := "../../../../config_examples"

	kubeconfigPath := k8stest.TestKubeConfig
	if kubeConfigFromEnv := os.Getenv(k8stest.KubeConfigEnvVar); kubeConfigFromEnv != "" {
		kubeconfigPath = kubeConfigFromEnv
	}

	k8sClient, err := otelk8stest.NewK8sClient(kubeconfigPath)
	require.NoError(t, err)

	testID := uuid.NewString()[:8]
	host := otelk8stest.HostEndpoint(t)

	// Create the namespace specific for the test
	nsObjs := otelk8stest.CreateCollectorObjects(
		t,
		k8sClient,
		testID,
		filepath.Join(testDir, "namespace"),
		map[string]string{
			"Namespace": testNs,
		},
		host,
	)

	defer func() {
		for _, obj := range nsObjs {
			require.NoErrorf(t, otelk8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
		}
	}()

	metricsConsumer := new(consumertest.MetricsSink)
	shutdownSinks := oteltest.StartUpSinks(t, oteltest.ReceiverSinks{
		Metrics: &oteltest.MetricSinkConfig{
			Consumer: metricsConsumer,
		},
	})
	defer shutdownSinks()

	collectorConfigPath := path.Join(configExamplesDir, configPath)
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
			"K8sCluster":        "cluster-" + testNs,
			"Namespace":         testNs,
		},
		host,
	)

	defer func() {
		for _, obj := range collectorObjs {
			require.NoErrorf(t, otelk8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
		}
	}()

	wantEntries := 1 // Minimal number of metrics to wait for.
	oteltest.WaitForMetrics(t, wantEntries, metricsConsumer)

	metrics := pmetric.NewMetrics()
	resource := metrics.ResourceMetrics().AppendEmpty()

	scope1 := resource.ScopeMetrics().AppendEmpty()
	scope1.Scope().SetName("go.opentelemetry.io/collector/exporter/exporterhelper")
	scope1.Metrics().AppendEmpty().SetName("otelcol_exporter_queue_capacity")
	scope1.Metrics().AppendEmpty().SetName("otelcol_exporter_queue_size")

	scope2 := resource.ScopeMetrics().AppendEmpty()
	scope2.Scope().SetName("go.opentelemetry.io/collector/service")
	scope2.Metrics().AppendEmpty().SetName("otelcol_process_cpu_seconds")
	scope2.Metrics().AppendEmpty().SetName("otelcol_process_memory_rss")
	scope2.Metrics().AppendEmpty().SetName("otelcol_process_runtime_heap_alloc_bytes")
	scope2.Metrics().AppendEmpty().SetName("otelcol_process_runtime_total_alloc_bytes")
	scope2.Metrics().AppendEmpty().SetName("otelcol_process_runtime_total_sys_memory_bytes")
	scope2.Metrics().AppendEmpty().SetName("otelcol_process_uptime")

	m := metricsConsumer.AllMetrics()[0]
	require.NoError(t, oteltest.AssertExpectedAttributes(m.ResourceMetrics().At(0).Resource().Attributes(), expectedAttributes))

	for i := 0; i < m.ResourceMetrics().At(0).ScopeMetrics().Len(); i++ {
		s := m.ResourceMetrics().At(0).ScopeMetrics().At(i)
		if s.Scope().Name() == scope1.Scope().Name() {
			for j := 0; j < s.Metrics().Len(); j++ {
				require.True(t, isMetricPresent(s.Metrics().At(j), scope1.Metrics()), "metric with name %s not found in expected output", s.Metrics().At(j).Name())
			}
		} else if s.Scope().Name() == scope2.Scope().Name() {
			for j := 0; j < s.Metrics().Len(); j++ {
				require.True(t, isMetricPresent(s.Metrics().At(j), scope2.Metrics()), "metric with name %s not found in expected output", s.Metrics().At(j).Name())
			}
		}
	}
}

func isMetricPresent(m pmetric.Metric, expected pmetric.MetricSlice) bool {
	for j := 0; j < expected.Len(); j++ {
		if m.Name() == expected.At(j).Name() {
			return true
		}
	}
	return false
}

func Test_Selfmonitoring_checkMetrics(t *testing.T) {
	testNs := "e2eselfmonitoringcheckmetrics"
	expectedFile := "./testdata/e2e/expected-check.yaml"
	testDir := filepath.Join("testdata")
	configExamplesDir := "../../../../config_examples"

	kubeconfigPath := k8stest.TestKubeConfig
	if kubeConfigFromEnv := os.Getenv(k8stest.KubeConfigEnvVar); kubeConfigFromEnv != "" {
		kubeconfigPath = kubeConfigFromEnv
	}

	k8sClient, err := otelk8stest.NewK8sClient(kubeconfigPath)
	require.NoError(t, err)

	testID := uuid.NewString()[:8]
	host := otelk8stest.HostEndpoint(t)

	// Create the namespace specific for the test
	nsObjs := otelk8stest.CreateCollectorObjects(
		t,
		k8sClient,
		testID,
		filepath.Join(testDir, "namespace"),
		map[string]string{
			"Namespace": testNs,
		},
		host,
	)

	defer func() {
		for _, obj := range nsObjs {
			require.NoErrorf(t, otelk8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
		}
	}()

	metricsConsumer := new(consumertest.MetricsSink)
	shutdownSinks := oteltest.StartUpSinks(t, oteltest.ReceiverSinks{
		Metrics: &oteltest.MetricSinkConfig{
			Consumer: metricsConsumer,
		},
	})
	defer shutdownSinks()

	collectorConfigPath := path.Join(configExamplesDir, "self-monitoring-check-metrics.yaml")
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
			"K8sCluster":        "cluster-" + testNs,
			"Namespace":         testNs,
		},
		host,
	)

	createTeleOpts := &otelk8stest.TelemetrygenCreateOpts{
		ManifestsDir: filepath.Join(testDir, "telemetrygen"),
		TestID:       testID,
		OtlpEndpoint: fmt.Sprintf("otelcol-%s.%s:4317", testID, testNs),
		DataTypes:    []string{"traces", "metrics", "logs"},
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

	wantEntries := 5 // Minimal number of metrics to wait for.
	oteltest.WaitForMetrics(t, wantEntries, metricsConsumer)

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
