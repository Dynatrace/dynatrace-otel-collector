package integration

import (
	"testing"
	"time"

	"github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
	"github.com/stretchr/testify/require"
)

func FilteringScenario(
	t *testing.T,
	dataProvider testbed.DataProvider,
	sender testbed.DataSender,
	receiver testbed.DataReceiver,
	validator testbed.TestCaseValidator,
	processors map[string]string,
	extensions map[string]string,
) {
	agentProc := testbed.NewChildProcessCollector(testbed.WithAgentExePath(testutil.CollectorTestsExecPath))

	configStr := testutil.CreateGeneralConfigYaml(t, sender, receiver, processors, extensions)

	configCleanup, err := agentProc.PrepareConfig(configStr)
	require.NoError(t, err)
	t.Cleanup(configCleanup)

	tc := testbed.NewTestCase(
		t,
		dataProvider,
		sender,
		receiver,
		agentProc,
		validator,
		&testbed.CorrectnessResults{},
	)
	t.Cleanup(tc.Stop)

	tc.EnableRecording()
	tc.StartBackend()
	tc.StartAgent()

	tc.StartLoad(testbed.LoadOptions{
		DataItemsPerSecond: 3,
		ItemsPerBatch:      3,
	})
	tc.Sleep(2 * time.Second)
	tc.StopLoad()

	tc.WaitForN(func() bool { return tc.LoadGenerator.DataItemsSent() == tc.MockBackend.DataItemsReceived() },
		time.Second*30,
		"all data items received")

	tc.ValidateData()
}
