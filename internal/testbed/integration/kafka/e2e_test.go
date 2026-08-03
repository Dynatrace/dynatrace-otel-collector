// Copyright Dynatrace LLC
// SPDX-License-Identifier: Apache-2.0

//go:build e2e

package kafka

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/k8stest"
	"github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/oteltest"
	"github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetricassert"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/ptracetest"
	otelk8stest "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/xk8stest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func TestE2E_Kafka(t *testing.T) {
	// Paths
	testDir := filepath.Join("testdata")
	expectedLogsFile := filepath.Join(testDir, "e2e", "expected-logs.yaml")
	expectedTracesFile := filepath.Join(testDir, "e2e", "expected-traces.yaml")
	expectedMetricsAssertFile := filepath.Join(testDir, "e2e", "expected-metrics.assert.yaml")
	expectedKMetricsAssertFile := filepath.Join(testDir, "e2e", "expected-kafka-metrics.assert.yaml")
	configExamplesDir := filepath.Join("..", "..", "..", "..", "config_examples")

	// K8s client
	kubeconfigPath := k8stest.KubeconfigFromEnvOrDefault()
	k8sClient, err := otelk8stest.NewK8sClient(kubeconfigPath)
	require.NoError(t, err)

	// Namespace
	nsObj := k8stest.CreateObjectFromFile(t, k8sClient, filepath.Join(testDir, "namespace.yaml"))
	testNs := nsObj.GetName()
	defer func() {
		require.NoErrorf(t, otelk8stest.DeleteObject(k8sClient, nsObj), "failed to delete namespace %s", testNs)
	}()

	metricsConsumer := new(consumertest.MetricsSink)
	kmetricsConsumer := new(consumertest.MetricsSink)
	tracesConsumer := new(consumertest.TracesSink)
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
				Consumer: metricsConsumer,
				Ports: &oteltest.ReceiverPorts{
					Http: 4320,
				},
			},
			{
				Consumer: kmetricsConsumer,
				Ports: &oteltest.ReceiverPorts{
					Http: 4322,
				},
			},
		},
		Traces: []*oteltest.TraceSinkConfig{
			{
				Consumer: tracesConsumer,
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
	}()

	// Kafka server (deployment + service)
	k8stest.CreateObjectFromFile(t, k8sClient, filepath.Join(testDir, "testobjects", "kafka-deployment.yaml"))
	k8stest.CreateObjectFromFile(t, k8sClient, filepath.Join(testDir, "testobjects", "kafka-service.yaml"))

	// Wait for Kafka to accept connections before starting collectors.
	// Without this, collectors reach "Ready" (health-check HTTP up) while Kafka
	// is still initialising: the exporter drops/queues messages it can't deliver
	// and the receiver never registers its consumer group, causing the
	// kafka-metrics assertion to time out.
	err = k8stest.WaitForDeploymentPods(k8sClient.DynamicClient, testNs, "redpanda", 5*time.Minute)
	require.NoError(t, err, "Kafka broker (Redpanda) did not become ready in time")

	// Host endpoint for the receiver exporters
	host := otelk8stest.HostEndpoint(t)

	// Receiver collector
	testIDReceiver, err := testutil.GenerateRandomString(10)
	require.NoError(t, err)

	collectorConfigPathReceiver := filepath.Join(configExamplesDir, "kafka-receiver.yaml")

	// Read overlays from files
	localOverlay := fmt.Sprintf(k8stest.MustRead(t, filepath.Join(testDir, "config-overlays", "receiver-local.yaml")), host)

	collectorConfigReceiver, err := k8stest.GetCollectorConfig(collectorConfigPathReceiver, k8stest.ConfigTemplate{
		Host: host,
		Templates: []string{
			localOverlay,
		},
	})
	require.NoErrorf(t, err, "Failed to read collector config from file %s", collectorConfigPathReceiver)

	_ = otelk8stest.CreateCollectorObjects(
		t,
		k8sClient,
		testIDReceiver,
		filepath.Join(testDir, "collector-receiver"),
		map[string]string{
			"ContainerRegistry": os.Getenv("CONTAINER_REGISTRY"),
			"CollectorConfig":   collectorConfigReceiver,
		},
		host,
	)

	// KafkaMetrics Receiver collector
	testIDKMReceiver, err := testutil.GenerateRandomString(10)
	require.NoError(t, err)
	collectorConfigPathKMReceiver := filepath.Join(configExamplesDir, "kafka-metrics-receiver.yaml")

	// Read overlays from files
	localOverlay = fmt.Sprintf(k8stest.MustRead(t, filepath.Join(testDir, "config-overlays", "kafkametrics-receiver-local.yaml")), host)

	collectorConfigKMReceiver, err := k8stest.GetCollectorConfig(collectorConfigPathKMReceiver, k8stest.ConfigTemplate{
		Host: host,
		Templates: []string{
			localOverlay,
		},
	})
	require.NoErrorf(t, err, "Failed to read collector config from file %s", collectorConfigPathKMReceiver)

	k8stest.CreateCollectorObjects(
		t,
		k8sClient,
		testIDKMReceiver,
		filepath.Join(testDir, "collector-kafka"),
		map[string]string{
			"ContainerRegistry": os.Getenv("CONTAINER_REGISTRY"),
			"CollectorConfig":   collectorConfigKMReceiver,
		},
		host,
	)

	// Exporter collector
	testIDExporter, err := testutil.GenerateRandomString(10)
	require.NoError(t, err)

	collectorConfigPathExporter := filepath.Join(configExamplesDir, "kafka-exporter.yaml")
	collectorConfigExporter, err := k8stest.GetCollectorConfig(collectorConfigPathExporter, k8stest.ConfigTemplate{
		Host: host,
	})
	require.NoErrorf(t, err, "Failed to read collector config from file %s", collectorConfigPathExporter)

	otelk8stest.CreateCollectorObjects(
		t,
		k8sClient,
		testIDExporter,
		filepath.Join(testDir, "collector-exporter"),
		map[string]string{
			"ContainerRegistry": os.Getenv("CONTAINER_REGISTRY"),
			"CollectorConfig":   collectorConfigExporter,
		},
		host,
	)

	// Create Telemetries
	k8stest.CreateObjectFromFile(t, k8sClient, filepath.Join(testDir, "testobjects", "telemetrygen.yaml"))

	// Wait for telemetrygen pods to be running before starting assertions,
	// so the compare window is not consumed by a slow image pull or scheduler delay.
	err = k8stest.WaitForDeploymentPods(k8sClient.DynamicClient, testNs, "temeletrygen-deployment", 3*time.Minute)
	require.NoError(t, err, "telemetrygen did not become ready in time")

	// Compare timeouts
	const (
		compareTimeout   = 3 * time.Minute
		compareTick      = 3 * time.Second
		compareTraceTick = 1 * time.Second
	)

	// Metrics
	t.Logf("Checking metrics...")
	oteltest.WaitForMetrics(t, 1, metricsConsumer)

	// To regenerate: uncomment, run the test once, re-comment.
	// require.NoError(t, pmetricassert.WriteAssertionFile(t, expectedMetricsAssertFile, metricsConsumer.AllMetrics()[len(metricsConsumer.AllMetrics())-1]))

	require.EventuallyWithT(t, func(tt *assert.CollectT) {
		all := metricsConsumer.AllMetrics()
		actual := all[len(all)-1]
		actualForAssert := pmetric.NewMetrics()
		actual.CopyTo(actualForAssert)
		testutil.ReplaceAttrValsWithStar(actualForAssert, nil, nil)
		testutil.DeduplicateResources(actualForAssert)
		assert.NoError(tt, pmetricassert.AssertMetrics(expectedMetricsAssertFile, actualForAssert))
	}, compareTimeout, compareTick)

	t.Logf("Metrics checked successfully")

	// Logs
	t.Logf("Checking logs...")
	oteltest.WaitForLogs(t, 1, logsConsumer)

	// the commented line below writes the received list of logs to the expected.yaml
	// require.Nil(t, golden.WriteLogs(t, expectedLogsFile, logsConsumer.AllLogs()[len(logsConsumer.AllLogs())-1]))

	expectedLogs, err := golden.ReadLogs(expectedLogsFile)
	require.NoError(t, err)

	logCompareOptions := []plogtest.CompareLogsOption{
		plogtest.IgnoreTimestamp(),
	}

	require.EventuallyWithT(t, func(tt *assert.CollectT) {
		all := logsConsumer.AllLogs()
		require.NotEmpty(tt, all)
		got := all[len(all)-1]
		assert.NoError(tt, plogtest.CompareLogs(expectedLogs, got, logCompareOptions...))
	}, compareTimeout, compareTick)

	t.Logf("Logs checked successfully")

	// Traces
	t.Log("Checking traces...")
	oteltest.WaitForTraces(t, 1, tracesConsumer)

	// the commented line below writes the received list of metrics to the expected.yaml
	// require.Nil(t, golden.WriteTraces(t, expectedTracesFile, tracesConsumer.AllTraces()[len(tracesConsumer.AllTraces())-1]))

	traceCompareOptions := []ptracetest.CompareTracesOption{
		ptracetest.IgnoreStartTimestamp(),
		ptracetest.IgnoreEndTimestamp(),
		ptracetest.IgnoreTraceID(),
		ptracetest.IgnoreSpanID(),
		ptracetest.IgnoreResourceSpansOrder(),
		ptracetest.IgnoreScopeSpansOrder(),
		ptracetest.IgnoreSpansOrder(),
	}

	expectedTraces, err := golden.ReadTraces(expectedTracesFile)
	require.NoError(t, err)

	require.EventuallyWithT(t, func(tt *assert.CollectT) {
		all := tracesConsumer.AllTraces()
		require.NotEmpty(tt, all)
		got := all[len(all)-1]
		testutil.MaskParentSpanID(expectedTraces)
		testutil.MaskParentSpanID(got)
		assert.NoError(tt, ptracetest.CompareTraces(expectedTraces, got, traceCompareOptions...))
	}, compareTimeout, compareTraceTick)

	t.Logf("Traces checked successfully")

	// KMetrics
	t.Logf("Checking kafka metrics...")
	oteltest.WaitForMetrics(t, 10, kmetricsConsumer)

	// To regenerate: uncomment, run the test once, re-comment.
	// require.NoError(t, pmetricassert.WriteAssertionFile(t, expectedKMetricsAssertFile, kmetricsConsumer.AllMetrics()[len(kmetricsConsumer.AllMetrics())-1]))

	require.EventuallyWithT(t, func(tt *assert.CollectT) {
		allKMetrics := kmetricsConsumer.AllMetrics()
		actualKMetrics := allKMetrics[len(allKMetrics)-1]
		actualForAssert := pmetric.NewMetrics()
		actualKMetrics.CopyTo(actualForAssert)
		testutil.ReplaceAttrValsWithStar(actualForAssert, nil, nil)
		testutil.DeduplicateResources(actualForAssert)
		assert.NoError(tt, pmetricassert.AssertMetrics(expectedKMetricsAssertFile, actualForAssert))
	}, compareTimeout, compareTick)

	t.Logf("Kafka Metrics checked successfully")

}
