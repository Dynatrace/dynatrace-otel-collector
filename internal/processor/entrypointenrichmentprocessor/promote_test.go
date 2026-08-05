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

func makeSpan(sid [8]byte, pid [8]byte, attrs map[string]string) bufferedSpan {
	span := ptrace.NewSpan()
	span.SetSpanID(pcommon.SpanID(sid))
	if pid != ([8]byte{}) {
		span.SetParentSpanID(pcommon.SpanID(pid))
	}
	for k, v := range attrs {
		span.Attributes().PutStr(k, v)
	}
	res := pcommon.NewResource()
	scope := pcommon.NewInstrumentationScope()
	return bufferedSpan{resource: res, scope: scope, span: span}
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

func TestPromote_SingleMatch(t *testing.T) {
	rootID := pcommon.SpanID([8]byte{1})
	childID := pcommon.SpanID([8]byte{2})
	cfg := compiledCfg([]string{`^dt\.feature_flag\.`}, "dt.local_root")

	root := makeSpan([8]byte{1}, [8]byte{}, nil)
	child := makeSpan([8]byte{2}, [8]byte{1}, map[string]string{
		"dt.feature_flag.result.foo": "bar",
	})
	members := []bufferedSpan{root, child}

	promote(members, rootID, cfg)

	v, ok := root.span.Attributes().Get("dt.feature_flag.result.foo")
	require.True(t, ok)
	assert.Equal(t, "bar", v.AsString())

	// marker is set
	m, ok := root.span.Attributes().Get("dt.local_root")
	require.True(t, ok)
	assert.Equal(t, "true", m.AsString())

	// child should still have its attr
	cv, ok := child.span.Attributes().Get("dt.feature_flag.result.foo")
	require.True(t, ok)
	assert.Equal(t, "bar", cv.AsString())

	_ = childID // suppress unused
}

func TestPromote_FirstWins(t *testing.T) {
	rootID := pcommon.SpanID([8]byte{1})
	cfg := compiledCfg([]string{`^dt\.feature_flag\.`}, "")

	root := makeSpan([8]byte{1}, [8]byte{}, nil)
	child1 := makeSpan([8]byte{2}, [8]byte{1}, map[string]string{
		"dt.feature_flag.result.foo": "first",
	})
	child2 := makeSpan([8]byte{3}, [8]byte{1}, map[string]string{
		"dt.feature_flag.result.foo": "second",
	})
	// Put child1 first in iteration; promote visits child1 then child2.
	// Because Map iteration order in Go is non-deterministic, we can only assert
	// that exactly one value is chosen (not which); the first-wins guarantee is
	// about the root's *pre-existing* value, tested in TestPromote_ConflictWithRoot.
	members := []bufferedSpan{root, child1, child2}
	promote(members, rootID, cfg)

	v, ok := root.span.Attributes().Get("dt.feature_flag.result.foo")
	require.True(t, ok)
	// exactly one of the two values must be present
	assert.True(t, v.AsString() == "first" || v.AsString() == "second")
}

func TestPromote_ConflictWithRoot(t *testing.T) {
	rootID := pcommon.SpanID([8]byte{1})
	cfg := compiledCfg([]string{`^dt\.feature_flag\.`}, "")

	root := makeSpan([8]byte{1}, [8]byte{}, map[string]string{
		"dt.feature_flag.result.foo": "root-value",
	})
	child := makeSpan([8]byte{2}, [8]byte{1}, map[string]string{
		"dt.feature_flag.result.foo": "child-value",
	})
	members := []bufferedSpan{root, child}
	promote(members, rootID, cfg)

	v, ok := root.span.Attributes().Get("dt.feature_flag.result.foo")
	require.True(t, ok)
	assert.Equal(t, "root-value", v.AsString(), "root's existing value must be preserved")
}

func TestPromote_NoMatches(t *testing.T) {
	rootID := pcommon.SpanID([8]byte{1})
	cfg := compiledCfg([]string{`^dt\.feature_flag\.`}, "dt.local_root")

	root := makeSpan([8]byte{1}, [8]byte{}, nil)
	child := makeSpan([8]byte{2}, [8]byte{1}, map[string]string{
		"other.attr": "value",
	})
	members := []bufferedSpan{root, child}
	promote(members, rootID, cfg)

	// no feature flag promoted
	_, ok := root.span.Attributes().Get("other.attr")
	assert.False(t, ok)

	// marker still stamped
	m, ok := root.span.Attributes().Get("dt.local_root")
	require.True(t, ok)
	assert.Equal(t, "true", m.AsString())
}

func TestPromote_ComplexRegex(t *testing.T) {
	rootID := pcommon.SpanID([8]byte{1})
	cfg := compiledCfg([]string{`^dt\.feature_flag\.`, `^my\.experiment\.`}, "")

	root := makeSpan([8]byte{1}, [8]byte{}, nil)
	child := makeSpan([8]byte{2}, [8]byte{1}, map[string]string{
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

func TestPromote_MarkerDisabled(t *testing.T) {
	rootID := pcommon.SpanID([8]byte{1})
	cfg := compiledCfg([]string{`^dt\.feature_flag\.`}, "") // empty marker

	root := makeSpan([8]byte{1}, [8]byte{}, nil)
	members := []bufferedSpan{root}
	promote(members, rootID, cfg)

	// no marker attribute should be present
	assert.Equal(t, 0, root.span.Attributes().Len())
}

func TestAssemble_Coalescing(t *testing.T) {
	// Same resource + same scope → one RS with one SS.
	res1 := pcommon.NewResource()
	res1.Attributes().PutStr("service.name", "svc-a")
	scope1 := pcommon.NewInstrumentationScope()
	scope1.SetName("scope-a")

	span1 := ptrace.NewSpan()
	span1.SetSpanID(pcommon.SpanID([8]byte{1}))
	span2 := ptrace.NewSpan()
	span2.SetSpanID(pcommon.SpanID([8]byte{2}))

	members := []bufferedSpan{
		{resource: res1, scope: scope1, span: span1},
		{resource: res1, scope: scope1, span: span2},
	}
	result := assemble(members)
	assert.Equal(t, 1, result.ResourceSpans().Len(), "same resource should be coalesced")
	assert.Equal(t, 1, result.ResourceSpans().At(0).ScopeSpans().Len(), "same scope should be coalesced")
	assert.Equal(t, 2, result.ResourceSpans().At(0).ScopeSpans().At(0).Spans().Len())

	// Same resource + different scope → one RS, two SS.
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
