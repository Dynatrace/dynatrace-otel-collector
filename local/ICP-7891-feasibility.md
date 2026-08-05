# ICP-7891 — Feasibility & Prototype of Option 1 (Collector Processor) for Feature Flag Context Enrichment

Research spike for [ICP-7891](https://dt-rnd.atlassian.net/browse/ICP-7891), part of VI [PRODUCT-16403](https://dt-rnd.atlassian.net/browse/PRODUCT-16403).

## Background

OpenPipeline generates RED metrics from service entrypoint (local root) spans. Because ingest is stateless, every metric dimension must be present as an attribute on the entrypoint span at ingest time. Feature flag evaluations happen throughout the call stack on whatever span is active, and OpenTelemetry has no mechanism for a child span to set an attribute on the service entrypoint (local root) span.

**Option 1** (this spike): build a Dynatrace-collector processor that buffers spans, identifies the local root, and copies attributes matching a configurable prefix (e.g. `dt.feature_flag.result.*`) from child spans up to the local root before export.

---

## Scope item 1 — Does an existing component already cover this?

**Short answer: No.** Nothing in `opentelemetry-collector-contrib` or `observIQ/bindplane-otel-contrib` performs child-to-local-root attribute promotion. The pattern is a documented, long-standing unfulfilled request in the OTel community. A new processor is required, but useful building blocks exist upstream.

### `open-telemetry/opentelemetry-collector-contrib`

| Component | Relevant? | Notes |
|---|---|---|
| `processor/groupbytraceprocessor` | **Building block** | Beta, not deprecated. Buffers spans by trace ID, releases the full trace after a configurable wait. Does no root identification or attribute copying — pure organizational. Reusable as the buffering layer. |
| `processor/tailsamplingprocessor` | **Reference implementation** | Buffers full traces in a circular buffer keyed by trace ID; has `decision_wait_after_root_received`, so root-span awareness exists internally. Writes only `tailsampling.policy`-style metadata post-decision — no cross-span attribute copying. Its buffering internals are a useful reference. |
| `processor/transformprocessor` (OTTL) | Not sufficient | OTTL contexts (`resource`/`scope`/`span`/`spanevent`) are all single-span — no sibling/parent-span access. Provides `IsRootSpan()`, but that only checks `parent_span_id == 0` (global root), not local root. |
| `processor/spanprocessor` | No | Per-span only (rename, extract attrs from name). |
| `processor/attributesprocessor` | No | Per-span attribute upsert/delete; no trace context. |
| `processor/redactionprocessor` | No | Per-span. |
| `processor/k8sattributesprocessor` | No | Resource-level, not span-to-span. |
| `connector/spanmetricsconnector` | No | Aggregates uniformly by `span.kind`; no local-root distinction. |
| `connector/servicegraphconnector` | No | Pairs client/server spans by kind for edges; no root identification. |

### Prior community requests (all confirmed no upstream solution)

- **Discussion [#29504](https://github.com/open-telemetry/opentelemetry-collector-contrib/discussions/29504)** — "Copying attributes between spans within a trace" — asks precisely for this. Community workaround suggested is a language-level SDK `SpanProcessor`, not collector-side.
- **Issue [#14026](https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/14026)** — "Inheritable Attribute" (customerId/userId propagation). Closed as not-planned/stale.
- **Issue [#32918](https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/32918)** — OTTL `IsRootSpan()` ergonomics; converter is merged. Adjacent, not a solution.

### `observIQ/bindplane-otel-contrib`

No matching processor. No `connector/` directory at all. Closest neighbors:

- `spancountprocessor` — counts spans, passes through unchanged.
- `snapshotprocessor` — flat FIFO of recent spans/logs/metrics for OpAMP retrieval; not trace-keyed.
- `resourceattributetransposerprocessor` — resource → datapoint direction, logs/metrics only.
- `lookupprocessor` — enriches a single record; no cross-span logic.

No reuse value from Bindplane.

### Dynatrace collector distribution (this repo)

- OTel Collector `v0.156.0` (core APIs `v1.62.0`).
- **Bundled and relevant:** `transformprocessor`, `attributesprocessor`, `tailsamplingprocessor`, `redactionprocessor`, `k8sattributesprocessor`, `spanmetricsconnector`.
- **`groupbytraceprocessor` is *not* bundled** — would need to be added if reused.
- **No custom local processor** exists for this purpose.

### Local-root detection in the Collector

**Canonical OTel definition** (consistent across Java instrumentation, PHP, and the AWS X-Ray contrib span processor):

> A span is a local root iff its parent span context is **invalid (no parent)** OR **remote (context propagated from another application)**.

References:
- Java: [`LocalRootSpan.java`](https://github.com/open-telemetry/opentelemetry-java-instrumentation/blob/main/instrumentation-api/src/main/java/io/opentelemetry/instrumentation/api/instrumenter/LocalRootSpan.java) — `!spanContext.isValid() || spanContext.isRemote()`
- PHP: [`LocalRootSpan.php`](https://github.com/open-telemetry/opentelemetry-php/blob/main/src/API/Trace/LocalRootSpan.php) — "the root-most active span which has a remote or invalid parent"
- Java contrib (AWS X-Ray): [`AttributePropagatingSpanProcessor.java`](https://github.com/open-telemetry/opentelemetry-java-contrib/blob/main/aws-xray/src/main/java/io/opentelemetry/contrib/awsxray/AttributePropagatingSpanProcessor.java) — same predicate, applied SDK-side.

**In the Collector**, the "parent context is remote" bit is carried on the wire via OTLP `Span.flags` (`fixed32`), per [`opentelemetry-proto/opentelemetry/proto/trace/v1/trace.proto:347-356`](https://github.com/open-telemetry/opentelemetry-proto/blob/main/opentelemetry/proto/trace/v1/trace.proto):

| Constant | Bit | Mask | Meaning |
|---|---|---|---|
| `SPAN_FLAGS_TRACE_FLAGS_MASK` | 0–7 | `0x000000FF` | W3C trace flags |
| `SPAN_FLAGS_CONTEXT_HAS_IS_REMOTE_MASK` | 8 | `0x00000100` | The "is remote" value is known for this span's parent |
| `SPAN_FLAGS_CONTEXT_IS_REMOTE_MASK` | 9 | `0x00000200` | Parent context is remote |

The pdata Go library exposes `span.Flags() uint32`. The named enum constants live in an internal package (`pdata/internal/data/protogen/trace/v1/trace.pb.go`), so a collector-side consumer defines the two masks locally as `uint32` literals.

**Fallback for SDKs that don't set the flag.** Many older SDKs don't populate the `HAS_IS_REMOTE` bit. When it's not set, the established heuristic — named as the fallback in OTEP 4931's "Alternative: An implementation for traces on the Backend" section, and used in practice by the AWS X-Ray span processor — is `span.kind IN (SERVER, CONSUMER)`, indicating a service entrypoint.

**Detection algorithm (Collector-side):**

```
isLocalRoot(span):
    if span.ParentSpanID().IsEmpty():
        return true                                                     // global root is trivially a local root
    if span.Flags() & SPAN_FLAGS_CONTEXT_HAS_IS_REMOTE_MASK != 0:
        return span.Flags() & SPAN_FLAGS_CONTEXT_IS_REMOTE_MASK != 0    // authoritative path
    return span.Kind() in (SERVER, CONSUMER)                            // fallback for SDKs without the flag
```

### Adjacent spec work (not a substitute)

Spec PR [#4931 "Context-scoped attributes"](https://github.com/open-telemetry/opentelemetry-specification/issues/4931) (merged 2026-06-22) is the ticket's Option 3. It's SDK/context-side — attach attributes to a Context so any telemetry emitted within it gets them at emission time. Different architectural cut; explicitly out of scope for this spike, and doesn't help for child attributes added by downstream systems the caller doesn't control.

---

## Scope item 2 — Prototype

The prototype is a self-contained processor — `entrypoint_enrichment` (Go package `entrypointenrichmentprocessor`) — living at `internal/processor/entrypointenrichmentprocessor/` in this repo, wired into the DT distribution via `manifest.yaml` using the same `replaces:` pattern as `eecprovider`. Full implementation plan in [ICP-7891-implementation-plan.md](./ICP-7891-implementation-plan.md).

### High-level design

- **One processor, not two.** Buffering and attribute promotion are combined. No OTTL surface work is needed for the prototype ([lambda analysis](./ICP-7891-ottl-lambda-analysis.md) covers the follow-up path if we want OTTL-based extensibility later).
- **Buffering**: lazy resolution grouped by local root (see scope item 3 and the [strategy comparison](./ICP-7891-buffering-strategy-comparison.md)). Per-subtree wait timer starts when the local root arrives; per-trace fallback timer catches stragglers whose local root never arrives.
- **Local-root detection**: `flags_with_kind_fallback` by default (OTLP `Span.flags` bits 8–9 when known, `span.kind IN (SERVER, CONSUMER)` otherwise), with `flags_only` and `kind_only` modes for testing.
- **Enrichment**: on subtree flush, walk descendant spans; for every attribute key matching any user-supplied regex (`attributes_to_promote`), first-wins-insert onto the local root. Source spans keep their attributes unchanged.
- **Marker attribute** on the local root: `dt.local_root=true` by default (configurable via `local_root_marker_attribute`; empty disables).
- **Reassembly**: at flush, coalesce spans into `ptrace.Traces` by structural equality on Resource and Scope so the output batch has one `ResourceSpans` per unique Resource.

### Config sketch

```yaml
processors:
  entrypoint_enrichment:
    wait_duration: 500ms
    fallback_duration: 5s
    num_traces: 1000000
    local_root_detection: flags_with_kind_fallback
    attributes_to_promote:
      - "^dt\\.feature_flag\\.result\\..+$"
    local_root_marker_attribute: "dt.local_root"
```

### What the prototype delivers

- Processor source with unit, integration, and E2E test coverage.
- Distribution wire-up so `manifest.yaml` builds a Collector binary including the processor.
- An example config demonstrating FF-enrichment.
- Component README covering config, deployment constraints (cross-linking scope item 5), and latency tradeoffs (cross-linking scope item 4).

### What the prototype explicitly does not deliver

- OTTL work (`IsLocalRoot()`, cross-span lambda source resolution, etc.). Tracked separately.
- `EventMachine`-style concurrency; a single mutex is used. Optimization is a follow-up.
- Disk-backed buffering.
- Metrics or logs pipeline support (traces only).

The full plan document breaks this into ten phases with per-phase acceptance criteria and enumerated test cases.

## Scope item 3 — Buffering strategy & constraints

Grouping spans by **local root** (rather than by global trace ID as `groupbytraceprocessor` does today) is a design invariant for this problem. Within one service, an SDK typically emits spans of a request together or in close succession; across services, arrival can be seconds apart. Keying the buffer by local root instead of trace ID lets each subtree flush on the timescale of one SDK batch interval instead of the whole distributed trace, which reduces added latency (scope item 4), lowers steady-state memory (there's no requirement to hold cross-service spans together), and materially relaxes the trace-aware routing requirement (scope item 5) — spans of one service's subtree co-locate naturally at whatever collector that service sends to.

The remaining design question is **how to identify each span's local root at ingest time**. Two strategies were evaluated (see [ICP-7891-buffering-strategy-comparison.md](./ICP-7891-buffering-strategy-comparison.md) for the full comparison): a **lazy resolution** approach (walk parent pointers at flush time) and an **optimistic keying** approach (union-find as spans arrive). Both are correct. Lazy is leaner — one span index plus timers, O(1) ingest, O(N·D) flush walk — and is recommended.

### Lazy resolution algorithm (sketch, Go/pdata)

```go
// State
type Buffer struct {
    // Flat span index for lookup and parent-chain walking. bufferedSpan retains
    // the (Resource, Scope) context needed to reassemble ptrace.Traces on emit.
    spans         map[pcommon.TraceID]map[pcommon.SpanID]bufferedSpan
    subtreeTimers map[pcommon.TraceID]map[pcommon.SpanID]*time.Timer // per subtree
    traceTimers   map[pcommon.TraceID]*time.Timer                    // fallback for stragglers

    waitDuration     time.Duration // wait after local root arrives
    fallbackDuration time.Duration // longer wait for orphaned children when L never arrives
    next             consumer.Traces
}

type bufferedSpan struct {
    resource pcommon.Resource
    scope    pcommon.InstrumentationScope
    span     ptrace.Span
}

const (
    // Bits 8-9 of Span.flags in OTLP trace.proto (SPAN_FLAGS_CONTEXT_HAS_IS_REMOTE_MASK
    // and SPAN_FLAGS_CONTEXT_IS_REMOTE_MASK). Constants live in pdata's internal
    // protogen package, so we redeclare them locally.
    spanFlagsContextHasIsRemoteMask uint32 = 0x00000100
    spanFlagsContextIsRemoteMask    uint32 = 0x00000200
)

// Called for every span in every incoming batch.
func (b *Buffer) Insert(res pcommon.Resource, scope pcommon.InstrumentationScope, span ptrace.Span) {
    tid, sid := span.TraceID(), span.SpanID()

    if b.spans[tid] == nil {
        b.spans[tid] = make(map[pcommon.SpanID]bufferedSpan)
    }
    b.spans[tid][sid] = bufferedSpan{resource: res, scope: scope, span: span}

    if _, ok := b.traceTimers[tid]; !ok {
        b.traceTimers[tid] = time.AfterFunc(b.fallbackDuration, func() { b.flushTrace(tid) })
    }

    if isLocalRoot(span) {
        if b.subtreeTimers[tid] == nil {
            b.subtreeTimers[tid] = make(map[pcommon.SpanID]*time.Timer)
        }
        if _, ok := b.subtreeTimers[tid][sid]; !ok {
            b.subtreeTimers[tid][sid] = time.AfterFunc(b.waitDuration,
                func() { b.flushSubtree(tid, sid) })
        }
    }
}

func isLocalRoot(span ptrace.Span) bool {
    if span.ParentSpanID().IsEmpty() {
        return true
    }
    flags := span.Flags()
    if flags&spanFlagsContextHasIsRemoteMask != 0 {
        return flags&spanFlagsContextIsRemoteMask != 0
    }
    return span.Kind() == ptrace.SpanKindServer || span.Kind() == ptrace.SpanKindConsumer
}

// Emit all buffered spans whose ancestry reaches rootID, bounded by other local roots.
func (b *Buffer) flushSubtree(tid pcommon.TraceID, rootID pcommon.SpanID) {
    index := b.spans[tid]
    if index == nil {
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
        if t := b.traceTimers[tid]; t != nil {
            t.Stop()
        }
        delete(b.traceTimers, tid)
        delete(b.subtreeTimers, tid)
    }

    if len(members) > 0 {
        _ = b.next.ConsumeTraces(context.Background(), assemble(members))
    }
}

// Walk parent pointers upward through the buffered index.
// Returns true iff we hit target before hitting a different local root or running off the chain.
func reaches(spanID, target pcommon.SpanID, index map[pcommon.SpanID]bufferedSpan) bool {
    cur, ok := index[spanID]
    if !ok {
        return false
    }
    for {
        if cur.span.SpanID() == target {
            return true
        }
        parentID := cur.span.ParentSpanID()
        if parentID.IsEmpty() {
            return false
        }
        parent, ok := index[parentID]
        if !ok {
            return false
        }
        // Boundary: don't cross into a different local root's subtree.
        if isLocalRoot(parent.span) && parent.span.SpanID() != target {
            return false
        }
        cur = parent
    }
}

// Fallback: local root never arrived (or arrived and flushed, and stragglers remain).
// Emit whatever's left without promoting attributes.
func (b *Buffer) flushTrace(tid pcommon.TraceID) {
    index := b.spans[tid]
    delete(b.spans, tid)
    for _, t := range b.subtreeTimers[tid] {
        t.Stop()
    }
    delete(b.subtreeTimers, tid)
    delete(b.traceTimers, tid)

    if len(index) == 0 {
        return
    }
    members := make([]bufferedSpan, 0, len(index))
    for _, s := range index {
        members = append(members, s)
    }
    _ = b.next.ConsumeTraces(context.Background(), assemble(members))
}

// assemble regroups bufferedSpans into a ptrace.Traces by their original Resource/Scope.
// (Implementation elided: build one ResourceSpans per unique Resource, one ScopeSpans
// per unique Scope within it, copy spans into the appropriate ScopeSpans.)
func assemble(members []bufferedSpan) ptrace.Traces { ... }
```

Notes on the sketch:
- Mutex protection is elided; a single mutex around the maps is sufficient for the prototype (the current groupbytrace uses per-worker serialization via `EventMachine` for better concurrency — worth borrowing later).
- `assemble` needs care: `ptrace.Span` values are references into their parent `ResourceSpans`/`ScopeSpans`, so `bufferedSpan` must retain the surrounding structs at insert time (or we must copy the span data into a stand-alone `ResourceSpans` per span at ingest, as `groupbytraceprocessor` does today).
- The `AncestorChainReaches` boundary rule (stop at any local root ≠ target) handles the case where the collector receives spans from multiple services of the same trace at the same instance — each service's subtree is emitted independently.
- One knob (`wait_duration`) drives per-subtree timers; `fallback_duration` can be derived (e.g. 3× wait) or exposed separately.

## Scope item 4 — Metrics-pipeline latency impact

Added latency between "local root arrives at the collector" and "promoted-attribute batch leaves the processor" is dominated by `wait_duration`. Collector-internal processing is µs to low ms.

**Delay sources between an SDK finishing the request and its spans reaching the collector's promotion buffer:**

1. **SDK batch scheduling.** `BatchSpanProcessor.scheduleDelay = 5s` default; in practice earlier flushes are common (queue-full triggers, shutdown), but for low-throughput services the scheduled delay dominates and children can sit in the SDK for seconds.
2. **Same-request spans splitting across batches.** Children end throughout the request, entrypoint ends last. Short requests: everything ends up in one batch. Long requests: earlier children exported earlier — they arrive at the collector before the local root does, which favors us.
3. **OTLP export duration.** gRPC RTT: single-digit ms same-network, tens of ms across regions.
4. **Retries on transient network failures.** OTLP exporters retry with backoff — default initial interval 5s. A retried batch lands 5s+ late.
5. **Connection drops / re-establishment.** Same order of magnitude as (4).

**What `wait_duration` needs to absorb:** the gap between the local root's arrival and the last child of the same subtree arriving from the same service. Under normal conditions (single OTLP export contains the whole subtree), this is <1 OTLP RTT. Under (4) or (5) or a low-throughput SDK, it stretches to seconds.

**Trade-off framing (no fixed default in the prototype):**

| Profile | `wait_duration` | `fallback_duration` | Trade-off |
|---|---|---|---|
| Aggressive (metrics-first) | 100–300 ms | 1–2 s | Occasional missed enrichment for stragglers; lower metrics latency. Suitable for stable networks with well-behaved SDKs. |
| Conservative (enrichment-first) | 1–3 s | 5–10 s | Catches more late-arriving children; adds meaningful latency to the metrics pipeline. Suitable for lossy networks or slow SDK batch cadence. |

The right default depends on observed span-arrival distributions in real Dynatrace-customer traffic, which this spike does not measure. The prototype should pick a value for benchmarking (say 500 ms) and the eventual default should be validated against real traffic before landing.

**Behavior when `wait_duration` is exceeded by an arrival:** the local root emits without the late child. The late child sits in the buffer briefly, then gets emitted via the fallback timer without promotion. Attribute promotion is lost for that child, but the span itself is not.

**Comparison to today's trace-ID grouping:**

| | Trace-ID grouping | Local-root grouping |
|---|---|---|
| Wait must absorb | Slowest cross-service span of the whole distributed trace | Slowest intra-service straggler within one subtree |
| Typical `wait_duration` | 500 ms – multiple seconds | 100 ms – few seconds (profile-dependent) |
| Pathological worst case | Cross-service async fan-out can push into seconds | `fallback_duration`; still under a well-behaved SDK's schedule delay |

**Intrinsic floor.** The entrypoint span ends at the end of the request, so request duration itself lower-bounds when the local root can reach the collector. `wait_duration` adds on top of that. No collector-side approach can eliminate this; only SDK-side solutions (PRODUCT-18515 Options 1 and 2) can.

## Scope item 5 — Load-balancing / trace-aware routing implications

**Requirement.** All spans of one local-root subtree must reach the same collector instance. Weaker than the trace-ID model's "all spans of the whole trace to one instance," but not zero.

**How the requirement holds under common routing setups:**

| Routing model | Holds? | Notes |
|---|---|---|
| Persistent gRPC connection, no LB | ✓ | Same connection ⇒ same instance |
| Persistent gRPC through L4 LB (kube Service via kube-proxy, most cloud L4 LBs) | ✓ | L4 routes per-flow; a live connection stays on one instance for its lifetime |
| DNS resolving to a single IP | ✓ | Sticky by construction |
| gRPC client-side LB with `round_robin` across sub-channels | ✗ | Successive RPCs may rotate; subtree can split |
| L7 proxy with per-request routing | ✗ | Two OTLP exports from the same SDK can land on different instances |
| Connection re-established after drop | ✗ | New connection may pick a different instance |
| Trace-aware `loadbalancing` exporter (today's gateway tier) | ✓ | Stricter than needed; still satisfies |

**Deployment topology comparison:**

| | Trace-ID grouping | Local-root grouping |
|---|---|---|
| Required routing | Trace-aware (gateway `loadbalancing` exporter, or L7 proxy inspecting trace ID) | Connection-sticky L4, DNS single-IP, or L7 hashed by `service.instance.id` |
| Deploy tiers | Gateway + processing (2 tiers) | Single tier possible |
| Payload inspection at LB | Yes (extract trace ID from OTLP) | No |
| Cross-service coordination | Required at LB | None |
| Ops burden | Two collector tiers to size/monitor/upgrade | One |

**Recommendations for users:**
- Prefer a single OTLP endpoint per service (DNS name → L4 LB / VIP).
- Avoid gRPC client-side `round_robin` LB unless coupled with `service.instance.id`-hashed server-side routing.
- For active-active collector fleets where pure connection-stickiness isn't reliable, use L7 hashing by `service.instance.id`. This is substantially cheaper than trace-aware LB (`service.instance.id` is a resource attribute, easy to extract in a proxy header or metadata) and still guarantees subtree co-location.
- Trace-aware LB is *compatible* — customers already running one for tail sampling can keep it. Local-root grouping asks for a weaker constraint that the stricter one satisfies.

**Edge cases:**

1. **Multi-service traces reaching one collector.** Each service's subtree buffers and flushes independently. The lazy-resolution boundary rule prevents cross-service contamination.
2. **Subtree splits under connection churn or per-request L7 routing.** Each collector emits its partial subtree; missing children in each half are lost to enrichment. Same failure mode as any trace-split scenario; fallback handles the span emission, not the promotion.
3. **Long-running services with multiple OTLP batches to different instances.** Same as (2). Rare in typical deployments.

**Bottom line.** Local-root grouping does not eliminate LB requirements; it weakens them from *payload-aware trace-ID routing* to *connection-stickiness or hash-by-`service.instance.id`*. That is a meaningful operational simplification, especially for customers not already running a two-tier collector layout, and it doesn't foreclose the option of running trace-aware LB when desired for other reasons.

## Scope item 6 — Path to upstream contribution

_Pending._

---

## References

- [groupbytraceprocessor README](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/processor/groupbytraceprocessor/README.md)
- [tailsamplingprocessor README](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/processor/tailsamplingprocessor/README.md)
- [transformprocessor README](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/processor/transformprocessor/README.md)
- [spanprocessor README](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/processor/spanprocessor/README.md)
- [spanmetricsconnector README](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/connector/spanmetricsconnector/README.md)
- [servicegraphconnector README](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/connector/servicegraphconnector/README.md)
- [OTTL functions README (IsRootSpan)](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/pkg/ottl/ottlfuncs/README.md)
- [Discussion #29504 — Copying attributes between spans within a trace](https://github.com/open-telemetry/opentelemetry-collector-contrib/discussions/29504)
- [Issue #14026 — Inheritable Attribute](https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/14026)
- [Spec PR #4931 — Context-scoped Attributes](https://github.com/open-telemetry/opentelemetry-specification/issues/4931)
- [Bindplane `observIQ/bindplane-otel-contrib`](https://github.com/observIQ/bindplane-otel-contrib)
