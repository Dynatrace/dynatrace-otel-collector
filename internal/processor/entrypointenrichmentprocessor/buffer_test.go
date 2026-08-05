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
	"go.uber.org/zap"
)

const (
	testWait     = 5 * time.Millisecond
	testFallback = 50 * time.Millisecond
)

func testCfg(numTraces int) *Config {
	cfg := &Config{
		WaitDuration:             testWait,
		FallbackDuration:         testFallback,
		NumTraces:                numTraces,
		LocalRootDetection:       ModeFlagsWithKindFallback,
		AttributesToPromote:      []string{`^dt\.feature_flag\.`},
		LocalRootMarkerAttribute: "dt.local_root",
	}
	_ = cfg.Validate()
	return cfg
}

func newTestBuffer(numTraces int) (*Buffer, *consumertest.TracesSink) {
	sink := new(consumertest.TracesSink)
	buf := newBuffer(testCfg(numTraces), sink, zap.NewNop())
	return buf, sink
}

func insertSpan(buf *Buffer, traceID pcommon.TraceID, spanID pcommon.SpanID, parentID pcommon.SpanID, kind ptrace.SpanKind, attrs map[string]string) {
	res := pcommon.NewResource()
	scope := pcommon.NewInstrumentationScope()
	span := ptrace.NewSpan()
	span.SetTraceID(traceID)
	span.SetSpanID(spanID)
	if parentID != (pcommon.SpanID{}) {
		span.SetParentSpanID(parentID)
	}
	span.SetKind(kind)
	for k, v := range attrs {
		span.Attributes().PutStr(k, v)
	}
	buf.Insert(res, scope, span)
}

func totalSpans(sink *consumertest.TracesSink) int {
	total := 0
	for _, td := range sink.AllTraces() {
		total += td.SpanCount()
	}
	return total
}

func TestBuffer_HappyPath(t *testing.T) {
	buf, sink := newTestBuffer(100)
	tid := pcommon.TraceID([16]byte{1})
	rootID := pcommon.SpanID([8]byte{1})
	childID := pcommon.SpanID([8]byte{2})

	insertSpan(buf, tid, rootID, pcommon.SpanID{}, ptrace.SpanKindServer, nil)
	insertSpan(buf, tid, childID, rootID, ptrace.SpanKindInternal, map[string]string{
		"dt.feature_flag.result.foo": "bar",
	})

	require.Eventually(t, func() bool {
		return sink.SpanCount() == 2
	}, 200*time.Millisecond, time.Millisecond, "expected 2 spans after timer fires")

	// Assert FF attribute promoted to root.
	found := false
	for _, td := range sink.AllTraces() {
		for i := 0; i < td.ResourceSpans().Len(); i++ {
			for j := 0; j < td.ResourceSpans().At(i).ScopeSpans().Len(); j++ {
				for k := 0; k < td.ResourceSpans().At(i).ScopeSpans().At(j).Spans().Len(); k++ {
					sp := td.ResourceSpans().At(i).ScopeSpans().At(j).Spans().At(k)
					if sp.SpanID() == rootID {
						v, ok := sp.Attributes().Get("dt.feature_flag.result.foo")
						assert.True(t, ok)
						assert.Equal(t, "bar", v.AsString())
						found = true
					}
				}
			}
		}
	}
	assert.True(t, found, "root span not found in output")
}

func TestBuffer_LocalRootArrivesLate(t *testing.T) {
	buf, sink := newTestBuffer(100)
	tid := pcommon.TraceID([16]byte{2})
	rootID := pcommon.SpanID([8]byte{1})
	childID := pcommon.SpanID([8]byte{2})

	// Child arrives before root.
	insertSpan(buf, tid, childID, rootID, ptrace.SpanKindInternal, map[string]string{
		"dt.feature_flag.result.late": "yes",
	})
	// Root arrives after child.
	time.Sleep(2 * time.Millisecond)
	insertSpan(buf, tid, rootID, pcommon.SpanID{}, ptrace.SpanKindServer, nil)

	require.Eventually(t, func() bool {
		return sink.SpanCount() == 2
	}, 200*time.Millisecond, time.Millisecond)

	// Root should have FF promoted.
	for _, td := range sink.AllTraces() {
		for i := 0; i < td.ResourceSpans().Len(); i++ {
			for j := 0; j < td.ResourceSpans().At(i).ScopeSpans().Len(); j++ {
				for k := 0; k < td.ResourceSpans().At(i).ScopeSpans().At(j).Spans().Len(); k++ {
					sp := td.ResourceSpans().At(i).ScopeSpans().At(j).Spans().At(k)
					if sp.SpanID() == rootID {
						v, ok := sp.Attributes().Get("dt.feature_flag.result.late")
						assert.True(t, ok)
						assert.Equal(t, "yes", v.AsString())
					}
				}
			}
		}
	}
}

func TestBuffer_LocalRootNeverArrives(t *testing.T) {
	buf, sink := newTestBuffer(100)
	tid := pcommon.TraceID([16]byte{3})
	// Fake parent that never arrives.
	fakeRootID := pcommon.SpanID([8]byte{99})
	childID := pcommon.SpanID([8]byte{2})

	insertSpan(buf, tid, childID, fakeRootID, ptrace.SpanKindInternal, map[string]string{
		"dt.feature_flag.result.orphan": "yes",
	})

	// Fallback should fire and emit the child unpromoted.
	require.Eventually(t, func() bool {
		return sink.SpanCount() == 1
	}, 200*time.Millisecond, time.Millisecond, "expected fallback to flush child")
}

func TestBuffer_TwoLocalRootsOneTrace(t *testing.T) {
	buf, sink := newTestBuffer(100)
	tid := pcommon.TraceID([16]byte{4})
	root1ID := pcommon.SpanID([8]byte{1})
	root2ID := pcommon.SpanID([8]byte{10})
	child1ID := pcommon.SpanID([8]byte{2})
	child2ID := pcommon.SpanID([8]byte{11})

	// Two server spans with distinct subtrees.
	insertSpan(buf, tid, root1ID, pcommon.SpanID{}, ptrace.SpanKindServer, nil)
	insertSpan(buf, tid, child1ID, root1ID, ptrace.SpanKindInternal, map[string]string{
		"dt.feature_flag.result.a": "1",
	})
	// root2 has a (fake) remote parent — use flags 0x300.
	res := pcommon.NewResource()
	scope := pcommon.NewInstrumentationScope()
	sp2 := ptrace.NewSpan()
	sp2.SetTraceID(tid)
	sp2.SetSpanID(root2ID)
	sp2.SetParentSpanID(pcommon.SpanID([8]byte{99}))
	sp2.SetKind(ptrace.SpanKindServer)
	sp2.SetFlags(0x300)
	buf.Insert(res, scope, sp2)
	insertSpan(buf, tid, child2ID, root2ID, ptrace.SpanKindInternal, map[string]string{
		"dt.feature_flag.result.b": "2",
	})

	require.Eventually(t, func() bool {
		return sink.SpanCount() >= 4
	}, 500*time.Millisecond, time.Millisecond, "expected 4 spans from 2 subtrees")
}

func TestBuffer_LateChild(t *testing.T) {
	buf, sink := newTestBuffer(100)
	tid := pcommon.TraceID([16]byte{5})
	rootID := pcommon.SpanID([8]byte{1})
	earlyChildID := pcommon.SpanID([8]byte{2})
	lateChildID := pcommon.SpanID([8]byte{3})

	insertSpan(buf, tid, rootID, pcommon.SpanID{}, ptrace.SpanKindServer, nil)
	insertSpan(buf, tid, earlyChildID, rootID, ptrace.SpanKindInternal, nil)

	// Wait for subtree flush (root + earlyChild emitted).
	require.Eventually(t, func() bool {
		return sink.SpanCount() >= 2
	}, 200*time.Millisecond, time.Millisecond)

	// Late child arrives after flush — it'll go into a new trace state and be emitted by fallback.
	insertSpan(buf, tid, lateChildID, rootID, ptrace.SpanKindInternal, nil)

	require.Eventually(t, func() bool {
		return sink.SpanCount() >= 3
	}, 200*time.Millisecond, time.Millisecond, "late child should be emitted by fallback")
}

func TestBuffer_Eviction(t *testing.T) {
	// num_traces=1: inserting a second trace evicts the first.
	buf, sink := newTestBuffer(1)
	tid1 := pcommon.TraceID([16]byte{1})
	tid2 := pcommon.TraceID([16]byte{2})

	insertSpan(buf, tid1, pcommon.SpanID([8]byte{1}), pcommon.SpanID{}, ptrace.SpanKindInternal, nil)
	// This insert should trigger eviction of tid1.
	insertSpan(buf, tid2, pcommon.SpanID([8]byte{2}), pcommon.SpanID{}, ptrace.SpanKindInternal, nil)

	// tid1's span should be emitted via eviction goroutine.
	require.Eventually(t, func() bool {
		return sink.SpanCount() >= 1
	}, 200*time.Millisecond, time.Millisecond, "evicted trace should be emitted")
}

func TestBuffer_ShutdownDrains(t *testing.T) {
	buf, sink := newTestBuffer(100)
	tid := pcommon.TraceID([16]byte{9})

	insertSpan(buf, tid, pcommon.SpanID([8]byte{1}), pcommon.SpanID{}, ptrace.SpanKindInternal, nil)
	insertSpan(buf, tid, pcommon.SpanID([8]byte{2}), pcommon.SpanID([8]byte{1}), ptrace.SpanKindInternal, nil)

	// Immediately shutdown without waiting for timer.
	err := buf.Shutdown(context.Background())
	require.NoError(t, err)

	assert.Equal(t, 2, sink.SpanCount(), "shutdown should drain all spans")
}
