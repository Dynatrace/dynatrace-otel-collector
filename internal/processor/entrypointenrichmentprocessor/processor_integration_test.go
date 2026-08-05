// Copyright Dynatrace LLC
// SPDX-License-Identifier: Apache-2.0

package entrypointenrichmentprocessor

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor/processortest"
)

func newIntegrationProcessor(t *testing.T, sink *consumertest.TracesSink) *entrypointEnrichmentProcessor {
	t.Helper()
	cfg := &Config{
		WaitDuration:             10 * time.Millisecond,
		FallbackDuration:         200 * time.Millisecond,
		NumTraces:                10000,
		LocalRootDetection:       ModeFlagsWithKindFallback,
		AttributesToPromote:      []string{`^dt\.feature_flag\.`},
		LocalRootMarkerAttribute: "dt.local_root",
	}
	set := processortest.NewNopSettings(Type)
	p, err := newProcessor(cfg, set, sink)
	require.NoError(t, err)
	return p
}

func findSpanByID(sink *consumertest.TracesSink, sid pcommon.SpanID) (ptrace.Span, bool) {
	for _, td := range sink.AllTraces() {
		for i := 0; i < td.ResourceSpans().Len(); i++ {
			for j := 0; j < td.ResourceSpans().At(i).ScopeSpans().Len(); j++ {
				for k := 0; k < td.ResourceSpans().At(i).ScopeSpans().At(j).Spans().Len(); k++ {
					sp := td.ResourceSpans().At(i).ScopeSpans().At(j).Spans().At(k)
					if sp.SpanID() == sid {
						return sp, true
					}
				}
			}
		}
	}
	return ptrace.NewSpan(), false
}

func TestIntegration_FFEnrichment(t *testing.T) {
	sink := new(consumertest.TracesSink)
	p := newIntegrationProcessor(t, sink)

	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("service.name", "svc-a")
	ss := rs.ScopeSpans().AppendEmpty()

	tid := pcommon.TraceID([16]byte{1})
	rootID := pcommon.SpanID([8]byte{1})
	child1ID := pcommon.SpanID([8]byte{2})
	child2ID := pcommon.SpanID([8]byte{3})

	root := ss.Spans().AppendEmpty()
	root.SetTraceID(tid)
	root.SetSpanID(rootID)
	root.SetKind(ptrace.SpanKindServer)

	child1 := ss.Spans().AppendEmpty()
	child1.SetTraceID(tid)
	child1.SetSpanID(child1ID)
	child1.SetParentSpanID(rootID)
	child1.SetKind(ptrace.SpanKindInternal)
	child1.Attributes().PutStr("dt.feature_flag.result.pricing_v2", "on")

	child2 := ss.Spans().AppendEmpty()
	child2.SetTraceID(tid)
	child2.SetSpanID(child2ID)
	child2.SetParentSpanID(rootID)
	child2.SetKind(ptrace.SpanKindInternal)
	child2.Attributes().PutStr("dt.feature_flag.result.recs_algo", "collab")

	err := p.ConsumeTraces(context.Background(), td)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return sink.SpanCount() == 3
	}, 500*time.Millisecond, time.Millisecond)

	rootSpan, found := findSpanByID(sink, rootID)
	require.True(t, found)

	v, ok := rootSpan.Attributes().Get("dt.feature_flag.result.pricing_v2")
	assert.True(t, ok)
	assert.Equal(t, "on", v.AsString())

	v2, ok := rootSpan.Attributes().Get("dt.feature_flag.result.recs_algo")
	assert.True(t, ok)
	assert.Equal(t, "collab", v2.AsString())
}

func TestIntegration_OutOfOrder(t *testing.T) {
	sink := new(consumertest.TracesSink)
	p := newIntegrationProcessor(t, sink)

	tid := pcommon.TraceID([16]byte{2})
	rootID := pcommon.SpanID([8]byte{1})
	childID := pcommon.SpanID([8]byte{2})

	// Child arrives first.
	td1 := ptrace.NewTraces()
	rs1 := td1.ResourceSpans().AppendEmpty()
	ss1 := rs1.ScopeSpans().AppendEmpty()
	ch := ss1.Spans().AppendEmpty()
	ch.SetTraceID(tid)
	ch.SetSpanID(childID)
	ch.SetParentSpanID(rootID)
	ch.SetKind(ptrace.SpanKindInternal)
	ch.Attributes().PutStr("dt.feature_flag.result.oo", "yes")

	err := p.ConsumeTraces(context.Background(), td1)
	require.NoError(t, err)

	// Root arrives second.
	td2 := ptrace.NewTraces()
	rs2 := td2.ResourceSpans().AppendEmpty()
	ss2 := rs2.ScopeSpans().AppendEmpty()
	rt := ss2.Spans().AppendEmpty()
	rt.SetTraceID(tid)
	rt.SetSpanID(rootID)
	rt.SetKind(ptrace.SpanKindServer)

	err = p.ConsumeTraces(context.Background(), td2)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return sink.SpanCount() == 2
	}, 500*time.Millisecond, time.Millisecond)

	rootSpan, found := findSpanByID(sink, rootID)
	require.True(t, found)
	v, ok := rootSpan.Attributes().Get("dt.feature_flag.result.oo")
	assert.True(t, ok)
	assert.Equal(t, "yes", v.AsString())
}

func TestIntegration_CrossBatch(t *testing.T) {
	sink := new(consumertest.TracesSink)
	p := newIntegrationProcessor(t, sink)

	tid := pcommon.TraceID([16]byte{3})
	rootID := pcommon.SpanID([8]byte{1})
	child1ID := pcommon.SpanID([8]byte{2})
	child2ID := pcommon.SpanID([8]byte{3})

	// First batch: root + child1.
	td1 := ptrace.NewTraces()
	rs1 := td1.ResourceSpans().AppendEmpty()
	rs1.Resource().Attributes().PutStr("service.name", "svc-cb")
	ss1 := rs1.ScopeSpans().AppendEmpty()

	rt := ss1.Spans().AppendEmpty()
	rt.SetTraceID(tid)
	rt.SetSpanID(rootID)
	rt.SetKind(ptrace.SpanKindServer)

	c1 := ss1.Spans().AppendEmpty()
	c1.SetTraceID(tid)
	c1.SetSpanID(child1ID)
	c1.SetParentSpanID(rootID)
	c1.SetKind(ptrace.SpanKindInternal)

	err := p.ConsumeTraces(context.Background(), td1)
	require.NoError(t, err)

	// Second batch: child2 (same trace, arrives before timer fires).
	td2 := ptrace.NewTraces()
	rs2 := td2.ResourceSpans().AppendEmpty()
	rs2.Resource().Attributes().PutStr("service.name", "svc-cb")
	ss2 := rs2.ScopeSpans().AppendEmpty()
	c2 := ss2.Spans().AppendEmpty()
	c2.SetTraceID(tid)
	c2.SetSpanID(child2ID)
	c2.SetParentSpanID(rootID)
	c2.SetKind(ptrace.SpanKindInternal)
	c2.Attributes().PutStr("dt.feature_flag.result.cross", "batch")

	err = p.ConsumeTraces(context.Background(), td2)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return sink.SpanCount() == 3
	}, 500*time.Millisecond, time.Millisecond)

	rootSpan, found := findSpanByID(sink, rootID)
	require.True(t, found)
	v, ok := rootSpan.Attributes().Get("dt.feature_flag.result.cross")
	assert.True(t, ok)
	assert.Equal(t, "batch", v.AsString())
}

func TestIntegration_TwoLocalRoots(t *testing.T) {
	sink := new(consumertest.TracesSink)
	p := newIntegrationProcessor(t, sink)

	tid := pcommon.TraceID([16]byte{4})
	root1ID := pcommon.SpanID([8]byte{1})
	child1ID := pcommon.SpanID([8]byte{2})
	root2ID := pcommon.SpanID([8]byte{10})
	child2ID := pcommon.SpanID([8]byte{11})

	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()

	// Root 1: empty parent → local root.
	r1 := ss.Spans().AppendEmpty()
	r1.SetTraceID(tid)
	r1.SetSpanID(root1ID)
	r1.SetKind(ptrace.SpanKindServer)

	c1 := ss.Spans().AppendEmpty()
	c1.SetTraceID(tid)
	c1.SetSpanID(child1ID)
	c1.SetParentSpanID(root1ID)
	c1.SetKind(ptrace.SpanKindInternal)
	c1.SetFlags(0x100) // HAS_IS_REMOTE, not remote → local child
	c1.Attributes().PutStr("dt.feature_flag.result.svc_a", "val-a")

	// Root 2: remote parent (flags=0x300) → also local root.
	r2 := ss.Spans().AppendEmpty()
	r2.SetTraceID(tid)
	r2.SetSpanID(root2ID)
	r2.SetParentSpanID(pcommon.SpanID([8]byte{99})) // remote parent (not in buffer)
	r2.SetKind(ptrace.SpanKindServer)
	r2.SetFlags(0x300)

	c2 := ss.Spans().AppendEmpty()
	c2.SetTraceID(tid)
	c2.SetSpanID(child2ID)
	c2.SetParentSpanID(root2ID)
	c2.SetKind(ptrace.SpanKindInternal)
	c2.SetFlags(0x100)
	c2.Attributes().PutStr("dt.feature_flag.result.svc_b", "val-b")

	err := p.ConsumeTraces(context.Background(), td)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return sink.SpanCount() >= 4
	}, 500*time.Millisecond, time.Millisecond)

	// Root1 should have svc_a promoted, not svc_b.
	r1Span, found := findSpanByID(sink, root1ID)
	require.True(t, found)
	v, ok := r1Span.Attributes().Get("dt.feature_flag.result.svc_a")
	assert.True(t, ok)
	assert.Equal(t, "val-a", v.AsString())
	_, ok = r1Span.Attributes().Get("dt.feature_flag.result.svc_b")
	assert.False(t, ok, "root1 should NOT have root2's FF attr")

	// Root2 should have svc_b promoted, not svc_a.
	r2Span, found := findSpanByID(sink, root2ID)
	require.True(t, found)
	v2, ok := r2Span.Attributes().Get("dt.feature_flag.result.svc_b")
	assert.True(t, ok)
	assert.Equal(t, "val-b", v2.AsString())
	_, ok = r2Span.Attributes().Get("dt.feature_flag.result.svc_a")
	assert.False(t, ok, "root2 should NOT have root1's FF attr")
}
