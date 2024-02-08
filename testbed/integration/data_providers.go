package integration

import (
	"sync/atomic"

	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

type LoadOptions struct {
	// DataItemsPerSecond specifies how many spans, metric data points, or log
	// records to generate each second.
	DataItemsPerSecond int

	// ItemsPerBatch specifies how many spans, metric data points, or log
	// records per batch to generate. Should be greater than zero. The number
	// of batches generated per second will be DataItemsPerSecond/ItemsPerBatch.
	ItemsPerBatch int

	// Attributes to add to each generated data item. Can be empty.
	Attributes map[string]string

	// Parallel specifies how many goroutines to send from.
	Parallel int
}

// sampleConfigsDataProvider in an implementation of the DataProvider
// for use to e2e test the configuration examples in ./config_examples
type sampleConfigsDataProvider struct {
	traces             ptrace.Traces
	dataItemsGenerated *atomic.Uint64
}

// GenerateLogs implements testbed.DataProvider.
func (*sampleConfigsDataProvider) GenerateLogs() (plog.Logs, bool) {
	return plog.NewLogs(), true
}

// GenerateMetrics implements testbed.DataProvider.
func (*sampleConfigsDataProvider) GenerateMetrics() (pmetric.Metrics, bool) {
	return pmetric.NewMetrics(), true
}

// GenerateTraces implements testbed.DataProvider.
func (dp *sampleConfigsDataProvider) GenerateTraces() (ptrace.Traces, bool) {
	// We want to send a fixed number of spans always
	if int(dp.dataItemsGenerated.Load()) == dp.traces.SpanCount() {
		return ptrace.NewTraces(), false
	}

	dp.dataItemsGenerated.Add(uint64(dp.traces.SpanCount()))
	return dp.traces, false
}

// SetLoadGeneratorCounters implements testbed.DataProvider.
func (dp *sampleConfigsDataProvider) SetLoadGeneratorCounters(dataItemsGenerated *atomic.Uint64) {
	dp.dataItemsGenerated = dataItemsGenerated
}

func NewSampleConfigsDataProvider(traces ptrace.Traces) testbed.DataProvider {
	return &sampleConfigsDataProvider{
		traces: traces,
	}
}
