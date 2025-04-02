package integration

import (
	"os"
	"path"
	"testing"
	"time"

	"github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
	"github.com/stretchr/testify/require"
)

const ConfigExamplesDir = "../../../config_examples"

type SenderFunc func(host string, port int) testbed.DataSender
type ReceiverFunc func(port int) *testbed.BaseOTLPDataReceiver

func FilteringScenario(
	t *testing.T,
	dataProvider testbed.DataProvider,
	senderFunc SenderFunc,
	receiverFunc ReceiverFunc,
	validator testbed.TestCaseValidator,
	configName string,
) {
	agentProc := testbed.NewChildProcessCollector(testbed.WithAgentExePath(testutil.CollectorTestsExecPath))

	cfg, err := os.ReadFile(path.Join(ConfigExamplesDir, configName))
	require.NoError(t, err)

	receiverPort := testutil.GetAvailablePort(t)
	exporterPort := testutil.GetAvailablePort(t)

	parsedConfig := string(cfg)
	parsedConfig = testutil.ReplaceOtlpGrpcReceiverPort(parsedConfig, receiverPort)
	parsedConfig = testutil.ReplaceDynatraceExporterEndpoint(parsedConfig, exporterPort)

	configCleanup, err := agentProc.PrepareConfig(t, parsedConfig)
	require.NoError(t, err)
	t.Cleanup(configCleanup)

	tc := testbed.NewTestCase(
		t,
		dataProvider,
		senderFunc(testbed.DefaultHost, receiverPort),
		receiverFunc(exporterPort),
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

func NewOTLPTraceDataSenderWrapper(host string, port int) testbed.DataSender {
	return testbed.NewOTLPTraceDataSender(host, port)
}

func NewOTLPMetricDataSenderWrapper(host string, port int) testbed.DataSender {
	return testbed.NewOTLPMetricDataSender(host, port)
}

func NewOTLPLogsDataSenderWrapper(host string, port int) testbed.DataSender {
	return testbed.NewOTLPLogsDataSender(host, port)
}
