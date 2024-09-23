package loadtest

import (
	"context"
	"fmt"
	"path"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/exp/rand"

	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
)

type ExtendedLoadOptions struct {
	loadOptions     *testbed.LoadOptions
	resourceSpec    testbed.ResourceSpec
	attrCount       int
	attrSizeByte    int
	attrKeySizeByte int
}

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

func GenericScenario(
	t *testing.T,
	sender testbed.DataSender,
	receiver testbed.DataReceiver,
	resultsSummary testbed.TestResultsSummary,
	processors map[string]string,
	extensions map[string]string,
	loadOptions ExtendedLoadOptions,
) {
	resultDir, err := filepath.Abs(path.Join("results", t.Name()))
	require.NoError(t, err)
	loadOptions = constructAttributes(loadOptions)

	agentProc := testbed.NewChildProcessCollector(testbed.WithEnvVar("GOMAXPROCS", "2"))

	configStr := createConfigYaml(t, sender, receiver, resultDir, processors, extensions)
	configCleanup, err := agentProc.PrepareConfig(configStr)
	require.NoError(t, err)
	defer configCleanup()

	dataProvider := testbed.NewPerfTestDataProvider(*loadOptions.loadOptions)
	tc := testbed.NewTestCase(
		t,
		dataProvider,
		sender,
		receiver,
		agentProc,
		&testbed.PerfTestValidator{},
		resultsSummary,
		testbed.WithResourceLimits(loadOptions.resourceSpec),
	)
	t.Cleanup(tc.Stop)

	tc.StartBackend()
	tc.StartAgent()

	tc.StartLoad(*loadOptions.loadOptions)

	tc.WaitForN(func() bool { return tc.LoadGenerator.DataItemsSent() > 0 }, 30*time.Second, "load generator started")

	tc.Sleep(tc.Duration)

	tc.StopLoad()

	tc.WaitForN(func() bool { return tc.LoadGenerator.DataItemsSent() == tc.MockBackend.DataItemsReceived() },
		time.Second*30,
		"all data items received")

	tc.ValidateData()
}

func PullBasedSenderScenario(
	t *testing.T,
	sender testbed.DataSender,
	receiver testbed.DataReceiver,
	resultsSummary testbed.TestResultsSummary,
	processors map[string]string,
	extensions map[string]string,
	loadOptions ExtendedLoadOptions,
) {
	resultDir, err := filepath.Abs(path.Join("results", t.Name()))
	require.NoError(t, err)
	loadOptions = constructAttributes(loadOptions)

	agentProc := testbed.NewChildProcessCollector(testbed.WithEnvVar("GOMAXPROCS", "2"))

	configStr := createConfigYaml(t, sender, receiver, resultDir, processors, extensions)

	configCleanup, err := agentProc.PrepareConfig(configStr)
	require.NoError(t, err)
	defer configCleanup()

	dataProvider := testbed.NewPerfTestDataProvider(*loadOptions.loadOptions)
	tc := testbed.NewTestCase(
		t,
		dataProvider,
		sender,
		receiver,
		agentProc,
		&simpleTestcaseValidator{
			perfTestValidator: &testbed.PerfTestValidator{},
		},
		resultsSummary,
		testbed.WithResourceLimits(loadOptions.resourceSpec),
	)
	t.Cleanup(tc.Stop)

	tc.StartBackend()

	// first generate 10k metrics

	sender.Start()
	for i := 0; i < 1000; i++ {
		dataItemsSent := atomic.Uint64{}
		tc.LoadGenerator.(*testbed.ProviderSender).Provider.SetLoadGeneratorCounters(&dataItemsSent)
		metrics, _ := tc.LoadGenerator.(*testbed.ProviderSender).Provider.GenerateMetrics()
		sender.(testbed.MetricDataSender).ConsumeMetrics(context.Background(), metrics)
		tc.LoadGenerator.IncDataItemsSent()
	}

	//tc.StartLoad(*loadOptions.loadOptions)
	tc.StartAgent()

	tc.WaitForN(func() bool { return tc.LoadGenerator.DataItemsSent() > 0 }, 30*time.Second, "load generator started")

	tc.Sleep(tc.Duration)

	tc.StopLoad()

	tc.WaitForN(func() bool { return tc.LoadGenerator.DataItemsSent() <= tc.MockBackend.DataItemsReceived() },
		time.Second*300,
		"all data items received")

	tc.ValidateData()
}

func constructAttributes(loadOptions ExtendedLoadOptions) ExtendedLoadOptions {
	loadOptions.loadOptions.Attributes = make(map[string]string)

	// Generate attributes.
	for i := 0; i < loadOptions.attrCount; i++ {
		attrName := genRandByteString(rand.Intn(loadOptions.attrKeySizeByte*2-1) + 1)
		loadOptions.loadOptions.Attributes[attrName] = genRandByteString(rand.Intn(loadOptions.attrSizeByte*2-1) + 1)
	}
	return loadOptions
}

func genRandByteString(length int) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = byte(rand.Intn(128))
	}
	return string(b)
}

type simpleTestcaseValidator struct {
	perfTestValidator *testbed.PerfTestValidator
}

func (simpleTestcaseValidator) Validate(tc *testbed.TestCase) {
}

func (s simpleTestcaseValidator) RecordResults(tc *testbed.TestCase) {
	s.perfTestValidator.RecordResults(tc)
}
