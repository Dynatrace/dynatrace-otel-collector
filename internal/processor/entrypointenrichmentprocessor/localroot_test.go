// Copyright Dynatrace LLC
// SPDX-License-Identifier: Apache-2.0

package entrypointenrichmentprocessor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// makeLocalRootBS builds a bufferedSpan for isLocalRoot tests.
func makeLocalRootBS(parentID pcommon.SpanID, flags uint32, svcName, svcInstanceID string) bufferedSpan {
	span := ptrace.NewSpan()
	span.SetSpanID(pcommon.SpanID([8]byte{1}))
	if parentID != (pcommon.SpanID{}) {
		span.SetParentSpanID(parentID)
	}
	span.SetFlags(flags)
	res := pcommon.NewResource()
	if svcName != "" {
		res.Attributes().PutStr("service.name", svcName)
	}
	if svcInstanceID != "" {
		res.Attributes().PutStr("service.instance.id", svcInstanceID)
	}
	return bufferedSpan{resource: res, span: span}
}

// makeParentEntry adds a parent bufferedSpan to an index for the given service attrs.
func makeParentEntry(index map[pcommon.SpanID]bufferedSpan, parentID pcommon.SpanID, svcName, svcInstanceID string) {
	res := pcommon.NewResource()
	if svcName != "" {
		res.Attributes().PutStr("service.name", svcName)
	}
	if svcInstanceID != "" {
		res.Attributes().PutStr("service.instance.id", svcInstanceID)
	}
	ps := ptrace.NewSpan()
	ps.SetSpanID(parentID)
	index[parentID] = bufferedSpan{resource: res, span: ps}
}

var parentID = pcommon.SpanID([8]byte{2})

func TestIsLocalRoot(t *testing.T) {
	tests := []struct {
		name          string
		bs            func() bufferedSpan
		buildIndex    func() map[pcommon.SpanID]bufferedSpan
		want          bool
	}{
		{
			// Row 1: empty parent → always local root.
			name: "empty parent",
			bs:   func() bufferedSpan { return makeLocalRootBS(pcommon.SpanID{}, 0, "svc", "") },
			buildIndex: func() map[pcommon.SpanID]bufferedSpan { return map[pcommon.SpanID]bufferedSpan{} },
			want: true,
		},
		{
			// Row 2: HAS_IS_REMOTE=1, IS_REMOTE=1 → authoritative local root.
			name: "flags HAS+IS remote",
			bs:   func() bufferedSpan { return makeLocalRootBS(parentID, 0x300, "svc", "") },
			buildIndex: func() map[pcommon.SpanID]bufferedSpan { return map[pcommon.SpanID]bufferedSpan{} },
			want: true,
		},
		{
			// Row 3: HAS_IS_REMOTE=1, IS_REMOTE=0, parent in index → authoritative: NOT local root.
			name: "flags HAS but not IS, parent in index",
			bs:   func() bufferedSpan { return makeLocalRootBS(parentID, 0x100, "svc", "i") },
			buildIndex: func() map[pcommon.SpanID]bufferedSpan {
				idx := map[pcommon.SpanID]bufferedSpan{}
				makeParentEntry(idx, parentID, "svc", "i")
				return idx
			},
			want: false,
		},
		{
			// Row 4: HAS_IS_REMOTE=1, IS_REMOTE=0, parent NOT in index → authoritative: NOT local root.
			name: "flags HAS but not IS, parent absent",
			bs:   func() bufferedSpan { return makeLocalRootBS(parentID, 0x100, "svc", "") },
			buildIndex: func() map[pcommon.SpanID]bufferedSpan { return map[pcommon.SpanID]bufferedSpan{} },
			want: false,
		},
		{
			// Row 5: no flags, parent in index, same (service.name, service.instance.id) → NOT local root.
			name: "no flags, same name and instance.id",
			bs:   func() bufferedSpan { return makeLocalRootBS(parentID, 0, "svc-a", "inst-1") },
			buildIndex: func() map[pcommon.SpanID]bufferedSpan {
				idx := map[pcommon.SpanID]bufferedSpan{}
				makeParentEntry(idx, parentID, "svc-a", "inst-1")
				return idx
			},
			want: false,
		},
		{
			// Row 6: no flags, parent in index, same service.name but different instance.id → local root.
			name: "no flags, same name different instance.id",
			bs:   func() bufferedSpan { return makeLocalRootBS(parentID, 0, "svc-a", "inst-2") },
			buildIndex: func() map[pcommon.SpanID]bufferedSpan {
				idx := map[pcommon.SpanID]bufferedSpan{}
				makeParentEntry(idx, parentID, "svc-a", "inst-1")
				return idx
			},
			want: true,
		},
		{
			// Row 7: no flags, parent in index, different service.name → local root.
			name: "no flags, different service name",
			bs:   func() bufferedSpan { return makeLocalRootBS(parentID, 0, "svc-b", "") },
			buildIndex: func() map[pcommon.SpanID]bufferedSpan {
				idx := map[pcommon.SpanID]bufferedSpan{}
				makeParentEntry(idx, parentID, "svc-a", "")
				return idx
			},
			want: true,
		},
		{
			// Row 8: no flags, parent in index, same service.name only (no instance.id) → NOT local root.
			name: "no flags, same name only",
			bs:   func() bufferedSpan { return makeLocalRootBS(parentID, 0, "svc-a", "") },
			buildIndex: func() map[pcommon.SpanID]bufferedSpan {
				idx := map[pcommon.SpanID]bufferedSpan{}
				makeParentEntry(idx, parentID, "svc-a", "")
				return idx
			},
			want: false,
		},
		{
			// Row 9: no flags, parent in index, different service.name only (no instance.id) → local root.
			name: "no flags, different name only",
			bs:   func() bufferedSpan { return makeLocalRootBS(parentID, 0, "svc-b", "") },
			buildIndex: func() map[pcommon.SpanID]bufferedSpan {
				idx := map[pcommon.SpanID]bufferedSpan{}
				makeParentEntry(idx, parentID, "svc-a", "")
				return idx
			},
			want: true,
		},
		{
			// Row 10a: no flags, parent in index, no service attrs on either → same resource hash → NOT local root.
			name: "no flags, no service attrs, identical resource",
			bs:   func() bufferedSpan { return makeLocalRootBS(parentID, 0, "", "") },
			buildIndex: func() map[pcommon.SpanID]bufferedSpan {
				idx := map[pcommon.SpanID]bufferedSpan{}
				makeParentEntry(idx, parentID, "", "")
				return idx
			},
			want: false,
		},
		{
			// Row 10b: no flags, parent in index, no service attrs but different resource attributes → local root.
			name: "no flags, no service attrs, different resource",
			bs: func() bufferedSpan {
				res := pcommon.NewResource()
				res.Attributes().PutStr("host.name", "host-b")
				span := ptrace.NewSpan()
				span.SetSpanID(pcommon.SpanID([8]byte{1}))
				span.SetParentSpanID(parentID)
				return bufferedSpan{resource: res, span: span}
			},
			buildIndex: func() map[pcommon.SpanID]bufferedSpan {
				idx := map[pcommon.SpanID]bufferedSpan{}
				res := pcommon.NewResource()
				res.Attributes().PutStr("host.name", "host-a")
				ps := ptrace.NewSpan()
				ps.SetSpanID(parentID)
				idx[parentID] = bufferedSpan{resource: res, span: ps}
				return idx
			},
			want: true,
		},
		{
			// Row 11: no flags, parent NOT in index → safe default: local root.
			name: "no flags, parent absent from index",
			bs:   func() bufferedSpan { return makeLocalRootBS(parentID, 0, "svc-a", "") },
			buildIndex: func() map[pcommon.SpanID]bufferedSpan { return map[pcommon.SpanID]bufferedSpan{} },
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bs := tt.bs()
			idx := tt.buildIndex()
			// Add the span under test to the index so reaches() works correctly in the buffer,
			// but isLocalRoot only looks up the *parent* so self-presence doesn't affect the result.
			idx[bs.span.SpanID()] = bs
			assert.Equal(t, tt.want, isLocalRoot(bs, idx))
		})
	}
}
