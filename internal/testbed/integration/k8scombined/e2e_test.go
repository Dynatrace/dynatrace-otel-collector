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

	"github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/k8stest"
	"github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/oteltest"
	"github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/ptracetest"
	otelk8stest "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/xk8stest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
<<<<<<< HEAD
	"go.opentelemetry.io/collector/pdata/ptrace"
=======
	"go.opentelemetry.io/collector/pdata/pmetric"
>>>>>>> 64c4969 ([kafka] add Kafka receiver and exporter to the distribution)
)

var (
	metricsCompareOptions = []pmetrictest.CompareMetricsOption{
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

	traceCompareOptions = []ptracetest.CompareTracesOption{
		ptracetest.IgnoreStartTimestamp(),
		ptracetest.IgnoreEndTimestamp(),
		ptracetest.IgnoreTraceID(),
		ptracetest.IgnoreSpanID(),
		ptracetest.IgnoreSpansOrder(),
		ptracetest.IgnoreResourceAttributeValue("k8s.pod.uid"),
		ptracetest.IgnoreResourceAttributeValue("k8s.pod.ip"),
		ptracetest.IgnoreResourceAttributeValue("k8s.pod.name"),
		ptracetest.IgnoreResourceAttributeValue("k8s.deployment.uid"),
		ptracetest.IgnoreResourceAttributeValue("k8s.cluster.uid"),
		ptracetest.IgnoreResourceAttributeValue("k8s.node.name"),
	}

	templateOriginFilterProc = `  filter:
    error_mode: ignore
    metrics:
      metric:
        - 'IsMatch(name, "k8s.volume.*") and resource.attributes["k8s.volume.type"] == nil'
        - 'resource.attributes["k8s.volume.type"] == "configMap"'
        - 'resource.attributes["k8s.volume.type"] == "emptyDir"'
        - 'resource.attributes["k8s.volume.type"] == "secret"'`

	templateNewFilterProc = `  filter:
    error_mode: ignore
    metrics:
      metric:
        - 'IsMatch(name, "k8s.volume.*") and resource.attributes["k8s.volume.type"] == nil'
        - 'resource.attributes["k8s.volume.type"] == "emptyDir"'
        - 'resource.attributes["k8s.volume.type"] == "secret"'`

	templateOrigin = `
  otlphttp:
    endpoint: ${env:DT_ENDPOINT}
    headers:
      Authorization: "Api-Token ${env:DT_API_TOKEN}"

service:
  extensions:
    - health_check
    - k8s_leader_elector
  pipelines:
    metrics/node:
      receivers:
        - kubeletstats
      processors:
        - filter
        - k8sattributes
        - transform
        - cumulativetodelta
      exporters:
        - otlphttp
    metrics:
      receivers:
        - k8s_cluster
      processors:
        - k8sattributes
        - transform
        - cumulativetodelta
      exporters:
        - otlphttp
    logs:
      receivers:
        - k8s_events
      processors:
        - transform
      exporters:
        - otlphttp
    traces:
      receivers:
        - otlp
      processors:
        - k8sattributes
        - transform
      exporters:
        - otlphttp`
	templateNew = `
  otlphttp/node:
    endpoint: http://%s:4321
  otlphttp/cluster:
    endpoint: http://%s:4320
  otlphttp/traces:
    endpoint: http://%s:4322
  otlphttp/logs:
    endpoint: http://%s:4319

service:
  extensions:
    - health_check
    - k8s_leader_elector
  pipelines:
    traces:
      receivers:
        - otlp
      processors:
        - k8sattributes
        - transform
      exporters:
        - otlphttp/traces
    metrics/node:
      receivers:
        - kubeletstats
      processors:
        - filter
        - k8sattributes
        - transform
        - cumulativetodelta
      exporters:
        - otlphttp/node
    metrics:
      receivers:
        - k8s_cluster
      processors:
        - k8sattributes
        - transform
        - cumulativetodelta
      exporters:
        - otlphttp/cluster
    logs:
      receivers:
        - k8s_events
      processors:
        - transform
      exporters:
        - otlphttp/logs`
)

func TestE2E_K8sCombinedReceiver(t *testing.T) {
	testDir := filepath.Join("testdata")
	expectedClusterFile := testDir + "/e2e/expected-cluster.yaml"
	expectedTracesFile := testDir + "/e2e/expected-traces.yaml"
	expectedNodeFile := testDir + "/e2e/expected-node.yaml"
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

	metricsConsumerCluster := new(consumertest.MetricsSink)
	tracesConsumer := new(consumertest.TracesSink)
	metricsConsumerNode := new(consumertest.MetricsSink)
	logsConsumer := new(consumertest.LogsSink)
	shutdownSinks := oteltest.StartUpSinks(t, oteltest.ReceiverSinks{
		Logs: []*oteltest.LogSinkConfig{
			{
				Consumer: logsConsumer,
				Ports: &oteltest.ReceiverPorts{
					Http: 4319,
				},
			},
		},
		Metrics: []*oteltest.MetricSinkConfig{
			{
				Consumer: metricsConsumerCluster,
				Ports: &oteltest.ReceiverPorts{
					Http: 4320,
				},
			},
		},
		Traces: []*oteltest.TraceSinkConfig{
			{
				Consumer: tracesConsumer,
				Ports: &oteltest.ReceiverPorts{
					Http: 4322,
				},
			},
		},
	})
	shutdownSinks2 := oteltest.StartUpSinks(t, oteltest.ReceiverSinks{
		Metrics: []*oteltest.MetricSinkConfig{
			{
				Consumer: metricsConsumerNode,
				Ports: &oteltest.ReceiverPorts{
					Http: 4321,
				},
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

	// create collector
	testID, err := testutil.GenerateRandomString(10)
	require.NoError(t, err)
	host := otelk8stest.HostEndpoint(t)
	collectorConfigPath := path.Join(configExamplesDir, "k8scombined.yaml")
	collectorConfig, err := k8stest.GetCollectorConfig(collectorConfigPath, k8stest.ConfigTemplate{
		Host: host,
		Templates: []string{
			templateOriginFilterProc,
			templateNewFilterProc,
			templateOrigin,
			fmt.Sprintf(templateNew, host, host, host, host),
		},
	})

	require.NoErrorf(t, err, "Failed to read collector config from file %s", collectorConfigPath)
	collectorObjs2 := otelk8stest.CreateCollectorObjects(
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
		for _, obj := range collectorObjs2 {
			require.NoErrorf(t, otelk8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
		}
	}()

	t.Logf("Waiting for node metrics...")

	oteltest.WaitForMetrics(t, 1, metricsConsumerNode)

	// the commented line below writes the received list of metrics to the expected.yaml
	// require.Nil(t, golden.WriteMetrics(t, expectedNodeFile, metricsConsumerNode.AllMetrics()[len(metricsConsumerNode.AllMetrics())-1]))

	t.Logf("Checking node metrics...")

	expected, err := golden.ReadMetrics(expectedNodeFile)
	require.NoError(t, err)

	nodeOptions := []pmetrictest.CompareMetricsOption{
		pmetrictest.ChangeResourceAttributeValue("k8s.daemonset.name", substituteRandomPartWithStar),
		pmetrictest.ChangeResourceAttributeValue("k8s.replicaset.name", substituteRandomPartWithStar),
		pmetrictest.ChangeResourceAttributeValue("k8s.deployment.name", substituteRandomPartWithStar),
		pmetrictest.ChangeResourceAttributeValue("k8s.node.name", substituteWorkerNodeName),
	}

	require.EventuallyWithT(t, func(tt *assert.CollectT) {
		assert.NoError(tt, pmetrictest.CompareMetrics(expected, metricsConsumerNode.AllMetrics()[len(metricsConsumerNode.AllMetrics())-1],
			append(nodeOptions, metricsCompareOptions...)...,
		),
		)
	}, 3*time.Minute, 1*time.Second)

	t.Logf("Node metrics checked successfully")

	t.Logf("Checking cluster metrics...")

	// the commented line below writes the received list of metrics to the expected.yaml
	// require.Nil(t, golden.WriteMetrics(t, expectedClusterFile, metricsConsumerCluster.AllMetrics()[len(metricsConsumerCluster.AllMetrics())-1]))

	expected, err = golden.ReadMetrics(expectedClusterFile)
	require.NoError(t, err)

	require.EventuallyWithT(t, func(tt *assert.CollectT) {
		assert.NoError(tt, pmetrictest.CompareMetrics(expected, metricsConsumerCluster.AllMetrics()[len(metricsConsumerCluster.AllMetrics())-1],
			metricsCompareOptions...,
		),
		)
	}, 3*time.Minute, 1*time.Second)

	t.Logf("Cluster metrics checked successfully")

	// create deployment for trace generation
	deploymentFile := filepath.Join(testDir, "testobjects", "deployment.yaml")
	buf, err = os.ReadFile(deploymentFile)
	require.NoErrorf(t, err, "failed to read deployment object file %s", deploymentFile)
	deploymentObj, err := otelk8stest.CreateObject(k8sClient, buf)
	require.NoErrorf(t, err, "failed to create k8s deployment from file %s", deploymentFile)

	defer func() {
		require.NoErrorf(t, otelk8stest.DeleteObject(k8sClient, deploymentObj), "failed to delete object %s", deploymentObj.GetName())
	}()

	t.Logf("Checking logs...")

	expectedLogEvents := false
	oteltest.WaitForLogs(t, 1, logsConsumer)

	for _, r := range logsConsumer.AllLogs() {
		for i := 0; i < r.ResourceLogs().Len(); i++ {
			clusterName, okCluster := r.ResourceLogs().At(i).Resource().Attributes().Get("k8s.cluster.name")
			if !okCluster || clusterName.AsString() != "k8s-testing-cluster" {
				break
			}
			sm := r.ResourceLogs().At(i).ScopeLogs().At(0).LogRecords()
			for j := 0; j < sm.Len(); j++ {
				if sm.At(j).Body().Type() == pcommon.ValueTypeStr {
					bodyStr := sm.At(j).Body().Str()
					_, ok := sm.At(j).Attributes().Get("k8s.event.name")
					if bodyStr != "" && ok {
						expectedLogEvents = true
					}
				}
			}
		}
	}

	require.True(t, expectedLogEvents, "Event logs not found")

	t.Logf("Logs checked successfully")

	t.Log("Waiting for traces...")
	oteltest.WaitForTraces(t, 1, tracesConsumer)

	t.Log("Checking traces...")

	// the commented line below writes the received list of metrics to the expected.yaml
	// require.Nil(t, golden.WriteTraces(t, expectedTracesFile, tracesConsumer.AllTraces()[len(tracesConsumer.AllTraces())-1]))

	expectedTraces, err := golden.ReadTraces(expectedTracesFile)
	require.NoError(t, err)

	require.EventuallyWithT(t, func(tt *assert.CollectT) {
		gotTraces := tracesConsumer.AllTraces()[len(tracesConsumer.AllTraces())-1]
		testutil.MaskParentSpanID(expectedTraces)
		testutil.MaskParentSpanID(gotTraces)
		assert.NoError(tt,
			ptracetest.CompareTraces(
				expectedTraces,
				gotTraces,
				traceCompareOptions...,
			),
		)
	}, 3*time.Minute, 1*time.Second)

	t.Logf("Traces checked successfully")
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
