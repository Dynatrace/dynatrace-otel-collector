package filtering

import (
	"sync/atomic"

	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"

	oteltestbed "github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
)

type dataProvider struct {
	dataItemsGenerated *atomic.Uint64
	InputData          data
	tracesCounter      int
	metricsCouter      int
	logsCounter        int
}

func NewDataProvider(inputData data) oteltestbed.DataProvider {
	return &dataProvider{
		InputData:     inputData,
		metricsCouter: 0,
		logsCounter:   0,
		tracesCounter: 0,
	}
}

func (dp *dataProvider) SetLoadGeneratorCounters(dataItemsGenerated *atomic.Uint64) {
	dp.dataItemsGenerated = dataItemsGenerated
}

func (dp *dataProvider) GenerateTraces() (ptrace.Traces, bool) {
	return dp.InputData.Traces[dp.tracesCounter], false
}

func (dp *dataProvider) GenerateMetrics() (pmetric.Metrics, bool) {
	return dp.InputData.Metrics[dp.metricsCouter], false
}

func (dp *dataProvider) GenerateLogs() (plog.Logs, bool) {
	return dp.InputData.Logs[dp.logsCounter], true
}
