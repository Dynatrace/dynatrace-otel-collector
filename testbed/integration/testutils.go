package integration

import (
	"encoding/binary"
	"fmt"
	"strconv"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

const CollectorTestsExecPath string = "../../bin/dynatrace-otel-collector"

func replaceOtlpGrpcReceiverPort(cfg string, receiverPort int) string {
	return strings.Replace(cfg, "4317", strconv.Itoa(receiverPort), 1)
}

func replaceDynatraceExporterEndpoint(cfg string, exporterPort int) string {
	r := strings.NewReplacer(
		"${env:DT_OTLP_ENDPOINT}", fmt.Sprintf("http://0.0.0.0:%v", exporterPort),
		"${env:API_TOKEN}", "",
	)
	return r.Replace(cfg)
}

func uInt64ToTraceID(high, low uint64) pcommon.TraceID {
	traceID := [16]byte{}
	binary.BigEndian.PutUint64(traceID[:8], high)
	binary.BigEndian.PutUint64(traceID[8:], low)
	return traceID
}

func uInt64ToSpanID(id uint64) pcommon.SpanID {
	spanID := [8]byte{}
	binary.BigEndian.PutUint64(spanID[:], id)
	return pcommon.SpanID(spanID)
}

func traceIDAndSpanIDToString(traceID pcommon.TraceID, spanID pcommon.SpanID) string {
	return fmt.Sprintf("%s-%s", traceID, spanID)
}
