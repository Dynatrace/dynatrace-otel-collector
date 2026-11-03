// Copyright Dynatrace LLC
// SPDX-License-Identifier: Apache-2.0

package k8sselfmonitoring

import (
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/k8stest"
	oteltest "github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/oteltest"
	"github.com/google/uuid"
	otelk8stest "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/xk8stest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
)

// Test_Selfmonitoring_withK8sEnrichment tests that k8sattributes enriches the self-monitoring
// metrics of the Collector, which it receives on its own OTLP receiver, through pod IP association.
// The env var based examples are covered in ../self-monitoring.
func Test_Selfmonitoring_withK8sEnrichment(t *testing.T) {
	testNs := "e2eselfmonitoringk8senrich"
	expectedAttributes := map[string]oteltest.ExpectedValue{
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
		Metrics: []*oteltest.MetricSinkConfig{
			{
				Consumer: metricsConsumer,
			},
		},
	})
	defer shutdownSinks()

	collectorConfigPath := path.Join(configExamplesDir, "self-monitoring-k8s-enrich.yaml")
	collectorConfig, err := k8stest.GetCollectorConfig(collectorConfigPath, k8stest.ConfigTemplate{
		Host: host,
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
			"Namespace":         testNs,
		},
		host,
	)

	defer func() {
		for _, obj := range collectorObjs {
			require.NoErrorf(t, otelk8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
		}
	}()

	oteltest.WaitForMetrics(t, 1, metricsConsumer)

	m := metricsConsumer.AllMetrics()[0]
	rm := m.ResourceMetrics().At(0)
	require.NoError(t, oteltest.AssertExpectedAttributes(rm.Resource().Attributes(), expectedAttributes))

	var names []string
	for i := 0; i < rm.ScopeMetrics().Len(); i++ {
		sm := rm.ScopeMetrics().At(i)
		for j := 0; j < sm.Metrics().Len(); j++ {
			names = append(names, sm.Metrics().At(j).Name())
		}
	}
	assert.Subset(t, names, []string{
		"otelcol_exporter_queue_capacity",
		"otelcol_exporter_queue_size",
		"otelcol_process_cpu_seconds",
		"otelcol_process_memory_rss",
		"otelcol_process_runtime_heap_alloc_bytes",
		"otelcol_process_runtime_total_alloc_bytes",
		"otelcol_process_runtime_total_sys_memory_bytes",
		"otelcol_process_uptime",
	})
}
