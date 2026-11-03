// Copyright Dynatrace LLC
// SPDX-License-Identifier: Apache-2.0

package kafka

import (
	"context"
	"testing"
	"time"

	"github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/oteltest"
	"github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetricassert"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/ptracetest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kfake"
	"github.com/twmb/franz-go/pkg/kgo"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/plog/plogotlp"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"
)

const (
	topicSpans   = "otlp_spans"
	topicMetrics = "otlp_metrics"
	topicLogs    = "otlp_logs"

	brokerAddressVar = "${env:BROKER_ADDRESS}"
	waitTimeout      = 60 * time.Second
	waitTick         = 500 * time.Millisecond
)

// startBroker starts an in-process Kafka broker with the default topics of the exporter and
// receiver and returns its address.
func startBroker(t *testing.T) string {
	c, err := kfake.NewCluster(
		kfake.NumBrokers(1),
		kfake.SeedTopics(1, topicSpans, topicMetrics, topicLogs),
	)
	require.NoError(t, err)
	t.Cleanup(c.Close)
	return c.ListenAddrs()[0]
}

func exporterConfig(t *testing.T, broker string, grpcPort, healthPort int) string {
	return testutil.LoadConfigExample(t, "kafka-exporter.yaml").
		ReplaceString(brokerAddressVar, broker).
		Replace("receiver port", func(s string) string { return testutil.ReplaceOtlpGrpcReceiverPort(s, grpcPort) }).
		Replace("health check port", func(s string) string { return testutil.ReplaceHealthCheckPort(s, healthPort) }).
		Replace("internal metrics", testutil.DisableInternalMetrics).
		ReplaceString("flush_timeout: 30s", "flush_timeout: 1s").
		String()
}

func receiverConfig(t *testing.T, broker string, healthPort, sinkPort int) string {
	return testutil.LoadConfigExample(t, "kafka-receiver.yaml").
		ReplaceString(brokerAddressVar, broker).
		Replace("health check port", func(s string) string { return testutil.ReplaceHealthCheckPort(s, healthPort) }).
		Replace("exporter endpoint", func(s string) string { return testutil.ReplaceDynatraceExporterEndpoint(s, sinkPort) }).
		Replace("sending_queue and retry_on_failure", func(s string) string { return testutil.DisableSendingQueueAndRetry(s, "otlp_http") }).
		Replace("internal metrics", testutil.DisableInternalMetrics).
		// With the default of "latest", records produced before the consumer group has resolved its offsets are skipped.
		ReplaceString("  kafka:\n    tls:\n", "  kafka:\n    initial_offset: earliest\n    tls:\n").
		String()
}

func metricsReceiverConfig(t *testing.T, broker string, healthPort, sinkPort int) string {
	return testutil.LoadConfigExample(t, "kafka-metrics-receiver.yaml").
		ReplaceString(brokerAddressVar, broker).
		Replace("health check port", func(s string) string { return testutil.ReplaceHealthCheckPort(s, healthPort) }).
		Replace("exporter endpoint", func(s string) string { return testutil.ReplaceDynatraceExporterEndpoint(s, sinkPort) }).
		Replace("sending_queue and retry_on_failure", func(s string) string { return testutil.DisableSendingQueueAndRetry(s, "otlp_http") }).
		Replace("internal metrics", testutil.DisableInternalMetrics).
		ReplaceString("collection_interval: 5s", "collection_interval: 1s").
		String()
}

var (
	ts0 = pcommon.NewTimestampFromTime(time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))
	ts1 = ts0 + pcommon.Timestamp(time.Second)
)

func buildTraces() ptrace.Traces {
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("service.name", "kafka-traces-test")
	ss := rs.ScopeSpans().AppendEmpty()
	ss.Scope().SetName("kafka-test-scope")

	traceID := pcommon.TraceID{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	parent := ss.Spans().AppendEmpty()
	parent.SetTraceID(traceID)
	parent.SetSpanID(pcommon.SpanID{1, 1, 1, 1, 1, 1, 1, 1})
	parent.SetName("parent")
	parent.SetKind(ptrace.SpanKindServer)
	parent.SetStartTimestamp(ts0)
	parent.SetEndTimestamp(ts1)
	parent.Attributes().PutStr("http.request.method", "GET")
	parent.Status().SetCode(ptrace.StatusCodeError)
	parent.Status().SetMessage("failed")
	ev := parent.Events().AppendEmpty()
	ev.SetName("exception")
	ev.SetTimestamp(ts0 + 1)
	ev.Attributes().PutStr("exception.message", "boom")
	link := parent.Links().AppendEmpty()
	link.SetTraceID(pcommon.TraceID{16, 15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1})
	link.SetSpanID(pcommon.SpanID{9, 9, 9, 9, 9, 9, 9, 9})
	link.Attributes().PutStr("link.key", "link-value")

	child := ss.Spans().AppendEmpty()
	child.SetTraceID(traceID)
	child.SetSpanID(pcommon.SpanID{2, 2, 2, 2, 2, 2, 2, 2})
	child.SetParentSpanID(parent.SpanID())
	child.SetName("child")
	child.SetKind(ptrace.SpanKindClient)
	child.SetStartTimestamp(ts0 + 2)
	child.SetEndTimestamp(ts1 - 2)
	return td
}

func buildMetrics() pmetric.Metrics {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("service.name", "kafka-metrics-test")
	sm := rm.ScopeMetrics().AppendEmpty()
	sm.Scope().SetName("kafka-test-scope")

	g := sm.Metrics().AppendEmpty()
	g.SetName("test.gauge")
	g.SetUnit("1")
	dp := g.SetEmptyGauge().DataPoints().AppendEmpty()
	dp.SetTimestamp(ts1)
	dp.SetDoubleValue(1.5)
	dp.Attributes().PutStr("key", "value")

	s := sm.Metrics().AppendEmpty()
	s.SetName("test.sum")
	s.SetUnit("By")
	sum := s.SetEmptySum()
	sum.SetIsMonotonic(true)
	sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
	sdp := sum.DataPoints().AppendEmpty()
	sdp.SetStartTimestamp(ts0)
	sdp.SetTimestamp(ts1)
	sdp.SetIntValue(42)

	h := sm.Metrics().AppendEmpty()
	h.SetName("test.histogram")
	h.SetUnit("ms")
	hist := h.SetEmptyHistogram()
	hist.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
	hdp := hist.DataPoints().AppendEmpty()
	hdp.SetStartTimestamp(ts0)
	hdp.SetTimestamp(ts1)
	hdp.SetCount(4)
	hdp.SetSum(10)
	hdp.SetMin(1)
	hdp.SetMax(5)
	hdp.ExplicitBounds().FromRaw([]float64{2, 4})
	hdp.BucketCounts().FromRaw([]uint64{1, 2, 1})
	return md
}

func buildLogs() plog.Logs {
	ld := plog.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("service.name", "kafka-logs-test")
	sl := rl.ScopeLogs().AppendEmpty()
	sl.Scope().SetName("kafka-test-scope")
	lr := sl.LogRecords().AppendEmpty()
	lr.SetTimestamp(ts0)
	lr.SetObservedTimestamp(ts1)
	lr.SetSeverityNumber(plog.SeverityNumberWarn)
	lr.SetSeverityText("WARN")
	lr.SetTraceID(pcommon.TraceID{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	lr.SetSpanID(pcommon.SpanID{1, 1, 1, 1, 1, 1, 1, 1})
	lr.Attributes().PutStr("key", "value")
	body := lr.Body().SetEmptyMap()
	body.PutStr("message", "hello")
	body.PutInt("count", 3)
	return ld
}

func TestE2E_KafkaExporterReceiver(t *testing.T) {
	broker := startBroker(t)
	sinks, sinkPorts := oteltest.StartSinks(t)

	receiverHealth := testutil.GetAvailablePort(t)
	testutil.StartCollector(t, testutil.CollectorParams{
		Name:      "kafka-receiver",
		Config:    receiverConfig(t, broker, receiverHealth, sinkPorts.Http),
		ReadyPort: receiverHealth,
	})

	grpcPort := testutil.GetAvailablePort(t)
	testutil.StartCollector(t, testutil.CollectorParams{
		Name:      "kafka-exporter",
		Config:    exporterConfig(t, broker, grpcPort, testutil.GetAvailablePort(t)),
		ReadyPort: grpcPort,
	})

	sentTraces, sentMetrics, sentLogs := buildTraces(), buildMetrics(), buildLogs()
	ctx := t.Context()
	_, err := oteltest.NewTracesClient(t, grpcPort).Export(ctx, ptraceotlp.NewExportRequestFromTraces(sentTraces))
	require.NoError(t, err)
	_, err = oteltest.NewMetricsClient(t, grpcPort).Export(ctx, pmetricotlp.NewExportRequestFromMetrics(sentMetrics))
	require.NoError(t, err)
	_, err = oteltest.NewLogsClient(t, grpcPort).Export(ctx, plogotlp.NewExportRequestFromLogs(sentLogs))
	require.NoError(t, err)

	t.Run("traces", func(t *testing.T) {
		require.EventuallyWithT(t, func(c *assert.CollectT) {
			got := ptrace.NewTraces()
			for _, td := range sinks.Traces.AllTraces() {
				td.ResourceSpans().MoveAndAppendTo(got.ResourceSpans())
			}
			assert.NoError(c, ptracetest.CompareTraces(sentTraces, got,
				ptracetest.IgnoreResourceSpansOrder(), ptracetest.IgnoreScopeSpansOrder(), ptracetest.IgnoreSpansOrder()))
		}, waitTimeout, waitTick)
	})
	t.Run("metrics", func(t *testing.T) {
		require.EventuallyWithT(t, func(c *assert.CollectT) {
			got := pmetric.NewMetrics()
			for _, md := range sinks.Metrics.AllMetrics() {
				md.ResourceMetrics().MoveAndAppendTo(got.ResourceMetrics())
			}
			assert.NoError(c, pmetrictest.CompareMetrics(sentMetrics, got,
				pmetrictest.IgnoreResourceMetricsOrder(), pmetrictest.IgnoreScopeMetricsOrder(),
				pmetrictest.IgnoreMetricsOrder(), pmetrictest.IgnoreMetricDataPointsOrder()))
		}, waitTimeout, waitTick)
	})
	t.Run("logs", func(t *testing.T) {
		require.EventuallyWithT(t, func(c *assert.CollectT) {
			got := plog.NewLogs()
			for _, ld := range sinks.Logs.AllLogs() {
				ld.ResourceLogs().MoveAndAppendTo(got.ResourceLogs())
			}
			assert.NoError(c, plogtest.CompareLogs(sentLogs, got,
				plogtest.IgnoreResourceLogsOrder(), plogtest.IgnoreScopeLogsOrder(), plogtest.IgnoreLogRecordsOrder()))
		}, waitTimeout, waitTick)
	})
}

func TestE2E_KafkaMetricsReceiver(t *testing.T) {
	const group = "otel-collector"
	broker := startBroker(t)

	// Produced records per topic and committed offset (same for all topics).
	produced := map[string]int{topicSpans: 3, topicMetrics: 2, topicLogs: 1}
	const committed = 1

	cl, err := kgo.NewClient(kgo.SeedBrokers(broker))
	require.NoError(t, err)
	t.Cleanup(cl.Close)
	ctx := context.Background()
	for topic, n := range produced {
		for range n {
			require.NoError(t, cl.ProduceSync(ctx, &kgo.Record{Topic: topic, Value: []byte("x")}).FirstErr())
		}
	}
	offsets := kadm.Offsets{}
	for topic := range produced {
		offsets.Add(kadm.Offset{Topic: topic, Partition: 0, At: committed})
	}
	resp, err := kadm.NewClient(cl).CommitOffsets(ctx, group, offsets)
	require.NoError(t, err)
	require.NoError(t, resp.Error())

	sink, sinkPorts := oteltest.StartMetricsSink(t)
	healthPort := testutil.GetAvailablePort(t)
	testutil.StartCollector(t, testutil.CollectorParams{
		Name:      "kafka-metrics",
		Config:    metricsReceiverConfig(t, broker, healthPort, sinkPorts.Http),
		ReadyPort: healthPort,
	})

	expectedFile := "testdata/expected-kafka-metrics.assert.yaml"
	oteltest.WaitForMetrics(t, 1, sink)

	// To regenerate: uncomment, run the test once, re-comment.
	// require.NoError(t, pmetricassert.WriteAssertionFile(t, expectedFile, sink.AllMetrics()[len(sink.AllMetrics())-1], pmetricassert.IncludeValues()))

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		all := sink.AllMetrics()
		actual := pmetric.NewMetrics()
		all[len(all)-1].CopyTo(actual)
		testutil.ReplaceAttrValsWithStar(actual, nil, nil)
		testutil.DeduplicateResources(actual)
		assert.NoError(c, pmetricassert.AssertMetrics(expectedFile, actual))
	}, waitTimeout, waitTick)
}
