package integration

import (
	"fmt"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/ptracetest"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

var _ testbed.TestCaseValidator = &TraceValidator{}

func WithHiddenTracesValidationErrorMessages() func(*TraceValidator) {
	return func(v *TraceValidator) {
		v.hideValidationErrorMessage = true
	}
}

type TraceValidator struct {
	expectedTraces []ptrace.Traces
	t              *testing.T

	hideValidationErrorMessage bool
}

func NewTraceValidator(t *testing.T, expectedTraces []ptrace.Traces, opts ...func(*TraceValidator)) *TraceValidator {
	v := &TraceValidator{
		expectedTraces: expectedTraces,
		t:              t,
	}

	for _, opt := range opts {
		opt(v)
	}

	return v
}

func (v *TraceValidator) Validate(tc *testbed.TestCase) {
	assertExpectedSpansAreInReceived(v.t, v.expectedTraces, tc.MockBackend.ReceivedTraces, v.hideValidationErrorMessage)
}

func (v *TraceValidator) RecordResults(tc *testbed.TestCase) {
}

func assertExpectedSpansAreInReceived(t *testing.T, expected, actual []ptrace.Traces, hideError bool) {
	expectedMap := make(map[string]ptrace.Traces)
	populateSpansMap(expectedMap, expected)

	for _, td := range actual {
		rss := td.ResourceSpans()
		for i := 0; i < rss.Len(); i++ {
			ss := rss.At(i).ScopeSpans()
			for j := 0; j < ss.Len(); j++ {
				spans := ss.At(j).Spans()
				for k := 0; k < spans.Len(); k++ {
					recdSpan := spans.At(k)
					require.Contains(t,
						expectedMap,
						traceIDAndSpanIDToString(recdSpan.TraceID(), recdSpan.SpanID()),
						fmt.Sprintf("Span with ID: %q not found among expected spans", recdSpan.SpanID()))

					err := ptracetest.CompareTraces(expectedMap[traceIDAndSpanIDToString(recdSpan.TraceID(), recdSpan.SpanID())],
						td,
						ptracetest.IgnoreSpansOrder(),
						ptracetest.IgnoreEndTimestamp(),
						ptracetest.IgnoreStartTimestamp(),
					)

					// if hideError is set, the actual error message is not logged. This is for testing scenarios where we validate the
					// redaction of API Tokens in the attributes of the received data items.
					// If the redaction is not applied correctly, the original values would otherwise be visible in the logs,
					// which in turn might lead to false positive security alerts (the sample tokens are in an allow list, but just to be on the safe side).
					if hideError && err != nil {
						t.Error("Received traces did not match expected traces")
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

func populateSpansMap(expectedMap map[string]ptrace.Traces, tds []ptrace.Traces) {
	for _, td := range tds {
		rss := td.ResourceSpans()
		for i := 0; i < rss.Len(); i++ {
			ilss := rss.At(i).ScopeSpans()
			for j := 0; j < ilss.Len(); j++ {
				spans := ilss.At(j).Spans()
				for k := 0; k < spans.Len(); k++ {
					span := spans.At(k)
					key := traceIDAndSpanIDToString(span.TraceID(), span.SpanID())
					expectedMap[key] = td
				}
			}
		}
	}
}

func traceIDAndSpanIDToString(traceID pcommon.TraceID, spanID pcommon.SpanID) string {
	return fmt.Sprintf("%s-%s", traceID.String(), spanID.String())
}
