package integration

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/datasenders"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func TestConfigTailSampling(t *testing.T) {
	// arrange
	col := testbed.NewChildProcessCollector(testbed.WithAgentExePath(CollectorTestsExecPath))
	cfg, err := os.ReadFile("../../config_examples/tail_sampling.yaml")
	require.NoError(t, err)

	receiverPort := testbed.GetAvailablePort(t)
	exporterPort := testbed.GetAvailablePort(t)

	parsedConfig := string(cfg)
	parsedConfig = replaceOtlpGrpcReceiverPort(parsedConfig, receiverPort)
	parsedConfig = replaceDynatraceExporterEndpoint(parsedConfig, exporterPort)

	// replaces the sampling decision wait so the test doesn't timeout
	parsedConfig = strings.Replace(parsedConfig, "decision_wait: 30s", "decision_wait: 10ms", 1)

	configCleanup, err := col.PrepareConfig(parsedConfig)
	require.NoError(t, err)
	t.Cleanup(configCleanup)

	actualSpansData := ptrace.NewTraces()
	actualSpans := actualSpansData.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans()

	expectedSpansData := ptrace.NewTraces()
	expectedSpans := expectedSpansData.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans()
	startTime := time.Now()

	// Error ers
	ers := actualSpans.AppendEmpty()
	ers.SetTraceID(uInt64ToTraceID(0, uint64(1)))
	ers.SetSpanID(uInt64ToSpanID(uint64(1)))
	ers.SetName("Error span")
	ers.SetKind(ptrace.SpanKindServer)
	ers.Status().SetCode(ptrace.StatusCodeError)
	ers.SetStartTimestamp(pcommon.NewTimestampFromTime(startTime))
	ers.SetEndTimestamp(pcommon.NewTimestampFromTime(startTime.Add(time.Millisecond * 501)))

	// Ok span
	oks := actualSpans.AppendEmpty()
	oks.SetTraceID(uInt64ToTraceID(0, uint64(2)))
	oks.SetSpanID(uInt64ToSpanID(uint64(2)))
	oks.SetName("OK span")
	oks.SetKind(ptrace.SpanKindServer)
	oks.Status().SetCode(ptrace.StatusCodeOk)
	oks.SetStartTimestamp(pcommon.NewTimestampFromTime(startTime.Add(time.Millisecond)))
	oks.SetEndTimestamp(pcommon.NewTimestampFromTime(startTime.Add(time.Millisecond * 3)))

	// Long-running span
	lrs := actualSpans.AppendEmpty()
	lrs.SetTraceID(uInt64ToTraceID(0, uint64(3)))
	lrs.SetSpanID(uInt64ToSpanID(uint64(3)))
	lrs.SetName("Long-running span")
	lrs.SetKind(ptrace.SpanKindServer)
	lrs.Status().SetCode(ptrace.StatusCodeOk)
	lrs.SetStartTimestamp(pcommon.NewTimestampFromTime(startTime.Add(time.Millisecond)))
	lrs.SetEndTimestamp(pcommon.NewTimestampFromTime(startTime.Add(time.Second * 1)))

	// Expected Spans should only have the Error and long-running spans
	actualSpans.CopyTo(expectedSpans)
	expectedSpans.RemoveIf(func(s ptrace.Span) bool { return s.Name() == "OK span" })

	dataProvider := NewSampleConfigsTraceDataProvider(actualSpansData)
	sender := testbed.NewOTLPTraceDataSender(testbed.DefaultHost, receiverPort)
	receiver := testbed.NewOTLPHTTPDataReceiver(exporterPort)
	validator := NewTraceSampleConfigsValidator(t, expectedSpansData)

	tc := testbed.NewTestCase(
		t,
		dataProvider,
		sender,
		receiver,
		col,
		validator,
		&testbed.CorrectnessResults{},
	)
	t.Cleanup(tc.Stop)

	tc.EnableRecording()
	tc.StartBackend()
	tc.StartAgent()

	// act
	tc.StartLoad(testbed.LoadOptions{
		DataItemsPerSecond: 3,
		ItemsPerBatch:      3,
	})
	tc.Sleep(2 * time.Second)
	tc.StopLoad()

	tc.WaitForN(func() bool {
		return tc.MockBackend.DataItemsReceived() == uint64(expectedSpansData.SpanCount())
	}, 5*time.Second, "all data items received")

	// assert
	tc.ValidateData()
}

func TestConfigJaegerGrpc(t *testing.T) {
	// arrange
	col := testbed.NewChildProcessCollector(testbed.WithAgentExePath(CollectorTestsExecPath))
	cfg, err := os.ReadFile("../../config_examples/jaeger.yaml")
	require.NoError(t, err)

	grpcReceiverPort := testbed.GetAvailablePort(t)
	exporterPort := testbed.GetAvailablePort(t)

	parsedConfig := string(cfg)
	parsedConfig = replaceJaegerGrpcReceiverPort(parsedConfig, grpcReceiverPort)
	parsedConfig = replaceDynatraceExporterEndpoint(parsedConfig, exporterPort)

	configCleanup, err := col.PrepareConfig(parsedConfig)
	require.NoError(t, err)
	t.Cleanup(configCleanup)

	actualSpansData := ptrace.NewTraces()
	actualSpans := actualSpansData.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans()

	expectedSpansData := ptrace.NewTraces()
	expectedSpans := expectedSpansData.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans()
	startTime := time.Now()

	// Error ers
	ers := actualSpans.AppendEmpty()
	ers.SetTraceID(uInt64ToTraceID(0, uint64(1)))
	ers.SetSpanID(uInt64ToSpanID(uint64(1)))
	ers.SetName("Error span")
	ers.SetKind(ptrace.SpanKindServer)
	ers.Status().SetCode(ptrace.StatusCodeError)
	ers.SetStartTimestamp(pcommon.NewTimestampFromTime(startTime))
	ers.SetEndTimestamp(pcommon.NewTimestampFromTime(startTime.Add(time.Millisecond * 501)))

	// Ok span
	oks := actualSpans.AppendEmpty()
	oks.SetTraceID(uInt64ToTraceID(0, uint64(2)))
	oks.SetSpanID(uInt64ToSpanID(uint64(2)))
	oks.SetName("OK span")
	oks.SetKind(ptrace.SpanKindServer)
	oks.Status().SetCode(ptrace.StatusCodeOk)
	oks.SetStartTimestamp(pcommon.NewTimestampFromTime(startTime.Add(time.Millisecond)))
	oks.SetEndTimestamp(pcommon.NewTimestampFromTime(startTime.Add(time.Millisecond * 3)))

	// Long-running span
	lrs := actualSpans.AppendEmpty()
	lrs.SetTraceID(uInt64ToTraceID(0, uint64(3)))
	lrs.SetSpanID(uInt64ToSpanID(uint64(3)))
	lrs.SetName("Long-running span")
	lrs.SetKind(ptrace.SpanKindServer)
	lrs.Status().SetCode(ptrace.StatusCodeOk)
	lrs.SetStartTimestamp(pcommon.NewTimestampFromTime(startTime.Add(time.Millisecond)))
	lrs.SetEndTimestamp(pcommon.NewTimestampFromTime(startTime.Add(time.Second * 1)))

	// Expected Spans should have all spans
	actualSpans.CopyTo(expectedSpans)

	dataProvider := NewSampleConfigsTraceDataProvider(actualSpansData)
	sender := datasenders.NewJaegerGRPCDataSender(testbed.DefaultHost, grpcReceiverPort)
	receiver := testbed.NewOTLPHTTPDataReceiver(exporterPort)
	validator := NewTraceSampleConfigsValidator(t, expectedSpansData)

	tc := testbed.NewTestCase(
		t,
		dataProvider,
		sender,
		receiver,
		col,
		validator,
		&testbed.CorrectnessResults{},
	)
	t.Cleanup(tc.Stop)

	tc.EnableRecording()
	tc.StartBackend()
	tc.StartAgent()

	// act
	tc.StartLoad(testbed.LoadOptions{
		DataItemsPerSecond: 3,
		ItemsPerBatch:      3,
	})
	tc.Sleep(2 * time.Second)
	tc.StopLoad()

	tc.WaitForN(func() bool {
		return tc.MockBackend.DataItemsReceived() == uint64(expectedSpansData.SpanCount())
	}, 5*time.Second, "all data items received")

	// assert
	tc.ValidateData()
}

func TestConfigHistogramTransform(t *testing.T) {
	// arrange
	col := testbed.NewChildProcessCollector(testbed.WithAgentExePath(CollectorTestsExecPath))
	cfg, err := os.ReadFile("../../config_examples/split_histogram.yaml")
	require.NoError(t, err)

	receiverPort := testbed.GetAvailablePort(t)
	exporterPort := testbed.GetAvailablePort(t)

	parsedConfig := string(cfg)
	parsedConfig = replaceOtlpGrpcReceiverPort(parsedConfig, receiverPort)
	parsedConfig = replaceDynatraceExporterEndpoint(parsedConfig, exporterPort)

	configCleanup, err := col.PrepareConfig(parsedConfig)
	require.NoError(t, err)
	t.Cleanup(configCleanup)

	actualMetricsData := pmetric.NewMetrics()
	actualMetrics := actualMetricsData.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()

	expectedMetricData := pmetric.NewMetrics()
	expectedMetrics := expectedMetricData.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()

	startTime := time.Now()
	// Metric start time will be set to the end of the previous collection cycle for DELTA metrics
	startTimeStamp := pcommon.NewTimestampFromTime(startTime)
	// Add a default PeriodicExportingMetricReader interval (30s)
	timeStamp := pcommon.NewTimestampFromTime(startTime.Add(time.Second * 30))

	// Histogram
	histogram := actualMetrics.AppendEmpty()
	histogram.SetName("my.histogram")
	histogram.SetUnit("custom_unit")
	histogram.SetDescription("My custom histogram")
	histogram.SetEmptyHistogram()
	histogram.Histogram().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
	histogramDataPoint := histogram.Histogram().DataPoints().AppendEmpty()
	histogramDataPoint.Attributes().PutStr("key", "value")
	histogramDataPoint.SetStartTimestamp(startTimeStamp)
	histogramDataPoint.SetTimestamp(timeStamp)

	// recorded 0.5, 3.5, 3.5
	histogramDataPoint.ExplicitBounds().Append(1, 2, 3)
	histogramDataPoint.SetCount(3)
	histogramDataPoint.SetSum(7.5)
	histogramDataPoint.SetMin(0.5)
	histogramDataPoint.SetMax(3.5)
	histogramDataPoint.BucketCounts().Append(1, 0, 0, 0, 2)

	// Split Histogram
	splitCountMetric := expectedMetrics.AppendEmpty()
	splitCountMetric.SetName("my.histogram_count")
	splitCountMetric.SetUnit("custom_unit")
	splitCountMetric.SetDescription("My custom histogram")
	splitCountMetric.SetEmptySum()
	splitCountMetric.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
	splitCountMetric.Sum().SetIsMonotonic(true)
	splitCountMetricDataPoint := splitCountMetric.Sum().DataPoints().AppendEmpty()
	splitCountMetricDataPoint.Attributes().PutStr("key", "value")
	splitCountMetricDataPoint.SetStartTimestamp(startTimeStamp)
	splitCountMetricDataPoint.SetTimestamp(timeStamp)
	splitCountMetricDataPoint.SetIntValue(3)

	splitSumMetric := expectedMetrics.AppendEmpty()
	splitSumMetric.SetName("my.histogram_sum")
	splitSumMetric.SetUnit("custom_unit")
	splitSumMetric.SetDescription("My custom histogram")
	splitSumMetric.SetEmptyGauge()
	splitMaxMetricDataPoint := splitSumMetric.Gauge().DataPoints().AppendEmpty()
	splitMaxMetricDataPoint.Attributes().PutStr("key", "value")
	splitMaxMetricDataPoint.SetStartTimestamp(startTimeStamp)
	splitMaxMetricDataPoint.SetTimestamp(timeStamp)
	splitMaxMetricDataPoint.SetDoubleValue(7.5)

	dataProvider := NewSampleConfigsMetricsDataProvider(actualMetricsData)
	sender := testbed.NewOTLPMetricDataSender(testbed.DefaultHost, receiverPort)
	receiver := testbed.NewOTLPHTTPDataReceiver(exporterPort)
	validator := NewMetricSampleConfigsValidator(t, expectedMetricData)

	tc := testbed.NewTestCase(
		t,
		dataProvider,
		sender,
		receiver,
		col,
		validator,
		&testbed.CorrectnessResults{},
	)
	t.Cleanup(tc.Stop)

	tc.EnableRecording()
	tc.StartBackend()
	tc.StartAgent()

	// act
	tc.StartLoad(testbed.LoadOptions{
		DataItemsPerSecond: 3,
		ItemsPerBatch:      3,
	})
	tc.Sleep(2 * time.Second)
	tc.StopLoad()

	tc.WaitForN(func() bool {
		return tc.MockBackend.DataItemsReceived() == uint64(expectedMetricData.MetricCount())
	}, 5*time.Second, "all data items received")

	// assert
	tc.ValidateData()
}
