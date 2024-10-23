package integration

import (
	"testing"

	"github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func TestFilteringCreditCard(t *testing.T) {
	attributesNonMasked := pcommon.NewMap()
	attributesNonMasked.PutStr("card_master_spaces", "2367 8901 2345 6789")
	attributesNonMasked.PutStr("card_master_no_spaces", "5105105105105100")
	attributesNonMasked.PutStr("card_visa_spaces", "4539 1488 0343 6467")
	attributesNonMasked.PutStr("card_visa_no_spaces", "4111111111111111")
	attributesNonMasked.PutStr("card_amex_spaces", "3714 496353 98431")
	attributesNonMasked.PutStr("card_amex_no_spaces", "378282246310005")

	attributesMasked := pcommon.NewMap()
	attributesMasked.PutStr("card_master_spaces", "****")
	attributesMasked.PutStr("card_master_no_spaces", "****")
	attributesMasked.PutStr("card_visa_spaces", "****")
	attributesMasked.PutStr("card_visa_no_spaces", "****")
	attributesMasked.PutStr("card_amex_spaces", "****")
	attributesMasked.PutStr("card_amex_no_spaces", "****")
	attributesMasked.PutInt("redaction.masked.count", int64(6))

	creditCardRedactionProcessor := map[string]string{
		"redaction": `
  redaction:
    allow_all_keys: false
    allowed_keys:
      - card_master_spaces
      - card_master_no_spaces
      - card_visa_spaces
      - card_visa_no_spaces
      - card_amex_spaces
      - card_amex_no_spaces
    blocked_values:
      - "^4(\\s*[0-9]){12}(?:(\\s*[0-9]){3})?(?:(\\s*[0-9]){3})?$"
      - "^5[1-5](\\s*[0-9]){14}|^(222[1-9]|22[3-9]\\d|2[3-6]\\d{2}|27[0-1]\\d|2720)(\\s*[0-9]){12}$"
      - "^3\\s*[47](\\s*[0-9]){13}$"
    summary: info
`,
	}
	tests := []struct {
		name         string
		dataProvider testbed.DataProvider
		sender       testbed.DataSender
		receiver     testbed.DataReceiver
		validator    testbed.TestCaseValidator
		processors   map[string]string
	}{
		{
			name:         "traces",
			dataProvider: NewSampleConfigsTraceDataProvider(generateBasicTracesWithAttributes(attributesNonMasked)),
			sender:       testbed.NewOTLPTraceDataSender(testbed.DefaultHost, testutil.GetAvailablePort(t)),
			receiver:     testbed.NewOTLPDataReceiver(testutil.GetAvailablePort(t)),
			validator:    NewTraceValidator(t, []ptrace.Traces{generateBasicTracesWithAttributes(attributesMasked)}),
			processors:   creditCardRedactionProcessor,
		},
		{
			name:         "metrics",
			dataProvider: NewSampleConfigsMetricsDataProvider(generateBasicMetricWithAttributes(attributesNonMasked)),
			sender:       testbed.NewOTLPMetricDataSender(testbed.DefaultHost, testutil.GetAvailablePort(t)),
			receiver:     testbed.NewOTLPDataReceiver(testutil.GetAvailablePort(t)),
			validator:    NewMetricValidator(t, []pmetric.Metrics{generateBasicMetricWithAttributes(attributesMasked)}),
			processors:   creditCardRedactionProcessor,
		},
		{
			name:         "logs",
			dataProvider: NewSampleConfigsLogsDataProvider(generateBasicLogsWithAttributes(attributesNonMasked)),
			sender:       testbed.NewOTLPLogsDataSender(testbed.DefaultHost, testutil.GetAvailablePort(t)),
			receiver:     testbed.NewOTLPDataReceiver(testutil.GetAvailablePort(t)),
			validator:    NewLogsValidator(t, []plog.Logs{generateBasicLogsWithAttributes(attributesMasked)}),
			processors:   creditCardRedactionProcessor,
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

func TestFilteringDTAPITokenRedactionProcessor(t *testing.T) {
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

func TestFilteringDTAPITokenTransformProcessor(t *testing.T) {
	transformProcessor := `
  transform:
    trace_statements:
      - context: span
        statements:
          - replace_all_patterns(attributes, "value", "(dt0s0[1-9].[A-Za-z0-9]{24}.)([A-Za-z0-9]{64})", "$1****")
    metric_statements:
      - context: datapoint
        statements:
          - replace_all_patterns(attributes, "value", "(dt0s0[1-9].[A-Za-z0-9]{24}.)([A-Za-z0-9]{64})", "$1****")
      - context: resource
        statements:
          - replace_all_patterns(attributes, "value", "(dt0s0[1-9].[A-Za-z0-9]{24}.)([A-Za-z0-9]{64})", "$1****")
    log_statements:
      - context: log
        statements:
          - replace_all_patterns(attributes, "value", "(dt0s0[1-9].[A-Za-z0-9]{24}.)([A-Za-z0-9]{64})", "$1****")
`
	ingestedAttrs := map[string]string{
		// NOTE: the sample token below is NOT an actual token, but an example taken from the DT docs: https://docs.dynatrace.com/docs/dynatrace-api/basics/dynatrace-api-authentication
		"t": "dt0s01.ST2EY72KQINMH574WMNVI7YN.G3DFPBEJYMODIDAEX454M7YWBUVEFOWKPRVMWFASS64NFH52PX6BNDVFFM572RZM",
	}

	expectedAttrs := map[string]string{
		"t": "dt0s01.ST2EY72KQINMH574WMNVI7YN.****",
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
				"transform": transformProcessor,
			},
		},
		{
			name:         "basic metrics",
			dataProvider: NewSampleConfigsMetricsDataProvider(ingestedMetric),
			sender:       testbed.NewOTLPMetricDataSender(testbed.DefaultHost, testutil.GetAvailablePort(t)),
			receiver:     testbed.NewOTLPDataReceiver(testutil.GetAvailablePort(t)),
			validator:    NewMetricSampleConfigsValidator(t, expectedMetric),
			processors: map[string]string{
				"transform": transformProcessor,
			},
		},
		{
			name:         "basic logs",
			dataProvider: NewSampleConfigsLogsDataProvider(ingestedLog),
			sender:       testbed.NewOTLPLogsDataSender(testbed.DefaultHost, testutil.GetAvailablePort(t)),
			receiver:     testbed.NewOTLPDataReceiver(testutil.GetAvailablePort(t)),
			validator:    NewSyslogSampleConfigValidator(t, expectedLog),
			processors: map[string]string{
				"transform": transformProcessor,
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

	func generateBasicTracesWithAttributes(attributes pcommon.Map) ptrace.Traces {
		traceData := ptrace.NewTraces()
		spans := traceData.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans()
		spans.EnsureCapacity(1)

		span := spans.AppendEmpty()
		span.SetName("filtering-span")
		attrs := span.Attributes()
		for k, v := range attributes.AsRaw() {
		switch v.(type) {
	case int64:
		attrs.PutInt(k, v.(int64))
	case string:
		attrs.PutStr(k, v.(string))
	}

	}

		return traceData
	}

func generateBasicMetricWithAttributes(attributes pcommon.Map) pmetric.Metrics {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()

	metrics := rm.ScopeMetrics().AppendEmpty().Metrics()
	metric := metrics.AppendEmpty()
	metric.SetName("filtering_metric")
	dps := metric.SetEmptyGauge().DataPoints()

	dataPoint := dps.AppendEmpty()
	dataPoint.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	dataPoint.SetIntValue(int64(42))
	attrs := dataPoint.Attributes()
	for k, v := range attributes.AsRaw() {
		switch v.(type) {
		case int64:
			attrs.PutInt(k, v.(int64))
		case string:
			attrs.PutStr(k, v.(string))
		}
	}

	return md
}

func generateBasicLogsWithAttributes(attributes pcommon.Map) plog.Logs {
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()

	logRecords := rl.ScopeLogs().AppendEmpty().LogRecords()
	logRecords.EnsureCapacity(1)

	record := logRecords.AppendEmpty()
	record.SetSeverityNumber(plog.SeverityNumberInfo3)
	record.SetSeverityText("INFO")
	record.Body().SetStr("Info testing filtering")

	attrs := record.Attributes()
	for k, v := range attributes.AsRaw() {
		switch v.(type) {
		case int64:
			attrs.PutInt(k, v.(int64))
		case string:
			attrs.PutStr(k, v.(string))
		}
	}

	return logs
}
