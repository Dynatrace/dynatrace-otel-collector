package kubeletstats

import (
	"os"
	"path"
	"path/filepath"
	"regexp"
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
	testID, err := testutil.GenerateRandomString(10)
	require.NoError(t, err)
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
			"container.cpu.usage",
			"container.uptime",
			"k8s.container.cpu.node.utilization",
			"k8s.container.cpu_limit_utilization",
			"k8s.container.cpu_request_utilization",
			"k8s.container.memory.node.utilization",
			"k8s.container.memory_limit_utilization",
			"k8s.container.memory_request_utilization",
			"k8s.node.cpu.usage",
			"k8s.node.uptime",
			"k8s.pod.cpu.node.utilization",
			"k8s.pod.cpu.usage",
			"k8s.pod.cpu_limit_utilization",
			"k8s.pod.cpu_request_utilization",
			"k8s.pod.memory.node.utilization",
			"k8s.pod.memory_limit_utilization",
			"k8s.pod.memory_request_utilization",
			"k8s.pod.uptime",
			"k8s.volume.available",
			"k8s.volume.capacity",
			"k8s.volume.inodes.used",
			"k8s.volume.inodes",
			"k8s.volume.inodes.free",
			"container.cpu.time",
			"container.filesystem.available",
			"container.filesystem.capacity",
			"container.filesystem.usage",
			"container.memory.available",
			"container.memory.major_page_faults",
			"container.memory.page_faults",
			"container.memory.rss",
			"container.memory.usage",
			"container.memory.working_set",
			"k8s.node.cpu.time",
			"k8s.node.filesystem.available",
			"k8s.node.filesystem.capacity",
			"k8s.node.filesystem.usage",
			"k8s.node.memory.available",
			"k8s.node.memory.major_page_faults",
			"k8s.node.memory.page_faults",
			"k8s.node.memory.rss",
			"k8s.node.memory.usage",
			"k8s.node.memory.working_set",
			"k8s.node.network.errors",
			"k8s.node.network.io",
			"k8s.pod.cpu.time",
			"k8s.pod.filesystem.available",
			"k8s.pod.filesystem.capacity",
			"k8s.pod.filesystem.usage",
			"k8s.pod.memory.available",
			"k8s.pod.memory.major_page_faults",
			"k8s.pod.memory.page_faults",
			"k8s.pod.memory.rss",
			"k8s.pod.memory.usage",
			"k8s.pod.memory.working_set",
			"k8s.pod.network.errors",
			"k8s.pod.network.io"),
		pmetrictest.IgnoreScopeVersion(),
		pmetrictest.ChangeDatapointAttributeValue("interface", substituteWithStar),
		pmetrictest.ChangeResourceAttributeValue("k8s.pod.uid", substituteWithStar),
		pmetrictest.ChangeResourceAttributeValue("k8s.pod.name", substituteRandomPartWithStar),
		pmetrictest.ChangeResourceAttributeValue("k8s.volume.name", substituteRandomPartWithStar),
		pmetrictest.IgnoreDatapointAttributesOrder(),
		pmetrictest.IgnoreMetricDataPointsOrder(),
		pmetrictest.IgnoreMetricsOrder(),
		pmetrictest.IgnoreScopeMetricsOrder(),
		pmetrictest.IgnoreResourceMetricsOrder(),
	}

	require.EventuallyWithT(t, func(tt *assert.CollectT) {
		assert.NoError(tt, pmetrictest.CompareMetrics(expected, metricsConsumer.AllMetrics()[len(metricsConsumer.AllMetrics())-1],
			defaultOptions...,
		),
		)
	}, 3*time.Minute, 1*time.Second)
}

func substituteWithStar(_ string) string { return "*" }

func substituteRandomPartWithStar(s string) string {
	re := regexp.MustCompile(`(-[a-z0-9]{10})?(-[a-z0-9]{5})?$`)
	return re.ReplaceAllString(s, "-*")
}
