// Copyright Dynatrace LLC
// SPDX-License-Identifier: Apache-2.0

package prometheus

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	promclient "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/oteltest"
	"github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/testutil"
)

// step is how much the target's counter grows with every scrape.
const step = 10

const (
	targetService    = "node-exporter"
	collectorService = "dynatrace-otel-collector"
)

// startTarget serves a Prometheus registry on a local port and returns its host:port. Every
// scrape moves the counter by exactly step and the histogram count by exactly 1.
func startTarget(t *testing.T) string {
	reg := promclient.NewRegistry()
	gauge := promclient.NewGauge(promclient.GaugeOpts{Name: "e2e_gauge"})
	counter := promclient.NewCounter(promclient.CounterOpts{Name: "e2e_requests_total"})
	histogram := promclient.NewHistogram(promclient.HistogramOpts{
		Name:    "e2e_request_duration_seconds",
		Buckets: []float64{0.1, 1},
	})
	summary := promclient.NewSummary(promclient.SummaryOpts{Name: "e2e_response_size_bytes"})
	reg.MustRegister(gauge, counter, histogram, summary)
	gauge.Set(42)

	// OpenMetrics is left off: its _created series would give the receiver start times and
	// bypass the metric_start_time processor.
	handler := promhttp.HandlerFor(reg, promhttp.HandlerOpts{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		counter.Add(step)
		histogram.Observe(0.5)
		summary.Observe(100)
		handler.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)
	return srv.Listener.Addr().String()
}

// pointsOf returns the metrics with the given name from all payloads of the resource with the
// given service.name.
func pointsOf(sink *consumertest.MetricsSink, service, metric string) []pmetric.Metric {
	var out []pmetric.Metric
	for _, md := range sink.AllMetrics() {
		for _, rm := range md.ResourceMetrics().All() {
			if v, ok := rm.Resource().Attributes().Get(oteltest.ServiceNameAttribute); !ok || v.AsString() != service {
				continue
			}
			for _, sm := range rm.ScopeMetrics().All() {
				for _, m := range sm.Metrics().All() {
					if m.Name() == metric {
						out = append(out, m)
					}
				}
			}
		}
	}
	return out
}

// assertStartTimestamps checks that every sum, histogram and summary point of the metrics has
// a non-zero start timestamp that is not after its timestamp. It returns the number of points seen.
func assertStartTimestamps(t *testing.T, metrics []pmetric.Metric) int {
	seen := 0
	check := func(name string, start, ts pcommon.Timestamp) {
		seen++
		assert.NotZero(t, start, "StartTimestamp should be non-zero for metric %s", name)
		assert.LessOrEqual(t, start, ts, "StartTimestamp should not be after Timestamp for metric %s", name)
	}
	for _, m := range metrics {
		switch m.Type() {
		case pmetric.MetricTypeSum:
			for _, dp := range m.Sum().DataPoints().All() {
				check(m.Name(), dp.StartTimestamp(), dp.Timestamp())
			}
		case pmetric.MetricTypeHistogram:
			for _, dp := range m.Histogram().DataPoints().All() {
				check(m.Name(), dp.StartTimestamp(), dp.Timestamp())
			}
		case pmetric.MetricTypeExponentialHistogram:
			for _, dp := range m.ExponentialHistogram().DataPoints().All() {
				check(m.Name(), dp.StartTimestamp(), dp.Timestamp())
			}
		case pmetric.MetricTypeSummary:
			for _, dp := range m.Summary().DataPoints().All() {
				check(m.Name(), dp.StartTimestamp(), dp.Timestamp())
			}
		case pmetric.MetricTypeGauge:
			// Gauges have no start time: they are not processed by metric_start_time.
		default:
			t.Errorf("unexpected metric type %s for metric %s", m.Type(), m.Name())
		}
	}
	return seen
}

// TestE2E_PrometheusScrape tests the "Scrape data from Prometheus" use case
// See: https://docs.dynatrace.com/docs/shortlink/otel-collector-cases-prometheus
func TestE2E_PrometheusScrape(t *testing.T) {
	sink, sinkPorts := oteltest.StartMetricsSink(t)
	targetAddr := startTarget(t)
	telemetryPort := strconv.Itoa(testutil.GetAvailablePort(t))
	healthPort := testutil.GetAvailablePort(t)

	cfg := testutil.LoadConfigExample(t, "prometheus.yaml").
		ReplaceString("prometheus-node-exporter:9100", targetAddr).
		ReplaceString("127.0.0.1:8888", "127.0.0.1:"+telemetryPort).
		Replace("internal metrics port", func(s string) string {
			port, err := strconv.Atoi(telemetryPort)
			require.NoError(t, err)
			return testutil.SetInternalMetricsPort(s, port)
		}).
		Replace("scrape intervals", func(s string) string {
			return strings.ReplaceAll(s, "scrape_interval: 60s", "scrape_interval: 1s")
		}).
		Replace("health_check port", func(s string) string { return testutil.ReplaceHealthCheckPort(s, healthPort) }).
		Replace("exporter endpoint", func(s string) string { return testutil.ReplaceDynatraceExporterEndpoint(s, sinkPorts.Http) }).
		Replace("sending_queue and retry_on_failure", func(s string) string { return testutil.DisableSendingQueueAndRetry(s, "otlp_http") })

	testutil.StartCollector(t, testutil.CollectorParams{
		Name:      "prometheus",
		Config:    cfg.String(),
		ReadyPort: healthPort,
	})

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		points := 0
		for _, m := range pointsOf(sink, targetService, "e2e_requests_total") {
			points += m.Sum().DataPoints().Len()
		}
		assert.GreaterOrEqual(c, points, 2)
	}, 60*time.Second, 200*time.Millisecond)

	t.Run("target", func(t *testing.T) {
		// ScanForServiceMetrics only inspects the first payload of the resource, which lacks
		// the cumulative metrics: cumulative_to_delta drops their first point.
		for _, name := range []string{"e2e_gauge", "e2e_requests_total", "e2e_request_duration_seconds", "e2e_response_size_bytes"} {
			assert.NotEmpty(t, pointsOf(sink, targetService, name), "metric %s not found", name)
		}
		gauges := pointsOf(sink, targetService, "e2e_gauge")
		require.NotEmpty(t, gauges)
		for _, m := range gauges {
			require.Equal(t, pmetric.MetricTypeGauge, m.Type())
			for _, dp := range m.Gauge().DataPoints().All() {
				assert.InDelta(t, 42, dp.DoubleValue(), 0)
			}
		}
	})

	t.Run("self-scrape", func(t *testing.T) {
		oteltest.ScanForServiceMetrics(t, sink, collectorService, []string{
			"otelcol_process_memory_rss", "scrape_duration_seconds", "scrape_samples_post_metric_relabeling",
		})
	})

	t.Run("cumulative to delta", func(t *testing.T) {
		counters := pointsOf(sink, targetService, "e2e_requests_total")
		require.NotEmpty(t, counters)
		for _, m := range counters {
			require.Equal(t, pmetric.MetricTypeSum, m.Type())
			assert.True(t, m.Sum().IsMonotonic())
			assert.Equal(t, pmetric.AggregationTemporalityDelta, m.Sum().AggregationTemporality())
			for _, dp := range m.Sum().DataPoints().All() {
				assert.InDelta(t, step, dp.DoubleValue(), 0)
			}
		}

		histograms := pointsOf(sink, targetService, "e2e_request_duration_seconds")
		require.NotEmpty(t, histograms)
		for _, m := range histograms {
			require.Equal(t, pmetric.MetricTypeHistogram, m.Type())
			assert.Equal(t, pmetric.AggregationTemporalityDelta, m.Histogram().AggregationTemporality())
			for _, dp := range m.Histogram().DataPoints().All() {
				assert.EqualValues(t, 1, dp.Count())
			}
		}
	})

	t.Run("start timestamps", func(t *testing.T) {
		var target []pmetric.Metric
		for _, name := range []string{"e2e_requests_total", "e2e_request_duration_seconds", "e2e_response_size_bytes"} {
			target = append(target, pointsOf(sink, targetService, name)...)
		}
		assert.Positive(t, assertStartTimestamps(t, target), "no sum, histogram or summary points from the target")

		// Only the latest payload of the Collector's own metrics, as the first scrapes may
		// predate the start time being known.
		latest := latestResourceMetrics(sink, collectorService)
		require.NotNil(t, latest, "no metrics for service %s", collectorService)
		var self []pmetric.Metric
		for _, sm := range latest.ScopeMetrics().All() {
			for _, m := range sm.Metrics().All() {
				self = append(self, m)
			}
		}
		assert.Positive(t, assertStartTimestamps(t, self), "no sum, histogram or summary points from the Collector")
	})
}

// latestResourceMetrics returns the last resource with the given service.name, or nil.
func latestResourceMetrics(sink *consumertest.MetricsSink, service string) *pmetric.ResourceMetrics {
	all := sink.AllMetrics()
	for i := len(all) - 1; i >= 0; i-- {
		for _, rm := range all[i].ResourceMetrics().All() {
			if v, ok := rm.Resource().Attributes().Get(oteltest.ServiceNameAttribute); ok && v.AsString() == service {
				return &rm
			}
		}
	}
	return nil
}
