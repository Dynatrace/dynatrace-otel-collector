package filtering

import (
	"path"
	"path/filepath"
	"testing"
	"time"

	"github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/components"
	"github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

type data struct {
	Traces  []ptrace.Traces
	Metrics []pmetric.Metrics
	Logs    []plog.Logs
}

var (
	defaultProcessors = map[string]string{
		"memory_limiter": `
  memory_limiter:
    check_interval: 1s
    limit_percentage: 100
`,
		"batch": `
  batch:
    send_batch_max_size: 1000
    timeout: 10s
    send_batch_size : 800
`,
	}
)

var (
	performanceResultsSummary testbed.TestResultsSummary = &testbed.PerformanceResults{}
)

func FilteringScenario(
	t *testing.T,
	sender testbed.DataSender,
	receiver testbed.DataReceiver,
	inputData data,
	processors map[string]string,
	extensions map[string]string,
) data {
	resultDir, err := filepath.Abs(path.Join("results", t.Name()))
	require.NoError(t, err)

	factories, err := components.Components()
	require.NoError(t, err, "default components resulted in: %v", err)
	agentProc := testbed.NewInProcessCollector(factories)

	configStr := testutil.CreateConfigYaml(t, sender, receiver, resultDir, processors, extensions)
	t.Log(configStr)
	configCleanup, err := agentProc.PrepareConfig(configStr)
	require.NoError(t, err)
	defer configCleanup()

	dataProvider := NewDataProvider(inputData)
	// options := testbed.LoadOptions{
	// 	DataItemsPerSecond: 1,
	// 	ItemsPerBatch:      1,
	// 	Parallel:           1,
	// }
	// dataProvider := testbed.NewPerfTestDataProvider(options)
	tc := testbed.NewTestCase(
		t,
		dataProvider,
		sender,
		receiver,
		agentProc,
		&testbed.PerfTestValidator{},
		performanceResultsSummary,
		testbed.WithResourceLimits(testbed.ResourceSpec{
			ExpectedMaxCPU: 130,
			ExpectedMaxRAM: 120,
		}),
	)
	t.Cleanup(tc.Stop)

	tc.StartBackend()
	tc.StartAgent()

	tc.StartLoad(testbed.LoadOptions{
		DataItemsPerSecond: 1,
		ItemsPerBatch:      1,
	})

	tc.WaitForN(func() bool { return tc.LoadGenerator.DataItemsSent() > 0 }, 30*time.Second, "load generator started")

	tc.Sleep(tc.Duration)

	tc.StopLoad()

	tc.WaitForN(func() bool { return tc.LoadGenerator.DataItemsSent() == tc.MockBackend.DataItemsReceived() },
		time.Second*30,
		"all data items received")

	tc.ValidateData()

	return data{
		Traces:  tc.MockBackend.ReceivedTraces,
		Metrics: tc.MockBackend.ReceivedMetrics,
		Logs:    tc.MockBackend.ReceivedLogs,
	}
}
