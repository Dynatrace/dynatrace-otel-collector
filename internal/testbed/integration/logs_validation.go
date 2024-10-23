package integration

import (
	"fmt"
	"testing"

	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
	"github.com/stretchr/testify/require"
)

var _ testbed.TestCaseValidator = &LogsValidator{}

type LogsValidator struct {
	expectedLogs []plog.Logs
	t            *testing.T
}

// NewLogsValidator ensures expected logs are present in the output.
func NewLogsValidator(t *testing.T, expectedLogs []plog.Logs) *LogsValidator {
	return &LogsValidator{
		expectedLogs: expectedLogs,
		t:            t,
	}
}

func (v *LogsValidator) Validate(tc *testbed.TestCase) {
	assertExpectedLogsAreInReceived(v.t, v.expectedLogs, tc.MockBackend.ReceivedLogs)
}

func (v *LogsValidator) RecordResults(tc *testbed.TestCase) {
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
					require.Contains(t,
						expectedMap,
						actualLogRecord.Body().AsString(),
						fmt.Sprintf("Actual log with body : %q not found among expected logRecords", actualLogRecord.Body().AsString()))

					require.Nil(t,
						plogtest.CompareLogs(
							expectedMap[actualLogRecord.Body().AsString()],
							td,
							plogtest.IgnoreObservedTimestamp(),
						),
					)
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
