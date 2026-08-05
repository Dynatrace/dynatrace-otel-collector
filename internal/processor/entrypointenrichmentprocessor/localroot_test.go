// Copyright Dynatrace LLC
// SPDX-License-Identifier: Apache-2.0

package entrypointenrichmentprocessor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func TestIsLocalRoot(t *testing.T) {
	tests := []struct {
		name        string
		parentEmpty bool
		hasIsRemote bool
		isRemote    bool
		kind        ptrace.SpanKind
		mode        LocalRootDetectionMode
		want        bool
	}{
		{"empty parent any mode", true, false, false, ptrace.SpanKindInternal, ModeFlagsWithKindFallback, true},
		{"remote parent HAS+IS flags_with_kind", false, true, true, ptrace.SpanKindInternal, ModeFlagsWithKindFallback, true},
		{"remote parent HAS but not IS flags_with_kind", false, true, false, ptrace.SpanKindInternal, ModeFlagsWithKindFallback, false},
		{"no flags SERVER flags_with_kind", false, false, false, ptrace.SpanKindServer, ModeFlagsWithKindFallback, true},
		{"no flags CONSUMER flags_with_kind", false, false, false, ptrace.SpanKindConsumer, ModeFlagsWithKindFallback, true},
		{"no flags INTERNAL flags_with_kind", false, false, false, ptrace.SpanKindInternal, ModeFlagsWithKindFallback, false},
		{"no flags CLIENT flags_only", false, false, false, ptrace.SpanKindClient, ModeFlagsOnly, false},
		{"HAS+IS INTERNAL flags_only", false, true, true, ptrace.SpanKindInternal, ModeFlagsOnly, true},
		{"HAS no IS SERVER flags_only", false, true, false, ptrace.SpanKindServer, ModeFlagsOnly, false},
		{"SERVER kind_only", false, false, false, ptrace.SpanKindServer, ModeKindOnly, true},
		{"CLIENT kind_only", false, false, false, ptrace.SpanKindClient, ModeKindOnly, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			span := ptrace.NewSpan()
			if !tt.parentEmpty {
				span.SetParentSpanID(pcommon.SpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8}))
			}
			var flags uint32
			if tt.hasIsRemote {
				flags |= spanFlagsContextHasIsRemoteMask
			}
			if tt.isRemote {
				flags |= spanFlagsContextIsRemoteMask
			}
			span.SetFlags(flags)
			span.SetKind(tt.kind)
			assert.Equal(t, tt.want, isLocalRoot(span, tt.mode))
		})
	}
}
