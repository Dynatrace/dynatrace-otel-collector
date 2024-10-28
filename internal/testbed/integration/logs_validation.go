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

func WithHiddenLogsValidationErrorMessages() func(validator *LogsValidator) {
	return func(v *LogsValidator) {
		v.hideValidationErrorMessage = true
	}
}

type LogsValidator struct {
	expectedLogs []plog.Logs
	t            *testing.T

	hideValidationErrorMessage bool
}

// NewLogsValidator ensures expected logs are present in the output.
func NewLogsValidator(t *testing.T, expectedLogs []plog.Logs, opts ...func(*LogsValidator)) *LogsValidator {
	v := &LogsValidator{
		expectedLogs: expectedLogs,
		t:            t,
	}

	for _, opt := range opts {
		opt(v)
	}

	return v
}

func (v *LogsValidator) Validate(tc *testbed.TestCase) {
	assertExpectedLogsAreInReceived(v.t, v.expectedLogs, tc.MockBackend.ReceivedLogs, v.hideValidationErrorMessage)
}

func (v *LogsValidator) RecordResults(tc *testbed.TestCase) {
}

func assertExpectedLogsAreInReceived(t *testing.T, expected, actual []plog.Logs, hideError bool) {
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

					err := plogtest.CompareLogs(
						expectedMap[actualLogRecord.Body().AsString()],
						td,
						plogtest.IgnoreObservedTimestamp(),
					)

					// if hideError is set, the actual error message is not logged. This is for testing scenarios where we validate the
					// redaction of API Tokens in the attributes of the received data items.
					// If the redaction is not applied correctly, the original values would otherwise be visible in the logs,
					// which in turn might lead to false positive security alerts (the sample tokens are in an allow list, but just to be on the safe side).
					if hideError && err != nil {
						t.Error("Received logs did not match expected logs")
					} else {
						require.NoError(
							t,
							err,
						)
					}
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
