//go:build e2e

package self_monitoring_prometheus

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
	"github.com/google/uuid"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetricassert"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	otelk8stest "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/xk8stest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
)

// Test_Selfmonitoring_Prometheus_checkMetrics verifies the full self-monitoring
// metrics set when the collector exposes selfmon via Prometheus and scrapes it
// with a Prometheus receiver, exporting the result via OTLP (redirected to sinks by overlays).
func Test_Selfmonitoring_Prometheus_checkMetrics(t *testing.T) {
	testNs := "e2eselfmonitoringpromcheck"

	// Use a dedicated golden for Prometheus path (it will differ from OTLP push path)
	expectedFile := "./testdata/e2e/expected-prom-check.yaml"
	expectedAssertionFile := "./testdata/e2e/expected-prom-check.assert.yaml"

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

	// Create namespace
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

	// Sink for internal metrics (self-monitoring)
	metricsConsumer := new(consumertest.MetricsSink)
	// Sinks for telemetrygen data
	telemetrygenMetricsConsumer := new(consumertest.MetricsSink)

	shutdownSinks := oteltest.StartUpSinks(t, oteltest.ReceiverSinks{
		Metrics: []*oteltest.MetricSinkConfig{
			{
				Consumer: metricsConsumer,
			},
			{
				Consumer: telemetrygenMetricsConsumer,
				Ports: &oteltest.ReceiverPorts{
					Http: 4320,
				},
			},
		},
	})
	defer shutdownSinks()

	collectorConfigPath := path.Join(configExamplesDir, "self-monitoring-prometheus-check-metrics.yaml")

	// Read overlay from files
	localOverlay := fmt.Sprintf(k8stest.MustRead(t, filepath.Join(testDir, "config-overlays", "prom-selfmon-local.yaml")), host)

	collectorConfig, err := k8stest.GetCollectorConfig(collectorConfigPath, k8stest.ConfigTemplate{
		Host: host,
		Templates: []string{
			localOverlay,
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
			"K8sCluster":        "cluster-" + testNs,
			"Namespace":         testNs,
		},
		host,
	)

	createTeleOpts := &otelk8stest.TelemetrygenCreateOpts{
		ManifestsDir: filepath.Join(testDir, "telemetrygen"),
		TestID:       testID,
		OtlpEndpoint: fmt.Sprintf("otelcol-%s", testID),
		DataTypes:    []string{"metrics"},
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

	// testing data creating load
	oteltest.WaitForMetrics(t, 1, telemetrygenMetricsConsumer)

	// self monitoring metrics
	oteltest.WaitForMetrics(t, 5, metricsConsumer)

	actual := metricsConsumer.AllMetrics()[len(metricsConsumer.AllMetrics())-1]

	dpIgnoreList := []string{
		"server.address",
		"server.port",
	}
	resourceIgnoreList := []string{
		"k8s.node.name",
		"k8s.pod.name",
		"service.instance.id",
		"service.version",
	}

	testutil.ReplaceAttrValsWithStar(actual, resourceIgnoreList, dpIgnoreList)

	// To regenerate: uncomment, run the test once, re-comment.
	// require.NoError(t, pmetricassert.WriteAssertionFile(t, expectedAssertionFile, actual))

	require.EventuallyWithT(t, func(tt *assert.CollectT) {
		err := pmetricassert.AssertMetrics(expectedAssertionFile, actual)
		assert.NoError(tt, err)
	}, 3*time.Minute, 1*time.Second)

	// Uncomment to regenerate golden:
	// require.NoError(t, golden.WriteMetrics(t, expectedFile, metricsConsumer.AllMetrics()[len(metricsConsumer.AllMetrics())-1]))

	expected, err := golden.ReadMetrics(expectedFile)
	require.NoError(t, err)

	defaultOptions := []pmetrictest.CompareMetricsOption{
		pmetrictest.IgnoreResourceAttributeValue("service.instance.id"),
		pmetrictest.IgnoreResourceAttributeValue("k8s.pod.name"),
		pmetrictest.IgnoreResourceAttributeValue("k8s.node.name"),
		pmetrictest.IgnoreResourceAttributeValue("server.port"),
		pmetrictest.IgnoreResourceAttributeValue("url.scheme"),
		pmetrictest.IgnoreResourceAttributeValue("service.version"),
		pmetrictest.IgnoreMetricAttributeValue("server.address"),
		pmetrictest.IgnoreMetricAttributeValue("server.port"),
		pmetrictest.IgnoreTimestamp(),
		pmetrictest.IgnoreStartTimestamp(),
		pmetrictest.IgnoreMetricDataPointsOrder(),
		pmetrictest.IgnoreScopeMetricsOrder(),
		pmetrictest.IgnoreResourceMetricsOrder(),
		pmetrictest.IgnoreMetricsOrder(),
		pmetrictest.IgnoreMetricValues(),
		pmetrictest.IgnoreScopeVersion(),
	}

	require.EventuallyWithT(t, func(tt *assert.CollectT) {
		actual := metricsConsumer.AllMetrics()[len(metricsConsumer.AllMetrics())-1]
		err := pmetrictest.CompareMetrics(expected, actual, defaultOptions...)
		assert.NoError(tt, err)
	}, 3*time.Minute, 1*time.Second)
}
