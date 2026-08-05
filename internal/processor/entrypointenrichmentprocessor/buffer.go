// Copyright Dynatrace LLC
// SPDX-License-Identifier: Apache-2.0

package entrypointenrichmentprocessor

import (
	"context"
	"sync"
	"time"

	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

type bufferedSpan struct {
	resource pcommon.Resource
	scope    pcommon.InstrumentationScope
	span     ptrace.Span
}

type traceState struct {
	spans         map[pcommon.SpanID]bufferedSpan
	subtreeTimers map[pcommon.SpanID]*time.Timer
	fallbackTimer *time.Timer
}

// Buffer is the lazy-resolution span buffer.
type Buffer struct {
	mu     sync.Mutex
	traces map[pcommon.TraceID]*traceState
	order  []pcommon.TraceID // insertion order for eviction

	cfg    *Config
	mode   LocalRootDetectionMode
	next   consumer.Traces
	logger *zap.Logger
}

func newBuffer(cfg *Config, next consumer.Traces, logger *zap.Logger) *Buffer {
	return &Buffer{
		traces: make(map[pcommon.TraceID]*traceState),
		cfg:    cfg,
		mode:   cfg.LocalRootDetection,
		next:   next,
		logger: logger,
	}
}

func (b *Buffer) Insert(res pcommon.Resource, scope pcommon.InstrumentationScope, span ptrace.Span) {
	tid := span.TraceID()
	sid := span.SpanID()

	rCopy := pcommon.NewResource()
	res.CopyTo(rCopy)
	sCopy := pcommon.NewInstrumentationScope()
	scope.CopyTo(sCopy)
	spCopy := ptrace.NewSpan()
	span.CopyTo(spCopy)

	b.mu.Lock()

	ts, exists := b.traces[tid]
	if !exists {
		// Evict oldest if at capacity.
		if len(b.traces) >= b.cfg.NumTraces && len(b.order) > 0 {
			oldest := b.order[0]
			b.order = b.order[1:]

			// Evict inline while holding the lock: stop timers, collect members, then
			// emit outside the lock via goroutine to avoid blocking.
			if old, ok := b.traces[oldest]; ok {
				if old.fallbackTimer != nil {
					old.fallbackTimer.Stop()
				}
				for _, t := range old.subtreeTimers {
					t.Stop()
				}
				members := make([]bufferedSpan, 0, len(old.spans))
				for _, s := range old.spans {
					members = append(members, s)
				}
				delete(b.traces, oldest)
				if len(members) > 0 {
					go func() {
						_ = b.next.ConsumeTraces(context.Background(), assemble(members))
					}()
				}
			}
			b.logger.Warn("entrypoint_enrichment: evicted oldest trace due to num_traces limit")
		}
		ts = &traceState{
			spans:         make(map[pcommon.SpanID]bufferedSpan),
			subtreeTimers: make(map[pcommon.SpanID]*time.Timer),
		}
		b.traces[tid] = ts
		b.order = append(b.order, tid)
		ts.fallbackTimer = time.AfterFunc(b.cfg.FallbackDuration, func() {
			b.flushTrace(tid)
		})
	}

	ts.spans[sid] = bufferedSpan{resource: rCopy, scope: sCopy, span: spCopy}

	if isLocalRoot(spCopy, b.mode) {
		if _, ok := ts.subtreeTimers[sid]; !ok {
			localSid := sid
			ts.subtreeTimers[localSid] = time.AfterFunc(b.cfg.WaitDuration, func() {
				b.flushSubtree(tid, localSid)
			})
		}
	}

	b.mu.Unlock()
}

func (b *Buffer) flushSubtree(tid pcommon.TraceID, rootID pcommon.SpanID) {
	b.mu.Lock()
	ts := b.traces[tid]
	if ts == nil {
		b.mu.Unlock()
		return
	}
	if t, ok := ts.subtreeTimers[rootID]; ok {
		t.Stop()
		delete(ts.subtreeTimers, rootID)
	}

	// Collect members first (without deleting) so reaches() can walk the full index.
	var memberIDs []pcommon.SpanID
	var members []bufferedSpan
	for sid, s := range ts.spans {
		if reaches(sid, rootID, ts.spans, b.mode) {
			memberIDs = append(memberIDs, sid)
			members = append(members, s)
		}
	}
	// Now delete collected spans from the trace state.
	for _, sid := range memberIDs {
		delete(ts.spans, sid)
	}
	if len(ts.spans) == 0 {
		if ts.fallbackTimer != nil {
			ts.fallbackTimer.Stop()
		}
		b.removeFromOrder(tid)
		delete(b.traces, tid)
	}
	b.mu.Unlock()

	if len(members) == 0 {
		return
	}

	promote(members, rootID, b.cfg)
	_ = b.next.ConsumeTraces(context.Background(), assemble(members))
}

func (b *Buffer) removeFromOrder(tid pcommon.TraceID) {
	for i, id := range b.order {
		if id == tid {
			b.order = append(b.order[:i], b.order[i+1:]...)
			return
		}
	}
}

// reaches walks parent pointers from spanID toward target.
// Returns true iff the walk reaches target without crossing a different local root.
func reaches(spanID, target pcommon.SpanID, index map[pcommon.SpanID]bufferedSpan, mode LocalRootDetectionMode) bool {
	cur, ok := index[spanID]
	if !ok {
		return false
	}
	for {
		if cur.span.SpanID() == target {
			return true
		}
		// If the current span is itself a local root (and it isn't the target),
		// it belongs to a different subtree — stop here.
		if isLocalRoot(cur.span, mode) {
			return false
		}
		pid := cur.span.ParentSpanID()
		if pid.IsEmpty() {
			return false
		}
		parent, ok := index[pid]
		if !ok {
			return false
		}
		cur = parent
	}
}

func (b *Buffer) flushTrace(tid pcommon.TraceID) {
	b.mu.Lock()
	ts := b.traces[tid]
	if ts == nil {
		b.mu.Unlock()
		return
	}
	for _, t := range ts.subtreeTimers {
		t.Stop()
	}
	if ts.fallbackTimer != nil {
		ts.fallbackTimer.Stop()
	}
	b.removeFromOrder(tid)
	delete(b.traces, tid)

	members := make([]bufferedSpan, 0, len(ts.spans))
	for _, s := range ts.spans {
		members = append(members, s)
	}
	b.mu.Unlock()

	if len(members) == 0 {
		return
	}
	_ = b.next.ConsumeTraces(context.Background(), assemble(members))
}

func (b *Buffer) Shutdown(ctx context.Context) error {
	b.mu.Lock()
	tids := make([]pcommon.TraceID, 0, len(b.traces))
	for tid := range b.traces {
		tids = append(tids, tid)
	}
	b.mu.Unlock()

	for _, tid := range tids {
		b.flushTrace(tid)
	}
	return nil
}
