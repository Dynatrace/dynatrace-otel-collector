// Copyright Dynatrace LLC
// SPDX-License-Identifier: Apache-2.0

package entrypointenrichmentprocessor

import "go.opentelemetry.io/collector/pdata/ptrace"

// OTLP Span.flags bits (opentelemetry-proto trace.proto:347-356).
const (
	spanFlagsContextHasIsRemoteMask uint32 = 0x00000100
	spanFlagsContextIsRemoteMask    uint32 = 0x00000200
)

// LocalRootDetectionMode controls how local root spans are identified.
type LocalRootDetectionMode string

const (
	ModeFlagsWithKindFallback LocalRootDetectionMode = "flags_with_kind_fallback"
	ModeFlagsOnly             LocalRootDetectionMode = "flags_only"
	ModeKindOnly              LocalRootDetectionMode = "kind_only"
)

func isLocalRoot(span ptrace.Span, mode LocalRootDetectionMode) bool {
	if span.ParentSpanID().IsEmpty() {
		return true
	}
	switch mode {
	case ModeKindOnly:
		return isEntryKind(span.Kind())
	case ModeFlagsOnly:
		f := span.Flags()
		return f&spanFlagsContextHasIsRemoteMask != 0 &&
			f&spanFlagsContextIsRemoteMask != 0
	default: // ModeFlagsWithKindFallback
		f := span.Flags()
		if f&spanFlagsContextHasIsRemoteMask != 0 {
			return f&spanFlagsContextIsRemoteMask != 0
		}
		return isEntryKind(span.Kind())
	}
}

func isEntryKind(k ptrace.SpanKind) bool {
	return k == ptrace.SpanKindServer || k == ptrace.SpanKindConsumer
}
