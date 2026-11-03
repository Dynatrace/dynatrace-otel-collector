// Copyright Dynatrace LLC
// SPDX-License-Identifier: Apache-2.0

package selfmonitoring

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/oteltest"
	"github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetricassert"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

const (
	collectorServiceName = "dynatrace-otel-collector"
	testServiceName      = "selfmon-test"
)

// podEnv replaces the environment that the downward API sets in Kubernetes.
var podEnv = map[string]string{
	"MY_POD_NAMESPACE": "e2e-selfmonitoring",
	"MY_POD_NAME":      "otelcol-local",
	"MY_NODE_NAME":     "local-node",
	"MY_POD_IP":        "127.0.0.1",
}

// startCollector starts the DT Collector with an example config. It sends all its data and its
// internal metrics to the OTLP/HTTP receiver on sinkHTTPPort. edit adapts the config to the
// example on top of the common local overrides.
func startCollector(t *testing.T, example string, sinkHTTPPort, readyPort int, edit func(*testutil.Config)) {
	healthPort := testutil.GetAvailablePort(t)
	cfg := testutil.LoadConfigExample(t, example).
		Replace("health_check port", func(s string) string { return testutil.ReplaceHealthCheckPort(s, healthPort) }).
		Replace("exporter endpoint", func(s string) string { return testutil.ReplaceDynatraceExporterEndpoint(s, sinkHTTPPort) })
	edit(cfg)

	testutil.StartCollector(t, testutil.CollectorParams{
		Name:      strings.TrimSuffix(example, ".yaml"),
		Config:    cfg.String(),
		ReadyPort: readyPort,
		Env:       podEnv,
	})
	testutil.WaitForHealthCheck(t, healthPort)
}

// fastReader shortens the interval of the periodic metric reader, which only speeds up the test.
func fastReader(c *testutil.Config) {
	c.ReplaceString("interval: 10000", "interval: 1000").
		ReplaceString("timeout: 5000", "timeout: 1000")
}

// filterResources returns a copy of the resources of md for which keep returns true for the service name.
func filterResources(md []pmetric.Metrics, keep func(serviceName string) bool) pmetric.Metrics {
	out := pmetric.NewMetrics()
	for _, m := range md {
		rms := m.ResourceMetrics()
		for i := 0; i < rms.Len(); i++ {
			name, _ := rms.At(i).Resource().Attributes().Get("service.name")
			if keep(name.AsString()) {
				rms.At(i).CopyTo(out.ResourceMetrics().AppendEmpty())
			}
		}
	}
	return out
}

// selfMetrics returns the resources of the Collector's internal metrics from all payloads. The
// internal SDK uses delta temporality and omits idle streams, so a one-shot event shows up in
// exactly one payload. Filtering is by resource, because the batcher can merge internal metrics
// and test data into one request.
func selfMetrics(sink *consumertest.MetricsSink) pmetric.Metrics {
	return filterResources(sink.AllMetrics(), func(n string) bool { return n == collectorServiceName })
}

// testResources returns the resources of the test data, i.e. the complement of selfMetrics.
func testResources(sink *consumertest.MetricsSink) pmetric.Metrics {
	return filterResources(sink.AllMetrics(), func(n string) bool { return n == testServiceName })
}

// sumDelta sums the values of all Sum data points of the metric name whose attributes contain attrs.
func sumDelta(md pmetric.Metrics, name string, attrs map[string]any) float64 {
	var sum float64
	rms := md.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		sms := rms.At(i).ScopeMetrics()
		for j := 0; j < sms.Len(); j++ {
			ms := sms.At(j).Metrics()
			for k := 0; k < ms.Len(); k++ {
				m := ms.At(k)
				if m.Name() != name || m.Type() != pmetric.MetricTypeSum {
					continue
				}
				dps := m.Sum().DataPoints()
				for l := 0; l < dps.Len(); l++ {
					dp := dps.At(l)
					if !containsAttrs(dp.Attributes(), attrs) {
						continue
					}
					if dp.ValueType() == pmetric.NumberDataPointValueTypeInt {
						sum += float64(dp.IntValue())
					} else {
						sum += dp.DoubleValue()
					}
				}
			}
		}
	}
	return sum
}

func containsAttrs(m pcommon.Map, attrs map[string]any) bool {
	for k, want := range attrs {
		got, ok := m.Get(k)
		if !ok || got.AsRaw() != want {
			return false
		}
	}
	return true
}

// metricNames returns the names of all metrics in md.
func metricNames(md pmetric.Metrics) []string {
	var names []string
	rms := md.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		sms := rms.At(i).ScopeMetrics()
		for j := 0; j < sms.Len(); j++ {
			ms := sms.At(j).Metrics()
			for k := 0; k < ms.Len(); k++ {
				names = append(names, ms.At(k).Name())
			}
		}
	}
	return names
}

// assertGolden compares md, with the ignored attributes masked and resources deduplicated, to the golden file.
func assertGolden(t testing.TB, tt assert.TestingT, path string, md pmetric.Metrics, resourceIgnore, dpIgnore []string) {
	actual := pmetric.NewMetrics()
	md.CopyTo(actual)
	testutil.ReplaceAttrValsWithStar(actual, resourceIgnore, dpIgnore)
	testutil.DeduplicateResources(actual)
	// To regenerate: uncomment, run the test once, re-comment.
	// require.NoError(t, pmetricassert.WriteAssertionFile(t, path, actual))
	assert.NoError(tt, pmetricassert.AssertMetrics(path, actual))
}

// Test_Selfmonitoring tests that the Collector reads the pod environment into the resource of its
// internal metrics and sends them with the periodic OTLP reader.
// The config under test is config_examples/self-monitoring.yaml.
func Test_Selfmonitoring(t *testing.T) {
	sink, sinkPorts := oteltest.StartMetricsSink(t)
	httpPort := testutil.GetAvailablePort(t)

	startCollector(t, "self-monitoring.yaml", sinkPorts.Http, httpPort, func(c *testutil.Config) {
		c.Replace("http receiver port", func(s string) string { return testutil.ReplaceOtlpHttpReceiverPort(s, httpPort) })
		fastReader(c)
	})

	oteltest.WaitForMetrics(t, 1, sink)
	self := selfMetrics(sink)
	require.NotZero(t, self.ResourceMetrics().Len())

	require.NoError(t, oteltest.AssertExpectedAttributes(self.ResourceMetrics().At(0).Resource().Attributes(), map[string]oteltest.ExpectedValue{
		"k8s.namespace.name":          oteltest.NewExpectedValue(oteltest.AttributeMatchTypeEqual, podEnv["MY_POD_NAMESPACE"]),
		"k8s.pod.name":                oteltest.NewExpectedValue(oteltest.AttributeMatchTypeEqual, podEnv["MY_POD_NAME"]),
		"k8s.node.name":               oteltest.NewExpectedValue(oteltest.AttributeMatchTypeEqual, podEnv["MY_NODE_NAME"]),
		oteltest.ServiceNameAttribute: oteltest.NewExpectedValue(oteltest.AttributeMatchTypeEqual, collectorServiceName),
		"service.instance.id":         oteltest.NewExpectedValue(oteltest.AttributeMatchTypeRegex, oteltest.UidRe),
		"service.version":             oteltest.NewExpectedValue(oteltest.AttributeMatchTypeExist, ""),
	}))

	assert.Subset(t, metricNames(self), []string{
		"otelcol_exporter_queue_capacity",
		"otelcol_exporter_queue_size",
		"otelcol_process_cpu_seconds",
		"otelcol_process_memory_rss",
		"otelcol_process_runtime_heap_alloc_bytes",
		"otelcol_process_runtime_total_alloc_bytes",
		"otelcol_process_runtime_total_sys_memory_bytes",
		"otelcol_process_uptime",
	})
}

// testTraces, testLogs and addGauge build the test data. Every resource has service.name=selfmon-test.
func testTraces(names ...string) ptrace.Traces {
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("service.name", testServiceName)
	spans := rs.ScopeSpans().AppendEmpty().Spans()
	for i, n := range names {
		s := spans.AppendEmpty()
		s.SetName(n)
		s.SetTraceID([16]byte{1})
		s.SetSpanID([8]byte{byte(i + 1)})
	}
	return td
}

func testLogs(bodies ...string) plog.Logs {
	ld := plog.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("service.name", testServiceName)
	recs := rl.ScopeLogs().AppendEmpty().LogRecords()
	for _, b := range bodies {
		recs.AppendEmpty().Body().SetStr(b)
	}
	return ld
}

// addGauge adds a resource with the label my_label and one gauge data point to md.
func addGauge(md pmetric.Metrics, name, label string, value int64) {
	rm := md.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("service.name", testServiceName)
	rm.Resource().Attributes().PutStr("my_label", label)
	m := rm.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	m.SetName(name)
	dp := m.SetEmptyGauge().DataPoints().AppendEmpty()
	dp.SetIntValue(value)
	dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
}

// Test_Selfmonitoring_checkMetrics tests the loop of the example: telemetry sent to the HTTP
// receiver goes through filter into the gRPC receiver, and again through filter to the sink.
// The internal metrics are checked for exact values on each hop and against a golden file.
// The config under test is config_examples/self-monitoring-check-metrics.yaml.
func Test_Selfmonitoring_checkMetrics(t *testing.T) {
	sinks, sinkPorts := oteltest.StartSinks(t)
	grpcPort := testutil.GetAvailablePort(t)
	httpPort := testutil.GetAvailablePort(t)

	startCollector(t, "self-monitoring-check-metrics.yaml", sinkPorts.Http, httpPort, func(c *testutil.Config) {
		c.Replace("grpc receiver port", func(s string) string { return testutil.ReplaceOtlpGrpcReceiverPort(s, grpcPort) }).
			Replace("http receiver port", func(s string) string { return testutil.ReplaceOtlpHttpReceiverPort(s, httpPort) })
		fastReader(c)
	})

	oteltest.ExportTracesHTTP(t, httpPort, testTraces("keep-1", "keep-2", "app_3"))
	oteltest.ExportLogsHTTP(t, httpPort, testLogs("hello", "world", "my password is hunter2"))
	md := pmetric.NewMetrics()
	addGauge(md, "my.metric", "abc123", 1) // dropped by the filter
	addGauge(md, "my.metric", "other", 2)
	addGauge(md, "other.metric", "abc123", 3)
	oteltest.ExportMetricsHTTP(t, httpPort, md)

	// Data path. The filtered items were in the same requests, so they would have arrived too.
	dataPoints := func() []string {
		var names []string
		rms := testResources(sinks.Metrics).ResourceMetrics()
		for i := 0; i < rms.Len(); i++ {
			names = append(names, rms.At(i).ScopeMetrics().At(0).Metrics().At(0).Name())
		}
		return names
	}
	require.Eventually(t, func() bool {
		return sinks.Traces.SpanCount() >= 2 && len(dataPoints()) >= 2 && sinks.Logs.LogRecordCount() >= 2
	}, 60*time.Second, 200*time.Millisecond, "data did not reach the sinks")
	// Without filtering, a late request could still be in flight.
	time.Sleep(time.Second)

	var spans []string
	for _, td := range sinks.Traces.AllTraces() {
		for i := 0; i < td.ResourceSpans().Len(); i++ {
			for j := 0; j < td.ResourceSpans().At(i).ScopeSpans().Len(); j++ {
				ss := td.ResourceSpans().At(i).ScopeSpans().At(j).Spans()
				for k := 0; k < ss.Len(); k++ {
					spans = append(spans, ss.At(k).Name())
				}
			}
		}
	}
	assert.ElementsMatch(t, []string{"keep-1", "keep-2"}, spans)
	assert.ElementsMatch(t, []string{"my.metric", "other.metric"}, dataPoints())
	var bodies []string
	for _, ld := range sinks.Logs.AllLogs() {
		for i := 0; i < ld.ResourceLogs().Len(); i++ {
			for j := 0; j < ld.ResourceLogs().At(i).ScopeLogs().Len(); j++ {
				recs := ld.ResourceLogs().At(i).ScopeLogs().At(j).LogRecords()
				for k := 0; k < recs.Len(); k++ {
					bodies = append(bodies, recs.At(k).Body().Str())
				}
			}
		}
	}
	assert.ElementsMatch(t, []string{"hello", "world"}, bodies)

	// Values and shape of the internal metrics.
	type hop struct {
		accepted, filtered, sent string
	}
	signals := map[string]hop{
		"spans":         {"otelcol_receiver_accepted_spans", "otelcol_processor_filter_spans.filtered", "otelcol_exporter_sent_spans"},
		"logs":          {"otelcol_receiver_accepted_log_records", "otelcol_processor_filter_logs.filtered", "otelcol_exporter_sent_log_records"},
		"metric points": {"otelcol_receiver_accepted_metric_points", "otelcol_processor_filter_datapoints.filtered", "otelcol_exporter_sent_metric_points"},
	}
	require.EventuallyWithT(t, func(tt *assert.CollectT) {
		self := selfMetrics(sinks.Metrics)
		for name, h := range signals {
			assert.EqualValues(tt, 3, sumDelta(self, h.accepted, map[string]any{"receiver": "otlp/2", "transport": "http"}), "%s accepted by otlp/2", name)
			assert.EqualValues(tt, 1, sumDelta(self, h.filtered, map[string]any{"filter": "filter"}), "%s filtered", name)
			assert.EqualValues(tt, 2, sumDelta(self, h.accepted, map[string]any{"receiver": "otlp", "transport": "grpc"}), "%s accepted by otlp", name)
			assert.EqualValues(tt, 2, sumDelta(self, h.sent, map[string]any{"exporter": "otlp_grpc/self"}), "%s sent by otlp_grpc/self", name)
			assert.EqualValues(tt, 2, sumDelta(self, h.sent, map[string]any{"exporter": "otlp_http"}), "%s sent by otlp_http", name)
		}
		assertGolden(t, tt, filepath.Join("testdata", "expected-check-metrics.assert.yaml"), self,
			[]string{"service.instance.id", "service.version"},
			[]string{"server.address", "net.peer.name", "server.port"})
	}, 60*time.Second, time.Second)
}

// Test_Selfmonitoring_Prometheus_checkMetrics tests that the Collector's internal metrics, scraped
// from its Prometheus endpoint, are renamed, converted to delta and exported with correct values.
// The config under test is config_examples/self-monitoring-prometheus-check-metrics.yaml.
func Test_Selfmonitoring_Prometheus_checkMetrics(t *testing.T) {
	sink, sinkPorts := oteltest.StartMetricsSink(t)
	grpcPort := testutil.GetAvailablePort(t)
	promPort := testutil.GetAvailablePort(t)

	startCollector(t, "self-monitoring-prometheus-check-metrics.yaml", sinkPorts.Http, grpcPort, func(c *testutil.Config) {
		c.Replace("grpc receiver port", func(s string) string { return testutil.ReplaceOtlpGrpcReceiverPort(s, grpcPort) }).
			ReplaceString("port: 8888", fmt.Sprintf("port: %d", promPort)).
			ReplaceString(`"localhost:8888"`, fmt.Sprintf(`"localhost:%d"`, promPort)).
			ReplaceString("scrape_interval: 15s", "scrape_interval: 2s")
	})

	client := oteltest.NewMetricsClient(t, grpcPort)
	send := func(values ...int64) {
		md := pmetric.NewMetrics()
		for i, v := range values {
			addGauge(md, "test.gauge", fmt.Sprintf("p%d", i), v)
		}
		_, err := client.Export(t.Context(), pmetricotlp.NewExportRequestFromMetrics(md))
		require.NoError(t, err)
	}
	accepted := map[string]any{"receiver": "otlp", "transport": "grpc"}

	// cumulative_to_delta drops the first point of every stream, so the first round is lost by
	// design. The second round must come out exactly.
	send(1)
	require.EventuallyWithT(t, func(tt *assert.CollectT) {
		assert.Contains(tt, metricNames(selfMetrics(sink)), "otelcol_receiver_accepted_metric_points")
	}, 60*time.Second, time.Second)

	send(1, 2, 3)
	require.EventuallyWithT(t, func(tt *assert.CollectT) {
		assert.Equal(tt, 4, testResources(sink).DataPointCount(), "test data points")
		self := selfMetrics(sink)
		assert.EqualValues(tt, 3, sumDelta(self, "otelcol_receiver_accepted_metric_points", accepted))
		assertGolden(t, tt, filepath.Join("testdata", "expected-prometheus-check-metrics.assert.yaml"), self,
			[]string{"server.port", "service.instance.id", "service.version"},
			[]string{"server.address", "server.port"})
	}, 60*time.Second, time.Second)
}
