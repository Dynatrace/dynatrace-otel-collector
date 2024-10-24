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
