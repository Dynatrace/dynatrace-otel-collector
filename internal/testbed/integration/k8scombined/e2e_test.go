// Copyright Dynatrace LLC
// SPDX-License-Identifier: Apache-2.0

//go:build e2e

package k8scombined

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
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetricassert"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/ptracetest"
	otelk8stest "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/xk8stest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

var (
	traceCompareOptions = []ptracetest.CompareTracesOption{
		ptracetest.IgnoreResourceAttributeValue("k8s.pod.uid"),
		ptracetest.IgnoreResourceAttributeValue("k8s.pod.ip"),
		ptracetest.IgnoreResourceAttributeValue("k8s.pod.name"),
		ptracetest.IgnoreResourceAttributeValue("k8s.deployment.uid"),
		ptracetest.IgnoreResourceAttributeValue("k8s.cluster.uid"),
		ptracetest.IgnoreResourceAttributeValue("k8s.node.name"),
		ptracetest.IgnoreStartTimestamp(),
		ptracetest.IgnoreEndTimestamp(),
		ptracetest.IgnoreTraceID(),
		ptracetest.IgnoreSpanID(),
		ptracetest.IgnoreSpansOrder(),
		ptracetest.IgnoreResourceSpansOrder(),
		ptracetest.IgnoreScopeSpansOrder(),
	}
)

func TestE2E_K8sCombinedReceiver(t *testing.T) {
	testDir := filepath.Join("testdata")
	expectedTracesFile := testDir + "/e2e/expected-traces.yaml"
	expectedNodeAssertFile := testDir + "/e2e/expected-node.assert.yaml"
	expectedClusterAssertFile := testDir + "/e2e/expected-cluster.assert.yaml"
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
	localOverlay := fmt.Sprintf(k8stest.MustRead(t, filepath.Join(testDir, "config-overlays", "local.yaml")), host)

	collectorConfig, err := k8stest.GetCollectorConfig(collectorConfigPath, k8stest.ConfigTemplate{
		Host: host,
		Templates: []string{
			localOverlay,
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

	t.Logf("Checking node metrics...")

	nodeResourceIgnoreList := []string{
		"k8s.pod.uid",
		"k8s.pod.ip",
		"k8s.pod.name",
		"k8s.volume.name",
		"k8s.daemonset.uid",
		"k8s.deployment.uid",
		"k8s.namespace.uid",
		"k8s.node.uid",
		"k8s.replicaset.uid",
		"k8s.cluster.uid",
		"container.id",
		"container.image.tag",
		"container.image.name",
		"k8s.container.name",
		"k8s.daemonset.name",
		"k8s.deployment.name",
		"k8s.namespace.name",
		"k8s.replicaset.name",
		"k8s.workload.name",
		"k8s.node.name",
	}
	nodeDpIgnoreList := []string{
		"interface",
	}

	// To regenerate: uncomment, run the test once, re-comment.
	// require.NoError(t, pmetricassert.WriteAssertionFile(t, expectedNodeAssertFile, metricsConsumerNode.AllMetrics()[len(metricsConsumerNode.AllMetrics())-1]))

	require.EventuallyWithT(t, func(tt *assert.CollectT) {
		actual := metricsConsumerNode.AllMetrics()[len(metricsConsumerNode.AllMetrics())-1]
		actualForAssert := pmetric.NewMetrics()
		actual.CopyTo(actualForAssert)
		testutil.ReplaceAttrValsWithStar(actualForAssert, nodeResourceIgnoreList, nodeDpIgnoreList)
		testutil.DeduplicateResources(actualForAssert)
		assert.NoError(tt, pmetricassert.AssertMetrics(expectedNodeAssertFile, actualForAssert))
	}, 3*time.Minute, 1*time.Second)

	t.Logf("Node metrics checked successfully")

	t.Logf("Checking cluster metrics...")

	clusterResourceIgnoreList := []string{
		"k8s.pod.uid",
		"k8s.pod.ip",
		"k8s.pod.name",
		"k8s.volume.name",
		"k8s.daemonset.uid",
		"k8s.deployment.uid",
		"k8s.namespace.uid",
		"k8s.node.uid",
		"k8s.replicaset.uid",
		"k8s.cluster.uid",
		"container.id",
		"container.image.tag",
		"container.image.name",
		"k8s.container.name",
		"k8s.daemonset.name",
		"k8s.deployment.name",
		"k8s.namespace.name",
		"k8s.replicaset.name",
		"k8s.workload.name",
	}
	clusterDpIgnoreList := []string{
		"interface",
	}

	// To regenerate: uncomment, run the test once, re-comment.
	// require.NoError(t, pmetricassert.WriteAssertionFile(t, expectedClusterAssertFile, metricsConsumerCluster.AllMetrics()[len(metricsConsumerCluster.AllMetrics())-1]))

	require.EventuallyWithT(t, func(tt *assert.CollectT) {
		actualCluster := metricsConsumerCluster.AllMetrics()[len(metricsConsumerCluster.AllMetrics())-1]
		actualForAssert := pmetric.NewMetrics()
		actualCluster.CopyTo(actualForAssert)
		testutil.ReplaceAttrValsWithStar(actualForAssert, clusterResourceIgnoreList, clusterDpIgnoreList)
		testutil.DeduplicateResources(actualForAssert)
		assert.NoError(tt, pmetricassert.AssertMetrics(expectedClusterAssertFile, actualForAssert))
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
				continue
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
