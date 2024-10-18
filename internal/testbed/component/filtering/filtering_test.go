package filtering

import (
	"testing"
	"time"

	"github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func TestFiltering(t *testing.T) {
	trace := generateBasicTrace()
	tests := []struct {
		name         string
		sender       testbed.DataSender
		receiver     testbed.DataReceiver
		inputData    data
		expectedData data
		compareFunc  func(t *testing.T, expectedData data, out data)
		processors   map[string]string
	}{
		{
			name:       "basic",
			sender:     testbed.NewOTLPTraceDataSender(testbed.DefaultHost, testutil.GetAvailablePort(t)),
			receiver:   testbed.NewOTLPDataReceiver(testutil.GetAvailablePort(t)),
			processors: defaultProcessors,
			inputData: data{
				Traces: []ptrace.Traces{trace},
			},
			expectedData: data{
				Traces: []ptrace.Traces{trace},
			},
			compareFunc: func(t *testing.T, expectedData data, out data) {
				require.Nil(t, expectedData)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_ = FilteringScenario(
				t,
				test.sender,
				test.receiver,
				test.inputData,
				test.processors,
				nil,
			)

			//require.Equal(t, test.expectedData, outputData)

			//test.compareFunc(t, test.expectedData, outputData)
		})
	}
}

func generateBasicTrace() ptrace.Traces {
	traceData := ptrace.NewTraces()
	spans := traceData.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans()
	spans.EnsureCapacity(1)

	startTime := time.Now()
	endTime := startTime.Add(time.Millisecond)

	span := spans.AppendEmpty()

	// Create a span.
	span.SetName("filtering-span")
	span.SetKind(ptrace.SpanKindClient)
	attrs := span.Attributes()
	attrs.PutStr("key", "value")

	span.SetStartTimestamp(pcommon.NewTimestampFromTime(startTime))
	span.SetEndTimestamp(pcommon.NewTimestampFromTime(endTime))

	return traceData
}
