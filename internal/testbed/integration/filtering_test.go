package integration

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
		dataProvider testbed.DataProvider
		sender       testbed.DataSender
		receiver     testbed.DataReceiver
		expectedData inputData
		compareFunc  func(t *testing.T, expectedData inputData, out receivedData)
		processors   map[string]string
	}{
		{
			name:         "basic",
			dataProvider: NewSampleConfigsTraceDataProvider(trace),
			sender:       testbed.NewOTLPTraceDataSender(testbed.DefaultHost, testutil.GetAvailablePort(t)),
			receiver:     testbed.NewOTLPDataReceiver(testutil.GetAvailablePort(t)),
			processors:   defaultProcessors,
			expectedData: inputData{
				Traces: trace,
			},
			compareFunc: func(t *testing.T, expectedData inputData, out receivedData) {
				expectedSpan := expectedData.Traces.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)
				receivedSpan := out.Traces[0].ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)
				require.Equal(t, expectedSpan, receivedSpan)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			outputData := FilteringScenario(
				t,
				test.dataProvider,
				test.sender,
				test.receiver,
				test.processors,
				nil,
			)

			test.compareFunc(t, test.expectedData, outputData)
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
