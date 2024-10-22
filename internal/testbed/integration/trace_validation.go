package integration

import (
	"fmt"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/ptracetest"
	"github.com/stretchr/testify/require"
	"testing"

	"github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/idutils"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

var _ testbed.TestCaseValidator = &TraceSampleConfigsValidator{}

func WithTraceAttributeCheck() func(validator *TraceSampleConfigsValidator) {
	return func(validator *TraceSampleConfigsValidator) {
		validator.checkAttributes = true
	}
}

type TraceSampleConfigsValidator struct {
	expectedTraces  ptrace.Traces
	t               *testing.T
	checkAttributes bool
}

func NewTraceSampleConfigsValidator(
	t *testing.T,
	expectedTraces ptrace.Traces,
	opts ...func(v *TraceSampleConfigsValidator),
) *TraceSampleConfigsValidator {
	v := &TraceSampleConfigsValidator{
		expectedTraces: expectedTraces,
		t:              t,
	}

	for _, o := range opts {
		o(v)
	}
	return v
}

func (v *TraceSampleConfigsValidator) Validate(tc *testbed.TestCase) {
	actualSpans := 0
	for _, td := range tc.MockBackend.ReceivedTraces {
		actualSpans += td.SpanCount()
	}

	assert.EqualValues(v.t, v.expectedTraces.SpanCount(), actualSpans, "Expected %d spans, received %d.", v.expectedTraces.SpanCount(), actualSpans)
	assertExpectedSpansAreInReceived(v.t, []ptrace.Traces{v.expectedTraces}, tc.MockBackend.ReceivedTraces, v.checkAttributes)
}

func (v *TraceSampleConfigsValidator) RecordResults(tc *testbed.TestCase) {
}

func assertExpectedSpansAreInReceived(t *testing.T, expected, actual []ptrace.Traces, checkAttributes bool) {
	spansMap := make(map[string]ptrace.Span)
	populateSpansMap(spansMap, expected)

	for _, td := range actual {
		rss := td.ResourceSpans()
		for i := 0; i < rss.Len(); i++ {
			ss := rss.At(i).ScopeSpans()
			for j := 0; j < ss.Len(); j++ {
				spans := ss.At(j).Spans()
				for k := 0; k < spans.Len(); k++ {
					recdSpan := spans.At(k)
					require.Contains(t,
						spansMap,
						idutils.TraceIDAndSpanIDToString(recdSpan.TraceID(), recdSpan.SpanID()),
						fmt.Sprintf("Span with ID: %q not found among expected spans", recdSpan.SpanID()))

					expectedSpan := spansMap[idutils.TraceIDAndSpanIDToString(recdSpan.TraceID(), recdSpan.SpanID())]
					require.NoError(t, ptracetest.CompareSpan(expectedSpan, recdSpan))
				}
			}
		}
	}
}

func populateSpansMap(spansMap map[string]ptrace.Span, tds []ptrace.Traces) {
	for _, td := range tds {
		rss := td.ResourceSpans()
		for i := 0; i < rss.Len(); i++ {
			ilss := rss.At(i).ScopeSpans()
			for j := 0; j < ilss.Len(); j++ {
				spans := ilss.At(j).Spans()
				for k := 0; k < spans.Len(); k++ {
					span := spans.At(k)
					key := idutils.TraceIDAndSpanIDToString(span.TraceID(), span.SpanID())
					spansMap[key] = span
				}
			}
		}
	}
}
