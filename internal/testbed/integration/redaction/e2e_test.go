// Copyright Dynatrace LLC
// SPDX-License-Identifier: Apache-2.0

package redaction

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/oteltest"
	"github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"
)

// startCollector starts the DT Collector with an example config. edit adapts the config
// to the test on top of the common local overrides. It returns the port of the OTLP/gRPC
// receiver and the sink the Collector exports to.
func startCollector(t *testing.T, example string, edit func(*testutil.Config)) (int, *consumertest.TracesSink) {
	sink, sinkPorts := oteltest.StartTracesSink(t)
	grpcPort := testutil.GetAvailablePort(t)
	healthPort := testutil.GetAvailablePort(t)

	cfg := testutil.LoadConfigExample(t, example).
		Replace("grpc receiver port", func(s string) string { return testutil.ReplaceOtlpGrpcReceiverPort(s, grpcPort) }).
		Replace("health_check port", func(s string) string { return testutil.ReplaceHealthCheckPort(s, healthPort) }).
		Replace("exporter endpoint", func(s string) string { return testutil.ReplaceDynatraceExporterEndpoint(s, sinkPorts.Http) }).
		Replace("sending_queue and retry_on_failure", func(s string) string { return testutil.DisableSendingQueueAndRetry(s, "otlp_http") }).
		Replace("internal metrics", testutil.DisableInternalMetrics)
	edit(cfg)

	testutil.StartCollector(t, testutil.CollectorParams{
		Name:      strings.TrimSuffix(example, ".yaml"),
		Config:    cfg.String(),
		ReadyPort: grpcPort,
	})
	return grpcPort, sink
}

// export sends the traces and fails the test if the export is not accepted.
func export(t *testing.T, grpcPort int, td ptrace.Traces) {
	client := oteltest.NewTracesClient(t, grpcPort)
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	_, err := client.Export(ctx, ptraceotlp.NewExportRequestFromTraces(td))
	require.NoError(t, err)
}

// addSpan appends a resource with the given attributes holding one span with the given attributes.
func addSpan(td ptrace.Traces, resAttrs, spanAttrs map[string]any) {
	rs := td.ResourceSpans().AppendEmpty()
	_ = rs.Resource().Attributes().FromRaw(resAttrs)
	span := rs.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.SetName("redaction-test")
	span.SetTraceID([16]byte{1})
	span.SetSpanID([8]byte{byte(td.ResourceSpans().Len())})
	_ = span.Attributes().FromRaw(spanAttrs)
}

// assertSpanAttrs asserts the exact attributes of the only span of rs.
func assertSpanAttrs(t *testing.T, rs ptrace.ResourceSpans, want map[string]any) {
	require.Equal(t, 1, rs.ScopeSpans().Len())
	require.Equal(t, 1, rs.ScopeSpans().At(0).Spans().Len())
	assert.Equal(t, want, rs.ScopeSpans().At(0).Spans().At(0).Attributes().AsRaw())
}

// TestE2E_RedactionExample tests config_examples/redaction.yaml on span attributes.
func TestE2E_RedactionExample(t *testing.T) {
	grpcPort, sink := startCollector(t, "redaction.yaml", func(c *testutil.Config) {
		// The example also opens the OTLP/HTTP receiver, which is not used here.
		c.Replace("http receiver port", func(s string) string {
			return testutil.ReplaceOtlpHttpReceiverPort(s, testutil.GetAvailablePort(t))
		})
	})

	td := ptrace.NewTraces()
	addSpan(td, map[string]any{"service.name": "redaction-example"}, map[string]any{
		"description":    "card 4111111111111111",
		"name":           "user@example.com",
		"group":          "10.0.0.1",
		"safe_attribute": "4111111111111111", // ignored
		"other":          "value",            // not allowed
	})
	export(t, grpcPort, td)

	oteltest.WaitForTraces(t, 1, sink)
	require.Equal(t, 1, sink.SpanCount())
	rs := sink.AllTraces()[0].ResourceSpans().At(0)
	assertSpanAttrs(t, rs, map[string]any{
		"description":              "card ****",
		"name":                     "****",
		"group":                    "****",
		"safe_attribute":           "4111111111111111",
		"redaction.masked.count":   int64(3),
		"redaction.ignored.count":  int64(1),
		"redaction.redacted.count": int64(1),
	})
}
