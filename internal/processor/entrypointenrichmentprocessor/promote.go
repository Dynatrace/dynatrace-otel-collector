// Copyright Dynatrace LLC
// SPDX-License-Identifier: Apache-2.0

package entrypointenrichmentprocessor

import (
	"fmt"
	"regexp"
	"sort"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)


// promote copies matching attributes from every member onto every SERVER/CONSUMER
// span in the subtree (first-wins per target). The marker attribute is stamped
// on the local root span only.
func promote(members []bufferedSpan, rootID pcommon.SpanID, cfg *Config) {
	// Collect all SERVER/CONSUMER spans as promotion targets.
	var targets []pcommon.Map
	for i := range members {
		k := members[i].span.Kind()
		if k == ptrace.SpanKindServer || k == ptrace.SpanKindConsumer {
			targets = append(targets, members[i].span.Attributes())
		}
	}

	// Copy matching attributes onto every target (first-wins per target).
	// When a target span iterates its own matching attributes, the key already
	// exists in its own map, so it skips itself without modification.
	for i := range members {
		members[i].span.Attributes().Range(func(k string, v pcommon.Value) bool {
			if !matchesAny(k, cfg.compiledPatterns) {
				return true
			}
			for _, t := range targets {
				if _, exists := t.Get(k); exists {
					continue // first-wins for this target
				}
				v.CopyTo(t.PutEmpty(k))
			}
			return true
		})
	}

	// Marker goes on the local root only.
	if cfg.LocalRootMarkerAttribute != "" {
		for i := range members {
			if members[i].span.SpanID() == rootID {
				members[i].span.Attributes().PutBool(cfg.LocalRootMarkerAttribute, true)
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

// assemble reconstructs a ptrace.Traces from bufferedSpans, coalescing by (Resource, Scope).
func assemble(members []bufferedSpan) ptrace.Traces {
	traces := ptrace.NewTraces()

	type scopeGroup struct {
		rs     ptrace.ResourceSpans
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

func hashResource(r pcommon.Resource) string {
	return hashMap(r.Attributes())
}

func hashScope(s pcommon.InstrumentationScope) string {
	return s.Name() + "|" + s.Version() + "|" + hashMap(s.Attributes())
}

func hashMap(m pcommon.Map) string {
	keys := make([]string, 0, m.Len())
	m.Range(func(k string, _ pcommon.Value) bool {
		keys = append(keys, k)
		return true
	})
	sort.Strings(keys)
	var result string
	for _, k := range keys {
		v, _ := m.Get(k)
		result += fmt.Sprintf("%s=%s;", k, v.AsString())
	}
	return result
}
