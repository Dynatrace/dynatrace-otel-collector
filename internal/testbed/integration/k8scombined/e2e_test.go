//go:build e2e

package k8scombined

import (
	"fmt"
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

var (
	defaultOptions = []pmetrictest.CompareMetricsOption{
		pmetrictest.IgnoreTimestamp(),
		pmetrictest.IgnoreStartTimestamp(),
		pmetrictest.IgnoreMetricValues(
			"container.cpu.usage",
			"k8s.node.cpu.usage",
			"k8s.pod.cpu.usage",
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
			"k8s.pod.network.io",
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

		pmetrictest.ChangeDatapointAttributeValue("interface", substituteWithStar),
		pmetrictest.ChangeResourceAttributeValue("k8s.pod.uid", substituteWithStar),
		pmetrictest.ChangeResourceAttributeValue("k8s.pod.ip", substituteWithStar),
		pmetrictest.ChangeResourceAttributeValue("k8s.pod.name", substituteRandomPartWithStar),
		pmetrictest.ChangeResourceAttributeValue("k8s.volume.name", substituteRandomPartWithStar),

		pmetrictest.ChangeResourceAttributeValue("k8s.daemonset.uid", substituteWithStar),
		pmetrictest.ChangeResourceAttributeValue("k8s.deployment.uid", substituteWithStar),
		pmetrictest.ChangeResourceAttributeValue("k8s.namespace.uid", substituteWithStar),
		pmetrictest.ChangeResourceAttributeValue("k8s.node.uid", substituteWithStar),
		pmetrictest.ChangeResourceAttributeValue("k8s.replicaset.uid", substituteWithStar),
		pmetrictest.ChangeResourceAttributeValue("k8s.cluster.uid", substituteWithStar),
		pmetrictest.ChangeResourceAttributeValue("container.id", substituteWithStar),
		pmetrictest.ChangeResourceAttributeValue("container.image.tag", substituteWithStar),
		pmetrictest.ChangeResourceAttributeValue("container.image.name", substituteLocalhostImagePrefix),

		pmetrictest.ChangeResourceAttributeValue("container.image.name", substituteRandomPartWithStar),
		pmetrictest.ChangeResourceAttributeValue("k8s.container.name", substituteRandomPartWithStar),
		pmetrictest.ChangeResourceAttributeValue("k8s.daemonset.name", substituteRandomPartWithStar),
		pmetrictest.ChangeResourceAttributeValue("k8s.deployment.name", substituteRandomPartWithStar),
		pmetrictest.ChangeResourceAttributeValue("k8s.namespace.name", substituteRandomPartWithStar),
		pmetrictest.ChangeResourceAttributeValue("k8s.replicaset.name", substituteRandomPartWithStar),
		pmetrictest.ChangeResourceAttributeValue("k8s.workload.name", substituteRandomPartWithStar),

		pmetrictest.IgnoreDatapointAttributesOrder(),
		pmetrictest.IgnoreMetricDataPointsOrder(),
		pmetrictest.IgnoreMetricsOrder(),
		pmetrictest.IgnoreScopeMetricsOrder(),
		pmetrictest.IgnoreResourceMetricsOrder(),
	}
)

func TestE2E_K8sCombinedReceiver(t *testing.T) {
	testDir := filepath.Join("testdata")
	expectedGatewayFile := testDir + "/e2e/expected-gateway.yaml"
	expectedAgentFile := testDir + "/e2e/expected-agent.yaml"
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

	metricsConsumerGateway := new(consumertest.MetricsSink)
	metricsConsumerAgent := new(consumertest.MetricsSink)
	logsConsumer := new(consumertest.LogsSink)
	shutdownSinks := oteltest.StartUpSinks(t, oteltest.ReceiverSinks{
		Logs: &oteltest.LogSinkConfig{
			Consumer: logsConsumer,
			Ports: &oteltest.ReceiverPorts{
				Http: 4319,
			},
		},
		Metrics: &oteltest.MetricSinkConfig{
			Consumer: metricsConsumerGateway,
			Ports: &oteltest.ReceiverPorts{
				Http: 4320,
			},
		},
	})
	shutdownSinks2 := oteltest.StartUpSinks(t, oteltest.ReceiverSinks{
		Metrics: &oteltest.MetricSinkConfig{
			Consumer: metricsConsumerAgent,
			Ports: &oteltest.ReceiverPorts{
				Http: 4321,
			},
		},
	})
	defer func() {
		// give some more time to the collector to finish exporting before stopping the sinks
		// so we do not have any dropped data after the test is finished
		time.Sleep(10 * time.Second)
		shutdownSinks()
		shutdownSinks2()
	}()

	// create agent collector
	testID, err := testutil.GenerateRandomString(10)
	require.NoError(t, err)
	host := otelk8stest.HostEndpoint(t)
	collectorConfigPath := path.Join(configExamplesDir, "k8scombined-agent.yaml")
	collectorConfig, err := k8stest.GetCollectorConfig(collectorConfigPath, k8stest.ConfigTemplate{
		Host: host,
		Templates: []string{
			templateAgentOrigin,
			fmt.Sprintf(templateAgentNew, host),
		},
	})

	require.NoErrorf(t, err, "Failed to read collector config from file %s", collectorConfigPath)
	collectorObjs2 := otelk8stest.CreateCollectorObjects(
		t,
		k8sClient,
		testID,
		filepath.Join(testDir, "collector-agent"),
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

	t.Logf("Checking agent metrics...")

	oteltest.WaitForMetrics(t, 5, metricsConsumerAgent)

	// the commented line below writes the received list of metrics to the expected.yaml
	// require.Nil(t, golden.WriteMetrics(t, expectedAgentFile, metricsConsumerAgent.AllMetrics()[len(metricsConsumerAgent.AllMetrics())-1]))

	var expected pmetric.Metrics
	expected, err = golden.ReadMetrics(expectedAgentFile)
	require.NoError(t, err)

	agentOptions := []pmetrictest.CompareMetricsOption{
		pmetrictest.ChangeResourceAttributeValue("k8s.daemonset.name", substituteRandomPartWithStar),
		pmetrictest.ChangeResourceAttributeValue("k8s.replicaset.name", substituteRandomPartWithStar),
		pmetrictest.ChangeResourceAttributeValue("k8s.deployment.name", substituteRandomPartWithStar),
		pmetrictest.ChangeResourceAttributeValue("k8s.node.name", substituteWorkerNodeName),
	}

	require.EventuallyWithT(t, func(tt *assert.CollectT) {
		assert.NoError(tt, pmetrictest.CompareMetrics(expected, metricsConsumerAgent.AllMetrics()[len(metricsConsumerAgent.AllMetrics())-1],
			append(agentOptions, defaultOptions...)...,
		),
		)
	}, 3*time.Minute, 1*time.Second)

	t.Logf("Agent metrics checked successfully")

	// create gateway collector
	collectorConfigPath = path.Join(configExamplesDir, "k8scombined-gateway.yaml")
	collectorConfig, err = k8stest.GetCollectorConfig(collectorConfigPath, k8stest.ConfigTemplate{
		Host: host,
		Templates: []string{
			templateGatewayOrigin,
			fmt.Sprintf(templateGatewayNew, host, host),
		},
	})

	require.NoErrorf(t, err, "Failed to read collector config from file %s", collectorConfigPath)
	collectorObjs1 := otelk8stest.CreateCollectorObjects(
		t,
		k8sClient,
		testID,
		filepath.Join(testDir, "collector-gateway"),
		map[string]string{
			"ContainerRegistry": os.Getenv("CONTAINER_REGISTRY"),
			"CollectorConfig":   collectorConfig,
		},
		host,
	)

	// create deployment
	deploymentFile := filepath.Join(testDir, "testobjects", "deployment.yaml")
	buf, err = os.ReadFile(deploymentFile)
	require.NoErrorf(t, err, "failed to read deployment object file %s", deploymentFile)
	deploymentObj, err := otelk8stest.CreateObject(k8sClient, buf)
	require.NoErrorf(t, err, "failed to create k8s deployment from file %s", deploymentFile)

	defer func() {
		for _, obj := range append(collectorObjs1, deploymentObj) {
			require.NoErrorf(t, otelk8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
		}
	}()

	t.Logf("Checking logs...")

	expectedLogEvents := false
	oteltest.WaitForLogs(t, 1, logsConsumer)

	for _, r := range logsConsumer.AllLogs() {
		for i := 0; i < r.ResourceLogs().Len(); i++ {
			sm := r.ResourceLogs().At(i).ScopeLogs().At(0).LogRecords()
			for j := 0; j < sm.Len(); j++ {
				bodyMap := sm.At(j).Body().Map()
				if kind, ok := bodyMap.Get("kind"); ok && kind.Str() == "Event" {
					if _, ok := bodyMap.Get("message"); ok {
						expectedLogEvents = true
					}
				}
			}
		}
	}

	require.True(t, expectedLogEvents, "Event logs not found")

	t.Logf("Logs checked successfully")
	t.Logf("Checking gateway metrics...")

	oteltest.WaitForMetrics(t, 5, metricsConsumerGateway)

	// the commented line below writes the received list of metrics to the expected.yaml
	// require.Nil(t, golden.WriteMetrics(t, expectedGatewayFile, metricsConsumerGateway.AllMetrics()[len(metricsConsumerGateway.AllMetrics())-1]))

	expected, err = golden.ReadMetrics(expectedGatewayFile)
	require.NoError(t, err)

	require.EventuallyWithT(t, func(tt *assert.CollectT) {
		assert.NoError(tt, pmetrictest.CompareMetrics(expected, metricsConsumerGateway.AllMetrics()[len(metricsConsumerGateway.AllMetrics())-1],
			defaultOptions...,
		),
		)
	}, 3*time.Minute, 1*time.Second)

	t.Logf("Gateway metrics checked successfully")
}

func substituteWithStar(_ string) string { return "*" }

func substituteRandomPartWithStar(s string) string {
	re := regexp.MustCompile(`(-[a-z0-9]{10})?(-[a-z0-9]{6,10})?(-[a-z0-9]{5})?$`)
	return re.ReplaceAllString(s, "-*")
}

func substituteWorkerNodeName(s string) string {
	re := regexp.MustCompile(`kind-worker2`)
	return re.ReplaceAllString(s, "kind-worker")
}

func substituteLocalhostImagePrefix(s string) string {
	return strings.Replace(s, "localhost/", "docker.io/library/", 1)
}
