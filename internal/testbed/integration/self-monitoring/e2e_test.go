//go:build e2e

package selfmonitoring

import (
	"go.opentelemetry.io/collector/pdata/pmetric"
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/k8stest"
	oteltest "github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/oteltest"
	"github.com/google/uuid"
	otelk8stest "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/xk8stest"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
)

const testNs = "e2eselfmonitoring"

func Test_Selfmonitoring_withK8sEnrichment(t *testing.T) {
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
	selfMonitoring_general(t, "self-monitoring-k8s-enrich.yaml", expectedk8sEnrichResourceAttributes)
}

func Test_Selfmonitoring_withoutK8sEnrichment(t *testing.T) {
	expectedResourceAttributes := map[string]oteltest.ExpectedValue{
		"k8s.pod.name":                oteltest.NewExpectedValue(oteltest.AttributeMatchTypeRegex, "otelcol-.*"),
		"k8s.namespace.name":          oteltest.NewExpectedValue(oteltest.AttributeMatchTypeEqual, testNs),
		"k8s.node.name":               oteltest.NewExpectedValue(oteltest.AttributeMatchTypeExist, ""),
		oteltest.ServiceNameAttribute: oteltest.NewExpectedValue(oteltest.AttributeMatchTypeEqual, "dynatrace-otel-collector"),
		"service.instance.id":         oteltest.NewExpectedValue(oteltest.AttributeMatchTypeRegex, oteltest.UidRe),
		"service.version":             oteltest.NewExpectedValue(oteltest.AttributeMatchTypeExist, ""),
	}
	selfMonitoring_general(t, "self-monitoring.yaml", expectedResourceAttributes)
}

func selfMonitoring_general(t *testing.T, configPath string, expectedAttributes map[string]oteltest.ExpectedValue) {
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

	testID := uuid.NewString()[:8]
	collectorConfigPath := path.Join(configExamplesDir, configPath)
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
			"K8sCluster":        "cluster-" + testNs,
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
