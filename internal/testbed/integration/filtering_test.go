package integration

import (
	"testing"
	"time"

	"github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func TestFiltering(t *testing.T) {
	trace := generateBasicTrace(nil)
	metric := generateBasicMetric(nil)
	logs := generateBasicLogs(pcommon.NewTimestampFromTime(time.Now()), nil)
	tests := []struct {
		name         string
		dataProvider testbed.DataProvider
		sender       testbed.DataSender
		receiver     testbed.DataReceiver
		validator    testbed.TestCaseValidator
		processors   map[string]string
	}{
		{
			name:         "basic traces",
			dataProvider: NewSampleConfigsTraceDataProvider(trace),
			sender:       testbed.NewOTLPTraceDataSender(testbed.DefaultHost, testutil.GetAvailablePort(t)),
			receiver:     testbed.NewOTLPDataReceiver(testutil.GetAvailablePort(t)),
			validator:    NewTraceSampleConfigsValidator(t, trace),
			processors:   map[string]string{},
		},
		{
			name:         "basic metrics",
			dataProvider: NewSampleConfigsMetricsDataProvider(metric),
			sender:       testbed.NewOTLPMetricDataSender(testbed.DefaultHost, testutil.GetAvailablePort(t)),
			receiver:     testbed.NewOTLPDataReceiver(testutil.GetAvailablePort(t)),
			validator:    NewMetricSampleConfigsValidator(t, metric),
			processors:   map[string]string{},
		},
		{
			name:         "basic logs",
			dataProvider: NewSampleConfigsLogsDataProvider(logs),
			sender:       testbed.NewOTLPLogsDataSender(testbed.DefaultHost, testutil.GetAvailablePort(t)),
			receiver:     testbed.NewOTLPDataReceiver(testutil.GetAvailablePort(t)),
			validator:    NewSyslogSampleConfigValidator(t, logs),
			processors:   map[string]string{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			FilteringScenario(
				t,
				test.dataProvider,
				test.sender,
				test.receiver,
				test.validator,
				test.processors,
				nil,
			)
		})
	}
}

func TestFilteringDTAPIToken(t *testing.T) {
	redactionProcessor := `
  redaction:
    allow_all_keys: true
    blocked_values:
      - dt0s0[1-9]\.[A-Za-z0-9]{24}\.([A-Za-z0-9]{64})
`
	ingestedAttrs := map[string]string{
		// NOTE: the sample token below is NOT an actual token, but an example taken from the DT docs: https://docs.dynatrace.com/docs/dynatrace-api/basics/dynatrace-api-authentication
		"t": "dt0s01.ST2EY72KQINMH574WMNVI7YN.G3DFPBEJYMODIDAEX454M7YWBUVEFOWKPRVMWFASS64NFH52PX6BNDVFFM572RZM",
	}

	expectedAttrs := map[string]string{
		"t": "****",
	}

	ingestedTrace := generateBasicTrace(ingestedAttrs)
	expectedTrace := generateBasicTrace(expectedAttrs)

	ingestedMetric := generateBasicMetric(ingestedAttrs)
	expectedMetric := generateBasicMetric(expectedAttrs)

	now := pcommon.NewTimestampFromTime(time.Now())
	ingestedLog := generateBasicLogs(now, ingestedAttrs)
	expectedLog := generateBasicLogs(now, expectedAttrs)

	tests := []struct {
		name         string
		dataProvider testbed.DataProvider
		sender       testbed.DataSender
		receiver     testbed.DataReceiver
		validator    testbed.TestCaseValidator
		processors   map[string]string
	}{
		{
			name:         "basic traces",
			dataProvider: NewSampleConfigsTraceDataProvider(ingestedTrace),
			sender:       testbed.NewOTLPTraceDataSender(testbed.DefaultHost, testutil.GetAvailablePort(t)),
			receiver:     testbed.NewOTLPDataReceiver(testutil.GetAvailablePort(t)),
			validator:    NewTraceSampleConfigsValidator(t, expectedTrace, WithTraceAttributeCheck()),
			processors: map[string]string{
				"redaction": redactionProcessor,
			},
		},
		{
			name:         "basic metrics",
			dataProvider: NewSampleConfigsMetricsDataProvider(ingestedMetric),
			sender:       testbed.NewOTLPMetricDataSender(testbed.DefaultHost, testutil.GetAvailablePort(t)),
			receiver:     testbed.NewOTLPDataReceiver(testutil.GetAvailablePort(t)),
			validator:    NewMetricSampleConfigsValidator(t, expectedMetric),
			processors: map[string]string{
				"redaction": redactionProcessor,
			},
		},
		{
			name:         "basic logs",
			dataProvider: NewSampleConfigsLogsDataProvider(ingestedLog),
			sender:       testbed.NewOTLPLogsDataSender(testbed.DefaultHost, testutil.GetAvailablePort(t)),
			receiver:     testbed.NewOTLPDataReceiver(testutil.GetAvailablePort(t)),
			validator:    NewSyslogSampleConfigValidator(t, expectedLog),
			processors: map[string]string{
				"redaction": redactionProcessor,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			FilteringScenario(
				t,
				test.dataProvider,
				test.sender,
				test.receiver,
				test.validator,
				test.processors,
				nil,
			)
		})
	}
}

func generateBasicTrace(attributes map[string]string) ptrace.Traces {
	traceData := ptrace.NewTraces()
	spans := traceData.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans()
	spans.EnsureCapacity(1)

	//startTime := time.Now()
	//endTime := startTime.Add(time.Millisecond)

	span := spans.AppendEmpty()
	span.SetName("filtering-span")
	span.SetKind(ptrace.SpanKindClient)
	attrs := span.Attributes()

	for k, v := range attributes {
		attrs.PutStr(k, v)
	}

	//span.SetStartTimestamp(pcommon.NewTimestampFromTime(startTime))
	//span.SetEndTimestamp(pcommon.NewTimestampFromTime(endTime))

	return traceData
}

func generateBasicMetric(attributes map[string]string) pmetric.Metrics {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()

	for k, v := range attributes {
		rm.Resource().Attributes().PutStr(k, v)
	}

	metrics := rm.ScopeMetrics().AppendEmpty().Metrics()
	metrics.EnsureCapacity(1)

	metric := metrics.AppendEmpty()
	metric.SetName("filtering_metric")
	dps := metric.SetEmptyGauge().DataPoints()

	dataPoint := dps.AppendEmpty()
	dataPoint.SetIntValue(int64(42))

	for k, v := range attributes {
		dataPoint.Attributes().PutStr(k, v)
	}

	return md
}

func generateBasicLogs(timestamp pcommon.Timestamp, attributes map[string]string) plog.Logs {
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()

	logRecords := rl.ScopeLogs().AppendEmpty().LogRecords()
	logRecords.EnsureCapacity(1)

	record := logRecords.AppendEmpty()
	record.SetSeverityNumber(plog.SeverityNumberInfo3)
	record.SetSeverityText("INFO")
	record.Body().SetStr("Info testing filtering")
	record.SetFlags(plog.DefaultLogRecordFlags.WithIsSampled(true))
	record.SetTimestamp(timestamp)

	attrs := record.Attributes()
	for k, v := range attributes {
		attrs.PutStr(k, v)
	}

	return logs
}
