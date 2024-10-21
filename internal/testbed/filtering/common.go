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

type inputData struct {
	Traces  ptrace.Traces
	Metrics pmetric.Metrics
	Logs    plog.Logs
}

type receivedData struct {
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
	inputData inputData,
	processors map[string]string,
	extensions map[string]string,
) receivedData {
	resultDir, err := filepath.Abs(path.Join("results", t.Name()))
	require.NoError(t, err)

	factories, err := components.Components()
	require.NoError(t, err, "default components resulted in: %v", err)
	agentProc := testbed.NewInProcessCollector(factories)

	configStr := testutil.CreateConfigYaml(t, sender, receiver, resultDir, processors, extensions)
	configCleanup, err := agentProc.PrepareConfig(configStr)
	require.NoError(t, err)
	defer configCleanup()

	dataProvider := NewDataProvider(inputData)
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
	tc.MockBackend.EnableRecording()
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

	return receivedData{
		Traces:  tc.MockBackend.ReceivedTraces,
		Metrics: tc.MockBackend.ReceivedMetrics,
		Logs:    tc.MockBackend.ReceivedLogs,
	}
}
