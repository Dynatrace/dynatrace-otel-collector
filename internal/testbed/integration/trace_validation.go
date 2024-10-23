package integration

import (
	"fmt"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/ptracetest"
	"github.com/stretchr/testify/require"

	"github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/idutils"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

var _ testbed.TestCaseValidator = &TraceValidator{}

type TraceValidator struct {
	expectedTraces []ptrace.Traces
	t              *testing.T
}

func NewTraceValidator(t *testing.T, expectedTraces []ptrace.Traces) *TraceValidator {
	return &TraceValidator{
		expectedTraces: expectedTraces,
		t:              t,
	}
}

func (v *TraceValidator) Validate(tc *testbed.TestCase) {
	assertExpectedSpansAreInReceived(v.t, v.expectedTraces, tc.MockBackend.ReceivedTraces)
}

func (v *TraceValidator) RecordResults(tc *testbed.TestCase) {
}

func assertExpectedSpansAreInReceived(t *testing.T, expected, actual []ptrace.Traces) {
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
						idutils.TraceIDAndSpanIDToString(recdSpan.TraceID(), recdSpan.SpanID()),
						fmt.Sprintf("Span with ID: %q not found among expected spans", recdSpan.SpanID()))

					require.Nil(t, ptracetest.CompareTraces(expectedMap[idutils.TraceIDAndSpanIDToString(recdSpan.TraceID(), recdSpan.SpanID())], td, ptracetest.IgnoreSpansOrder(), ptracetest.IgnoreEndTimestamp(), ptracetest.IgnoreStartTimestamp()))
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
					key := idutils.TraceIDAndSpanIDToString(span.TraceID(), span.SpanID())
					expectedMap[key] = td
				}
			}
		}
	}
}
