package integration

import (
	"os"
	"path"
	"testing"

	"github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func TestFilteringCreditCard(t *testing.T) {
	attributesNonMasked := pcommon.NewMap()
	attributesNonMasked.PutStr("card_master_spaces1", "2367 8901 2345 6789")
	attributesNonMasked.PutStr("card_master_spaces2", "5105 1051 0510 5100")
	attributesNonMasked.PutStr("card_master_spaces3", "2720 1051 0510 5100")
	attributesNonMasked.PutStr("card_master_no_spaces1", "2367890123456789")
	attributesNonMasked.PutStr("card_master_no_spaces2", "5105105105105100")
	attributesNonMasked.PutStr("card_master_no_spaces3", "2720105105105100")
	attributesNonMasked.PutStr("card_visa_spaces1", "4539 1488 0343 6467")
	attributesNonMasked.PutStr("card_visa_spaces2", "4539 1488 0343 6")
	attributesNonMasked.PutStr("card_visa_spaces3", "4539 1488 0343 6467 234")
	attributesNonMasked.PutStr("card_visa_no_spaces1", "4539148803436467")
	attributesNonMasked.PutStr("card_visa_no_spaces2", "4539148803436")
	attributesNonMasked.PutStr("card_visa_no_spaces3", "4539148803436467234")
	attributesNonMasked.PutStr("card_amex_spaces1", "3714 496353 98431")
	attributesNonMasked.PutStr("card_amex_spaces2", "3487 344936 71000")
	attributesNonMasked.PutStr("card_amex_spaces3", "3782 822463 10005")
	attributesNonMasked.PutStr("card_amex_no_spaces1", "371449635398431")
	attributesNonMasked.PutStr("card_amex_no_spaces2", "348734493671000")
	attributesNonMasked.PutStr("card_amex_no_spaces3", "378282246310005")
	attributesNonMasked.PutStr("safe_attribute1", "371")
	attributesNonMasked.PutStr("safe_attribute2", "37810005")

	attributesMasked := pcommon.NewMap()
	attributesMasked.PutStr("card_master_spaces1", "****")
	attributesMasked.PutStr("card_master_spaces2", "****")
	attributesMasked.PutStr("card_master_spaces3", "****")
	attributesMasked.PutStr("card_master_no_spaces1", "****")
	attributesMasked.PutStr("card_master_no_spaces2", "****")
	attributesMasked.PutStr("card_master_no_spaces3", "****")
	attributesMasked.PutStr("card_visa_spaces1", "****")
	attributesMasked.PutStr("card_visa_spaces2", "****")
	attributesMasked.PutStr("card_visa_spaces3", "****")
	attributesMasked.PutStr("card_visa_no_spaces1", "****")
	attributesMasked.PutStr("card_visa_no_spaces2", "****")
	attributesMasked.PutStr("card_visa_no_spaces3", "****")
	attributesMasked.PutStr("card_amex_spaces1", "****")
	attributesMasked.PutStr("card_amex_spaces2", "****")
	attributesMasked.PutStr("card_amex_spaces3", "****")
	attributesMasked.PutStr("card_amex_no_spaces1", "****")
	attributesMasked.PutStr("card_amex_no_spaces2", "****")
	attributesMasked.PutStr("card_amex_no_spaces3", "****")
	attributesMasked.PutInt("redaction.masked.count", int64(18))
	attributesMasked.PutStr("safe_attribute1", "371")
	attributesMasked.PutStr("safe_attribute2", "37810005")

	attributesFiltered := pcommon.NewMap()
	attributesFiltered.PutStr("card_master_spaces1", "**** 6789")
	attributesFiltered.PutStr("card_master_spaces2", "**** 5100")
	attributesFiltered.PutStr("card_master_spaces3", "**** 5100")
	attributesFiltered.PutStr("card_master_no_spaces1", "**** 6789")
	attributesFiltered.PutStr("card_master_no_spaces2", "**** 5100")
	attributesFiltered.PutStr("card_master_no_spaces3", "**** 5100")
	attributesFiltered.PutStr("card_visa_spaces1", "**** 6467")
	attributesFiltered.PutStr("card_visa_spaces2", "**** 343 6")
	attributesFiltered.PutStr("card_visa_spaces3", "**** 7 234")
	attributesFiltered.PutStr("card_visa_no_spaces1", "**** 6467")
	attributesFiltered.PutStr("card_visa_no_spaces2", "**** 3436")
	attributesFiltered.PutStr("card_visa_no_spaces3", "**** 7234")
	attributesFiltered.PutStr("card_amex_spaces1", "**** 8431")
	attributesFiltered.PutStr("card_amex_spaces2", "**** 1000")
	attributesFiltered.PutStr("card_amex_spaces3", "**** 0005")
	attributesFiltered.PutStr("card_amex_no_spaces1", "**** 8431")
	attributesFiltered.PutStr("card_amex_no_spaces2", "**** 1000")
	attributesFiltered.PutStr("card_amex_no_spaces3", "**** 0005")
	attributesFiltered.PutStr("safe_attribute1", "371")
	attributesFiltered.PutStr("safe_attribute2", "37810005")

	content, err := os.ReadFile(path.Join(ConfigExamplesDir, "masking_creditcards.yaml"))
	require.Nil(t, err)

	creditCardTransformProcessor, err := extractProcessorsFromYAML(content)
	require.Nil(t, err)

	content, err = os.ReadFile(path.Join(ConfigExamplesDir, "redaction_creditcards.yaml"))
	require.Nil(t, err)

	creditCardRedactionProcessor, err := extractProcessorsFromYAML(content)
	require.Nil(t, err)

	tests := []struct {
		name         string
		dataProvider testbed.DataProvider
		sender       testbed.DataSender
		receiver     testbed.DataReceiver
		validator    testbed.TestCaseValidator
		processors   map[string]string
	}{
		{
			name:         "traces redaction",
			dataProvider: NewSampleConfigsTraceDataProvider(generateBasicTracesWithAttributes(attributesNonMasked)),
			sender:       testbed.NewOTLPTraceDataSender(testbed.DefaultHost, testutil.GetAvailablePort(t)),
			receiver:     testbed.NewOTLPDataReceiver(testutil.GetAvailablePort(t)),
			validator:    NewTraceValidator(t, []ptrace.Traces{generateBasicTracesWithAttributes(attributesMasked)}),
			processors:   creditCardRedactionProcessor,
		},
		{
			name:         "metrics redaction",
			dataProvider: NewSampleConfigsMetricsDataProvider(generateBasicMetricWithAttributes(attributesNonMasked)),
			sender:       testbed.NewOTLPMetricDataSender(testbed.DefaultHost, testutil.GetAvailablePort(t)),
			receiver:     testbed.NewOTLPDataReceiver(testutil.GetAvailablePort(t)),
			validator:    NewMetricValidator(t, []pmetric.Metrics{generateBasicMetricWithAttributes(attributesMasked)}),
			processors:   creditCardRedactionProcessor,
		},
		{
			name:         "logs redaction",
			dataProvider: NewSampleConfigsLogsDataProvider(generateBasicLogsWithAttributes(attributesNonMasked)),
			sender:       testbed.NewOTLPLogsDataSender(testbed.DefaultHost, testutil.GetAvailablePort(t)),
			receiver:     testbed.NewOTLPDataReceiver(testutil.GetAvailablePort(t)),
			validator:    NewLogsValidator(t, []plog.Logs{generateBasicLogsWithAttributes(attributesMasked)}),
			processors:   creditCardRedactionProcessor,
		},
		{
			name:         "traces transform",
			dataProvider: NewSampleConfigsTraceDataProvider(generateBasicTracesWithAttributes(attributesNonMasked)),
			sender:       testbed.NewOTLPTraceDataSender(testbed.DefaultHost, testutil.GetAvailablePort(t)),
			receiver:     testbed.NewOTLPDataReceiver(testutil.GetAvailablePort(t)),
			validator:    NewTraceValidator(t, []ptrace.Traces{generateBasicTracesWithAttributes(attributesFiltered)}),
			processors:   creditCardTransformProcessor,
		},
		{
			name:         "metrics transform",
			dataProvider: NewSampleConfigsMetricsDataProvider(generateBasicMetricWithAttributes(attributesNonMasked)),
			sender:       testbed.NewOTLPMetricDataSender(testbed.DefaultHost, testutil.GetAvailablePort(t)),
			receiver:     testbed.NewOTLPDataReceiver(testutil.GetAvailablePort(t)),
			validator:    NewMetricValidator(t, []pmetric.Metrics{generateBasicMetricWithAttributes(attributesFiltered)}),
			processors:   creditCardTransformProcessor,
		},
		{
			name:         "logs transform",
			dataProvider: NewSampleConfigsLogsDataProvider(generateBasicLogsWithAttributes(attributesNonMasked)),
			sender:       testbed.NewOTLPLogsDataSender(testbed.DefaultHost, testutil.GetAvailablePort(t)),
			receiver:     testbed.NewOTLPDataReceiver(testutil.GetAvailablePort(t)),
			validator:    NewLogsValidator(t, []plog.Logs{generateBasicLogsWithAttributes(attributesFiltered)}),
			processors:   creditCardTransformProcessor,
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

func TestFilteringIBAN(t *testing.T) {
	attributesNonMasked := pcommon.NewMap()
	attributesNonMasked.PutStr("iban1", "DE89 3704 0044 0532 0130 00")
	attributesNonMasked.PutStr("iban2", "FR14 2004 1010 0505 0001 3M02 606")
	attributesNonMasked.PutStr("iban3", "ES91 2100 0418 4502 0005 1332")
	attributesNonMasked.PutStr("iban4", "IT60 X054 2811 1010 0000 0123 456")
	attributesNonMasked.PutStr("iban5", "NL91 ABNA 0417 1643 00")
	attributesNonMasked.PutStr("iban6", "BE71 0961 2345 6769")
	attributesNonMasked.PutStr("iban7", "AT48 3200 0000 0123 4568")
	attributesNonMasked.PutStr("iban8", "SE72 8000 0810 0340 0978 3242")
	attributesNonMasked.PutStr("iban9", "PL61 1090 1014 0000 0712 1981 2874")
	attributesNonMasked.PutStr("iban10", "GB29 NWBK 6016 1331 9268 19")
	attributesNonMasked.PutStr("iban11", "AL47 2121 1009 0000 0002 3569 8741")
	attributesNonMasked.PutStr("iban12", "CY17 0020 0128 0000 0012 0052 7600")
	attributesNonMasked.PutStr("iban13", "KW81 CBKU 0000 0000 0000 1234 5601 01")
	attributesNonMasked.PutStr("iban14", "LU28 0019 4006 4475 0000")
	attributesNonMasked.PutStr("iban15", "NO93 8601 1117 947")
	attributesNonMasked.PutStr("iban16", "DE89370400440532013000")
	attributesNonMasked.PutStr("iban17", "FR1420041010050500013M02606")
	attributesNonMasked.PutStr("iban18", "ES9121000418450200051332")
	attributesNonMasked.PutStr("iban19", "IT60X0542811101000000123456")
	attributesNonMasked.PutStr("iban20", "NL91ABNA0417164300")
	attributesNonMasked.PutStr("iban21", "BE71096123456769")
	attributesNonMasked.PutStr("iban22", "AT483200000001234568")
	attributesNonMasked.PutStr("iban23", "SE7280000810034009783242")
	attributesNonMasked.PutStr("iban24", "PL61109010140000071219812874")
	attributesNonMasked.PutStr("iban25", "GB29NWBK60161331926819")
	attributesNonMasked.PutStr("iban26", "AL47212110090000000235698741")
	attributesNonMasked.PutStr("iban27", "CY17002001280000001200527600")
	attributesNonMasked.PutStr("iban28", "KW81CBKU0000000000001234560101")
	attributesNonMasked.PutStr("iban29", "LU280019400644750000")
	attributesNonMasked.PutStr("iban30", "NO9386011117947")
	attributesNonMasked.PutStr("non-iban", "no4444 ds")

	attributesMasked := pcommon.NewMap()
	attributesMasked.PutStr("iban1", "****")
	attributesMasked.PutStr("iban2", "****")
	attributesMasked.PutStr("iban3", "****")
	attributesMasked.PutStr("iban4", "****")
	attributesMasked.PutStr("iban5", "****")
	attributesMasked.PutStr("iban6", "****")
	attributesMasked.PutStr("iban7", "****")
	attributesMasked.PutStr("iban8", "****")
	attributesMasked.PutStr("iban9", "****")
	attributesMasked.PutStr("iban10", "****")
	attributesMasked.PutStr("iban11", "****")
	attributesMasked.PutStr("iban12", "****")
	attributesMasked.PutStr("iban13", "****")
	attributesMasked.PutStr("iban14", "****")
	attributesMasked.PutStr("iban15", "****")
	attributesMasked.PutStr("iban16", "****")
	attributesMasked.PutStr("iban17", "****")
	attributesMasked.PutStr("iban18", "****")
	attributesMasked.PutStr("iban19", "****")
	attributesMasked.PutStr("iban20", "****")
	attributesMasked.PutStr("iban21", "****")
	attributesMasked.PutStr("iban22", "****")
	attributesMasked.PutStr("iban23", "****")
	attributesMasked.PutStr("iban24", "****")
	attributesMasked.PutStr("iban25", "****")
	attributesMasked.PutStr("iban26", "****")
	attributesMasked.PutStr("iban27", "****")
	attributesMasked.PutStr("iban28", "****")
	attributesMasked.PutStr("iban29", "****")
	attributesMasked.PutStr("iban30", "****")
	attributesMasked.PutStr("non-iban", "no4444 ds")
	attributesMasked.PutInt("redaction.masked.count", int64(30))

	attributesFiltered := pcommon.NewMap()
	attributesFiltered.PutStr("iban1", "DE **** 30 00")
	attributesFiltered.PutStr("iban2", "FR **** 2 606")
	attributesFiltered.PutStr("iban3", "ES **** 1332")
	attributesFiltered.PutStr("iban4", "IT **** 3 456")
	attributesFiltered.PutStr("iban5", "NL **** 43 00")
	attributesFiltered.PutStr("iban6", "BE **** 6769")
	attributesFiltered.PutStr("iban7", "AT **** 4568")
	attributesFiltered.PutStr("iban8", "SE **** 3242")
	attributesFiltered.PutStr("iban9", "PL **** 2874")
	attributesFiltered.PutStr("iban10", "GB **** 68 19")
	attributesFiltered.PutStr("iban11", "AL **** 8741")
	attributesFiltered.PutStr("iban12", "CY **** 7600")
	attributesFiltered.PutStr("iban13", "KW **** 01 01")
	attributesFiltered.PutStr("iban14", "LU **** 0000")
	attributesFiltered.PutStr("iban15", "NO **** 7 947")
	attributesFiltered.PutStr("iban16", "DE **** 3000")
	attributesFiltered.PutStr("iban17", "FR **** 2606")
	attributesFiltered.PutStr("iban18", "ES **** 1332")
	attributesFiltered.PutStr("iban19", "IT **** 3456")
	attributesFiltered.PutStr("iban20", "NL **** 4300")
	attributesFiltered.PutStr("iban21", "BE **** 6769")
	attributesFiltered.PutStr("iban22", "AT **** 4568")
	attributesFiltered.PutStr("iban23", "SE **** 3242")
	attributesFiltered.PutStr("iban24", "PL **** 2874")
	attributesFiltered.PutStr("iban25", "GB **** 6819")
	attributesFiltered.PutStr("iban26", "AL **** 8741")
	attributesFiltered.PutStr("iban27", "CY **** 7600")
	attributesFiltered.PutStr("iban28", "KW **** 0101")
	attributesFiltered.PutStr("iban29", "LU **** 0000")
	attributesFiltered.PutStr("iban30", "NO **** 7947")
	attributesFiltered.PutStr("non-iban", "no4444 ds")

	ibanRedactionProcessor := map[string]string{
		"redaction": `
  redaction:
    allow_all_keys: true
    blocked_values:
      - "^[A-Z]{2}[0-9]{2}(\\s*[A-Z0-9]){8,30}$"
    summary: info
`,
	}

	ibanTransformProcessor := map[string]string{
		"transform": `
  transform:
    error_mode: ignore
    trace_statements:
      - context: span
        statements:
          - replace_all_patterns(attributes, "value", "^([A-Z]{2})[0-9]{2}((\\s*[A-Z0-9]){4,26})\\s*((\\s*[A-Z0-9]){4})$", "$$1 **** $$4")
    metric_statements:
      - context: datapoint
        statements:
          - replace_all_patterns(attributes, "value", "^([A-Z]{2})[0-9]{2}((\\s*[A-Z0-9]){4,26})\\s*((\\s*[A-Z0-9]){4})$", "$$1 **** $$4")
    log_statements:
      - context: log
        statements:
          - replace_all_patterns(attributes, "value", "^([A-Z]{2})[0-9]{2}((\\s*[A-Z0-9]){4,26})\\s*((\\s*[A-Z0-9]){4})$", "$$1 **** $$4")
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
			name:         "traces redaction",
			dataProvider: NewSampleConfigsTraceDataProvider(generateBasicTracesWithAttributes(attributesNonMasked)),
			sender:       testbed.NewOTLPTraceDataSender(testbed.DefaultHost, testutil.GetAvailablePort(t)),
			receiver:     testbed.NewOTLPDataReceiver(testutil.GetAvailablePort(t)),
			validator:    NewTraceValidator(t, []ptrace.Traces{generateBasicTracesWithAttributes(attributesMasked)}),
			processors:   ibanRedactionProcessor,
		},
		{
			name:         "metrics redaction",
			dataProvider: NewSampleConfigsMetricsDataProvider(generateBasicMetricWithAttributes(attributesNonMasked)),
			sender:       testbed.NewOTLPMetricDataSender(testbed.DefaultHost, testutil.GetAvailablePort(t)),
			receiver:     testbed.NewOTLPDataReceiver(testutil.GetAvailablePort(t)),
			validator:    NewMetricValidator(t, []pmetric.Metrics{generateBasicMetricWithAttributes(attributesMasked)}),
			processors:   ibanRedactionProcessor,
		},
		{
			name:         "logs redaction",
			dataProvider: NewSampleConfigsLogsDataProvider(generateBasicLogsWithAttributes(attributesNonMasked)),
			sender:       testbed.NewOTLPLogsDataSender(testbed.DefaultHost, testutil.GetAvailablePort(t)),
			receiver:     testbed.NewOTLPDataReceiver(testutil.GetAvailablePort(t)),
			validator:    NewLogsValidator(t, []plog.Logs{generateBasicLogsWithAttributes(attributesMasked)}),
			processors:   ibanRedactionProcessor,
		},
		{
			name:         "traces transform",
			dataProvider: NewSampleConfigsTraceDataProvider(generateBasicTracesWithAttributes(attributesNonMasked)),
			sender:       testbed.NewOTLPTraceDataSender(testbed.DefaultHost, testutil.GetAvailablePort(t)),
			receiver:     testbed.NewOTLPDataReceiver(testutil.GetAvailablePort(t)),
			validator:    NewTraceValidator(t, []ptrace.Traces{generateBasicTracesWithAttributes(attributesFiltered)}),
			processors:   ibanTransformProcessor,
		},
		{
			name:         "metrics transform",
			dataProvider: NewSampleConfigsMetricsDataProvider(generateBasicMetricWithAttributes(attributesNonMasked)),
			sender:       testbed.NewOTLPMetricDataSender(testbed.DefaultHost, testutil.GetAvailablePort(t)),
			receiver:     testbed.NewOTLPDataReceiver(testutil.GetAvailablePort(t)),
			validator:    NewMetricValidator(t, []pmetric.Metrics{generateBasicMetricWithAttributes(attributesFiltered)}),
			processors:   ibanTransformProcessor,
		},
		{
			name:         "logs transform",
			dataProvider: NewSampleConfigsLogsDataProvider(generateBasicLogsWithAttributes(attributesNonMasked)),
			sender:       testbed.NewOTLPLogsDataSender(testbed.DefaultHost, testutil.GetAvailablePort(t)),
			receiver:     testbed.NewOTLPDataReceiver(testutil.GetAvailablePort(t)),
			validator:    NewLogsValidator(t, []plog.Logs{generateBasicLogsWithAttributes(attributesFiltered)}),
			processors:   ibanTransformProcessor,
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
