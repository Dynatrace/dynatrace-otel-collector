// Copyright Dynatrace LLC
// SPDX-License-Identifier: Apache-2.0

package entrypointenrichmentprocessor

import "go.opentelemetry.io/collector/pdata/pcommon"

// OTLP Span.flags bits (opentelemetry-proto trace.proto:347-356).
const (
	spanFlagsContextHasIsRemoteMask uint32 = 0x00000100
	spanFlagsContextIsRemoteMask    uint32 = 0x00000200
)

// isLocalRoot reports whether bs is the entry-point span of its service subtree.
//
// Detection order:
//  1. Empty parent span ID → global root, always a local root.
//  2. HAS_IS_REMOTE flag set → authoritative: local root iff IS_REMOTE is also set.
//  3. Flags absent (older SDKs): compare service identity of bs against its parent.
//     If the parent is not in the index, default to true (safe: emit rather than suppress).
func isLocalRoot(bs bufferedSpan, index map[pcommon.SpanID]bufferedSpan) bool {
	span := bs.span
	if span.ParentSpanID().IsEmpty() {
		return true
	}
	f := span.Flags()
	if f&spanFlagsContextHasIsRemoteMask != 0 {
		return f&spanFlagsContextIsRemoteMask != 0
	}
	parent, ok := index[span.ParentSpanID()]
	if !ok {
		return true // parent not in buffer; safe default
	}
	return serviceIdentity(bs.resource) != serviceIdentity(parent.resource)
}

// serviceIdentity returns a stable string key for the service that owns a resource.
// Priority:
//  1. (service.name, service.instance.id) both present → "name|instance_id"
//  2. service.name only → "name"
//  3. Otherwise → stable hash of all resource attributes
func serviceIdentity(r pcommon.Resource) string {
	attrs := r.Attributes()
	name, hasName := attrs.Get("service.name")
	id, hasID := attrs.Get("service.instance.id")
	if hasName && hasID {
		return name.AsString() + "|" + id.AsString()
	}
	if hasName {
		return name.AsString()
	}
	return hashMap(attrs)
}
