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
	dataItemsGenerated *atomic.Uint64
}

func (*sampleConfigsDataProvider) GenerateLogs() (plog.Logs, bool) {
	return plog.NewLogs(), true
}

func (*sampleConfigsDataProvider) GenerateMetrics() (pmetric.Metrics, bool) {
	return pmetric.NewMetrics(), true
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

func NewSampleConfigsDataProvider(traces ptrace.Traces) testbed.DataProvider {
	return &sampleConfigsDataProvider{
		traces: traces,
	}
}
