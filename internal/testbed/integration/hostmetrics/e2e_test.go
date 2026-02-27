//go:build e2e

package hostmetrics

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"

	"github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/k8stest"
	"github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/oteltest"
	"github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	otelk8stest "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/xk8stest"
)

// TestE2E_HostMetricsReceiver validates the Host Metrics Receiver functionality
// against expected data collected from a GitHub runner machine. Therefore, local test results can vary based on
// different metrics being produced based on the underlying OS.
func TestE2E_HostMetricsReceiver(t *testing.T) {
	testDir := filepath.Join("testdata")
	expectedFile1m := testDir + "/e2e/expected-1m.yaml"
	expectedFile5m := testDir + "/e2e/expected-5m.yaml"
	expectedFile1h := testDir + "/e2e/expected-1h.yaml"
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

	metricsConsumer1m := new(consumertest.MetricsSink)
	metricsConsumer5m := new(consumertest.MetricsSink)
	metricsConsumer1h := new(consumertest.MetricsSink)
	logsConsumer := new(consumertest.LogsSink)

	shutdownSinks := oteltest.StartUpSinks(t, oteltest.ReceiverSinks{
		Metrics: []*oteltest.MetricSinkConfig{
			{
				Consumer: metricsConsumer1m,
				Ports: &oteltest.ReceiverPorts{
					Http: 4320,
				},
			},
			{
				Consumer: metricsConsumer5m,
				Ports: &oteltest.ReceiverPorts{
					Http: 4321,
				},
			},
			{
				Consumer: metricsConsumer1h,
				Ports: &oteltest.ReceiverPorts{
					Http: 4322,
				},
			},
		},
		Logs: []*oteltest.LogSinkConfig{
			{
				Consumer: logsConsumer,
				Ports: &oteltest.ReceiverPorts{
					Http: 4323,
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
	collectorConfigPath := path.Join(configExamplesDir, "host-metrics.yaml")
	host := otelk8stest.HostEndpoint(t)

	// Read overlay from file
	envOverlay := k8stest.MustRead(t, filepath.Join(testDir, "config-overlays", "service-env.yaml"))
	localOverlay := fmt.Sprintf(k8stest.MustRead(t, filepath.Join(testDir, "config-overlays", "service-local.yaml")), host)
	intervalOverlay1m := k8stest.MustRead(t, filepath.Join(testDir, "config-overlays", "collection-interval-1m.yaml"))
	intervalOverlay5m := k8stest.MustRead(t, filepath.Join(testDir, "config-overlays", "collection-interval-5m.yaml"))
	intervalOverlay1h := k8stest.MustRead(t, filepath.Join(testDir, "config-overlays", "collection-interval-1h.yaml"))
	shortIntervalOverlay := k8stest.MustRead(t, filepath.Join(testDir, "config-overlays", "collection-interval-short.yaml"))

	collectorConfig, err := k8stest.GetCollectorConfig(collectorConfigPath, k8stest.ConfigTemplate{
		Host: host,
		Templates: []string{
			envOverlay,
			localOverlay,
			intervalOverlay1m,
			shortIntervalOverlay,
			intervalOverlay5m,
			shortIntervalOverlay,
			intervalOverlay1h,
			shortIntervalOverlay,
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

	defer func() {
		for _, obj := range collectorObjs {
			require.NoErrorf(t, otelk8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
		}
	}()

	// Create Telemetry Generator
	createTeleOpts := &otelk8stest.TelemetrygenCreateOpts{
		ManifestsDir: filepath.Join(testDir, "telemetrygen"),
		TestID:       testID,
		OtlpEndpoint: fmt.Sprintf("otelcol-%s.%s:4317", testID, testNs),
		DataTypes:    []string{"logs"},
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

	// Compare timeouts
	const (
		compareTimeout = 3 * time.Minute
		compareTick    = 5 * time.Second
	)

	defaultOptions := []pmetrictest.CompareMetricsOption{
		pmetrictest.IgnoreTimestamp(),
		pmetrictest.IgnoreStartTimestamp(),
		pmetrictest.IgnoreMetricValues(
			"system.uptime",
			"system.paging.faults",
			"system.paging.operations",
			"system.paging.usage",
			"system.processes.count",
			"system.processes.created",
			"system.network.connections",
			"system.network.dropped",
			"system.network.errors",
			"system.network.io",
			"system.network.packets",
			"system.memory.usage",
			"system.memory.utilization",
			"system.filesystem.inodes.usage",
			"system.filesystem.usage",
			"system.filesystem.utilization",
			"system.cpu.logical.count",
			"system.cpu.physical.count",
			"system.cpu.time",
			"system.cpu.utilization",
			"system.cpu.load_average.1m",
			"system.cpu.load_average.5m",
			"system.cpu.load_average.15m",
			"system.disk.io",
			"system.disk.io_time",
			"system.disk.operation_time",
			"system.disk.operations",
			"process.cpu.time",
			"process.cpu.utilization",
			"process.disk.io",
			"process.memory.usage",
			"process.memory.virtual",
			"system.memory.limit"),
		pmetrictest.IgnoreScopeVersion(),

		pmetrictest.ChangeResourceAttributeValue("host.arch", substituteWithStar),
		pmetrictest.ChangeResourceAttributeValue("host.ip", substituteWithStar),
		pmetrictest.ChangeResourceAttributeValue("host.mac", substituteWithStar),
		pmetrictest.ChangeResourceAttributeValue("host.name", substituteWithStar),
		pmetrictest.ChangeResourceAttributeValue("host.cpu.model.name", substituteWithStar),
		pmetrictest.ChangeResourceAttributeValue("host.interface", substituteWithStar),
		pmetrictest.ChangeResourceAttributeValue("os.type", substituteWithStar),
		pmetrictest.ChangeResourceAttributeValue("os.description", substituteWithStar),
		pmetrictest.ChangeResourceAttributeValue("os.build.id", substituteWithStar),
		pmetrictest.ChangeResourceAttributeValue("os.name", substituteWithStar),
		pmetrictest.ChangeResourceAttributeValue("os.version", substituteWithStar),

		pmetrictest.ChangeResourceAttributeValue("process.executable.name", substituteWithStar),

		pmetrictest.ChangeDatapointAttributeValue("mountpoint", substituteWithStar),
		pmetrictest.ChangeDatapointAttributeValue("direction", substituteWithStar),
		pmetrictest.ChangeDatapointAttributeValue("cpu", substituteWithStar),
		pmetrictest.ChangeDatapointAttributeValue("state", substituteWithStar),
		pmetrictest.ChangeDatapointAttributeValue("interface", substituteWithStar),
		pmetrictest.ChangeDatapointAttributeValue("device", substituteWithStar),
		pmetrictest.ChangeDatapointAttributeValue("status", substituteWithStar),

		pmetrictest.IgnoreDatapointAttributesOrder(),
		pmetrictest.IgnoreMetricDataPointsOrder(),
		pmetrictest.IgnoreMetricsOrder(),
		pmetrictest.IgnoreScopeMetricsOrder(),
		pmetrictest.IgnoreResourceMetricsOrder(),
		pmetrictest.IgnoreSubsequentDataPoints(),
	}

	t.Log("Waiting for host metrics...")
	oteltest.WaitForMetrics(t, 1, metricsConsumer1m)
	oteltest.WaitForMetrics(t, 1, metricsConsumer5m)
	oteltest.WaitForMetrics(t, 1, metricsConsumer1h)
	t.Logf("Received metrics on all consumers...")

	// Logs
	t.Logf("Checking logs...")
	oteltest.WaitForLogs(t, 1, logsConsumer)

	t.Log("Logs checked successfully")

	// 1m Metrics
	t.Logf("Checking 1m metrics...")

	// the commented line below writes the received list of metrics to the expected.yaml
	//require.Nil(t, golden.WriteMetrics(t, expectedFile1m, metricsConsumer1m.AllMetrics()[len(metricsConsumer1m.AllMetrics())-1]))
	checkMetrics(t, expectedFile1m, metricsConsumer1m, defaultOptions, compareTimeout, compareTick)

	// 5m Metrics
	t.Logf("Checking 5m metrics...")

	// the commented line below writes the received list of metrics to the expected.yaml
	//require.Nil(t, golden.WriteMetrics(t, expectedFile5m, metricsConsumer5m.AllMetrics()[len(metricsConsumer5m.AllMetrics())-1]))
	checkMetrics(t, expectedFile5m, metricsConsumer5m, defaultOptions, compareTimeout, compareTick)

	// 1h Metrics
	t.Logf("Checking 1h metrics...")

	// the commented line below writes the received list of metrics to the expected.yaml
	//require.Nil(t, golden.WriteMetrics(t, expectedFile1h, metricsConsumer1h.AllMetrics()[len(metricsConsumer1h.AllMetrics())-1]))
	checkMetrics(t, expectedFile1h, metricsConsumer1h, defaultOptions, compareTimeout, compareTick)

	t.Log("Host metrics checked successfully")
}

func checkMetrics(t *testing.T, expectedFile string, consumer *consumertest.MetricsSink, options []pmetrictest.CompareMetricsOption, timeout, tick time.Duration) {
	expectedMetrics, err := golden.ReadMetrics(expectedFile)
	require.NoError(t, err)

	expectedMerged := testutil.MergeResources(expectedMetrics)
	require.EventuallyWithT(t, func(tt *assert.CollectT) {
		assert.NoError(tt, pmetrictest.CompareMetrics(expectedMerged, testutil.MergeResources(consumer.AllMetrics()[len(consumer.AllMetrics())-1]),
			options...,
		),
		)
	}, timeout, tick)
}

func substituteWithStar(_ string) string { return "*" }
