package integration

import (
	"fmt"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
	"github.com/stretchr/testify/assert"
)

var _ testbed.TestCaseValidator = &SyslogSampleConfigsValidator{}

type SyslogSampleConfigsValidator struct {
	expectedLogs plog.Logs
	t            *testing.T
}

// NewSyslogSampleConfigValidator ensures expected logs are present in the output.
func NewSyslogSampleConfigValidator(t *testing.T, expectedLogs plog.Logs) *SyslogSampleConfigsValidator {
	return &SyslogSampleConfigsValidator{
		expectedLogs: expectedLogs,
		t:            t,
	}
}

func (v *SyslogSampleConfigsValidator) Validate(tc *testbed.TestCase) {
	actualLogs := 0
	for _, td := range tc.MockBackend.ReceivedLogs {
		actualLogs += td.LogRecordCount()
	}

	assert.EqualValues(v.t, v.expectedLogs.LogRecordCount(), actualLogs, "Expected %d logs, received %d.", v.expectedLogs.LogRecordCount(), actualLogs)
	assertExpectedLogsAreInReceived(v.t, []plog.Logs{v.expectedLogs}, tc.MockBackend.ReceivedLogs)
}

func (v *SyslogSampleConfigsValidator) RecordResults(tc *testbed.TestCase) {
}

func assertExpectedLogsAreInReceived(t *testing.T, expected, actual []plog.Logs) {
	expectedMap := make(map[string]plog.LogRecord)
	populateLogsMap(expectedMap, expected)

	for _, td := range actual {
		resourceLogs := td.ResourceLogs()
		for i := 0; i < resourceLogs.Len(); i++ {
			scopeLogs := resourceLogs.At(i).ScopeLogs()
			for j := 0; j < scopeLogs.Len(); j++ {
				logRecords := scopeLogs.At(j).LogRecords()
				for k := 0; k < logRecords.Len(); k++ {
					actualLogRecord := logRecords.At(k)
					hasEntry := assert.Contains(t,
						expectedMap,
						actualLogRecord.Body().AsString(),
						fmt.Sprintf("Actual log with body : %q not found among expected logRecords", actualLogRecord.Body().AsString()))

					// avoid panic due to expectedLogRecord being nil
					if !hasEntry {
						return
					}

					expectedLogRecord := expectedMap[actualLogRecord.Body().AsString()]
					assert.Equal(t, expectedLogRecord.Timestamp().String(), actualLogRecord.Timestamp().String())
					assertMap(t, expectedLogRecord.Attributes(), actualLogRecord.Attributes())
					assert.Equal(t, expectedLogRecord.SpanID(), actualLogRecord.SpanID())
					assert.Equal(t, expectedLogRecord.TraceID(), actualLogRecord.TraceID())
					assert.Equal(t, expectedLogRecord.Body(), actualLogRecord.Body())
					// the syslog receiver will override the ObservedTimestamp with the current timestamp on the collector, so we test if the actual timestamp has been after the expected one.
					assert.LessOrEqual(t, expectedLogRecord.ObservedTimestamp().AsTime(), actualLogRecord.ObservedTimestamp().AsTime())
				}
			}
		}
	}
}

// populateLogsMap populates a map with the body as the key and a LogRecord as the value for easier log record matching
func populateLogsMap(logsMap map[string]plog.LogRecord, tds []plog.Logs) {
	for _, td := range tds {
		resourceLogs := td.ResourceLogs()
		for i := 0; i < resourceLogs.Len(); i++ {
			scopeLogs := resourceLogs.At(i).ScopeLogs()
			for j := 0; j < scopeLogs.Len(); j++ {
				logs := scopeLogs.At(j).LogRecords()
				for k := 0; k < logs.Len(); k++ {
					log := logs.At(k)
					key := log.Body().AsString()
					logsMap[key] = log
				}
			}
		}
	}
}

func assertMap(t *testing.T, expected pcommon.Map, actual pcommon.Map) {
	assert.Equal(t, expected.Len(), actual.Len())
	expected.Range(func(expectedKey string, expectedValue pcommon.Value) bool {
		actualValue, exists := actual.Get(expectedKey)
		assert.True(t, exists, "Expected attribute %s, but no attribute was present", expectedKey)
		assertValue(t, expectedValue, actualValue)
		return true
	})
}

func assertValue(t *testing.T, expected pcommon.Value, actual pcommon.Value) {
	assert.Equal(t, expected.Type(), actual.Type())
	switch expected.Type() {
	case pcommon.ValueTypeMap:
		assertMap(t, expected.Map(), actual.Map())
		break
	default:
		assert.Equal(t, expected, actual)
	}
}
