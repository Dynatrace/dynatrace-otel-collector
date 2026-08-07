# Entrypoint Enrichment Processor

| Status                   |           |
|--------------------------|-----------|
| Stability                | [development]: traces |
| Distributions            | [Dynatrace] |

## Overview

The entrypoint enrichment processor promotes attributes from descendant spans
onto every **SERVER and CONSUMER span** in each local-root subtree, using a
lazy-resolution buffering strategy.

A local root span is the service-entrypoint span for a given process — the
first span in a service that was initiated by a remote caller (or the absolute
root of the whole trace). Attributes such as `dt.feature_flag.result.<name>`
are often evaluated in child or helper spans, but Dynatrace OpenPipeline RED
metrics are anchored to the service entrypoint span. This processor bridges
that gap by copying matching descendant attributes up to every SERVER/CONSUMER
span in the subtree before forwarding.

## Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `wait_duration` | duration | `500ms` | How long to wait after a local root is seen before flushing its subtree. |
| `fallback_duration` | duration | `5s` | Maximum time to hold any trace. After this, all buffered spans for the trace are emitted without promotion. Must be ≥ `wait_duration`. |
| `num_traces` | int | `1000000` | Upper bound on the number of in-flight traces buffered. When at capacity, the oldest trace is evicted. |
| `attributes_to_promote` | []string | `["^dt\\.feature_flag\\.result\\..+$"]` | List of RE2 regex patterns. Attributes from descendant spans whose keys match any pattern are promoted onto every SERVER/CONSUMER span in the subtree. |
| `local_root_marker_attribute` | string | `dt.local_root` | If non-empty, this attribute is set to `true` on the local root span only (as a debug aid). Set to `""` to disable. |

### Example

```yaml
processors:
  entrypoint_enrichment:
    wait_duration: 500ms
    fallback_duration: 5s
    num_traces: 1000000
    attributes_to_promote:
      - "^dt\\.feature_flag\\.result\\..+$"
    local_root_marker_attribute: "dt.local_root"
```

## Local root detection

A span is classified as a local root in the following order:

1. **Empty `parentSpanID`** → always a local root (global trace root).
2. **`HAS_IS_REMOTE` flag set** (OTLP `Span.flags` bit 8) → authoritative:
   local root if and only if `IS_REMOTE` (bit 9) is also set. Spans with
   `HAS_IS_REMOTE=1, IS_REMOTE=0` are explicitly marked as *not* a service
   boundary and are never classified as local roots via this path.
3. **Flags absent (older SDKs)** → compare the span's service identity against
   its parent's service identity. Service identity is derived from the resource:
   - If both `service.name` and `service.instance.id` are present: `"<name>|<instance_id>"`
   - If only `service.name` is present: `"<name>"`
   - Otherwise: a stable hash of all resource attributes.

   If the identities differ, the span is a local root. If the parent is not
   yet in the buffer, the span defaults to local root (safe default — see [Limitations](#limitations)).

## Attribute promotion semantics

- **Promotion targets**: every SERVER and CONSUMER span in the subtree
  (including the local root if it is a SERVER/CONSUMER, and any nested SERVER
  spans within the same service).
- **First-wins**: if a target span already has an attribute with a given key,
  its existing value is preserved and descendant values for that key are
  ignored.
- **Pattern matching**: all RE2 patterns in `attributes_to_promote` are tested
  against each attribute key using `MatchString`. Anchoring with `^` and `$` is
  recommended for precision.
- **Marker attribute**: `local_root_marker_attribute` is stamped only on the
  local root span (identified by `SpanID`), not on other SERVER/CONSUMER
  promotion targets. It is intended as a debug aid, not as a promotion target.
- **Source retention**: source (descendant) spans are not modified; they retain
  their original attributes.

## Buffering and latency

When a local root span arrives, the processor starts a `wait_duration` timer.
When the timer fires, it collects all buffered spans that belong to that
subtree (those whose parent chain reaches the local root without crossing
another local root's boundary), promotes matching attributes, and forwards the
batch to the next component.

A separate `fallback_duration` timer covers the whole trace. If the local root
never arrives within `fallback_duration`, all buffered spans for the trace are
emitted as-is (no promotion).

## Deployment constraints

This processor buffers spans in memory and correlates them by `traceID`. All
spans for a given trace's local-root subtree **must reach the same collector
instance**. If spans are load-balanced across multiple instances, promotion
will be incomplete or absent for spans routed to different instances.

**Recommendation**: place a
[Load Balancing Exporter](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/exporter/loadbalancingexporter)
in a preceding collector tier to pin traces to the same downstream instance
before this processor.

## Limitations

**Same-service span arriving across batches.** When the `HAS_IS_REMOTE` flag is
unset (older SDKs) and a same-service parent arrives in a *later* OTLP batch
than one of its children, the child is classified as a local root at insert time
(safe default: parent not visible → assume service boundary) and is not
reclassified when the parent arrives. Parent and child then end up in separate
subtree emissions. Real-world incidence is low: most SDKs export a request's
spans in one batch, and modern SDKs set the `HAS_IS_REMOTE`/`IS_REMOTE` flags.

**Concurrency model.** The current implementation uses a single mutex around all
buffer state. For high-throughput deployments, this may become a bottleneck. A
future optimization is to adopt per-trace worker serialization (similar to
`groupbytraceprocessor`'s EventMachine pattern).

[development]: https://github.com/open-telemetry/opentelemetry-collector#development
[Dynatrace]: https://www.dynatrace.com
