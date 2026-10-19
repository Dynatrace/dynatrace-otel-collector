// Copyright Dynatrace LLC
// SPDX-License-Identifier: Apache-2.0

package featureflags

import (
	"slices"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

const (
	// Span flag masks from the OTLP proto (SPAN_FLAGS_CONTEXT_HAS_IS_REMOTE_MASK and
	// SPAN_FLAGS_CONTEXT_IS_REMOTE_MASK). pdata doesn't expose named constants for them.
	spanFlagsHasIsRemote = 0x100
	spanFlagsIsRemote    = 0x200

	ffEventName  = "feature_flag.evaluation"
	ffAttrPrefix = "dt.feature_flag.result."
)

type ffEvent struct {
	key     string
	setID   string // "" = attribute absent
	variant string // "" = attribute absent
	value   any    // nil = attribute absent; otherwise string, bool, int64, or float64
	errType string // "" = attribute absent (sets error.type)
}

type testSpan struct {
	id         byte // unique within a topology; becomes the last byte of the SpanID
	parent     byte // 0 = no parent (trace root)
	service    string
	name       string
	kind       ptrace.SpanKind
	remote     bool // sets Span.Flags to HAS_IS_REMOTE|IS_REMOTE
	ffEvents   []ffEvent
	otherEvent bool // adds a non-feature-flag event "log" with attribute message="hello"
	// wantFF holds the expected dt.feature_flag.result.* attributes after processing. Keys are
	// written without the "dt.feature_flag.result." prefix.
	wantFF map[string]any
}

type topology struct {
	name  string
	spans []testSpan
	calls [][]byte // the expected groupbytrace releases: each entry is the span ids of one call
}

type ffMap = map[string]any

const (
	server   = ptrace.SpanKindServer
	consumer = ptrace.SpanKindConsumer
	client   = ptrace.SpanKindClient
	producer = ptrace.SpanKindProducer
	internal = ptrace.SpanKindInternal
)

func variantEvent(key, variant string) ffEvent { return ffEvent{key: key, variant: variant} }

func topologies() []topology {
	t1 := topology{
		name: "single-service",
		spans: []testSpan{
			{id: 1, parent: 0, service: "frontend", name: "GET /", kind: server, wantFF: ffMap{"new-header": "enabled"}},
			{id: 2, parent: 1, service: "frontend", name: "render", kind: internal,
				ffEvents: []ffEvent{variantEvent("new-header", "enabled")}, wantFF: ffMap{"new-header": "enabled"}},
		},
		calls: [][]byte{{1, 2}},
	}

	t2 := topology{
		name: "multi-eval",
		spans: []testSpan{
			{id: 1, parent: 0, service: "checkout", name: "POST /checkout", kind: server,
				wantFF: ffMap{"theme": "dark", "fast-path": true, "cart/max-items": int64(42), "discount": "ten-percent"}},
			{id: 2, parent: 1, service: "checkout", name: "step-1", kind: internal, otherEvent: true,
				ffEvents: []ffEvent{variantEvent("theme", "dark"), {key: "fast-path", value: true}},
				wantFF:   ffMap{"theme": "dark", "fast-path": true}},
			{id: 3, parent: 2, service: "checkout", name: "step-2", kind: internal,
				ffEvents: []ffEvent{{key: "max-items", setID: "cart", value: int64(42)}},
				wantFF:   ffMap{"cart/max-items": int64(42)}},
			{id: 4, parent: 1, service: "checkout", name: "discount", kind: client,
				ffEvents: []ffEvent{{key: "discount", variant: "ten-percent", value: 0.1}},
				wantFF:   ffMap{"discount": "ten-percent"}},
			{id: 5, parent: 1, service: "checkout", name: "beta", kind: internal,
				ffEvents: []ffEvent{{key: "beta", errType: "flag_not_found"}}, wantFF: ffMap{}},
			{id: 6, parent: 1, service: "checkout", name: "step-3", kind: internal,
				ffEvents: []ffEvent{variantEvent("theme", "dark")}, wantFF: ffMap{"theme": "dark"}},
		},
		calls: [][]byte{{1, 2, 3, 4, 5, 6}},
	}

	t3 := topology{
		name: "multi-service",
		spans: []testSpan{
			{id: 1, parent: 0, service: "frontend", name: "GET /", kind: server,
				wantFF: ffMap{"promo-banner": "summer", "theme": "dark"}},
			{id: 2, parent: 1, service: "frontend", name: "render", kind: internal,
				ffEvents: []ffEvent{variantEvent("promo-banner", "summer"), variantEvent("theme", "dark")},
				wantFF:   ffMap{"promo-banner": "summer", "theme": "dark"}},
			{id: 3, parent: 1, service: "frontend", name: "call checkout", kind: client, wantFF: ffMap{}},
			{id: 4, parent: 1, service: "frontend", name: "publish", kind: producer, wantFF: ffMap{}},
			{id: 5, parent: 3, service: "checkout", name: "POST /checkout", kind: server, wantFF: ffMap{"theme": "light"}},
			{id: 6, parent: 5, service: "checkout", name: "pricing", kind: internal,
				ffEvents: []ffEvent{variantEvent("theme", "light")}, wantFF: ffMap{"theme": "light"}},
			{id: 7, parent: 5, service: "checkout", name: "call inventory", kind: client, wantFF: ffMap{}},
			{id: 8, parent: 7, service: "inventory", name: "GET /stock", kind: server, wantFF: ffMap{}},
			{id: 9, parent: 8, service: "inventory", name: "db", kind: internal, wantFF: ffMap{}},
			{id: 10, parent: 4, service: "notifier", name: "process", kind: consumer, wantFF: ffMap{"email-template": "v2"}},
			{id: 11, parent: 10, service: "notifier", name: "template", kind: internal,
				ffEvents: []ffEvent{{key: "email-template", value: "v2"}}, wantFF: ffMap{"email-template": "v2"}},
		},
		calls: [][]byte{{1, 2, 3, 4}, {5, 6, 7}, {8, 9}, {10, 11}},
	}

	t4 := topology{
		name: "sibling-repeat",
		spans: []testSpan{
			{id: 1, parent: 0, service: "frontend", name: "GET /", kind: server, wantFF: ffMap{}},
			{id: 2, parent: 1, service: "frontend", name: "call 1", kind: client, wantFF: ffMap{}},
			{id: 3, parent: 1, service: "frontend", name: "call 2", kind: client, wantFF: ffMap{}},
			{id: 4, parent: 2, service: "checkout", name: "POST /a", kind: server,
				wantFF: ffMap{"theme": "dark", "only-call-1": "yes"}},
			{id: 5, parent: 4, service: "checkout", name: "work a", kind: internal,
				ffEvents: []ffEvent{variantEvent("theme", "dark"), variantEvent("only-call-1", "yes")},
				wantFF:   ffMap{"theme": "dark", "only-call-1": "yes"}},
			{id: 6, parent: 3, service: "checkout", name: "POST /b", kind: server,
				wantFF: ffMap{"theme": "light", "only-call-2": "yes"}},
			{id: 7, parent: 6, service: "checkout", name: "work b", kind: internal,
				ffEvents: []ffEvent{variantEvent("theme", "light"), variantEvent("only-call-2", "yes")},
				wantFF:   ffMap{"theme": "light", "only-call-2": "yes"}},
		},
		calls: [][]byte{{1, 2, 3}, {4, 5}, {6, 7}},
	}

	t5 := reentrant("reentrant", false)
	t6 := reentrant("reentrant-remote-flags", true)

	return []topology{t1, t2, t3, t4, t5, t6}
}

// reentrant builds the A -> B -> C -> B topology, optionally marking cross-service spans as remote.
func reentrant(name string, remote bool) topology {
	return topology{
		name: name,
		spans: []testSpan{
			{id: 1, parent: 0, service: "frontend", name: "GET /", kind: server, wantFF: ffMap{}},
			{id: 2, parent: 1, service: "frontend", name: "call checkout", kind: client, wantFF: ffMap{}},
			{id: 3, parent: 2, service: "checkout", name: "outer", kind: server, remote: remote,
				wantFF: ffMap{"theme": "dark", "outer-only": "yes"}},
			{id: 4, parent: 3, service: "checkout", name: "outer work", kind: internal,
				ffEvents: []ffEvent{variantEvent("theme", "dark"), variantEvent("outer-only", "yes")},
				wantFF:   ffMap{"theme": "dark", "outer-only": "yes"}},
			{id: 5, parent: 3, service: "checkout", name: "call payment", kind: client, wantFF: ffMap{}},
			{id: 6, parent: 5, service: "payment", name: "charge", kind: server, remote: remote,
				wantFF: ffMap{"fraud-check": "strict"}},
			{id: 7, parent: 6, service: "payment", name: "fraud", kind: internal,
				ffEvents: []ffEvent{variantEvent("fraud-check", "strict")}, wantFF: ffMap{"fraud-check": "strict"}},
			{id: 8, parent: 6, service: "payment", name: "call checkout again", kind: client, wantFF: ffMap{}},
			{id: 9, parent: 8, service: "checkout", name: "inner", kind: server, remote: remote,
				wantFF: ffMap{"theme": "light", "inner-only": "yes"}},
			{id: 10, parent: 9, service: "checkout", name: "inner work", kind: internal,
				ffEvents: []ffEvent{variantEvent("theme", "light"), variantEvent("inner-only", "yes")},
				wantFF:   ffMap{"theme": "light", "inner-only": "yes"}},
		},
		calls: [][]byte{{1, 2}, {3, 4, 5}, {6, 7, 8}, {9, 10}},
	}
}

func traceID(modeIdx, topoIdx int) pcommon.TraceID {
	var id pcommon.TraceID
	id[0] = byte(modeIdx + 1)
	id[1] = byte(topoIdx + 1)
	return id
}

func spanID(topoIdx int, id byte) pcommon.SpanID {
	var sid pcommon.SpanID
	sid[6] = byte(topoIdx + 1)
	sid[7] = id
	return sid
}

// item is a span together with the topology it belongs to.
type item struct {
	topoIdx int
	span    testSpan
}

func allItems(topos []topology) []item {
	var items []item
	for ti, topo := range topos {
		for _, s := range topo.spans {
			items = append(items, item{topoIdx: ti, span: s})
		}
	}
	return items
}

// buildTraces builds one payload from the items, with one ResourceSpans per (topology, service).
func buildTraces(modeIdx int, items []item) ptrace.Traces {
	td := ptrace.NewTraces()
	type key struct {
		topoIdx int
		service string
	}
	scopes := map[key]ptrace.ScopeSpans{}
	for _, it := range items {
		k := key{it.topoIdx, it.span.service}
		ss, ok := scopes[k]
		if !ok {
			rs := td.ResourceSpans().AppendEmpty()
			rs.Resource().Attributes().PutStr("service.name", it.span.service)
			ss = rs.ScopeSpans().AppendEmpty()
			ss.Scope().SetName("feature-flags-test")
			scopes[k] = ss
		}
		fillSpan(ss.Spans().AppendEmpty(), modeIdx, it)
	}
	return td
}

func fillSpan(sp ptrace.Span, modeIdx int, it item) {
	s := it.span
	sp.SetTraceID(traceID(modeIdx, it.topoIdx))
	sp.SetSpanID(spanID(it.topoIdx, s.id))
	if s.parent != 0 {
		sp.SetParentSpanID(spanID(it.topoIdx, s.parent))
	}
	sp.SetName(s.name)
	sp.SetKind(s.kind)
	if s.remote {
		sp.SetFlags(spanFlagsHasIsRemote | spanFlagsIsRemote)
	}
	now := time.Now()
	sp.SetStartTimestamp(pcommon.NewTimestampFromTime(now))
	sp.SetEndTimestamp(pcommon.NewTimestampFromTime(now.Add(10 * time.Millisecond)))
	sp.Attributes().PutStr("test.span.name", s.name)

	for _, e := range s.ffEvents {
		ev := sp.Events().AppendEmpty()
		ev.SetName(ffEventName)
		ev.SetTimestamp(pcommon.NewTimestampFromTime(now))
		ev.Attributes().PutStr("feature_flag.key", e.key)
		if e.setID != "" {
			ev.Attributes().PutStr("feature_flag.set.id", e.setID)
		}
		if e.variant != "" {
			ev.Attributes().PutStr("feature_flag.result.variant", e.variant)
		}
		switch v := e.value.(type) {
		case nil:
		case string:
			ev.Attributes().PutStr("feature_flag.result.value", v)
		case bool:
			ev.Attributes().PutBool("feature_flag.result.value", v)
		case int64:
			ev.Attributes().PutInt("feature_flag.result.value", v)
		case float64:
			ev.Attributes().PutDouble("feature_flag.result.value", v)
		default:
			panic("unsupported feature flag value type")
		}
		if e.errType != "" {
			ev.Attributes().PutStr("error.type", e.errType)
		}
	}
	if s.otherEvent {
		ev := sp.Events().AppendEmpty()
		ev.SetName("log")
		ev.SetTimestamp(pcommon.NewTimestampFromTime(now))
		ev.Attributes().PutStr("message", "hello")
	}
}

type payloadMode struct {
	name string
	// split turns the topologies into the payloads that are sent.
	split func(topos []topology) [][]item
}

func payloadModes() []payloadMode {
	return []payloadMode{
		{name: "single-payload", split: func(topos []topology) [][]item {
			return [][]item{allItems(topos)}
		}},
		{name: "span-per-payload", split: func(topos []topology) [][]item {
			var out [][]item
			for _, it := range allItems(topos) {
				out = append(out, []item{it})
			}
			return out
		}},
		{name: "span-per-payload-reversed", split: func(topos []topology) [][]item {
			items := allItems(topos)
			var out [][]item
			for _, it := range slices.Backward(items) {
				out = append(out, []item{it})
			}
			return out
		}},
		{name: "payload-per-service", split: func(topos []topology) [][]item {
			var out [][]item
			for ti, topo := range topos {
				var order []string
				byService := map[string][]item{}
				for _, s := range topo.spans {
					if _, ok := byService[s.service]; !ok {
						order = append(order, s.service)
					}
					byService[s.service] = append(byService[s.service], item{topoIdx: ti, span: s})
				}
				for _, svc := range order {
					out = append(out, byService[svc])
				}
			}
			return out
		}},
	}
}
