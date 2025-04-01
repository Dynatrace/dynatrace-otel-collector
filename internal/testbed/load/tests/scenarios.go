package loadtest

import (
	"context"
	"fmt"
	"math/rand"
	"path"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/testutil"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
)

type scrapeLoadOptions struct {
	numberOfMetrics            int
	scrapeIntervalMilliSeconds int
}

type ExtendedLoadOptions struct {
	loadOptions       *testbed.LoadOptions
	resourceSpec      testbed.ResourceSpec
	attrCount         int
	attrSizeByte      int
	attrKeySizeByte   int
	scrapeLoadOptions scrapeLoadOptions
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

	configStr := testutil.CreateLoadTestConfigYaml(t, sender, receiver, resultDir, processors, extensions)
	configCleanup, err := agentProc.PrepareConfig(t, configStr)
	require.NoError(t, err)
	defer configCleanup()

	dataProvider := testbed.NewPerfTestDataProvider(*loadOptions.loadOptions)
	tc := testbed.NewTestCase(
		t,
		dataProvider,
		sender,
		receiver,
		agentProc,
		&testbed.PerfTestValidator{
			IncludeLimitsInReport: true,
		},
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

	agentProc := testbed.NewChildProcessCollector(testbed.WithEnvVar("GOMAXPROCS", "2"))

	configStr := testutil.CreateLoadTestConfigYaml(t, sender, receiver, resultDir, processors, extensions)

	// replace the default scrape interval duration with the interval defined in the load options
	configStr = strings.Replace(
		configStr,
		"scrape_interval: 100ms",
		fmt.Sprintf("scrape_interval: %dms", loadOptions.scrapeLoadOptions.scrapeIntervalMilliSeconds),
		1,
	)
	configCleanup, err := agentProc.PrepareConfig(t, configStr)
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
			perfTestValidator: &testbed.PerfTestValidator{
				IncludeLimitsInReport: true,
			},
		},
		resultsSummary,
		testbed.WithResourceLimits(loadOptions.resourceSpec),
	)
	t.Cleanup(tc.Stop)

	tc.StartBackend()

	// first generate a fixed number of metrics
	err = sender.Start()
	require.NoError(t, err)

	providerSender, ok := tc.LoadGenerator.(*testbed.ProviderSender)
	require.True(t, ok)
	metricSender, ok := sender.(testbed.MetricDataSender)
	require.True(t, ok)

	for i := 0; i < loadOptions.scrapeLoadOptions.numberOfMetrics; i++ {
		dataItemsSent := atomic.Uint64{}
		providerSender.Provider.SetLoadGeneratorCounters(&dataItemsSent)
		metrics, _ := providerSender.Provider.GenerateMetrics()
		metricSender.ConsumeMetrics(context.Background(), metrics)
		tc.LoadGenerator.IncDataItemsSent()
	}

	tc.StartAgent()

	tc.Sleep(tc.Duration)

	tc.StopLoad()

	tc.WaitForN(func() bool { return tc.LoadGenerator.DataItemsSent() <= tc.MockBackend.DataItemsReceived() },
		time.Second*300,
		"all data items received")

	tc.StopAgent()

	// increase the data items sent by the LoadGenerator until they match the number of received items.
	// this is done because for the pull based scenario, the number of sent data items is the static number of
	// metrics exposed via each prometheus endpoint (e.g. 1000), whereas the number of received items is the
	// number of metrics times the number of performed scrape iterations
	// due to the internal mechanics of the testbed validator's benchmark summary this is then recorded as "dropped_span_count".
	// The name "dropped_span_count" is also misleading here, as this is calculated from the number of generated data items minus
	// the number of received items - which don't have to be spans in each case, but can also be metrics or logs.

	for i := tc.LoadGenerator.DataItemsSent(); i < tc.MockBackend.DataItemsReceived(); i++ {
		tc.LoadGenerator.IncDataItemsSent()
	}

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
