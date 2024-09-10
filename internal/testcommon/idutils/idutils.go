package idutils

import (
	"encoding/binary"
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

// UInt64ToTraceID converts the pair of uint64 representation of a TraceID to pcommon.TraceID.
func UInt64ToTraceID(high, low uint64) pcommon.TraceID {
	traceID := [16]byte{}
	binary.BigEndian.PutUint64(traceID[:8], high)
	binary.BigEndian.PutUint64(traceID[8:], low)
	return traceID
}

// UInt64ToSpanID converts the uint64 representation of a SpanID to pcommon.SpanID.
func UInt64ToSpanID(id uint64) pcommon.SpanID {
	spanID := [8]byte{}
	binary.BigEndian.PutUint64(spanID[:], id)
	return pcommon.SpanID(spanID)
}

func TraceIDAndSpanIDToString(traceID pcommon.TraceID, spanID pcommon.SpanID) string {
	return fmt.Sprintf("%s-%s", traceID, spanID)
}
