// Copyright Dynatrace LLC
// SPDX-License-Identifier: Apache-2.0

package statsd

import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/oteltest"
	"github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/testutil"
)

// startCollector starts the DT Collector with config_examples/statsd.yaml and returns the UDP
// port of the StatsD receiver and the sink the Collector exports to.
func startCollector(t *testing.T) (int, *consumertest.MetricsSink) {
	sink, sinkPorts := oteltest.StartMetricsSink(t)
	statsdPort := testutil.GetAvailableUDPPort(t)
	healthPort := testutil.GetAvailablePort(t)

	cfg := testutil.LoadConfigExample(t, "statsd.yaml").
		Replace("statsd receiver port", func(s string) string { return testutil.ReplaceStatsdReceiverPort(s, statsdPort) }).
		ReplaceString("aggregation_interval: 60s", "aggregation_interval: 1s").
		Replace("health_check port", func(s string) string { return testutil.ReplaceHealthCheckPort(s, healthPort) }).
		Replace("exporter endpoint", func(s string) string { return testutil.ReplaceDynatraceExporterEndpoint(s, sinkPorts.Http) }).
		Replace("sending_queue and retry_on_failure", func(s string) string { return testutil.DisableSendingQueueAndRetry(s, "otlp_http") }).
		Replace("internal metrics", testutil.DisableInternalMetrics)

	testutil.StartCollector(t, testutil.CollectorParams{
		Name:      "statsd",
		Config:    cfg.String(),
		ReadyPort: healthPort,
	})
	testutil.WaitForHealthCheck(t, healthPort)
	return statsdPort, sink
}

// TestE2E_StatsdReceiver tests the "Ingest data from Statsd" use case
// See: https://docs.dynatrace.com/docs/shortlink/otel-collector-cases-statsd
func TestE2E_StatsdReceiver(t *testing.T) {
	port, sink := startCollector(t)

	conn, err := net.Dial("udp", fmt.Sprintf("127.0.0.1:%d", port))
	require.NoError(t, err)
	defer conn.Close()

	// One datagram, one metric per line.
	payload := "test.metric:42|g|#myKey:myVal\n" +
		"timerMetric:320|ms|@0.1|#timerKey:timerVal\n" +
		"histogramMetric:5|h|#histKey:histVal\n" +
		"distributionMetric:7|d|#distKey:distVal\n"
	_, err = conn.Write([]byte(payload))
	require.NoError(t, err)

	expected := []string{"test.metric", "timerMetric", "histogramMetric", "distributionMetric"}
	var got map[string][]pmetric.Metric
	require.Eventually(t, func() bool {
		got = collectMetrics(sink)
		for _, name := range expected {
			if len(got[name]) == 0 {
				return false
			}
		}
		return true
	}, 30*time.Second, 200*time.Millisecond, "not all metrics were received")

	assert.Len(t, got, len(expected), "unexpected metrics received")
	for _, name := range expected {
		require.Len(t, got[name], 1, "metric %s", name)
	}

	gauge := got["test.metric"][0]
	require.Equal(t, pmetric.MetricTypeGauge, gauge.Type())
	require.Equal(t, 1, gauge.Gauge().DataPoints().Len())
	gdp := gauge.Gauge().DataPoints().At(0)
	assert.Equal(t, 42.0, gdp.DoubleValue())
	assertAttrs(t, gdp.Attributes().AsRaw(), "myKey", "myVal")

	timer := expHistogramPoint(t, got["timerMetric"][0])
	assert.Equal(t, uint64(10), timer.Count()) // sample rate 0.1
	assert.Equal(t, 3200.0, timer.Sum())
	assert.Equal(t, 320.0, timer.Min())
	assert.Equal(t, 320.0, timer.Max())
	require.Equal(t, 1, timer.Positive().BucketCounts().Len())
	assert.Equal(t, uint64(10), timer.Positive().BucketCounts().At(0))
	assertAttrs(t, timer.Attributes().AsRaw(), "timerKey", "timerVal")

	for _, tc := range []struct {
		name, key, val string
		v              float64
	}{
		{"histogramMetric", "histKey", "histVal", 5},
		{"distributionMetric", "distKey", "distVal", 7},
	} {
		dp := expHistogramPoint(t, got[tc.name][0])
		assert.Equal(t, uint64(1), dp.Count(), tc.name)
		assert.Equal(t, tc.v, dp.Sum(), tc.name)
		assert.Equal(t, tc.v, dp.Min(), tc.name)
		assert.Equal(t, tc.v, dp.Max(), tc.name)
		assertAttrs(t, dp.Attributes().AsRaw(), tc.key, tc.val)
	}
}

// collectMetrics groups every metric received by the sink by name.
func collectMetrics(sink *consumertest.MetricsSink) map[string][]pmetric.Metric {
	out := map[string][]pmetric.Metric{}
	for _, md := range sink.AllMetrics() {
		rms := md.ResourceMetrics()
		for i := 0; i < rms.Len(); i++ {
			sms := rms.At(i).ScopeMetrics()
			for j := 0; j < sms.Len(); j++ {
				ms := sms.At(j).Metrics()
				for k := 0; k < ms.Len(); k++ {
					out[ms.At(k).Name()] = append(out[ms.At(k).Name()], ms.At(k))
				}
			}
		}
	}
	return out
}

func expHistogramPoint(t *testing.T, m pmetric.Metric) pmetric.ExponentialHistogramDataPoint {
	t.Helper()
	require.Equal(t, pmetric.MetricTypeExponentialHistogram, m.Type(), m.Name())
	require.Equal(t, 1, m.ExponentialHistogram().DataPoints().Len(), m.Name())
	return m.ExponentialHistogram().DataPoints().At(0)
}

func assertAttrs(t *testing.T, got map[string]any, key, val string) {
	t.Helper()
	assert.Equal(t, map[string]any{key: val}, got)
}
