# ICP-7891 — Implementation Plan: `entrypointenrichmentprocessor`

> Self-contained plan for a subagent to prototype the entrypoint-enrichment processor. If you are picking up this task cold, read this document top-to-bottom before starting. Cross-references to the other design docs in this directory (`local/`) provide the "why" behind each decision.

## Context

The Dynatrace collector distribution needs a processor that promotes attributes from descendant spans onto the **local root** (service-entrypoint span) of each trace. Motivating use case: feature-flag context enrichment — attributes like `dt.feature_flag.result.<name>` are set on child spans during a request, but OpenPipeline's RED-metric generation requires them on the entrypoint span at ingest time.

This is ICP-7891, a research spike under VI PRODUCT-16403. The design chose "Option 1: Collector Processor" — a collector-side buffering approach — over SDK-side alternatives that are being pursued in parallel via VI PRODUCT-18515.

**Definitions (from `local/ICP-7891-feasibility.md`):**
- **Local root span**: a span whose parent context is invalid (no parent) *or* remote (context propagated from another application). Verified across the Java, PHP, and X-Ray SDK codebases.
- **Detection algorithm** (see `local/ICP-7891-feasibility.md` § Local-root detection for full rationale):
  ```
  isLocalRoot(span, index):
      if span.ParentSpanID().IsEmpty(): return true
      if span.Flags() & 0x100 != 0:  // SPAN_FLAGS_CONTEXT_HAS_IS_REMOTE_MASK
          return span.Flags() & 0x200 != 0  // SPAN_FLAGS_CONTEXT_IS_REMOTE_MASK
      parent, ok := index[span.ParentSpanID()]
      if !ok: return true                                              // parent not visible; safe default
      return serviceIdentity(span.resource) != serviceIdentity(parent.resource)
  ```
  Where `serviceIdentity(resource)` returns a stable key: `(service.name, service.instance.id)` when both present, `service.name` alone when instance.id absent, or a full-Resource hash if `service.name` is missing.

**Prior design docs (all in `local/`):**
- `ICP-7891-feasibility.md` — full feasibility write-up. Contains the Go/pdata sketch of the lazy-resolution algorithm and scope-item analysis of buffering, latency, and load-balancing implications.
- `ICP-7891-buffering-strategy-comparison.md` — comparison of lazy vs. optimistic strategies for grouping spans by local root. Decision: lazy.
- `ICP-7891-ottl-lambda-analysis.md` — analysis of doing enrichment via OTTL lambdas. Decision: **not for this prototype.** The prototype does enrichment inline in the processor. OTTL-based extensibility is deferred; the buffering processor is useful on its own.

## Design decisions (final, locked in)

| Decision | Value | Rationale |
|---|---|---|
| Component name (config type) | `entrypoint_enrichment` | Follows collector convention (drops `_processor` suffix). VI language emphasizes "entrypoint" — the config key matches. |
| Go module / package | `entrypointenrichmentprocessor` | Standard Go package name |
| Location | `internal/processor/entrypointenrichmentprocessor/` in this repo | DT-local for the prototype; own Go module with a `replaces` entry in the top-level `manifest.yaml`. Same pattern as `internal/confmap/provider/eecprovider`. |
| Buffering strategy | Lazy resolution (see `ICP-7891-buffering-strategy-comparison.md`) | Simpler than optimistic; O(1) ingest; correctness invariants easier to hold |
| Attribute matching | `attributes_to_promote`: list of regex patterns | More flexible than prefix-only |
| Promotion target | Every SERVER/CONSUMER span in the emitted subtree (not just the local root) | Nested SERVER spans within one service (ingress + app) all need FF attributes for RED-metric generation |
| Local-root detection | Flags-first (`HAS_IS_REMOTE`/`IS_REMOTE`); service-identity comparison against parent as fallback (`service.name`/`service.instance.id`); missing-parent defaults to local root | Aligns with the [otel-subtrace-demo](https://github.com/davidHaunschmied/otel-subtrace-demo) boundary heuristic; more precise than kind-based fallback |
| Conflict handling | First-wins per target (target keeps existing value; among descendants, whichever arrives first in the flush walk wins) | Deterministic given traversal, simple to implement |
| Source retention | Source (descendant) span keeps the attribute | Spans are read-only from downstream consumers' perspective |
| Local-root marker attribute | Configurable via `local_root_marker_attribute` (string); default `dt.local_root`; empty disables. Stamped value: `true` (bool). Stamped on the local root only — not on other SERVER/CONSUMER promotion targets | Debug aid only; keeps the marker's "buffering boundary" meaning clear |
| Resource/Scope grouping at emit | Coalesce by structural equality (attribute-map serialization + schema URL) | Prototype-appropriate correctness; performance tuning deferred |
| Concurrency | Single mutex around all buffer state | Simple correctness for prototype. Follow-up: adopt `groupbytraceprocessor`'s `EventMachine` pattern for per-worker serialization |
| DT distribution wire-up | Add to `manifest.yaml` with a `replaces` entry | Same pattern as `eecprovider` |
| E2E test | Programmatic (Go generator + mock exporter) with option to point at an external endpoint | Ticket's deliverable calls for "end-to-end" |

## Repo layout to create

```
internal/
└── processor/
    └── entrypointenrichmentprocessor/
        ├── go.mod
        ├── go.sum
        ├── Makefile                        # standard collector component makefile
        ├── README.md                       # component docs (see Phase 10)
        ├── doc.go                          # package doc
        ├── factory.go                      # component factory (NewFactory)
        ├── factory_test.go
        ├── config.go                       # Config struct + Validate()
        ├── config_test.go
        ├── processor.go                    # Traces processor implementation
        ├── processor_test.go
        ├── buffer.go                       # Lazy-resolution buffer
        ├── buffer_test.go
        ├── localroot.go                    # isLocalRoot + mode enum
        ├── localroot_test.go
        ├── promote.go                      # Attribute-promotion pass + assemble()
        └── promote_test.go
```

The Go module path: `github.com/Dynatrace/dynatrace-otel-collector/internal/processor/entrypointenrichmentprocessor`.

## Component behavior specification

### Config schema

```yaml
processors:
  entrypoint_enrichment:
    # Per-subtree wait timer duration (starts when the local root arrives).
    # Trades enrichment completeness against added metrics-pipeline latency.
    # See feasibility doc scope item 4 for the tradeoff framing.
    wait_duration: 500ms

    # Trace-level fallback timer for spans whose local root never arrives.
    # Should be materially longer than wait_duration.
    fallback_duration: 5s

    # Upper bound on the number of in-flight traces buffered.
    num_traces: 1000000

    # Regex patterns for attribute keys to promote from any descendant onto
    # every SERVER/CONSUMER span in the emitted subtree. First-wins: if a
    # target already has the attribute, its value is preserved.
    attributes_to_promote:
      - "^dt\\.feature_flag\\.result\\..+$"

    # If non-empty, stamp this attribute with value `true` on the local root
    # (as identified by the detection algorithm). Debug aid only — the
    # marker is NOT stamped on other SERVER/CONSUMER spans in the subtree.
    # Set to "" to disable.
    local_root_marker_attribute: "dt.local_root"
```

There is no `local_root_detection` mode knob. The algorithm is a single well-defined path: flags first, service-identity fallback, missing-parent-defaults-to-local-root.

### Validation rules (implement in `Config.Validate()`)

- `wait_duration >= 0`. If zero, warn (behavior degenerates to a single-batch pass-through).
- `fallback_duration >= wait_duration`. If not, error.
- `num_traces > 0`.
- Every regex in `attributes_to_promote` must compile; compiled results cached on the Config.
- `local_root_marker_attribute` may be empty; otherwise treat as an attribute key (no validation beyond string).

### Data flow

1. Incoming `ptrace.Traces` batch arrives at `ConsumeTraces(ctx, td)`.
2. For each span in the batch, `buffer.Insert(resource, scope, span)`:
   - Copy the span (and its `pcommon.Resource` and `pcommon.InstrumentationScope`) into a `bufferedSpan` and store in the span index.
   - Start the per-trace fallback timer if not already running.
   - If the span is a local root, start its per-subtree wait timer.
3. On subtree-timer fire (`flushSubtree`):
   - Collect all buffered spans whose ancestry reaches the local root (via the `reaches` walk).
   - Run the promotion pass over those spans: for each member, for each attribute key matching any `attributes_to_promote` regex, first-wins-insert onto every SERVER/CONSUMER span in the subtree.
   - If `local_root_marker_attribute` is set, stamp `<marker>=true` on the local root.
   - Reassemble into a `ptrace.Traces` via `assemble` (structural coalescing by Resource+Scope).
   - Call `next.ConsumeTraces(ctx, assembled)`.
   - Remove those spans from the index; if the trace's index is now empty, cancel the fallback timer and delete the trace entry.
4. On fallback-timer fire (`flushTrace`):
   - Emit whatever spans remain in the trace's index as-is (no promotion — local root either never arrived or already flushed).
   - Cascade-delete all state for that traceID including any lingering subtree timers.
5. On `Shutdown(ctx)`:
   - Stop all timers.
   - For every remaining trace in the buffer: call `flushTrace` semantics to drain.

### Local-root detection

```go
package entrypointenrichmentprocessor

import (
    "go.opentelemetry.io/collector/pdata/pcommon"
    "go.opentelemetry.io/collector/pdata/ptrace"
)

// OTLP Span.flags bits (see opentelemetry-proto trace.proto:347-356).
// Constants are only in pdata's internal protogen package, so we redeclare here.
const (
    spanFlagsContextHasIsRemoteMask uint32 = 0x00000100
    spanFlagsContextIsRemoteMask    uint32 = 0x00000200
)

// isLocalRoot decides whether span sits at a local-root (service-entrypoint)
// boundary. It uses the OTLP `HAS_IS_REMOTE`/`IS_REMOTE` flag bits when the SDK
// populates them; otherwise it compares service identity against the parent's
// resource. index is the current buffer's span map for this trace; used to
// look up the parent.
func isLocalRoot(span ptrace.Span, index map[pcommon.SpanID]bufferedSpan) bool {
    if span.ParentSpanID().IsEmpty() {
        return true
    }
    f := span.Flags()
    if f&spanFlagsContextHasIsRemoteMask != 0 {
        return f&spanFlagsContextIsRemoteMask != 0
    }
    parent, ok := index[span.ParentSpanID()]
    if !ok {
        // Parent not visible in the buffer. Safe default: treat as local root.
        // See feasibility doc's "known limitation" note.
        return true
    }
    return serviceIdentity(span.resource) != serviceIdentity(parent.resource)  // pseudocode: span.resource must be accessible via the surrounding bufferedSpan; adjust the signature to take the current bufferedSpan or accept a Resource arg.
}

// serviceIdentity returns a stable string key for a resource's service.
// Preference order:
//   1. "<service.name>|<service.instance.id>" if both present
//   2. "<service.name>" if only name present
//   3. hash of the full resource attributes as a best-effort fallback
func serviceIdentity(r pcommon.Resource) string {
    name, hasName := r.Attributes().Get("service.name")
    inst, hasInst := r.Attributes().Get("service.instance.id")
    switch {
    case hasName && hasInst:
        return name.AsString() + "|" + inst.AsString()
    case hasName:
        return name.AsString()
    default:
        return hashResourceAttrs(r)
    }
}
```

Note on signature: `isLocalRoot` needs access to the current span's resource *and* the parent's resource. In practice the buffer stores `bufferedSpan{resource, scope, span}`, so pass the whole `bufferedSpan` or an equivalent tuple rather than just `ptrace.Span`. Choose one; keep it consistent between `Insert` and `reaches`.

### Buffer core (lazy resolution)

```go
package entrypointenrichmentprocessor

import (
    "context"
    "sync"
    "time"

    "go.opentelemetry.io/collector/consumer"
    "go.opentelemetry.io/collector/pdata/pcommon"
    "go.opentelemetry.io/collector/pdata/ptrace"
)

type bufferedSpan struct {
    resource pcommon.Resource
    scope    pcommon.InstrumentationScope
    span     ptrace.Span
}

type Buffer struct {
    mu             sync.Mutex
    spans          map[pcommon.TraceID]map[pcommon.SpanID]bufferedSpan
    subtreeTimers  map[pcommon.TraceID]map[pcommon.SpanID]*time.Timer
    traceTimers    map[pcommon.TraceID]*time.Timer

    cfg  *Config          // holds compiled regex patterns and marker attribute name
    next consumer.Traces
}

// Insert stores one span into the buffer. Called for every span in every
// incoming batch, ideally without holding the mutex across the whole batch:
// callers should iterate the batch and call Insert per span.
func (b *Buffer) Insert(res pcommon.Resource, scope pcommon.InstrumentationScope, span ptrace.Span) {
    tid, sid := span.TraceID(), span.SpanID()

    // Copy the resource, scope, and span so we own the storage.
    // See assemble() for how these are recombined.
    rCopy := pcommon.NewResource()
    res.CopyTo(rCopy)
    sCopy := pcommon.NewInstrumentationScope()
    scope.CopyTo(sCopy)
    spCopy := ptrace.NewSpan()
    span.CopyTo(spCopy)

    b.mu.Lock()
    defer b.mu.Unlock()

    if b.spans[tid] == nil {
        b.spans[tid] = make(map[pcommon.SpanID]bufferedSpan)
    }
    b.spans[tid][sid] = bufferedSpan{resource: rCopy, scope: sCopy, span: spCopy}

    if _, ok := b.traceTimers[tid]; !ok {
        b.traceTimers[tid] = time.AfterFunc(b.cfg.FallbackDuration, func() {
            b.flushTrace(tid)
        })
    }

    if isLocalRoot(spCopy, b.spans[tid]) {
        if b.subtreeTimers[tid] == nil {
            b.subtreeTimers[tid] = make(map[pcommon.SpanID]*time.Timer)
        }
        if _, ok := b.subtreeTimers[tid][sid]; !ok {
            b.subtreeTimers[tid][sid] = time.AfterFunc(b.cfg.WaitDuration, func() {
                b.flushSubtree(tid, sid)
            })
        }
    }
}

func (b *Buffer) flushSubtree(tid pcommon.TraceID, rootID pcommon.SpanID) {
    b.mu.Lock()
    index := b.spans[tid]
    if index == nil {
        b.mu.Unlock()
        return
    }
    delete(b.subtreeTimers[tid], rootID)

    var members []bufferedSpan
    for sid, s := range index {
        if reaches(sid, rootID, index) {
            members = append(members, s)
            delete(index, sid)
        }
    }
    if len(index) == 0 {
        delete(b.spans, tid)
        if t, ok := b.traceTimers[tid]; ok {
            t.Stop()
            delete(b.traceTimers, tid)
        }
        delete(b.subtreeTimers, tid)
    }
    b.mu.Unlock()

    if len(members) == 0 {
        return
    }

    promote(members, rootID, b.cfg)
    traces := assemble(members)
    _ = b.next.ConsumeTraces(context.Background(), traces)
}

// reaches walks parent pointers from spanID up through the buffered index.
// Returns true iff we hit target before hitting a *different* local root
// (subtree boundary) or running off the chain (parent not in buffer).
func reaches(spanID, target pcommon.SpanID, index map[pcommon.SpanID]bufferedSpan) bool {
    cur, ok := index[spanID]
    if !ok {
        return false
    }
    for {
        if cur.span.SpanID() == target {
            return true
        }
        pid := cur.span.ParentSpanID()
        if pid.IsEmpty() {
            return false
        }
        parent, ok := index[pid]
        if !ok {
            return false
        }
        if isLocalRoot(parent.span, index) && parent.span.SpanID() != target {
            return false // hit another subtree's root
        }
        cur = parent
    }
}

func (b *Buffer) flushTrace(tid pcommon.TraceID) {
    b.mu.Lock()
    index := b.spans[tid]
    delete(b.spans, tid)
    for _, t := range b.subtreeTimers[tid] {
        t.Stop()
    }
    delete(b.subtreeTimers, tid)
    delete(b.traceTimers, tid)
    b.mu.Unlock()

    if len(index) == 0 {
        return
    }
    members := make([]bufferedSpan, 0, len(index))
    for _, s := range index {
        members = append(members, s)
    }
    // No promotion pass — no local root available.
    traces := assemble(members)
    _ = b.next.ConsumeTraces(context.Background(), traces)
}

// Shutdown stops all timers and flushes remaining traces.
func (b *Buffer) Shutdown(ctx context.Context) error {
    b.mu.Lock()
    tids := make([]pcommon.TraceID, 0, len(b.spans))
    for tid := range b.spans {
        tids = append(tids, tid)
    }
    b.mu.Unlock()
    for _, tid := range tids {
        b.flushTrace(tid)
    }
    return nil
}
```

Enforce `num_traces` in `Insert`: if `len(b.spans) >= b.cfg.NumTraces` and the incoming trace is new, evict the oldest trace (LRU or arbitrary — arbitrary is fine for prototype; record a metric later). Suggested impl: track insertion order via a slice, evict head. Do not overthink for the prototype.

### Promotion + reassembly

```go
package entrypointenrichmentprocessor

import (
    "regexp"

    "go.opentelemetry.io/collector/pdata/pcommon"
    "go.opentelemetry.io/collector/pdata/ptrace"
)

// promote copies matching attributes from any span in the emitted subtree
// onto every SERVER/CONSUMER span in the subtree, using first-wins semantics.
// The local root (identified by rootID) additionally receives the marker
// attribute if configured. Multiple SERVER/CONSUMER spans (e.g., ingress
// SERVER + app SERVER within one service) each get their own copy.
func promote(members []bufferedSpan, rootID pcommon.SpanID, cfg *Config) {
    // Collect target spans (kind == SERVER || CONSUMER) as pcommon.Map refs to
    // their attributes.
    var targets []pcommon.Map
    for _, m := range members {
        k := m.span.Kind()
        if k == ptrace.SpanKindServer || k == ptrace.SpanKindConsumer {
            targets = append(targets, m.span.Attributes())
        }
    }

    // Promote matching attributes from any span onto each target.
    // First-wins per (target, key): once a target already has a matched key,
    // subsequent same-key values from other spans are skipped for that target.
    for _, m := range members {
        m.span.Attributes().Range(func(k string, v pcommon.Value) bool {
            if !matchesAny(k, cfg.compiledPatterns) {
                return true
            }
            for _, tAttrs := range targets {
                if _, exists := tAttrs.Get(k); exists {
                    continue // first-wins for this target
                }
                v.CopyTo(tAttrs.PutEmpty(k))
            }
            return true
        })
    }

    // Marker attribute goes on the local root only (not on other targets).
    if cfg.LocalRootMarkerAttribute != "" {
        for _, m := range members {
            if m.span.SpanID() == rootID {
                m.span.Attributes().PutBool(cfg.LocalRootMarkerAttribute, true)
                break
            }
        }
    }
}

func matchesAny(s string, patterns []*regexp.Regexp) bool {
    for _, p := range patterns {
        if p.MatchString(s) {
            return true
        }
    }
    return false
}

// assemble reconstructs a ptrace.Traces from bufferedSpans, coalescing
// by (Resource, Scope) via structural equality on attributes + schema URL.
func assemble(members []bufferedSpan) ptrace.Traces {
    traces := ptrace.NewTraces()

    // Key each Resource by a stable hash of its attributes.
    // Simplest: use the pcommon.Map's rendered form.
    type scopeGroup struct {
        rs   ptrace.ResourceSpans
        scopes map[string]ptrace.ScopeSpans
    }
    resourceGroups := map[string]*scopeGroup{}

    for _, m := range members {
        rKey := hashResource(m.resource)
        rg, ok := resourceGroups[rKey]
        if !ok {
            rs := traces.ResourceSpans().AppendEmpty()
            m.resource.CopyTo(rs.Resource())
            rg = &scopeGroup{rs: rs, scopes: map[string]ptrace.ScopeSpans{}}
            resourceGroups[rKey] = rg
        }

        sKey := hashScope(m.scope)
        ss, ok := rg.scopes[sKey]
        if !ok {
            ss = rg.rs.ScopeSpans().AppendEmpty()
            m.scope.CopyTo(ss.Scope())
            rg.scopes[sKey] = ss
        }

        m.span.CopyTo(ss.Spans().AppendEmpty())
    }

    return traces
}

// hashResource and hashScope produce stable keys.
// For the prototype: sort attribute keys and concatenate `k=v`; append
// schema URL. Not cryptographic; identity-only. Optimize later.
func hashResource(r pcommon.Resource) string { /* ... */ }
func hashScope(s pcommon.InstrumentationScope) string { /* ... */ }
```

## Implementation phases

Each phase has a concrete acceptance criterion. Do not move to the next phase until the current one meets it.

### Phase 1 — Scaffold

**Steps:**
1. Create the directory tree at `internal/processor/entrypointenrichmentprocessor/`.
2. Write `go.mod` with module path `github.com/Dynatrace/dynatrace-otel-collector/internal/processor/entrypointenrichmentprocessor`. Reference: `internal/testbed/go.mod` and `internal/confmap/provider/eecprovider/go.mod` for the dependency set (`go.opentelemetry.io/collector/component`, `consumer`, `pdata`, `processor`, `processortest`).
3. Write `doc.go` — package doc comment only.
4. Declare the component type as a stable `component.Type` value in the package (e.g., `var Type = component.MustNewType("entrypoint_enrichment")`), referenced by `factory.go`.
5. Write minimal `factory.go` (`NewFactory` returning a processor factory with a trace consumer via `processor.WithTraces`), empty `config.go` (`Config` struct), empty `processor.go` (`newProcessor` returning a struct that implements `processor.Traces`).
6. Write a small `factory_test.go` asserting `NewFactory()` returns a non-nil factory whose `Type()` matches the declared type and whose default config is non-nil.
7. Ensure `go test ./...` passes.

**Acceptance:** `go test ./...` inside the new module directory passes. `NewFactory()` returns a valid factory.

### Phase 2 — Config + validation

**Steps:**
1. Flesh out `Config` with all fields from the schema above. Use `time.Duration` for durations, `int` for `num_traces`, `[]string` for `attributes_to_promote`, and a private `compiledPatterns []*regexp.Regexp` field populated during `Validate`.
2. Implement `Validate() error` per the validation rules above.
3. Write `config_test.go`: table-driven tests covering every validation rule, plus a "happy path" with realistic values.

**Acceptance:** `go test -run Config ./...` passes with 100% branch coverage of `Validate`.

### Phase 3 — Local-root detection

**Steps:**
1. Write `localroot.go` with `isLocalRoot(span, index)` and `serviceIdentity(resource)` as shown above.
2. Declare `spanFlagsContextHasIsRemoteMask` and `spanFlagsContextIsRemoteMask` constants.
3. Write `localroot_test.go` covering the truth table below. Setup: each row describes the current span and (when relevant) the parent span buffered under the same trace. `Parent in index?` = whether the parent SpanID is present in the buffer map at test time.

| # | ParentSpanID empty? | HAS_IS_REMOTE | IS_REMOTE | Parent in index? | Parent service.name | Parent service.instance.id | Child service.name | Child service.instance.id | Expected |
|---|---|---|---|---|---|---|---|---|---|
| 1 | Yes | any | any | n/a | n/a | n/a | any | any | **true** (empty parent) |
| 2 | No  | 1   | 1   | any    | any | any | any | any | **true** (flags authoritative — remote) |
| 3 | No  | 1   | 0   | Yes    | s   | i   | s   | i   | **false** (flags authoritative — local) |
| 4 | No  | 1   | 0   | No     | n/a | n/a | any | any | **false** (flags authoritative — local, parent not needed) |
| 5 | No  | 0   | any | Yes    | svcA | any | svcA | any | **false** (same service.name; instance.id ignored when both share name — actually see rule 6) |
| 6 | No  | 0   | any | Yes    | svcA | i1  | svcA | i1  | **false** (same name + same instance) |
| 7 | No  | 0   | any | Yes    | svcA | i1  | svcA | i2  | **true** (same name, different instance) |
| 8 | No  | 0   | any | Yes    | svcA | any | svcB | any | **true** (different name) |
| 9 | No  | 0   | any | Yes    | (absent) | any | (absent) | any | Depends on fallback hash — use identical resources ⇒ **false**; different attributes ⇒ **true** |
| 10 | No | 0   | any | No     | n/a | n/a | any | any | **true** (safe default) |

Row 5 nuance: with both spans having only `service.name` set (no instance.id), `serviceIdentity` returns the name alone. Add a row exercising this: parent-only-name = child-only-name ⇒ **false**; parent-only-name ≠ child-only-name ⇒ **true**.

**Acceptance:** all rows pass. Table-driven test. `isEntryKind` is no longer needed and should not appear in the code or tests.

### Phase 4 — Buffer core

**Steps:**
1. Write `buffer.go` per the code above. Include `Insert`, `flushSubtree`, `flushTrace`, `reaches`, `Shutdown`.
2. Add `num_traces` eviction in `Insert` (simplest: track insertion order in a slice; evict head when at capacity).
3. Write `buffer_test.go` with the following cases:
   - **Happy path**: single incoming batch containing a local root + descendants; wait_duration elapses; assert one output batch with the correct spans.
   - **Local root arrives late**: children first, then local root; assert timer only starts on root arrival; assert flush includes everything.
   - **Local root never arrives**: only children; assert fallback timer fires; assert flush emits the children unpromoted.
   - **Two local roots in one trace, same batch** (multi-service scenario): assert two separate output batches, one per subtree; assert descendants attach to the correct root via the `reaches` boundary check.
   - **Late child** (arrives after subtree flush): child sits under trace, gets emitted by fallback.
   - **Ring-buffer eviction**: fill to `num_traces`; assert oldest is evicted on new insert.
   - **Shutdown drains buffer**: shut down with in-flight state; assert all remaining spans are emitted.

Use a mockable clock or short durations (few ms) in tests to keep them fast.

**Acceptance:** All cases pass. Race-detector run (`go test -race`) passes.

### Phase 5 — Reassembly + promotion

**Steps:**
1. Write `promote.go` with `promote(members, rootID, cfg)`, `assemble(members)`, `hashResource`, `hashScope`.
2. Implement Resource/Scope coalescing:
   - `hashResource`: serialize the resource's attributes as a stable string (sorted keys, `k=v` concatenation) + schema URL.
   - `hashScope`: name + version + schema URL + hashed attributes.
   - Group members by these hashes as shown.
3. Implement `promote`:
   - Collect target spans: every span in `members` whose kind is SERVER or CONSUMER. Keep them as `pcommon.Map` references to their attributes.
   - For each span in `members`, iterate keys; if any pattern matches, copy the value into every target that doesn't already have that key (first-wins per target).
   - Stamp the marker attribute on the local root (identified by `rootID`) only, if configured.
4. Write `promote_test.go`:
   - **Single target — local root only**: subtree with one SERVER local root plus INTERNAL descendants; one descendant has a matching attribute → attribute lands on the SERVER root; marker on the root.
   - **Multiple targets — SERVER→SERVER**: subtree with an outer SERVER local root and a nested SERVER child (same service; typical for ingress + app) plus INTERNAL descendants carrying matching attributes → **both** SERVER spans receive each matching attribute; marker on the local root only.
   - **SERVER→SERVER→SERVER amplification**: three nested SERVER spans, matching attribute on a leaf descendant → all three SERVER spans receive it.
   - **CONSUMER target**: subtree with a CONSUMER local root and INTERNAL descendants → CONSUMER receives promoted attribute.
   - **Multiple matches, same key across descendants**: assert first-wins per target (once a target has the key, subsequent same-key values on other members are skipped for that target). Document the iteration order.
   - **Conflict with a target's existing attribute**: target span already has the key → target's value preserved.
   - **No matches**: no attributes promoted; marker still stamped on the local root.
   - **Multiple patterns, complex regex**: attributes matching different patterns land as expected on every target.
   - **Marker attribute disabled**: `local_root_marker_attribute: ""` → marker not stamped on any span.
   - **No SERVER/CONSUMER in subtree** (edge case): no promotion happens; batch still emits with source attributes intact.
   - **Assembly coalescing**: two spans with the same resource and scope → one `ResourceSpans` with one `ScopeSpans` containing both spans. Two spans with same resource, different scopes → one `ResourceSpans` with two `ScopeSpans`. Two spans with different resources → two `ResourceSpans`.

**Acceptance:** All cases pass.

### Phase 6 — Consumer plumbing

**Steps:**
1. Fill in `processor.go`:
   - `newProcessor(cfg, set, next)` constructs the Buffer.
   - `Start(ctx, host) error` — no-op (or clock initialization).
   - `Shutdown(ctx) error` — delegate to Buffer.
   - `Capabilities() consumer.Capabilities` — return `{MutatesData: true}` (we do mutate — we add attributes to SERVER/CONSUMER spans).
   - `ConsumeTraces(ctx, td)` — iterate ResourceSpans, ScopeSpans, Spans; call `buffer.Insert` for each.
2. Wire the factory in `factory.go`:
   - `createDefaultConfig() component.Config` — returns sensible defaults matching the schema example.
   - `createTracesProcessor(ctx, set, cfg, next)` — builds the processor.
3. Write `processor_test.go` — smoke: construct via factory, feed one batch, wait for flush, assert the mock consumer received a batch with promoted attributes.

**Acceptance:** Component instantiates and runs end-to-end (in tests) without errors.

### Phase 7 — Integration test

**Steps:**
1. Write `processor_integration_test.go` using `consumertest.TracesSink` as the mock consumer.
2. Cases:
   - Realistic FF-enrichment scenario: entrypoint SERVER span + several INTERNAL descendant spans, one carrying `dt.feature_flag.result.<flag>=<value>`; assert the emitted local-root batch has the promoted attribute.
   - Batch with spans out of order (children arriving before parent, parent last).
   - Cross-batch scenario: same trace's spans split across two `ConsumeTraces` calls; assert they get grouped together.
   - Trace with two local roots: two emissions, correct partition.
3. Use short `wait_duration` (10-50ms) in tests; block-await on the sink's span count using `require.Eventually`.

**Acceptance:** All cases pass reliably (no flakes) across `go test -count=100 -race`.

### Phase 8 — E2E test

**Approach:** a single test case that mirrors the pattern used in `internal/testbed/integration/config_examples_test.go`. Spawn the compiled DT collector binary as a child process using `testbed.NewChildProcessCollector`, feed it a distributed-trace `ptrace.Traces` built with pdata that simulates multiple services calling each other with feature-flag metadata scattered across their descendant spans, capture what the collector emits via a `testbed` receiver sink, and assert that each service's local root received the promoted attributes.

**Location:** `internal/testbed/integration/entrypointenrichment/e2e_test.go` (create the directory alongside sibling ones like `genainormalizer/`, `k8scombined/`).

**Steps:**

1. **Set up the test skeleton** following `TestConfigTailSampling` in `internal/testbed/integration/config_examples_test.go`:
   - `testbed.NewChildProcessCollector(testbed.WithAgentExePath(testutil.CollectorTestsExecPath))`.
   - Get free ports via `testutil.GetAvailablePort(t)` for the OTLP receiver and for the OTLP exporter destination (the test's own sink).
   - Prepare a collector config string with `otlp` receiver, `entrypoint_enrichment` processor (short `wait_duration` — e.g., 100 ms — for test speed), and `otlp` exporter pointing at the sink port.
   - Use `testutil.ReplaceOtlpGrpcReceiverPort` and equivalent helpers to inject ports into the config.
   - `col.PrepareConfig(t, cfg)` + start.

2. **Wire the receiver sink** for what the collector exports:
   - Use `oteltest.StartUpSinks` (see `internal/testcommon/oteltest/sinks.go:80`) with a `TraceSinkConfig` bound to a `consumertest.TracesSink`, listening on the exporter-port.
   - Alternatively use `testbed`-style sinks if `oteltest` doesn't compose here — the existing `TestConfigTailSampling` shows the receiver-sink shape (`MockBackend`).

3. **Build the distributed-trace test payload with pdata.** The scenario is a single trace crossing three services (A → B → C):
   ```
   Service A                Service B              Service C
     [SERVER root]  →         [SERVER root]  →       [SERVER root]
     ├─ [INTERNAL]            ├─ [INTERNAL]          ├─ [INTERNAL]
     ├─ [INTERNAL: FF!]       ├─ [INTERNAL: FF!]     ├─ [INTERNAL]
     └─ [CLIENT hop → B]      ├─ [INTERNAL: FF!]     └─ [INTERNAL: FF!]
                              └─ [CLIENT hop → C]
   ```
   All spans share a single `TraceID`. Each service is a distinct `ResourceSpans` with its own `service.name` resource attribute (`service-a`, `service-b`, `service-c`).
   - **Service A's local root**: `SpanKind = SERVER`, `ParentSpanID` empty (global entry). `Flags`: leave default 0 — the empty ParentSpanID means `isLocalRoot` returns true via the first branch.
   - **Service B's local root**: `SpanKind = SERVER`, `ParentSpanID` = A's client-hop child's `SpanID`. `Flags = 0x300` (`HAS_IS_REMOTE | IS_REMOTE`).
   - **Service C's local root**: `SpanKind = SERVER`, `ParentSpanID` = one of B's spans. `Flags = 0x300`.
   - **All INTERNAL descendants**: `SpanKind = INTERNAL`, `ParentSpanID` points inside the same service. `Flags = 0x100` (`HAS_IS_REMOTE`, IS_REMOTE unset — i.e., local child).
   - **Feature-flag attributes** on selected INTERNAL descendants:
     - Service A: `dt.feature_flag.result.pricing_v2 = "on"` on one descendant.
     - Service B: `dt.feature_flag.result.checkout_flow = "variant_b"` and `dt.feature_flag.result.recs_algo = "collab"` on two different descendants.
     - Service C: `dt.feature_flag.result.shipping_calc = "fast"` on one descendant.

4. **Send the payload** into the collector's OTLP receiver via `ptraceotlp.NewGRPCClient` (see the pattern in `genainormalizer/e2e_test.go:33-31` for the client construction).

5. **Wait for and inspect the sink output.** Use `require.Eventually` waiting for `tracesSink.SpanCount()` to reach the expected total (all spans of all three subtrees).

6. **Assertions** — one for each service:
   - Locate the emitted local root by matching `service.name` on the resource and the `SERVER` kind on the span.
   - Assert the local root has:
     - The `dt.local_root=true` marker attribute (bool).
     - Each `dt.feature_flag.result.*` attribute that was originally on any of that service's descendants.
   - Assert the descendant spans emitted alongside still carry their original feature-flag attributes (source retention).
   - Assert no cross-service contamination: service A's local root does not have service B's feature-flag attributes.

7. **Emitted-batch structure.** Because the buffering processor emits one `ptrace.Traces` per local-root subtree, the sink will accumulate three batches. Verify that the total span count matches the input and that each subtree is grouped under a single `ResourceSpans` in its emitted batch.

**Developer tie-in to Dynatrace.** Provide a helper function that, when called, adds a `otlp_http` exporter targeting the Dynatrace endpoint to the collector's exporter list and appends it to the traces pipeline. Read `DT_ENDPOINT` and `DT_API_TOKEN` from the environment. Config shape follows `config_examples/masking_api_token.yaml`:

```go
// applyDynatraceExporter injects an otlp_http exporter into the collector
// config string, authenticated via the DT_API_TOKEN env var and targeted at
// DT_ENDPOINT. Adds the exporter to the traces pipeline. Callers can uncomment
// the invocation in the test to also send data to Dynatrace during a run.
func applyDynatraceExporter(cfg string) (string, error) {
    endpoint := os.Getenv("DT_ENDPOINT")
    token := os.Getenv("DT_API_TOKEN")
    if endpoint == "" || token == "" {
        return "", fmt.Errorf("DT_ENDPOINT and DT_API_TOKEN must both be set")
    }
    // Append the otlp_http exporter definition. See config_examples/masking_api_token.yaml
    // for the auth pattern: Authorization: "Api-Token ${env:DT_API_TOKEN}".
    // Add "dynatrace" to the traces pipeline's exporters list.
    // Implementation: YAML-parse the cfg, mutate exporters and service.pipelines.traces.exporters,
    // marshal back. Keep it simple; a string-append with a heredoc is fine for the prototype.
    ...
}
```

Invocation in the test — commented out by default:

```go
// Uncomment to also send the trace to Dynatrace during the test run.
// Requires DT_ENDPOINT and DT_API_TOKEN env vars.
// cfg, err = applyDynatraceExporter(cfg)
// require.NoError(t, err)
```

**Acceptance:**
- Test passes reliably (no flakes) across `go test -count=20 -race ./internal/testbed/integration/entrypointenrichment/...`.
- The test compiles and runs without any env vars set — the Dynatrace hook is opt-in.
- When `DT_ENDPOINT` + `DT_API_TOKEN` are set and the invocation is uncommented, spans arrive at Dynatrace (visual/manual verification only).

### Phase 9 — Distribution wire-up

**Steps:**
1. Edit `manifest.yaml` at the repo root:
   - Add to the `processors:` list:
     ```yaml
     - gomod: github.com/Dynatrace/dynatrace-otel-collector/internal/processor/entrypointenrichmentprocessor v0.0.0
     ```
   - Add to the `replaces:` list:
     ```yaml
     - github.com/Dynatrace/dynatrace-otel-collector/internal/processor/entrypointenrichmentprocessor => ../internal/processor/entrypointenrichmentprocessor
     ```
2. Rebuild the distribution locally (per repo `Makefile` conventions; look at the top-level Makefile for the `build` target). Confirm the resulting binary starts with a config that references the new processor.
3. Provide an example config file under `config/examples/` (if that directory exists in the repo — check first) or under `local/` demonstrating the FF-enrichment pipeline.

**Acceptance:** DT collector builds cleanly. Sample config validates and starts. Manual smoke: send OTLP spans via `telemetrygen` or the E2E generator; observe promoted attributes in the debug exporter output.

### Phase 10 — README

**Steps:**
1. Write `README.md` following the pattern of an existing contrib processor (see `groupbytraceprocessor/README.md` for structure). Sections:
   - Description of what the processor does.
   - Config reference table with defaults.
   - Behavior notes: local-root detection, buffering strategy, first-wins conflict semantics, source retention.
   - **Deployment constraints**: warn about the routing requirement — spans of one local-root subtree must reach the same collector instance. Cross-link `local/ICP-7891-feasibility.md` scope item 5 for details.
   - Latency tradeoff notes referencing `wait_duration` — cross-link `local/ICP-7891-feasibility.md` scope item 4.
   - Example config.

**Acceptance:** README is complete and self-explanatory to a first-time user.

## Test matrix summary

| Phase | Test file | Coverage target |
|---|---|---|
| 2 | `config_test.go` | Validation rules, defaults, regex compile |
| 3 | `localroot_test.go` | Truth table across all modes |
| 4 | `buffer_test.go` | Buffer algorithm (happy path, edge cases, race-safe, shutdown drain) |
| 5 | `promote_test.go` | Promotion (conflict, no-match, marker) + assembly (coalescing) |
| 6 | `processor_test.go` | Factory smoke + one-batch integration |
| 7 | `processor_integration_test.go` | Realistic scenarios, cross-batch, multi-root |
| 8 | `internal/testbed/integration/entrypointenrichment/e2e_test.go` | Multi-service distributed trace through the built collector via OTLP; per-service local-root promotion asserted |

## Reference material

**Inside this repo:**
- `manifest.yaml` — DT builder manifest. Example of `replaces` for local modules: see the `eecprovider` entry (line 67 and line 70).
- `internal/confmap/provider/eecprovider/` — closest template for a locally-built module wired via `manifest.yaml`. Copy `go.mod` structure, `Makefile` conventions.
- `internal/testbed/go.mod` — example of dependencies (`consumertest`, `pdatatest`, `golden`) that will likely be needed for integration + E2E tests.
- `internal/tools/go.mod` — location of dev tools shared across the repo (linters, etc.).
- `local/ICP-7891-feasibility.md` — the "why" doc. Scope items 3–5 are the design rationale for buffering, latency, and routing.
- `local/ICP-7891-buffering-strategy-comparison.md` — full argument for lazy resolution over optimistic. Also contains the "invariants" table useful for buffer_test.go.
- `local/ICP-7891-ottl-lambda-analysis.md` — context on why OTTL is *not* used in this prototype. Do not reference in the processor itself; it's for future iterations.

**Inside the contrib checkout at `/Users/evan.bradley/code/github.com/evan-bradley/opentelemetry-collector-contrib`:**
- `processor/groupbytraceprocessor/` — reference implementation. Study `processor.go`, `event.go` (event machine, may inspire post-prototype concurrency improvements), `ring_buffer.go` (ring-buffer eviction — we won't fully replicate but the shape is a reference), `README.md`.
- `processor/transformprocessor/` — reference for factory boilerplate, config structure with multiple statement contexts, `traces/`, `metrics/`, `logs/` subpackage layout.
- `pkg/ottl/ottlfuncs/func_is_root_span.go` — the existing (global-root-only) `IsRootSpan()` OTTL converter. Not used in this prototype but referenced by the feasibility doc.

**External:**
- OTLP proto `Span.flags` definitions: `opentelemetry-proto/opentelemetry/proto/trace/v1/trace.proto` lines ~347–356.
- Java local-root reference: `open-telemetry/opentelemetry-java-instrumentation/instrumentation-api/src/main/java/io/opentelemetry/instrumentation/api/instrumenter/LocalRootSpan.java`.

## Out of scope for this prototype

Do not build:
- OTTL `IsLocalRoot()` converter or any OTTL surface work. Tracked in `ICP-7891-ottl-lambda-analysis.md` as follow-up.
- EventMachine-style concurrency. Single mutex is fine; note this as a future optimization in the README.
- Disk-backed buffering. In-memory only.
- Metrics/logs pipeline support. Traces only. The `entrypoint` concept doesn't map to metrics/logs in the same way.
- Downstream OTTL statements. The processor promotes inline.
- Support for changing `wait_duration` at runtime.
- Sophisticated ring-buffer eviction. Simple LRU or arbitrary eviction is fine; log a warning on eviction.

## Deliverables checklist

- [ ] Processor source in `internal/processor/entrypointenrichmentprocessor/` per repo layout above.
- [ ] Unit tests (Phases 2, 3, 5) — high coverage.
- [ ] Integration test (Phase 7) — realistic scenarios, race-safe.
- [ ] E2E test (Phase 8) — automated with mock exporter + optional external endpoint hook.
- [ ] Manifest wire-up (Phase 9) — DT collector builds with the new processor.
- [ ] Example config in the repo demonstrating FF-enrichment.
- [ ] README (Phase 10) documenting the component including deployment constraints.
- [ ] Scope item 2 in `local/ICP-7891-feasibility.md` updated with a high-level overview + link to this plan.

## When you're done

Verify:
1. `go test -race ./internal/processor/entrypointenrichmentprocessor/...` passes.
2. `make build` (or repo equivalent) produces a Collector binary including the new processor.
3. Manually run the built Collector with the example config; feed it OTLP data (via the E2E generator or `telemetrygen`); observe promoted attributes on entrypoint spans in the debug exporter output.
4. All items in the deliverables checklist are done.

Report back with:
- File tree of what was created.
- Test summary (pass count, coverage).
- Any deviations from this plan and their rationale.
- Open questions or issues encountered that couldn't be resolved.
