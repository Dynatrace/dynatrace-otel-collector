# ICP-7891 — Buffering Strategies for Local-Root Grouping

Supporting analysis for [ICP-7891-feasibility.md § scope item 3](./ICP-7891-feasibility.md#scope-item-3--buffering-strategy--constraints).

## Problem

A collector-side processor that promotes child-span attributes onto the local-root span must buffer spans of a trace long enough to see the local root and its descendants. Buffering by **trace ID** (as `groupbytraceprocessor` does today) requires the wait to accommodate the slowest cross-service arrival — hundreds of ms or more. Buffering by **local root** cuts the wait to one service's SDK batch cadence — often 100 ms or less — and relaxes the trace-aware routing requirement.

To buffer by local root, each incoming span must be attributed to its local-root subtree. This is not always resolvable at ingest: a child span may arrive before its ancestor chain is fully present in the buffer. Two strategies were considered.

Local-root detection (used by both strategies) — see [feasibility doc](./ICP-7891-feasibility.md#local-root-detection-in-the-collector):

```
isLocalRoot(span, index):
    if span.ParentSpanID().IsEmpty():
        return true
    if span.Flags() & SPAN_FLAGS_CONTEXT_HAS_IS_REMOTE_MASK != 0:
        return span.Flags() & SPAN_FLAGS_CONTEXT_IS_REMOTE_MASK != 0
    parent, ok := index[span.ParentSpanID()]
    if !ok:
        return true                                                   // parent not visible; safe default
    return serviceIdentity(span.resource) != serviceIdentity(parent.resource)
```

Note that the fallback is now a **service-identity comparison** against the parent's resource (using `service.name` + `service.instance.id` when both present), rather than a span-kind heuristic.

---

## Strategy 1 — Lazy resolution

**Idea:** buffer every span by `(traceID, spanID)`. Do no ancestry work at ingest. At flush time (per-subtree timer), walk parent pointers to collect members.

### State

```
spans:          map[traceID] -> map[spanID] -> bufferedSpan
subtreeTimers:  map[traceID] -> map[localRootSpanID] -> Timer
traceTimers:    map[traceID] -> Timer   // fallback for stragglers
```

### Insert(span)

1. Store `spans[traceID][spanID] = span`.
2. Start `traceTimers[traceID]` (fallback) if not set.
3. If `isLocalRoot(span)` and no timer yet for this spanID: start `subtreeTimers[traceID][spanID]` with `waitDuration`.

O(1) work per span. No parent-chain walking at ingest.

### FlushSubtree(traceID, rootID) — fires when the subtree wait timer expires

1. For each `(sid, span)` in `spans[traceID]`, check `reaches(sid, rootID, spans[traceID])`. If true, add to members set and remove from index.
2. If `spans[traceID]` is empty, cascade-delete trace timer + entry.
3. Emit collected members as one `ptrace.Traces`.

Where `reaches` walks parent pointers upward through the buffered index, returning true iff we hit `target` before hitting a *different* local root or running off the chain:

```
reaches(spanID, target, index):
    cur := index[spanID]
    while true:
        if cur.spanID == target: return true
        if cur.parentSpanID.IsEmpty(): return false
        parent := index[cur.parentSpanID]
        if parent not in index: return false
        if isLocalRoot(parent) and parent.spanID != target: return false   // boundary
        cur = parent
```

O(N·D) per flush, where N = spans of the trace remaining in the buffer, D = trace depth.

### FlushTrace(traceID) — fallback timer

Emit whatever's left as-is (no promotion possible: either L never arrived, or arrivals are late relative to L's flush). Cascade-delete state.

### Correctness invariants

| Case | Behavior |
|---|---|
| L arrives first, children follow | Timer starts on L, children arrive during wait, all emitted together. ✓ |
| L arrives last | Children stall in buffer with no subtree timer. When L arrives, timer starts; walk picks them up on fire. ✓ |
| L never arrives | Fallback timer fires; stragglers emitted without promotion. ✓ (bounded) |
| Two local roots in same trace at same collector | Each L has its own timer; boundary rule in `reaches` prevents cross-subtree contamination. ✓ |
| Late child (arrives after L's flush) | Sits in buffer under trace T; picked up by fallback timer. Emitted without promotion. ✓ |
| Child whose parent chain runs off (parent missing) | `reaches` returns false; span sits until fallback timer emits it. ✓ |

### Costs

- **Ingest:** O(1) per span.
- **Flush per subtree:** O(N·D). N is bounded by trace size and D is small (typical call depth ~5-20), so this is µs range in practice.
- **State:** 1 span index, 1 subtree timer map, 1 trace timer map.

---

## Strategy 2 — Optimistic keying (union-find)

**Idea:** every incoming span is a tentative singleton subtree. Union with parent as parent-child links are observed. Local-root spans do *not* union with their parent — the service boundary is a non-union point.

### State

```
spans:          map[traceID] -> map[spanID] -> bufferedSpan
parent:         map[traceID] -> map[spanID] -> spanID     // union-find parent pointer
meta:           map[traceID] -> map[repr] -> { confirmedRoot?, members: set[spanID], timer? }
waitingOn:      map[traceID] -> map[parentSpanID] -> [childSpanID]  // orphans awaiting parent
traceTimers:    map[traceID] -> Timer  // fallback
```

### Insert(span)

1. Store span; create singleton subtree keyed by spanID.
2. If `isLocalRoot(span)`: mark `confirmedRoot = spanID` in meta; start subtree timer.
3. If NOT `isLocalRoot(span)` AND `spans[traceID][parentSpanID]` exists: `union(spanID, parentSpanID)`. Merged subtree inherits any `confirmedRoot`; assert we never merge two `confirmedRoot`s (invariant: at most one local root per merged subtree).
4. Else if parent not present: add `spanID` to `waitingOn[traceID][parentSpanID]`.
5. If `waitingOn[traceID][spanID]` exists: for each waiter W, `union(W, spanID)`. Clear the pending entry.

### FlushSubtree

Iterate `meta[repr].members` set → emit → clean up. O(K).

### Correctness invariants

Same table as Strategy 1: same correctness outcomes across all cases. Differences are structural (where work happens), not semantic.

- Local-root spans never union with their parent — service boundaries preserved.
- Late-arriving L unions its accumulated waiters into one subtree in one shot.
- Two local roots in the same trace stay in disjoint sets.
- Late child after subtree flush: forms a singleton, times out via fallback.

### Costs

- **Ingest:** O(α(N)) amortized union-find, plus a hashmap lookup for `waitingOn`; each `union` may trigger cascade merges of accumulated waiters.
- **Flush per subtree:** O(K), K = subtree size — direct iteration of member set.
- **State:** span index + parent-pointer map + meta map + waitingOn map + timer maps.

---

## Side-by-side

| Dimension | Lazy | Optimistic |
|---|---|---|
| Data structures beyond span index | 2 timer maps | 4 maps (parent pointer, meta, waitingOn, timers) |
| Ingest cost per span | O(1) | O(α(N)) + possible cascade of pending unions |
| Flush cost per subtree | O(N·D) walk of the trace's remaining spans | O(K) iteration of the members set |
| Where the work lives | At flush time | At ingest time |
| Approximate code size | ~150 LOC | ~350 LOC (union-find + waitingOn + merge assertions) |
| Invariants to enforce | Boundary check inside `reaches` | Never-union-across-local-root; never-merge-two-confirmed-roots; pending-waiter integrity |
| L arrives late | ✓ (timer starts on L, walk collects) | ✓ (waitingOn holds children; union on L arrival) |
| L never arrives | ✓ (fallback) | ✓ (fallback) |
| Multiple local roots per trace | ✓ (boundary check) | ✓ (never-union-across-LR) |
| Late child after subtree flush | ✓ (fallback) | ✓ (fallback) |
| Correctness overall | ✓ | ✓ |

---

## Recommendation: **Lazy resolution**

For "as lean as possible while correct":

1. **Fewer moving parts.** Optimistic maintains union-find + a pending-waiter map + per-subtree membership sets. Lazy maintains one span index and lets the boundary rule live in one function.
2. **Ingest is the hot path.** Lazy is O(1) at ingest. Optimistic does more work per span, and the pending-waiter cascade can trigger multiple unions on one insert.
3. **Small traces dominate.** For subtrees of ~15 spans with call depth ~5, lazy's flush walk is on the order of 75 pointer chases — µs-range. Optimistic saves that at the cost of ingest work on every span. For typical workloads the total is comparable, and lazy wins on simplicity.
4. **Large traces don't break the model.** Even at N=1000 and D=20, lazy's per-flush walk is 20k pointer chases — sub-ms. Not a bottleneck.
5. **Easier to reason about.** Lazy's flush is a plain loop with one predicate. Optimistic requires holding the union-find semantics + the non-merge invariants in your head to modify safely.

Optimistic's only structural advantage is that subtree membership is known incrementally — useful if we ever want to emit a subtree early upon hitting a size threshold. That's a hypothetical need, not a current requirement. When and if we need it, we can revisit.

## Config surface

Both strategies fit a minimal config:

- `wait_duration` — per-subtree timer duration (starts when local root arrives). No fixed default recommended without validation against real span-arrival distributions; see the feasibility doc's scope item 4 for the tradeoff between an aggressive (~100–300 ms) and conservative (~1–3 s) setting.
- `fallback_duration` — trace-level timer for stragglers whose local root never arrived. Roughly 3–5× `wait_duration`, exposed as a separate knob.
- `num_traces` — bound on the number of in-flight traceID entries (existing groupbytrace knob).

No detection-mode knob: the algorithm is flags-first with service-identity fallback, all paths executed unconditionally.
