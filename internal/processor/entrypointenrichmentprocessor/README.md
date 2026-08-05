# Entrypoint Enrichment Processor

| Status                   |           |
|--------------------------|-----------|
| Stability                | [development]: traces |
| Distributions            | [Dynatrace] |

## Overview

The entrypoint enrichment processor promotes attributes from descendant spans
onto the **local root span** of each trace subtree, using a lazy-resolution
buffering strategy.

A local root span is the service-entrypoint span for a given process — the
first span in a service that was initiated by a remote caller (or the absolute
root of the whole trace). Attributes such as
`dt.feature_flag.result.<name>` are often evaluated in child or helper spans,
but Dynatrace OpenPipeline RED metrics are anchored to the local root span.
This processor bridges that gap by copying matching child attributes up to the
local root before forwarding.

## Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `wait_duration` | duration | `500ms` | How long to wait after a local root is seen before flushing its subtree. |
| `fallback_duration` | duration | `5s` | Maximum time to hold any trace. After this, all buffered spans for the trace are emitted without promotion. Must be ≥ `wait_duration`. |
| `num_traces` | int | `1000000` | Upper bound on the number of in-flight traces buffered. When at capacity, the oldest trace is evicted. |
| `local_root_detection` | string | `flags_with_kind_fallback` | How to identify local root spans. See [Local root detection](#local-root-detection). |
| `attributes_to_promote` | []string | `["^dt\\.feature_flag\\.result\\..+$"]` | List of RE2 regex patterns. Attributes from descendant spans whose keys match any pattern are promoted to the local root. |
| `local_root_marker_attribute` | string | `dt.local_root` | If non-empty, this attribute is set to `true` on every local root span. Set to `""` to disable. |

### Example

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

## Local root detection

The processor supports three modes for identifying local root spans, controlled
by `local_root_detection`:

| Mode | Behavior |
|------|----------|
| `flags_with_kind_fallback` (default) | Uses the OTLP `IS_REMOTE` flag when present; falls back to span kind (Server or Consumer) when the flag is absent. |
| `flags_only` | Uses the OTLP `IS_REMOTE` flag only. Spans without the flag set are not treated as local roots (unless their `parentSpanID` is empty). |
| `kind_only` | Treats Server and Consumer spans as local roots, regardless of flags. |

A span with an empty `parentSpanID` is always treated as a local root,
regardless of the detection mode.

**Recommendation**: use `flags_with_kind_fallback` (the default). Instrumentations
that emit the `IS_REMOTE` flag correctly will be handled by the flag check;
older or simpler instrumentations will be handled by the kind fallback.

## Buffering and latency

When a local root span arrives, the processor starts a `wait_duration` timer.
When the timer fires, it collects all buffered spans that belong to that
subtree (those whose parent chain reaches the local root without crossing
another local root's boundary), promotes matching attributes, and forwards the
batch to the next component.

A separate `fallback_duration` timer covers the whole trace. If the local root
never arrives within `fallback_duration`, all buffered spans for the trace are
emitted as-is (no promotion).

### Attribute promotion semantics

- **First-wins**: if the local root already has an attribute with a given key,
  its existing value is preserved and the descendant's value is ignored.
- **Pattern matching**: all RE2 patterns in `attributes_to_promote` are tested
  against each attribute key using full-string matching (the regex is tested
  with `MatchString`, so anchoring with `^` and `$` is recommended for precision).

## Deployment constraints

This processor buffers spans in memory and correlates them by `traceID`. All
spans for a given trace's local-root subtree **must reach the same collector
instance**. If spans are load-balanced across multiple instances, promotion
will be incomplete or absent for spans routed to different instances.

**Recommendation**: place a
[Load Balancing Exporter](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/exporter/loadbalancingexporter)
in a preceding collector tier to pin traces to the same downstream instance
before this processor.

[development]: https://github.com/open-telemetry/opentelemetry-collector#development
[Dynatrace]: https://www.dynatrace.com
