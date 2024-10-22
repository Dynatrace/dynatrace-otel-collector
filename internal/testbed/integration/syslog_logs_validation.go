package integration

import (
	"fmt"
	"testing"

	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
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
	expectedMap := make(map[string]plog.Logs)
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

					assert.Nil(t, plogtest.CompareLogs(expectedMap[actualLogRecord.Body().AsString()], td))
				}
			}
		}
	}
}

// populateLogsMap populates a map with the body as the key and a LogRecord as the value for easier log record matching
func populateLogsMap(logsMap map[string]plog.Logs, tds []plog.Logs) {
	for _, td := range tds {
		resourceLogs := td.ResourceLogs()
		for i := 0; i < resourceLogs.Len(); i++ {
			scopeLogs := resourceLogs.At(i).ScopeLogs()
			for j := 0; j < scopeLogs.Len(); j++ {
				logs := scopeLogs.At(j).LogRecords()
				for k := 0; k < logs.Len(); k++ {
					log := logs.At(k)
					key := log.Body().AsString()
					logsMap[key] = td
				}
			}
		}
	}
}
