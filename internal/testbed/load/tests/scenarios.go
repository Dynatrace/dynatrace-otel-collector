package loadtest

import (
	"fmt"
	"math/rand"
	"path"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
)

// createConfigYaml creates a collector config file that corresponds to the
// sender and receiver used in the test and returns the config file name.
// Map of processor names to their configs. Config is in YAML and must be
// indented by 2 spaces. Processors will be placed between batch and queue for traces
// pipeline. For metrics pipeline these will be sole processors.
func createConfigYaml(
	t *testing.T,
	sender testbed.DataSender,
	receiver testbed.DataReceiver,
	resultDir string,
	processors map[string]string,
	extensions map[string]string,
) string {

	// Create a config. Note that our DataSender is used to generate a config for Collector's
	// receiver and our DataReceiver is used to generate a config for Collector's exporter.
	// This is because our DataSender sends to Collector's receiver and our DataReceiver
	// receives from Collector's exporter.

	// Prepare extra processor config section and comma-separated list of extra processor
	// names to use in corresponding "processors" settings.
	processorsSections := ""
	processorsList := ""
	if len(processors) > 0 {
		first := true
		for name, cfg := range processors {
			processorsSections += cfg + "\n"
			if !first {
				processorsList += ","
			}
			processorsList += name
			first = false
		}
	}

	// Prepare extra extension config section and comma-separated list of extra extension
	// names to use in corresponding "extensions" settings.
	extensionsSections := ""
	extensionsList := ""
	if len(extensions) > 0 {
		first := true
		for name, cfg := range extensions {
			extensionsSections += cfg + "\n"
			if !first {
				extensionsList += ","
			}
			extensionsList += name
			first = false
		}
	}

	// Set pipeline based on DataSender type
	var pipeline string
	switch sender.(type) {
	case testbed.TraceDataSender:
		pipeline = "traces"
	case testbed.MetricDataSender:
		pipeline = "metrics"
	case testbed.LogDataSender:
		pipeline = "logs"
	default:
		t.Error("Invalid DataSender type")
	}

	format := `
receivers:%v
exporters:%v
processors:
  %s

extensions:
  pprof:
    save_to_file: %v/cpu.prof
  %s

service:
  extensions: [pprof, %s]
  pipelines:
    %s:
      receivers: [%v]
      processors: [%s]
      exporters: [%v]
`

	// Put corresponding elements into the config template to generate the final config.
	return fmt.Sprintf(
		format,
		sender.GenConfigYAMLStr(),
		receiver.GenConfigYAMLStr(),
		processorsSections,
		resultDir,
		extensionsSections,
		extensionsList,
		pipeline,
		sender.ProtocolName(),
		processorsList,
		receiver.ProtocolName(),
	)
}

// Scenario10kItemsPerSecond runs 10k data items/sec test using specified sender and receiver protocols.
func Scenario10kItemsPerSecond(
	t *testing.T,
	sender testbed.DataSender,
	receiver testbed.DataReceiver,
	resourceSpec testbed.ResourceSpec,
	resultsSummary testbed.TestResultsSummary,
	processors map[string]string,
	extensions map[string]string,
) {
	attributes := make(map[string]string)

	for i := 0; i < 50; i++ {
		key := "key" + strconv.Itoa(i)
		value := "value" + strconv.Itoa(rand.Intn(1000))
		attributes[key] = value
	}

	loadOptions := testbed.LoadOptions{
		DataItemsPerSecond: 10_000,
		ItemsPerBatch:      100,
		Parallel:           1,
		Attributes:         attributes,
	}
	GenericScenario(t, sender, receiver, resourceSpec, resultsSummary, processors, extensions, loadOptions)
}

// Scenario100kItemsPerSecond runs 10k data items/sec test using specified sender and receiver protocols.
func Scenario100kItemsPerSecond(
	t *testing.T,
	sender testbed.DataSender,
	receiver testbed.DataReceiver,
	resourceSpec testbed.ResourceSpec,
	resultsSummary testbed.TestResultsSummary,
	processors map[string]string,
	extensions map[string]string,
) {
	loadOptions := testbed.LoadOptions{
		DataItemsPerSecond: 100_000,
		ItemsPerBatch:      100,
		Parallel:           1,
	}
	GenericScenario(t, sender, receiver, resourceSpec, resultsSummary, processors, extensions, loadOptions)
}

func GenericScenario(
	t *testing.T,
	sender testbed.DataSender,
	receiver testbed.DataReceiver,
	resourceSpec testbed.ResourceSpec,
	resultsSummary testbed.TestResultsSummary,
	processors map[string]string,
	extensions map[string]string,
	loadOptions testbed.LoadOptions,
) {
	resultDir, err := filepath.Abs(path.Join("results", t.Name()))
	require.NoError(t, err)

	agentProc := testbed.NewChildProcessCollector(testbed.WithEnvVar("GOMAXPROCS", "2"))

	configStr := createConfigYaml(t, sender, receiver, resultDir, processors, extensions)
	configCleanup, err := agentProc.PrepareConfig(configStr)
	require.NoError(t, err)
	defer configCleanup()

	dataProvider := testbed.NewPerfTestDataProvider(loadOptions)
	tc := testbed.NewTestCase(
		t,
		dataProvider,
		sender,
		receiver,
		agentProc,
		&testbed.PerfTestValidator{},
		resultsSummary,
		testbed.WithResourceLimits(resourceSpec),
	)
	t.Cleanup(tc.Stop)

	tc.StartBackend()
	tc.StartAgent()

	tc.StartLoad(loadOptions)

	tc.WaitFor(func() bool { return tc.LoadGenerator.DataItemsSent() > 0 }, "load generator started")

	tc.Sleep(tc.Duration)

	tc.StopLoad()

	tc.WaitForN(func() bool { return tc.LoadGenerator.DataItemsSent() == tc.MockBackend.DataItemsReceived() },
		time.Second*30,
		"all data items received")

	tc.ValidateData()
}
