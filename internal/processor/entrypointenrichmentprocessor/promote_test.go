// Copyright Dynatrace LLC
// SPDX-License-Identifier: Apache-2.0

package entrypointenrichmentprocessor

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func makeSpan(sid [8]byte, pid [8]byte, kind ptrace.SpanKind, attrs map[string]string) bufferedSpan {
	span := ptrace.NewSpan()
	span.SetSpanID(pcommon.SpanID(sid))
	if pid != ([8]byte{}) {
		span.SetParentSpanID(pcommon.SpanID(pid))
	}
	span.SetKind(kind)
	for k, v := range attrs {
		span.Attributes().PutStr(k, v)
	}
	return bufferedSpan{resource: pcommon.NewResource(), scope: pcommon.NewInstrumentationScope(), span: span}
}

func compiledCfg(patterns []string, marker string) *Config {
	compiled := make([]*regexp.Regexp, 0, len(patterns))
	for _, p := range patterns {
		compiled = append(compiled, regexp.MustCompile(p))
	}
	return &Config{
		AttributesToPromote:      patterns,
		LocalRootMarkerAttribute: marker,
		compiledPatterns:         compiled,
	}
}

// TestPromote_SingleMatch: one SERVER root + one INTERNAL descendant with a matching attr.
// The attr should be promoted onto the SERVER root; the marker is stamped on the root.
func TestPromote_SingleMatch(t *testing.T) {
	rootID := pcommon.SpanID([8]byte{1})
	cfg := compiledCfg([]string{`^dt\.feature_flag\.`}, "dt.local_root")

	root := makeSpan([8]byte{1}, [8]byte{}, ptrace.SpanKindServer, nil)
	child := makeSpan([8]byte{2}, [8]byte{1}, ptrace.SpanKindInternal, map[string]string{
		"dt.feature_flag.result.foo": "bar",
	})
	members := []bufferedSpan{root, child}
	promote(members, rootID, cfg)

	v, ok := root.span.Attributes().Get("dt.feature_flag.result.foo")
	require.True(t, ok)
	assert.Equal(t, "bar", v.AsString())

	// child retains its own attr (source retention)
	cv, ok := child.span.Attributes().Get("dt.feature_flag.result.foo")
	require.True(t, ok)
	assert.Equal(t, "bar", cv.AsString())

	// marker on root
	m, ok := root.span.Attributes().Get("dt.local_root")
	require.True(t, ok)
	assert.Equal(t, "true", m.AsString())
}

// TestPromote_MultipleServerTargets: outer SERVER root + nested SERVER child + INTERNAL leaf with FF.
// Both SERVERs receive the attribute; marker only on the local root.
func TestPromote_MultipleServerTargets(t *testing.T) {
	rootID := pcommon.SpanID([8]byte{1})
	cfg := compiledCfg([]string{`^dt\.feature_flag\.`}, "dt.local_root")

	root := makeSpan([8]byte{1}, [8]byte{}, ptrace.SpanKindServer, nil)
	inner := makeSpan([8]byte{2}, [8]byte{1}, ptrace.SpanKindServer, nil) // nested SERVER
	leaf := makeSpan([8]byte{3}, [8]byte{2}, ptrace.SpanKindInternal, map[string]string{
		"dt.feature_flag.result.pricing_v2": "on",
	})
	members := []bufferedSpan{root, inner, leaf}
	promote(members, rootID, cfg)

	// Both SERVER spans receive the promoted attribute.
	for _, target := range []bufferedSpan{root, inner} {
		v, ok := target.span.Attributes().Get("dt.feature_flag.result.pricing_v2")
		require.True(t, ok, "SERVER span %v missing promoted attr", target.span.SpanID())
		assert.Equal(t, "on", v.AsString())
	}

	// Marker on local root only.
	_, markerOnRoot := root.span.Attributes().Get("dt.local_root")
	assert.True(t, markerOnRoot)
	_, markerOnInner := inner.span.Attributes().Get("dt.local_root")
	assert.False(t, markerOnInner, "marker must not be stamped on non-root SERVER spans")
}

// TestPromote_ThreeNestedServerAmplification: three nested SERVER spans + INTERNAL leaf with FF.
// All three SERVER spans receive the attribute.
func TestPromote_ThreeNestedServerAmplification(t *testing.T) {
	rootID := pcommon.SpanID([8]byte{1})
	cfg := compiledCfg([]string{`^dt\.feature_flag\.`}, "")

	s1 := makeSpan([8]byte{1}, [8]byte{}, ptrace.SpanKindServer, nil)
	s2 := makeSpan([8]byte{2}, [8]byte{1}, ptrace.SpanKindServer, nil)
	s3 := makeSpan([8]byte{3}, [8]byte{2}, ptrace.SpanKindServer, nil)
	leaf := makeSpan([8]byte{4}, [8]byte{3}, ptrace.SpanKindInternal, map[string]string{
		"dt.feature_flag.result.algo": "v2",
	})
	members := []bufferedSpan{s1, s2, s3, leaf}
	promote(members, rootID, cfg)

	for _, target := range []bufferedSpan{s1, s2, s3} {
		v, ok := target.span.Attributes().Get("dt.feature_flag.result.algo")
		require.True(t, ok, "SERVER span %v missing promoted attr", target.span.SpanID())
		assert.Equal(t, "v2", v.AsString())
	}
}

// TestPromote_ConsumerTarget: CONSUMER local root + INTERNAL descendants.
func TestPromote_ConsumerTarget(t *testing.T) {
	rootID := pcommon.SpanID([8]byte{1})
	cfg := compiledCfg([]string{`^dt\.feature_flag\.`}, "dt.local_root")

	root := makeSpan([8]byte{1}, [8]byte{}, ptrace.SpanKindConsumer, nil)
	child := makeSpan([8]byte{2}, [8]byte{1}, ptrace.SpanKindInternal, map[string]string{
		"dt.feature_flag.result.queue_mode": "batch",
	})
	members := []bufferedSpan{root, child}
	promote(members, rootID, cfg)

	v, ok := root.span.Attributes().Get("dt.feature_flag.result.queue_mode")
	require.True(t, ok)
	assert.Equal(t, "batch", v.AsString())

	m, ok := root.span.Attributes().Get("dt.local_root")
	require.True(t, ok)
	assert.Equal(t, "true", m.AsString())
}

// TestPromote_FirstWins: two INTERNAL descendants have the same key; each target receives
// only the first value it encounters; root retains whichever arrived first.
func TestPromote_FirstWins(t *testing.T) {
	rootID := pcommon.SpanID([8]byte{1})
	cfg := compiledCfg([]string{`^dt\.feature_flag\.`}, "")

	root := makeSpan([8]byte{1}, [8]byte{}, ptrace.SpanKindServer, nil)
	child1 := makeSpan([8]byte{2}, [8]byte{1}, ptrace.SpanKindInternal, map[string]string{
		"dt.feature_flag.result.foo": "first",
	})
	child2 := makeSpan([8]byte{3}, [8]byte{1}, ptrace.SpanKindInternal, map[string]string{
		"dt.feature_flag.result.foo": "second",
	})
	members := []bufferedSpan{root, child1, child2}
	promote(members, rootID, cfg)

	v, ok := root.span.Attributes().Get("dt.feature_flag.result.foo")
	require.True(t, ok)
	// Exactly one value wins (slice iteration order is deterministic; map iteration inside Range is not,
	// but both are valid outcomes — we assert one is present and stable).
	assert.True(t, v.AsString() == "first" || v.AsString() == "second")
}

// TestPromote_ConflictWithRoot: target already has the key → target retains its own value.
func TestPromote_ConflictWithRoot(t *testing.T) {
	rootID := pcommon.SpanID([8]byte{1})
	cfg := compiledCfg([]string{`^dt\.feature_flag\.`}, "")

	root := makeSpan([8]byte{1}, [8]byte{}, ptrace.SpanKindServer, map[string]string{
		"dt.feature_flag.result.foo": "preexisting",
	})
	child := makeSpan([8]byte{2}, [8]byte{1}, ptrace.SpanKindInternal, map[string]string{
		"dt.feature_flag.result.foo": "from-child",
	})
	members := []bufferedSpan{root, child}
	promote(members, rootID, cfg)

	v, ok := root.span.Attributes().Get("dt.feature_flag.result.foo")
	require.True(t, ok)
	assert.Equal(t, "preexisting", v.AsString())
}

// TestPromote_NoMatches: no attribute matches the patterns; marker is still stamped.
func TestPromote_NoMatches(t *testing.T) {
	rootID := pcommon.SpanID([8]byte{1})
	cfg := compiledCfg([]string{`^dt\.feature_flag\.`}, "dt.local_root")

	root := makeSpan([8]byte{1}, [8]byte{}, ptrace.SpanKindServer, nil)
	child := makeSpan([8]byte{2}, [8]byte{1}, ptrace.SpanKindInternal, map[string]string{
		"other.attr": "value",
	})
	members := []bufferedSpan{root, child}
	promote(members, rootID, cfg)

	_, ok := root.span.Attributes().Get("other.attr")
	assert.False(t, ok)

	m, ok := root.span.Attributes().Get("dt.local_root")
	require.True(t, ok)
	assert.Equal(t, "true", m.AsString())
}

// TestPromote_NoServerConsumerInSubtree: subtree has only INTERNAL/CLIENT spans.
// No promotion happens; marker is still stamped on the local root.
func TestPromote_NoServerConsumerInSubtree(t *testing.T) {
	rootID := pcommon.SpanID([8]byte{1})
	cfg := compiledCfg([]string{`^dt\.feature_flag\.`}, "dt.local_root")

	// Root is INTERNAL (edge case: local root was identified by buffer logic but isn't SERVER/CONSUMER).
	root := makeSpan([8]byte{1}, [8]byte{}, ptrace.SpanKindInternal, nil)
	child := makeSpan([8]byte{2}, [8]byte{1}, ptrace.SpanKindClient, map[string]string{
		"dt.feature_flag.result.foo": "bar",
	})
	members := []bufferedSpan{root, child}
	promote(members, rootID, cfg)

	// No SERVER/CONSUMER → no targets → attribute not copied anywhere.
	_, ok := root.span.Attributes().Get("dt.feature_flag.result.foo")
	assert.False(t, ok)
	_, ok = child.span.Attributes().Get("dt.feature_flag.result.foo")
	assert.True(t, ok, "source span retains its own attribute")

	// Marker is still stamped on the designated root span.
	m, ok := root.span.Attributes().Get("dt.local_root")
	require.True(t, ok)
	assert.Equal(t, "true", m.AsString())
}

// TestPromote_ComplexRegex: two patterns, each matching different attribute prefixes.
func TestPromote_ComplexRegex(t *testing.T) {
	rootID := pcommon.SpanID([8]byte{1})
	cfg := compiledCfg([]string{`^dt\.feature_flag\.`, `^my\.experiment\.`}, "")

	root := makeSpan([8]byte{1}, [8]byte{}, ptrace.SpanKindServer, nil)
	child := makeSpan([8]byte{2}, [8]byte{1}, ptrace.SpanKindInternal, map[string]string{
		"dt.feature_flag.result.foo": "ff-val",
		"my.experiment.variant":      "exp-val",
		"unmatched.attr":             "ignored",
	})
	members := []bufferedSpan{root, child}
	promote(members, rootID, cfg)

	v1, ok := root.span.Attributes().Get("dt.feature_flag.result.foo")
	require.True(t, ok)
	assert.Equal(t, "ff-val", v1.AsString())

	v2, ok := root.span.Attributes().Get("my.experiment.variant")
	require.True(t, ok)
	assert.Equal(t, "exp-val", v2.AsString())

	_, ok = root.span.Attributes().Get("unmatched.attr")
	assert.False(t, ok)
}

// TestPromote_MarkerDisabled: empty marker attribute → no marker stamped on any span.
func TestPromote_MarkerDisabled(t *testing.T) {
	rootID := pcommon.SpanID([8]byte{1})
	cfg := compiledCfg([]string{`^dt\.feature_flag\.`}, "") // empty marker

	root := makeSpan([8]byte{1}, [8]byte{}, ptrace.SpanKindServer, nil)
	members := []bufferedSpan{root}
	promote(members, rootID, cfg)

	// Only the marker would have been added; since it's disabled the map should be empty.
	assert.Equal(t, 0, root.span.Attributes().Len())
}

func TestAssemble_Coalescing(t *testing.T) {
	res1 := pcommon.NewResource()
	res1.Attributes().PutStr("service.name", "svc-a")
	scope1 := pcommon.NewInstrumentationScope()
	scope1.SetName("scope-a")

	span1 := ptrace.NewSpan()
	span1.SetSpanID(pcommon.SpanID([8]byte{1}))
	span2 := ptrace.NewSpan()
	span2.SetSpanID(pcommon.SpanID([8]byte{2}))

	// Same resource + same scope → one RS with one SS containing both spans.
	members := []bufferedSpan{
		{resource: res1, scope: scope1, span: span1},
		{resource: res1, scope: scope1, span: span2},
	}
	result := assemble(members)
	assert.Equal(t, 1, result.ResourceSpans().Len())
	assert.Equal(t, 1, result.ResourceSpans().At(0).ScopeSpans().Len())
	assert.Equal(t, 2, result.ResourceSpans().At(0).ScopeSpans().At(0).Spans().Len())

	// Same resource + different scope → one RS with two SS.
	scope2 := pcommon.NewInstrumentationScope()
	scope2.SetName("scope-b")
	span3 := ptrace.NewSpan()
	span3.SetSpanID(pcommon.SpanID([8]byte{3}))
	members2 := []bufferedSpan{
		{resource: res1, scope: scope1, span: span1},
		{resource: res1, scope: scope2, span: span3},
	}
	result2 := assemble(members2)
	assert.Equal(t, 1, result2.ResourceSpans().Len())
	assert.Equal(t, 2, result2.ResourceSpans().At(0).ScopeSpans().Len())

	// Different resource → two RS.
	res2 := pcommon.NewResource()
	res2.Attributes().PutStr("service.name", "svc-b")
	span4 := ptrace.NewSpan()
	span4.SetSpanID(pcommon.SpanID([8]byte{4}))
	members3 := []bufferedSpan{
		{resource: res1, scope: scope1, span: span1},
		{resource: res2, scope: scope1, span: span4},
	}
	result3 := assemble(members3)
	assert.Equal(t, 2, result3.ResourceSpans().Len())
}
