package integration

import (
	"fmt"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// SampleConfigsValidator implements TestCaseValidator.
// It validates the received traces against the provided expected traces.
type SampleConfigsValidator struct {
	expectedTraces ptrace.Traces
	t              *testing.T
}

func NewSampleConfigsValidator(t *testing.T, expectedTraces ptrace.Traces) *SampleConfigsValidator {
	return &SampleConfigsValidator{
		expectedTraces: expectedTraces,
		t:              t,
	}
}

func (v *SampleConfigsValidator) Validate(tc *testbed.TestCase) {
	actualSpans := tc.MockBackend.DataItemsReceived()

	assert.EqualValues(v.t, v.expectedTraces.SpanCount(), actualSpans, "Received and expected number of spans do not match.")
	assertExpectedSpansAreInReceived(v.t, []ptrace.Traces{v.expectedTraces}, tc.MockBackend.ReceivedTraces)
}

func (v *SampleConfigsValidator) RecordResults(tc *testbed.TestCase) {
}

func assertExpectedSpansAreInReceived(t *testing.T, expected, actual []ptrace.Traces) {
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
					assert.Contains(t,
						spansMap,
						traceIDAndSpanIDToString(recdSpan.TraceID(), recdSpan.SpanID()),
						fmt.Sprintf("Span with ID: '%s' not found among expected spans", recdSpan.SpanID()))
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
					key := traceIDAndSpanIDToString(span.TraceID(), span.SpanID())
					spansMap[key] = span
				}
			}
		}
	}
}
