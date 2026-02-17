//go:build e2e

package k8scluster

import (
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
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

func TestE2E_K8sClusterReceiver(t *testing.T) {
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
	collectorConfigPath := path.Join(configExamplesDir, "k8scluster.yaml")
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
			"k8s.node.allocatable_cpu",
			"k8s.node.allocatable_memory",
			"k8s.namespace.phase",
			"k8s.node.condition_ready",
			"k8s.replicaset.available",
			"k8s.container.ready",
			"k8s.replicaset.desired",
			"k8s.deployment.available",
			"k8s.container.restarts",
			"k8s.daemonset.ready_nodes",
			"k8s.daemonset.current_scheduled_nodes",
			"k8s.daemonset.desired_scheduled_nodes",
			"k8s.pod.phase",
			"k8s.daemonset.misscheduled_nodes",
			"k8s.deployment.desired",
			"k8s.node.allocatable_pods"),
		pmetrictest.IgnoreScopeVersion(),

		pmetrictest.ChangeResourceAttributeValue("k8s.daemonset.uid", substituteWithStar),
		pmetrictest.ChangeResourceAttributeValue("k8s.deployment.uid", substituteWithStar),
		pmetrictest.ChangeResourceAttributeValue("k8s.namespace.uid", substituteWithStar),
		pmetrictest.ChangeResourceAttributeValue("k8s.node.uid", substituteWithStar),
		pmetrictest.ChangeResourceAttributeValue("k8s.pod.uid", substituteWithStar),
		pmetrictest.ChangeResourceAttributeValue("k8s.replicaset.uid", substituteWithStar),
		pmetrictest.ChangeResourceAttributeValue("container.id", substituteWithStar),
		pmetrictest.ChangeResourceAttributeValue("container.image.tag", substituteWithStar),
		pmetrictest.ChangeResourceAttributeValue("container.image.name", substituteLocalhostImagePrefix),

		pmetrictest.ChangeResourceAttributeValue("container.image.name", substituteRandomPartWithStar),
		pmetrictest.ChangeResourceAttributeValue("k8s.container.name", substituteRandomPartWithStar),
		pmetrictest.ChangeResourceAttributeValue("k8s.daemonset.name", substituteRandomPartWithStar),
		pmetrictest.ChangeResourceAttributeValue("k8s.deployment.name", substituteRandomPartWithStar),
		pmetrictest.ChangeResourceAttributeValue("k8s.namespace.name", substituteRandomPartWithStar),
		pmetrictest.ChangeResourceAttributeValue("k8s.node.name", substituteRandomPartWithStar),
		pmetrictest.ChangeResourceAttributeValue("k8s.pod.name", substituteRandomPartWithStar),
		pmetrictest.ChangeResourceAttributeValue("k8s.replicaset.name", substituteRandomPartWithStar),

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
	re := regexp.MustCompile(`(-[a-z0-9]{8,10})?(-[a-z0-9]+)?(-[a-z0-9]{5})?$`)
	return re.ReplaceAllString(s, "-*")
}

func substituteLocalhostImagePrefix(s string) string {
	return strings.Replace(s, "localhost/", "docker.io/library/", 1)
}
