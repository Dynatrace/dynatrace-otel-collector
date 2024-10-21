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
	InputData          inputData
}

func NewDataProvider(inputData inputData) oteltestbed.DataProvider {
	return &dataProvider{
		InputData: inputData,
	}
}

func (dp *dataProvider) SetLoadGeneratorCounters(dataItemsGenerated *atomic.Uint64) {
	dp.dataItemsGenerated = dataItemsGenerated
}

func (dp *dataProvider) GenerateTraces() (ptrace.Traces, bool) {
	dp.dataItemsGenerated.Add(1)
	return dp.InputData.Traces, false
}

func (dp *dataProvider) GenerateMetrics() (pmetric.Metrics, bool) {
	dp.dataItemsGenerated.Add(1)
	return dp.InputData.Metrics, false
}

func (dp *dataProvider) GenerateLogs() (plog.Logs, bool) {
	dp.dataItemsGenerated.Add(1)
	return dp.InputData.Logs, true
}
