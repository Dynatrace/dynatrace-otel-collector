# ICP-7891 — Design Revision 2026-08-06

> **Read this first if you're updating the existing `entrypointenrichmentprocessor` code.** This document is a delta: it lists exactly what changed from the previous plan and where in the code to make the changes. The full "target state" is in the four design docs alongside this one; read those for background if anything below is unclear.

## What changed

1. **Local-root detection algorithm.** The kind-based fallback (`SERVER`/`CONSUMER`) is removed. Fallback is now **service-identity comparison** against the parent's resource.
2. **Promotion target.** Changed from "the local root span" to **every SERVER/CONSUMER span in the emitted subtree**.
3. **Config surface.** The `local_root_detection` mode enum is removed entirely (`flags_with_kind_fallback`, `flags_only`, `kind_only`).
4. **Marker attribute scope.** Unchanged — still stamped on the local root only, not on other SERVER/CONSUMER promotion targets. Documented as a debug aid.

## New local-root detection algorithm

```
isLocalRoot(span, index):
    // index maps SpanID -> bufferedSpan for all currently-buffered spans of this trace.

    if span.ParentSpanID().IsEmpty():
        return true                                                       // global root

    flags := span.Flags()
    if flags & SPAN_FLAGS_CONTEXT_HAS_IS_REMOTE_MASK != 0:
        return flags & SPAN_FLAGS_CONTEXT_IS_REMOTE_MASK != 0             // authoritative

    parent, ok := index[span.ParentSpanID()]
    if !ok:
        return true                                                       // parent not visible; safe default

    return serviceIdentity(span.resource) != serviceIdentity(parent.resource)


serviceIdentity(resource):
    // 1. (service.name, service.instance.id) — both present -> "<name>|<instance_id>"
    // 2. service.name only -> "<name>"
    // 3. Otherwise -> stable hash of the resource's attribute map
    // Returns a string comparable via ==.
```

## New promotion pass

```
promote(members, rootID, cfg):
    // Collect targets: every span with kind SERVER or CONSUMER in the subtree.
    targets := []pcommon.Map
    for m in members:
        if m.span.Kind() in (SERVER, CONSUMER):
            targets.append(m.span.Attributes())

    // Copy matching attributes onto every target; first-wins per target.
    for m in members:
        for (k, v) in m.span.Attributes():
            if not matchesAny(k, cfg.compiledPatterns):
                continue
            for t in targets:
                if t.Has(k):
                    continue                                              // first-wins for this target
                t.Put(k, v)

    // Marker attribute goes on the local root ONLY.
    if cfg.LocalRootMarkerAttribute != "":
        for m in members:
            if m.span.SpanID() == rootID:
                m.span.Attributes().PutBool(cfg.LocalRootMarkerAttribute, true)
                break
```

**Semantic notes:**
- If no SERVER/CONSUMER spans exist in the subtree: no promotion happens. The batch still emits normally.
- Multiple SERVER/CONSUMER spans (nested SERVER inside a service, e.g. ingress span + app handler span) all receive the promoted attributes.
- The marker attribute is only on the local root, keeping its "buffering boundary" semantics distinct from the promotion targets.

## File-by-file changes

Paths are relative to the processor module root: `internal/processor/entrypointenrichmentprocessor/`.

### `localroot.go` (or wherever `isLocalRoot` lives)

- **Delete** the `LocalRootDetectionMode` type and the three mode constants (`ModeFlagsWithKindFallback`, `ModeFlagsOnly`, `ModeKindOnly`).
- **Delete** the `isEntryKind` helper.
- **Change** the signature of `isLocalRoot` from `(span ptrace.Span, mode LocalRootDetectionMode) bool` to something like `(span ptrace.Span, resource pcommon.Resource, index map[pcommon.SpanID]bufferedSpan) bool`. The exact signature depends on whether callers hold onto the current bufferedSpan (recommended — pass that instead).
- **Reimplement** per the algorithm above. Keep the `SPAN_FLAGS_CONTEXT_*` constants.
- **Add** a `serviceIdentity(pcommon.Resource) string` helper. If a `hashResourceAttrs` (or similar) already exists for the assembly coalescing step, reuse it for the last-resort fallback.

### `config.go`

- **Delete** the `LocalRootDetection` field from the `Config` struct.
- **Delete** any related default in `createDefaultConfig`.
- **Delete** the validation that enforces the enum values in `Validate()`.
- Leave the other fields untouched: `WaitDuration`, `FallbackDuration`, `NumTraces`, `AttributesToPromote`, `LocalRootMarkerAttribute`, and any private `compiledPatterns` slice.

### `promote.go`

- **Rewrite** `promote(members, rootID, cfg)` per the pseudocode above. The main change: replace the single `rootAttrs` target with a slice of target attribute maps (every SERVER/CONSUMER span in `members`). First-wins semantics apply per-target — a target that already has key `k` skips subsequent same-key values from other members.
- **Keep** the marker-attribute stamping restricted to the local root (identified by matching `SpanID() == rootID`).
- **Keep** `matchesAny(s, patterns)` unchanged.
- **Keep** `assemble(members)` and `hashResource`/`hashScope` unchanged.

### `buffer.go`

- **Update** all `isLocalRoot(...)` call sites to pass the current buffer index for the trace (`b.spans[tid]`) — or equivalent — plus the current bufferedSpan (so the callee can access the current span's Resource). Watch out for the ordering: in `Insert`, the current span is added to `b.spans[tid]` *before* calling `isLocalRoot`, so the map contains the current span. That's fine — the parent lookup uses `ParentSpanID()`, which is different from the current SpanID, so no self-reference issues.
- **Remove** the `mode LocalRootDetectionMode` field from the `Buffer` struct.
- **Remove** `mode` from the `reaches` function signature and its call sites; update the `isLocalRoot(parent.span, ...)` call inside `reaches` to pass the index (already available as `index` in `reaches`) plus the parent's Resource.

### `factory.go`

- **Remove** any default assignment for `LocalRootDetection` in `createDefaultConfig`.

### `README.md`

- **Remove** the `local_root_detection` row from the config reference table.
- **Update** the algorithm-description paragraph to state: flags first, service-identity fallback (via `service.name` + `service.instance.id`), safe default when parent is not visible. Reference the "known limitation" (same-service parent arriving in a later batch is not reclassified) in a limitations subsection.
- **Update** the enrichment description to say: matching attributes are copied onto **every SERVER/CONSUMER span in the subtree**, not just the local root. Note the debug-only marker attribute is on the local root only.

### Tests

**`localroot_test.go`**
- Replace the existing truth table entirely. New table (each row also specifies `Parent in index?`, `Parent service.name`/`instance.id`, `Child service.name`/`instance.id`):

  | # | ParentSpanID empty? | HAS_IS_REMOTE | IS_REMOTE | Parent in index? | Parent svc.name | Parent svc.instance.id | Child svc.name | Child svc.instance.id | Expected |
  |---|---|---|---|---|---|---|---|---|---|
  | 1 | Yes | any | any | n/a | n/a | n/a | any | any | true |
  | 2 | No  | 1   | 1   | any | any | any | any | any | true |
  | 3 | No  | 1   | 0   | Yes | s | i | s | i | false |
  | 4 | No  | 1   | 0   | No  | n/a | n/a | any | any | false |
  | 5 | No  | 0   | any | Yes | svcA | i1 | svcA | i1 | false |
  | 6 | No  | 0   | any | Yes | svcA | i1 | svcA | i2 | true |
  | 7 | No  | 0   | any | Yes | svcA | any | svcB | any | true |
  | 8 | No  | 0   | any | Yes | svcA | (absent) | svcA | (absent) | false |
  | 9 | No  | 0   | any | Yes | svcA | (absent) | svcB | (absent) | true |
  | 10 | No | 0   | any | Yes | (absent) | any | (absent) | any | Depends on resource-hash equality: identical resource → false; different attributes → true |
  | 11 | No | 0   | any | No  | n/a | n/a | any | any | true |

- Delete any prior tests that reference the old mode enum. `isEntryKind` should not appear anywhere in the file.

**`promote_test.go`**
- Add cases (or expand existing ones) to cover:
  - **Single SERVER target** (typical case): one SERVER local root + INTERNAL descendants; one descendant has matching attribute → attribute lands on SERVER root; marker stamped on root.
  - **Multiple SERVER targets — SERVER→SERVER**: outer SERVER local root + nested SERVER child (same service) + INTERNAL descendants with matching attributes → **both** SERVER spans receive each matching attribute; marker on local root only.
  - **SERVER→SERVER→SERVER amplification**: three nested SERVER spans, matching attribute on a leaf INTERNAL descendant → all three SERVER spans receive it. Marker on local root only.
  - **CONSUMER target**: subtree with a CONSUMER local root and INTERNAL descendants → CONSUMER receives promoted attribute.
  - **First-wins per target**: two INTERNAL descendants have the same key with different values → each target keeps whichever it saw first.
  - **Target already has the key**: SERVER target has `dt.feature_flag.result.foo=preexisting`; descendant has `dt.feature_flag.result.foo=fromchild` → SERVER retains `preexisting`.
  - **No SERVER/CONSUMER in subtree**: subtree with only INTERNAL/CLIENT spans → no promotion; marker still stamped on the local root (which is one of the INTERNAL spans in this edge case).
  - **Marker disabled**: `local_root_marker_attribute: ""` → marker not stamped on any span; promotion still happens.
- Retain the assembly-coalescing cases unchanged.

**`buffer_test.go`**
- Existing cases still apply. Update any that construct `Buffer` with a `mode` field.
- Add cross-service test data where relevant: e.g., the "two local roots in one trace" case now must produce two subtrees regardless of kind — construct spans with the same kind (both INTERNAL, say) but different `service.name` on their resources, verifying the service-comparison fallback fires.

**`config_test.go`**
- Delete tests that validate the `local_root_detection` enum values.
- Any table-driven test cases referencing `flags_with_kind_fallback` / `flags_only` / `kind_only` should be dropped.

## Acceptance criteria

- `go test -race ./internal/processor/entrypointenrichmentprocessor/...` passes.
- The E2E test at `internal/testbed/integration/entrypointenrichment/e2e_test.go` still passes. If the test constructed spans with only kind-based local-root cues (i.e., relying on the removed kind fallback), it needs updating: the three-service payload should either set `HAS_IS_REMOTE`/`IS_REMOTE` on cross-service SERVER spans, OR the payload's cross-service SERVERs must have different `service.name` from their parent's resource (they already do — each service is its own `ResourceSpans`). The E2E test's assertions on "each service's local root receives its own service's feature-flag attributes" should hold as-is, but now the plural case (multiple SERVER spans in one subtree) is worth adding at least one occurrence of. If time permits, add a fourth pseudo-service with an internal SERVER→SERVER structure to exercise the new multi-target promotion.
- `grep -r "LocalRootDetectionMode\|ModeFlagsWithKindFallback\|ModeFlagsOnly\|ModeKindOnly\|isEntryKind\|local_root_detection" internal/processor/entrypointenrichmentprocessor/` returns nothing.
- The DT collector build (`make build` or equivalent) still succeeds and the resulting binary starts with a config that references the processor.

## Known limitation to document (README)

When `HAS_IS_REMOTE` is unset (older SDKs) and a same-service parent arrives in a *later* OTLP batch than one of its children, the child is classified as a local root at insert time and is not reclassified when the parent arrives. Parent and child then end up in separate subtree emissions. Real-world incidence is low: most SDKs export a request's spans in one batch, and modern SDKs set the flags. Document under a "Limitations" heading.

## Pointers

- Full target-state design: `local/ICP-7891-feasibility.md`, `local/ICP-7891-buffering-strategy-comparison.md`, `local/ICP-7891-implementation-plan.md`, `local/ICP-7891-ottl-lambda-analysis.md`.
- Reference implementation for the boundary-detection concept: [`davidHaunschmied/otel-subtrace-demo`](https://github.com/davidHaunschmied/otel-subtrace-demo) (their `assignSubtraces` in `processor/subtraceaggregator/processor.go` is the closest analog).
- OTLP flag definitions: [`opentelemetry-proto/opentelemetry/proto/trace/v1/trace.proto:347-356`](https://github.com/open-telemetry/opentelemetry-proto/blob/main/opentelemetry/proto/trace/v1/trace.proto).

## When you're done

Report back with:
- Files touched (path + short summary of the change).
- Test summary: `go test -race ./...` output, count of new/updated tests.
- Any deviations from the guidance above, with rationale.
- Any places where the current code diverged from the previous plan in ways that made these changes non-trivial (so we can fold the corrections back into the main design docs).
