package integration

import (
	"sync/atomic"

	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

var _ testbed.DataProvider = &sampleConfigsDataProvider{}

type sampleConfigsDataProvider struct {
	traces             ptrace.Traces
	metrics            pmetric.Metrics
	logs               plog.Logs
	dataItemsGenerated *atomic.Uint64
}

func (dp *sampleConfigsDataProvider) GenerateLogs() (plog.Logs, bool) {
	// We want to send a fixed number of logs always
	if int(dp.dataItemsGenerated.Load()) == dp.logs.LogRecordCount() {
		return plog.NewLogs(), false
	}

	dp.dataItemsGenerated.Add(uint64(dp.logs.LogRecordCount()))
	return dp.logs, false
}

func (dp *sampleConfigsDataProvider) GenerateMetrics() (pmetric.Metrics, bool) {
	// We want to send a fixed number of metrics always
	if int(dp.dataItemsGenerated.Load()) == dp.metrics.MetricCount() {
		return pmetric.NewMetrics(), false
	}

	dp.dataItemsGenerated.Add(uint64(dp.metrics.MetricCount()))
	return dp.metrics, false
}

func (dp *sampleConfigsDataProvider) GenerateTraces() (ptrace.Traces, bool) {
	// We want to send a fixed number of spans always
	if int(dp.dataItemsGenerated.Load()) == dp.traces.SpanCount() {
		return ptrace.NewTraces(), false
	}

	dp.dataItemsGenerated.Add(uint64(dp.traces.SpanCount()))
	return dp.traces, false
}

func (dp *sampleConfigsDataProvider) SetLoadGeneratorCounters(dataItemsGenerated *atomic.Uint64) {
	dp.dataItemsGenerated = dataItemsGenerated
}

func NewSampleConfigsTraceDataProvider(traces ptrace.Traces) testbed.DataProvider {
	return &sampleConfigsDataProvider{
		traces: traces,
	}
}

func NewSampleConfigsMetricsDataProvider(metrics pmetric.Metrics) testbed.DataProvider {
	return &sampleConfigsDataProvider{
		metrics: metrics,
	}
}

func NewSampleConfigsLogsDataProvider(logs plog.Logs) testbed.DataProvider {
	return &sampleConfigsDataProvider{
		logs: logs,
	}
}
