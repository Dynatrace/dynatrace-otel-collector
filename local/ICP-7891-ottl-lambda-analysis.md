# ICP-7891 — OTTL Lambda-Based Enrichment Design

Supporting analysis for [ICP-7891-feasibility.md § scope item 2](./ICP-7891-feasibility.md#scope-item-2--prototype). Considers whether OTTL — extended with pdata-slice-aware lambdas — is the right expression surface for child-to-local-root attribute promotion.

## Problem restatement

Downstream of the local-root buffering processor, we need to promote attributes matching a configurable prefix (e.g. `dt.feature_flag.result.*`) from any span in a batch onto the batch's local root. The buffering processor guarantees one local root per emitted `ptrace.Traces` batch, so the OTTL layer has a well-defined target.

The goal is to solve this in a way that generalizes to a class of "operate across sibling telemetry items within a batch" problems, rather than as a one-off custom function.

## What OTTL has now — lambda-powered functions

Behind the `ottl.functions.enableLambda` feature gate (alpha, contrib):

| Function | Signature | Role |
|---|---|---|
| `All(source, predicate)` | source: Slice/Map, predicate: `(k,v) => bool` | Universal quantifier |
| `Any(source, predicate)` | ↑ | Existential |
| `Filter(source, predicate)` | ↑ | Returns matching elements |
| `Find(source, predicate, mapper?)` | ↑, optional mapper | First match |
| `MapEach(source, mapper)` | source: Slice/Map, mapper: `(k,v) => value` | Element-wise transform |
| `MapKeys(source, keyMapper)` | ↑, keyMapper returns string | Rekeying |
| `Reduce(source, seed, accumulator)` | accumulator: `(acc,k,v) => value` | Fold |
| `When(condition)` | condition: `() => bool` | Conditional |
| `SliceToMap(...)` | | Collection conversion |

Reference: [pkg/ottl/ottlfuncs/README.md § All, Any, Filter, Find, MapEach, MapKeys, Reduce, When](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/pkg/ottl/ottlfuncs/README.md).

Lambda parameters cover `(index/key, value)`; the body is a full OTTL expression with access to the outer context via path expressions.

## Gaps blocking direct use for our problem

Verified against the contrib checkout:

1. **Lambda source resolution accepts only `pcommon.Slice` / `pcommon.Map`.**
   Source: [`pkg/ottl/ottlfuncs/internal/funcutil/getter.go:18-58`](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/pkg/ottl/ottlfuncs/internal/funcutil/getter.go) — the switch handles `pcommon.Map`, `pcommon.Slice`, and `pcommon.Value` of those types; falls back to `StandardPSliceGetter` / `StandardPMapGetter`. `ptrace.SpanSlice` is not accepted.

2. **`ottlspan` does not expose sibling spans as a path.**
   Source: `pkg/ottl/contexts/internal/ctxspan/span.go` — paths include `kind`, `attributes`, `name`, `trace_id`, `span_id`, `parent_span_id`, etc. No `spans` / `scope.spans`. The underlying `TransformContext` *does* hold `resourceSpans` and `scopeSpans`, so the plumbing is in place; only the path binding is missing.

3. **Path expressions on `ptrace.Span` values passed as lambda parameters.**
   The current setter/getter machinery for lambdas assumes pcommon-typed values. Path resolution on ptrace reference types passed as lambda parameters (e.g. `s.attributes` where `s` is a `ptrace.Span`) is not currently supported.

4. **No `IsLocalRoot()` OTTL converter.** `IsRootSpan()` exists (`pkg/ottl/ottlfuncs/func_is_root_span.go:32-36`) but only checks `ParentSpanID().IsEmpty()` — global-root only, ignores the flags-based local-root definition.

## The proposed extension

Three concrete OTTL additions:

### 1. `scope.spans` path in `ottlspan` (and analogues for other signals)

New path that resolves to the `ptrace.SpanSlice` of the containing `ScopeSpans`. Generalizes to `scope.logs` (LogRecordSlice), `scope.metrics` (MetricSlice), etc. — same pattern across signal contexts.

### 2. Lambda source resolution accepts pdata-native slices

Extend `GetSliceOrMapValue` (or equivalent) to accept `ptrace.SpanSlice`, `plog.LogRecordSlice`, `pmetric.NumberDataPointSlice`, `ptrace.SpanEventSlice`, etc. When the lambda body references paths on the element (e.g. `s.attributes`), resolution uses the corresponding signal-context path system.

This is where the design load-bearing decision sits — whether OTTL lambdas can meaningfully iterate pdata reference types with full path support inside the body.

### 3. `IsLocalRoot()` converter

Standalone, small. Implements the flags/kind algorithm ([feasibility doc § local-root detection](./ICP-7891-feasibility.md#local-root-detection-in-the-collector)):

```go
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
```

## What the OTTL config looks like with the extension

The enrichment expresses in native OTTL, no bespoke "hoist" function needed:

```yaml
- context: span
  statements:
    # On the local root, pull matching attrs from every sibling span into this span's attributes.
    - merge_maps(
        attributes,
        Reduce(
          scope.spans,
          {},
          (acc, _, s) => MergeMaps(
            acc,
            Filter(s.attributes, (k, _) => HasPrefix(k, "dt.feature_flag.result."))
          )
        ),
        "insert"
      ) where IsLocalRoot()
```

Notes:
- `where IsLocalRoot()` gates the statement to run once per batch (on the local root); `Reduce` iterates siblings once. O(N) overall.
- Exact syntax depends on whether `MergeMaps` (or an equivalent map-union converter) exists as a pure returning-a-value function. `merge_maps` today is a mutator; either a converter form is added or the equivalent is composed via `SliceToMap` and `MapEach`.
- If the buffering processor stamps `dt.local_root=true` on the local root, `where attributes["dt.local_root"] == true` substitutes for `where IsLocalRoot()` without any new function — a viable fallback if `IsLocalRoot()` faces resistance.
- Directional variant — copy any attribute matching a prefix onto any span matching a role predicate — is expressible with the same primitives.

## Fit with OTTL's vision

Strong.

- **Same design vocabulary as pcommon lambdas.** No new mental model for users; if you know `Filter(attributes, ...)`, you know `Filter(scope.spans, ...)`.
- **Uniform across signals.** The extension pattern (sibling-collection path + lambda source support + element paths) applies identically to spans, logs, metrics, span events, datapoints. Solves the general class of "operate across telemetry items within a batch," not just our problem.
- **Doesn't introduce a new context type.** Everything stays within existing signal contexts, preserving the single-unit-dispatch invariant of statements. The lambda body creates its own scope, which is already how OTTL lambdas work.
- **Complements recent OTTL direction.** The past ~12 months of OTTL evolution has been "more expressiveness within contexts" (profile contexts, otelcol context, lambdas) rather than "more contexts." This proposal lands cleanly in that trajectory.

Likely SIG concerns:
- Setter semantics on paths reached through lambda values (can `set(Find(scope.spans, ...).attributes["x"], "y")` work?). Design work, not a fundamental blocker.
- Performance envelope of nested lambdas over pdata slices — should be benchmarked.
- Naming: `scope.spans` vs `sibling_spans` vs `spans` vs `Spans()` — normal shape of an OTTL feature discussion.

None of those are showstoppers.

## Comparison to alternative approaches

| Approach | Fits OTTL vision | Generality | Prototype speed | Notes |
|---|---|---|---|---|
| Two-pass with cross-statement state | ✗ (OTTL has no batch cache) | Would need a batch-cache primitive | Slow | Rejected |
| Custom `HoistAttributes*` OTTL functions (no lambda extension) | Poor — one-off menagerie | Low | Fast | Fallback candidate |
| **Lambda-based: pdata-slice sources + `IsLocalRoot()`** | **Excellent** | **High — solves the class** | **Medium** | **Recommended** |
| Promotion inline in the buffering processor (no OTTL) | N/A | None | Fastest | Bridge if OTTL extension delays |
| New `ottltrace` context type | Doesn't fit — direction is *not* new contexts | Highest | Very slow | Not recommended |

## Recommendation

**Lambda-based extension is the right upstream design.** Concrete deliverables:

1. **Buffering processor** with local-root grouping (per feasibility doc scope item 3) — stamps `dt.local_root=true` on the local root before emit. Contribute upstream to contrib as its own processor. Ships independently of any OTTL work.
2. **`IsLocalRoot()` OTTL converter** — small, standalone contribution to `pkg/ottl/ottlfuncs`.
3. **OTTL enhancement proposal** for extending lambda source resolution to pdata slices + `scope.spans` path in `ottlspan`. Frame as a general pattern applicable to all signals. This is the load-bearing piece.
4. **Enrichment configured via native OTTL lambdas** in `transform_statements` once (3) lands.

**Bridge plan if (3) lands slowly:** the buffering processor can accept a `promote_prefixes` config option and do the promotion inline at flush time (Approach 3 from the design conversation). This keeps the feature shippable independent of OTTL SIG timing; the lambda-based configuration migrates to be the preferred surface once available.

## References

- [pkg/ottl/ottlfuncs/README.md — lambda functions section](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/pkg/ottl/ottlfuncs/README.md)
- [Lambda-powered functions land in OTTL — blog post](https://opentelemetry.io/blog/2026/lambda-powered-function-land-in-ottl/)
- [`pkg/ottl/ottlfuncs/internal/funcutil/getter.go`](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/pkg/ottl/ottlfuncs/internal/funcutil/getter.go) — current lambda source resolution
- [`pkg/ottl/contexts/ottlspan/span.go`](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/pkg/ottl/contexts/ottlspan/span.go) — current ottlspan context
- [`pkg/ottl/ottlfuncs/func_is_root_span.go`](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/pkg/ottl/ottlfuncs/func_is_root_span.go) — current IsRootSpan converter (global root only)
- [Feasibility doc](./ICP-7891-feasibility.md), [Buffering strategy comparison](./ICP-7891-buffering-strategy-comparison.md)
