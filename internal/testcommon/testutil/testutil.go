package testutil

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"net"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"gopkg.in/yaml.v3"
)

type portpair struct {
	first string
	last  string
}

// GetAvailableLocalAddress finds an available local port on tcp network and returns an endpoint
// describing it. The port is available for opening when this function returns
// provided that there is no race by some other code to grab the same port
// immediately.
func GetAvailableLocalAddress(t testing.TB) string {
	return GetAvailableLocalNetworkAddress(t, "tcp")
}

// GetAvailableLocalNetworkAddress finds an available local port on specified network and returns an endpoint
// describing it. The port is available for opening when this function returns
// provided that there is no race by some other code to grab the same port
// immediately.
func GetAvailableLocalNetworkAddress(t testing.TB, network string) string {
	// Retry has been added for windows as net.Listen can return a port that is not actually available. Details can be
	// found in https://github.com/docker/for-win/issues/3171 but to summarize Hyper-V will reserve ranges of ports
	// which do not show up under the "netstat -ano" but can only be found by
	// "netsh interface ipv4 show excludedportrange protocol=tcp".  We'll use []exclusions to hold those ranges and
	// retry if the port returned by GetAvailableLocalAddress falls in one of those them.
	var exclusions []portpair

	portFound := false
	if runtime.GOOS == "windows" {
		exclusions = getExclusionsList(t)
	}

	var endpoint string
	for !portFound {
		endpoint = findAvailableAddress(t, network)
		_, port, err := net.SplitHostPort(endpoint)
		require.NoError(t, err)
		portFound = true
		if runtime.GOOS == "windows" {
			for _, pair := range exclusions {
				if port >= pair.first && port <= pair.last {
					portFound = false
					break
				}
			}
		}
	}

	return endpoint
}

func findAvailableAddress(t testing.TB, network string) string {
	switch network {
	// net.Listen supported network strings
	case "tcp", "tcp4", "tcp6", "unix", "unixpacket":
		ln, err := net.Listen(network, "localhost:0")
		require.NoError(t, err, "Failed to get a free local port")
		// There is a possible race if something else takes this same port before
		// the test uses it, however, that is unlikely in practice.
		defer func() {
			assert.NoError(t, ln.Close())
		}()
		return ln.Addr().String()
	// net.ListenPacket supported network strings
	case "udp", "udp4", "udp6", "unixgram":
		ln, err := net.ListenPacket(network, "localhost:0")
		require.NoError(t, err, "Failed to get a free local port")
		// There is a possible race if something else takes this same port before
		// the test uses it, however, that is unlikely in practice.
		defer func() {
			assert.NoError(t, ln.Close())
		}()
		return ln.LocalAddr().String()
	}
	return ""
}

// Get excluded ports on Windows from the command: netsh interface ipv4 show excludedportrange protocol=tcp
func getExclusionsList(t testing.TB) []portpair {
	cmdTCP := exec.Command("netsh", "interface", "ipv4", "show", "excludedportrange", "protocol=tcp")
	outputTCP, errTCP := cmdTCP.CombinedOutput()
	require.NoError(t, errTCP)
	exclusions := createExclusionsList(t, string(outputTCP))

	cmdUDP := exec.Command("netsh", "interface", "ipv4", "show", "excludedportrange", "protocol=udp")
	outputUDP, errUDP := cmdUDP.CombinedOutput()
	require.NoError(t, errUDP)
	exclusions = append(exclusions, createExclusionsList(t, string(outputUDP))...)

	return exclusions
}

func createExclusionsList(t testing.TB, exclusionsText string) []portpair {
	var exclusions []portpair

	parts := strings.Split(exclusionsText, "--------")
	require.Len(t, parts, 3)
	portsText := strings.Split(parts[2], "*")
	require.Greater(t, len(portsText), 1) // original text may have a suffix like " - Administered port exclusions."
	lines := strings.Split(portsText[0], "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			entries := strings.Fields(strings.TrimSpace(line))
			require.Len(t, entries, 2)
			pair := portpair{entries[0], entries[1]}
			exclusions = append(exclusions, pair)
		}
	}
	return exclusions
}

func GetAvailablePort(t testing.TB) int {
	endpoint := GetAvailableLocalAddress(t)
	_, port, err := net.SplitHostPort(endpoint)
	require.NoError(t, err)

	portInt, err := strconv.Atoi(port)
	require.NoError(t, err)

	return portInt
}

func GetAvailablePorts(t testing.TB, numberOfPorts int) []int {
	ports := make([]int, numberOfPorts)

	for i := 0; i < numberOfPorts; i++ {
		ports[i] = GetAvailablePort(t)
	}
	return ports
}

// Force the state of feature gate for a test
// usage: defer SetFeatureGateForTest("gateName", true)()
func SetFeatureGateForTest(t testing.TB, gate *featuregate.Gate, enabled bool) func() {
	originalValue := gate.IsEnabled()
	require.NoError(t, featuregate.GlobalRegistry().Set(gate.ID(), enabled))
	return func() {
		require.NoError(t, featuregate.GlobalRegistry().Set(gate.ID(), originalValue))
	}
}

const CollectorTestsExecPath string = "../../../bin/dynatrace-otel-collector"

func ReplaceOtlpGrpcReceiverPort(cfg string, receiverPort int) string {
	return strings.Replace(cfg, "4317", strconv.Itoa(receiverPort), 1)
}

func ReplaceJaegerGrpcReceiverPort(cfg string, receiverPort int) string {
	return strings.Replace(cfg, "14250", strconv.Itoa(receiverPort), 1)
}

func ReplaceZipkinReceiverPort(cfg string, receiverPort int) string {
	return strings.Replace(cfg, "9411", strconv.Itoa(receiverPort), 1)
}

func ReplaceSyslogHostReceiverPort(cfg string, receiverPort int) string {
	return strings.Replace(cfg, "54527", strconv.Itoa(receiverPort), 1)
}

func ReplaceSyslogF5ReceiverPort(cfg string, receiverPort int) string {
	return strings.Replace(cfg, "54526", strconv.Itoa(receiverPort), 1)
}

func ReplaceDynatraceExporterEndpoint(cfg string, exporterPort int) string {
	r := strings.NewReplacer(
		"${env:DT_ENDPOINT}", fmt.Sprintf("http://0.0.0.0:%v", exporterPort),
		"${env:API_TOKEN}", "",
	)
	return r.Replace(cfg)
}

func CreateLoadTestConfigYaml(
	t *testing.T,
	sender testbed.DataSender,
	receiver testbed.DataReceiver,
	resultDir string,
	processors map[string]string,
	extensions map[string]string,
) string {

	pprofString := fmt.Sprintf(`
  pprof:
    save_to_file: %v/cpu.prof
`, resultDir)

	if extensions == nil {
		extensions = map[string]string{}
	}

	extensions["pprof"] = pprofString

	return CreateGeneralConfigYaml(t, sender, receiver, processors, extensions)
}

// CreateConfigYaml creates a collector config file that corresponds to the
// sender and receiver used in the test and returns the config file name.
// Map of processor names to their configs. Config is in YAML and must be
// indented by 2 spaces. Processors will be placed between batch and queue for traces
// pipeline. For metrics pipeline these will be sole processors.
func CreateGeneralConfigYaml(
	t *testing.T,
	sender testbed.DataSender,
	receiver testbed.DataReceiver,
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
%s

service:
  extensions: [%s]
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
		extensionsSections,
		extensionsList,
		pipeline,
		sender.ProtocolName(),
		processorsList,
		receiver.ProtocolName(),
	)
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
	for key, value := range data.Processors {
		processorYAML, err := yaml.Marshal(value)
		if err != nil {
			return nil, err
		}

		// marshall removes the starting indentation and aligns the root element(s) of value with indent == 0
		// adding the indentation back
		// name of the processor is indented by 2 spaces, rest of the body, by 4
		result[key] = "  " + key + ":\n    " + strings.ReplaceAll(string(processorYAML), "\n", "\n"+"    ")
	}

	return result, nil
}

const charset = "abcdefghijklmnopqrstuvwxyz0123456789"

// Function to generate a random string of specified length
func GenerateRandomString(length int) (string, error) {
	result := make([]byte, length)
	for i := range result {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", err
		}
		result[i] = charset[num.Int64()]
	}
	return string(result), nil
}

func MergeResources(metrics pmetric.Metrics) pmetric.Metrics {
	m := pmetric.NewMetrics()
	metrics.CopyTo(m)
	new := pmetric.NewMetrics()
	for i := 0; i < m.ResourceMetrics().Len(); i++ {
		attrsHash := pdatautil.MapHash(m.ResourceMetrics().At(i).Resource().Attributes())
		found := false
		for j := 0; j < new.ResourceMetrics().Len(); j++ {
			if pdatautil.MapHash(new.ResourceMetrics().At(j).Resource().Attributes()) == attrsHash {
				m.ResourceMetrics().At(i).ScopeMetrics().MoveAndAppendTo(new.ResourceMetrics().At(j).ScopeMetrics())
				found = true
				break
			}
		}
		if !found {
			m.ResourceMetrics().At(i).MoveTo(new.ResourceMetrics().AppendEmpty())
		}
	}

	return new
}

func MaskParentSpanID(traces ptrace.Traces) {
	for i := 0; i < traces.ResourceSpans().Len(); i++ {
		scopeSpans := traces.ResourceSpans().At(i).ScopeSpans()
		for j := 0; j < scopeSpans.Len(); j++ {
			for k := 0; k < scopeSpans.At(j).Spans().Len(); k++ {
				scopeSpans.At(j).Spans().At(k).SetParentSpanID(pcommon.NewSpanIDEmpty())
			}
		}
	}
}

// Debug can be used in integration tests after a pmetrictest.CompareMetrics to display diverging metrics values
// testutil.Debug(err, t, expectedMerged, actualMerged)
func Debug(err error, t *testing.T, expectedMerged pmetric.Metrics, actualMerged pmetric.Metrics) {
	if err != nil {
		// Print resource counts and details for debugging
		t.Logf("[DEBUG] Expected resource count: %d", expectedMerged.ResourceMetrics().Len())
		t.Logf("[DEBUG] Actual resource count: %d", actualMerged.ResourceMetrics().Len())

		logMetrics := func(prefix string, metrics pmetric.Metrics) {
			for i := 0; i < metrics.ResourceMetrics().Len(); i++ {
				rm := metrics.ResourceMetrics().At(i)
				t.Logf("[DEBUG] %s resource[%d] attributes: %v", prefix, i, rm.Resource().Attributes().AsRaw())
				for s := 0; s < rm.ScopeMetrics().Len(); s++ {
					sm := rm.ScopeMetrics().At(s)
					scope := sm.Scope()
					t.Logf("[DEBUG] %s resource[%d] scope[%d]: name=%q version=%q metrics=%d", prefix, i, s, scope.Name(), scope.Version(), sm.Metrics().Len())
					for mi := 0; mi < sm.Metrics().Len(); mi++ {
						met := sm.Metrics().At(mi)
						t.Logf("[DEBUG] %s resource[%d] scope[%d] metric[%d]: name=%q desc=%q unit=%q type=%s datapoints=%d", prefix, i, s, mi, met.Name(), met.Description(), met.Unit(), met.Type(), countDataPoints(met))
						printDataPoints(prefix, met, t, i, s, mi, scopeInfoFrom(sm))
					}
				}
			}
		}

		logMetrics("Expected", expectedMerged)
		logMetrics("Actual", actualMerged)
	}
}

// helper that builds a readable scope info string
func scopeInfoFrom(sm pmetric.ScopeMetrics) string {
	// for older API use sm.InstrumentationLibrary().Name() / Version()
	name := sm.Scope().Name()
	ver := sm.Scope().Version()
	if name == "" && ver == "" {
		return "unnamed"
	}
	if ver == "" {
		return name
	}
	return name + "@" + ver
}

// Example function to print all resources -> scopes -> metrics
func printAllMetrics(prefix string, rms pmetric.ResourceMetricsSlice, t *testing.T) {
	for rIdx := 0; rIdx < rms.Len(); rIdx++ {
		rm := rms.At(rIdx)
		// print resource attributes once
		t.Logf("[DEBUG] %s Actual resource[%d] attributes: %v", prefix, rIdx, rm.Resource().Attributes().AsRaw())

		// iterate scope metrics
		sms := rm.ScopeMetrics()
		for sIdx := 0; sIdx < sms.Len(); sIdx++ {
			sm := sms.At(sIdx)
			scopeInfo := scopeInfoFrom(sm)
			metrics := sm.Metrics()
			t.Logf("[DEBUG] %s Actual resource[%d] scope[%d:%s]: metrics=%d",
				prefix, rIdx, sIdx, scopeInfo, metrics.Len())

			for mIdx := 0; mIdx < metrics.Len(); mIdx++ {
				met := metrics.At(mIdx)
				// metric header - now includes scope name/version
				t.Logf("[DEBUG] %s Actual resource[%d] scope[%d:%s] metric[%d]: name=%q desc=%q unit=%q type=%v datapoints=%d",
					prefix, rIdx, sIdx, scopeInfo, mIdx, met.Name(), met.Description(), met.Unit(), met.Type(), dataPointsCount(met))

				// call printDataPoints - use version that accepts scope info
				printDataPoints(prefix, met, t, rIdx, sIdx, mIdx, scopeInfo)
			}
		}
	}
}

// small helper to get datapoints count (for the header)
func dataPointsCount(m pmetric.Metric) int {
	switch m.Type() {
	case pmetric.MetricTypeGauge:
		return m.Gauge().DataPoints().Len()
	case pmetric.MetricTypeSum:
		return m.Sum().DataPoints().Len()
	case pmetric.MetricTypeHistogram:
		return m.Histogram().DataPoints().Len()
	case pmetric.MetricTypeExponentialHistogram:
		return m.ExponentialHistogram().DataPoints().Len()
	case pmetric.MetricTypeSummary:
		return m.Summary().DataPoints().Len()
	default:
		return 0
	}
}

// Print datapoints for the metric (limited to a reasonable number)

func printDataPoints(prefix string, met pmetric.Metric, t *testing.T, r int, s int, mi int, scopeInfo string) {
	maxPrint := 10

	switch met.Type() {
	case pmetric.MetricTypeGauge:
		dps := met.Gauge().DataPoints()
		for dpi := 0; dpi < dps.Len() && dpi < maxPrint; dpi++ {
			dp := dps.At(dpi)
			t.Logf("[DEBUG] %s resource[%d] scope[%d:%s] metric[%d] dp[%d]: int=%d double=%v attrs=%v start=%d time=%d",
				prefix, r, s, scopeInfo, mi, dpi, dp.IntValue(), dp.DoubleValue(), dp.Attributes().AsRaw(), dp.StartTimestamp(), dp.Timestamp())
		}
	case pmetric.MetricTypeSum:
		dps := met.Sum().DataPoints()
		for dpi := 0; dpi < dps.Len() && dpi < maxPrint; dpi++ {
			dp := dps.At(dpi)
			t.Logf("[DEBUG] %s resource[%d] scope[%d:%s] metric[%d] dp[%d]: int=%d double=%v attrs=%v start=%d time=%d",
				prefix, r, s, scopeInfo, mi, dpi, dp.IntValue(), dp.DoubleValue(), dp.Attributes().AsRaw(), dp.StartTimestamp(), dp.Timestamp())
		}
	case pmetric.MetricTypeHistogram:
		dps := met.Histogram().DataPoints()
		for dpi := 0; dpi < dps.Len() && dpi < maxPrint; dpi++ {
			dp := dps.At(dpi)
			t.Logf("[DEBUG] %s resource[%d] scope[%d:%s] metric[%d] hist dp[%d]: count=%d sum=%v bounds=%v attrs=%v start=%d time=%d",
				prefix, r, s, scopeInfo, mi, dpi, dp.Count(), dp.Sum(), dp.BucketCounts(), dp.Attributes().AsRaw(), dp.StartTimestamp(), dp.Timestamp())
		}
	case pmetric.MetricTypeExponentialHistogram:
		dps := met.ExponentialHistogram().DataPoints()
		for dpi := 0; dpi < dps.Len() && dpi < maxPrint; dpi++ {
			dp := dps.At(dpi)
			t.Logf("[DEBUG] %s resource[%d] scope[%d:%s] metric[%d] exp-hist dp[%d]: count=%d sum=%v attrs=%v start=%d time=%d",
				prefix, r, s, scopeInfo, mi, dpi, dp.Count(), dp.Sum(), dp.Attributes().AsRaw(), dp.StartTimestamp(), dp.Timestamp())
		}
	case pmetric.MetricTypeSummary:
		dps := met.Summary().DataPoints()
		for dpi := 0; dpi < dps.Len() && dpi < maxPrint; dpi++ {
			dp := dps.At(dpi)
			t.Logf("[DEBUG] %s resource[%d] scope[%d:%s] metric[%d] summary dp[%d]: count=%d sum=%v attrs=%v start=%d time=%d",
				prefix, r, s, scopeInfo, mi, dpi, dp.Count(), dp.Sum(), dp.Attributes().AsRaw(), dp.StartTimestamp(), dp.Timestamp())
		}
	default:
		t.Logf("[DEBUG] %s resource[%d] scope[%d:%s] metric[%d]: unknown metric type=%v",
			prefix, r, s, scopeInfo, mi, met.Type())
	}
}

func countDataPoints(m pmetric.Metric) int {
	switch m.Type() {
	case pmetric.MetricTypeGauge:
		return m.Gauge().DataPoints().Len()
	case pmetric.MetricTypeSum:
		return m.Sum().DataPoints().Len()
	case pmetric.MetricTypeHistogram:
		return m.Histogram().DataPoints().Len()
	case pmetric.MetricTypeSummary:
		return m.Summary().DataPoints().Len()
	case pmetric.MetricTypeExponentialHistogram:
		return m.ExponentialHistogram().DataPoints().Len()
	default:
		return 0
	}
}
