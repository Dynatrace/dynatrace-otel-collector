//go:build e2e

package hostmetrics

import (
	"os"
	"path"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/k8stest"
	"github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/oteltest"
	"github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	otelk8stest "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/xk8stest"
)

func TestE2E_HostMetricsReceiver(t *testing.T) {
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
		Metrics: []*oteltest.MetricSinkConfig{
			{
				Consumer: metricsConsumer,
			},
		},
	})
	defer shutdownSinks()

	// create collector
	testID, err := testutil.GenerateRandomString(10)
	require.NoError(t, err)
	collectorConfigPath := path.Join(configExamplesDir, "host-metrics.yaml")
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

	defer func() {
		for _, obj := range collectorObjs {
			require.NoErrorf(t, otelk8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
		}
	}()

	t.Log("Waiting for host metrics...")

	oteltest.WaitForMetrics(t, 3, metricsConsumer)

	t.Log("Checking host metrics...")

	// the commented line below writes the received list of metrics to the expected.yaml
	// require.Nil(t, golden.WriteMetrics(t, expectedFile, metricsConsumer.AllMetrics()[len(metricsConsumer.AllMetrics())-1]))

	var expected pmetric.Metrics
	expected, err = golden.ReadMetrics(expectedFile)
	require.NoError(t, err)

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
			"system.memory.limit",
			"system.memory.usage",
			"system.memory.utilization",
			"system.filesystem.inodes.usage",
			"system.filesystem.usage",
			"system.filesystem.utilization",
			"system.cpu.logical.count",
			"system.cpu.physical.count",
			"system.cpu.time",
			"system.cpu.utilization",
			"system.disk.io",
			"system.disk.io_time",
			"system.disk.merged",
			"system.disk.operation_time",
			"system.disk.operations",
			"system.disk.pending_operations",
			"system.disk.weighted_io_time",
			"process.cpu.time",
			"process.cpu.utilization",
			"process.disk.io",
			"process.memory.usage",
			"process.memory.virtual"),
		pmetrictest.IgnoreScopeVersion(),

		pmetrictest.ChangeResourceAttributeValue("host.arch", substituteWithStar),
		pmetrictest.ChangeResourceAttributeValue("host.ip", substituteWithStar),
		pmetrictest.ChangeResourceAttributeValue("host.mac", substituteWithStar),
		pmetrictest.ChangeResourceAttributeValue("host.name", substituteWithStar),
		pmetrictest.ChangeResourceAttributeValue("os.type", substituteWithStar),

		pmetrictest.ChangeResourceAttributeValue("process.command", substituteWithStar),
		pmetrictest.ChangeResourceAttributeValue("process.command_line", substituteWithStar),
		pmetrictest.ChangeResourceAttributeValue("process.executable.name", substituteWithStar),
		pmetrictest.ChangeResourceAttributeValue("process.executable.path", substituteWithStar),
		pmetrictest.ChangeResourceAttributeValue("process.parent_pid", substituteWithStar),
		pmetrictest.ChangeResourceAttributeValue("process.pid", substituteWithStar),

		pmetrictest.ChangeDatapointAttributeValue("device", substituteWithStar),
		pmetrictest.ChangeDatapointAttributeValue("mode", substituteWithStar),
		pmetrictest.ChangeDatapointAttributeValue("mountpoint", substituteWithStar),
		pmetrictest.ChangeDatapointAttributeValue("direction", substituteWithStar),
		pmetrictest.ChangeDatapointAttributeValue("type", substituteWithStar),
		pmetrictest.ChangeDatapointAttributeValue("cpu", substituteWithStar),
		pmetrictest.ChangeDatapointAttributeValue("state", substituteWithStar),

		pmetrictest.IgnoreDatapointAttributesOrder(),
		pmetrictest.IgnoreMetricDataPointsOrder(),
		pmetrictest.IgnoreMetricsOrder(),
		pmetrictest.IgnoreScopeMetricsOrder(),
		pmetrictest.IgnoreResourceMetricsOrder(),
		pmetrictest.IgnoreSubsequentDataPoints(),
	}

	expectedMerged := testutil.MergeResources(expected)
	require.EventuallyWithT(t, func(tt *assert.CollectT) {
		assert.NoError(tt, pmetrictest.CompareMetrics(expectedMerged, testutil.MergeResources(metricsConsumer.AllMetrics()[len(metricsConsumer.AllMetrics())-1]),
			defaultOptions...,
		),
		)
	}, 3*time.Minute, 5*time.Second)

	t.Log("Host metrics checked successfully")
}

func TestE2E_HostMetricsExtension(t *testing.T) {
	testDir := filepath.Join("testdata")
	expectedFile1m := testDir + "/e2e/expected-host-extension-1m.yaml"
	expectedFile5m := testDir + "/e2e/expected-host-extension-5m.yaml"
	expectedFile1h := testDir + "/e2e/expected-host-extension-1h.yaml"
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
	collectorConfigPath := path.Join(configExamplesDir, "host-metrics-extension.yaml")
	host := otelk8stest.HostEndpoint(t)

	// Read overlay from file
	serviceOverlay := k8stest.MustRead(t, filepath.Join(testDir, "config-overlays", "service.yaml"))

	collectorConfig, err := k8stest.GetCollectorConfig(collectorConfigPath, k8stest.ConfigTemplate{
		Host: host,
		Templates: []string{
			serviceOverlay,
		},
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

	defer func() {
		for _, obj := range collectorObjs {
			require.NoErrorf(t, otelk8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
		}
	}()

	// Compare timeouts
	const (
		compareTimeout   = 3 * time.Minute
		compareTick      = 5 * time.Second
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
			"process.memory.virtual"),
		pmetrictest.IgnoreScopeVersion(),

		pmetrictest.ChangeResourceAttributeValue("host.arch", substituteWithStar),
		pmetrictest.ChangeResourceAttributeValue("host.ip", substituteWithStar),
		pmetrictest.ChangeResourceAttributeValue("host.mac", substituteWithStar),
		pmetrictest.ChangeResourceAttributeValue("host.name", substituteWithStar),
		pmetrictest.ChangeResourceAttributeValue("os.type", substituteWithStar),

		pmetrictest.ChangeResourceAttributeValue("process.command_line", substituteWithStar),
		pmetrictest.ChangeResourceAttributeValue("process.executable.name", substituteWithStar),
		pmetrictest.ChangeResourceAttributeValue("process.pid", substituteWithStar),

		pmetrictest.ChangeDatapointAttributeValue("mountpoint", substituteWithStar),
		pmetrictest.ChangeDatapointAttributeValue("direction", substituteWithStar),
		pmetrictest.ChangeDatapointAttributeValue("cpu", substituteWithStar),
		pmetrictest.ChangeDatapointAttributeValue("state", substituteWithStar),
		pmetrictest.ChangeDatapointAttributeValue("interface", substituteWithStar),

		pmetrictest.IgnoreDatapointAttributesOrder(),
		pmetrictest.IgnoreMetricDataPointsOrder(),
		pmetrictest.IgnoreMetricsOrder(),
		pmetrictest.IgnoreScopeMetricsOrder(),
		pmetrictest.IgnoreResourceMetricsOrder(),
		pmetrictest.IgnoreSubsequentDataPoints(),
	}

	t.Log("Checking host metrics...")

	// 1m Metrics
	t.Logf("Checking 1m metrics...")
	oteltest.WaitForMetrics(t, 1, metricsConsumer1m)

	// the commented line below writes the received list of metrics to the expected.yaml
	require.Nil(t, golden.WriteMetrics(t, expectedFile1m, metricsConsumer1m.AllMetrics()[len(metricsConsumer1m.AllMetrics())-1]))

	expectedMetrics1m, err := golden.ReadMetrics(expectedFile1m)
	require.NoError(t, err)

	expectedMerged1m := testutil.MergeResources(expectedMetrics1m)
	require.EventuallyWithT(t, func(tt *assert.CollectT) {
		assert.NoError(tt, pmetrictest.CompareMetrics(expectedMerged1m, testutil.MergeResources(metricsConsumer1m.AllMetrics()[len(metricsConsumer1m.AllMetrics())-1]),
			defaultOptions...,
		),
		)
	}, compareTimeout, compareTick)

	// 5m Metrics
	t.Logf("Checking 5m metrics...")
	oteltest.WaitForMetrics(t, 1, metricsConsumer5m)

	// the commented line below writes the received list of metrics to the expected.yaml
	require.Nil(t, golden.WriteMetrics(t, expectedFile5m, metricsConsumer5m.AllMetrics()[len(metricsConsumer5m.AllMetrics())-1]))

	expectedMetrics5m, err := golden.ReadMetrics(expectedFile5m)
	require.NoError(t, err)

	expectedMerged5m := testutil.MergeResources(expectedMetrics5m)
	require.EventuallyWithT(t, func(tt *assert.CollectT) {
		assert.NoError(tt, pmetrictest.CompareMetrics(expectedMerged5m, testutil.MergeResources(metricsConsumer5m.AllMetrics()[len(metricsConsumer5m.AllMetrics())-1]),
			defaultOptions...,
		),
		)
	}, compareTimeout, compareTick)

	t.Log("Host metrics checked successfully")

	// 1h Metrics
	t.Logf("Checking 1h metrics...")
	oteltest.WaitForMetrics(t, 1, metricsConsumer1h)

	// the commented line below writes the received list of metrics to the expected.yaml
	require.Nil(t, golden.WriteMetrics(t, expectedFile1h, metricsConsumer1h.AllMetrics()[len(metricsConsumer1h.AllMetrics())-1]))

	expectedMetrics1h, err := golden.ReadMetrics(expectedFile1h)
	require.NoError(t, err)

	expectedMerged1h := testutil.MergeResources(expectedMetrics1h)
	require.EventuallyWithT(t, func(tt *assert.CollectT) {
		assert.NoError(tt, pmetrictest.CompareMetrics(expectedMerged1h, testutil.MergeResources(metricsConsumer1h.AllMetrics()[len(metricsConsumer1h.AllMetrics())-1]),
			defaultOptions...,
		),
		)
	}, compareTimeout, compareTick)

	t.Log("Host metrics checked successfully")
}

func substituteWithStar(_ string) string { return "*" }
