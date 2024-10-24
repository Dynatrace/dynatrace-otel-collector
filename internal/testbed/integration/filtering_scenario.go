package integration

import (
	"strings"
	"testing"
	"time"

	"github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
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

type ProcessorConfig struct {
	Processors map[string]any `yaml:"processors"`
}

func extractProcessorsFromYAML(yamlStr []byte) (map[string]string, error) {
	var data ProcessorConfig
	err := yaml.Unmarshal(yamlStr, &data)
	if err != nil {
		return nil, err
	}

	result := make(map[string]string)
	for key, _ := range data.Processors {
		processorYAML, err := yaml.Marshal(data.Processors)
		if err != nil {
			return nil, err
		}

		result[key] = strings.ReplaceAll(string(processorYAML), "\n", "\n"+"  ")
	}

	return result, nil
}
