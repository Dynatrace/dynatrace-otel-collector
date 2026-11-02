// Copyright Dynatrace LLC
// SPDX-License-Identifier: Apache-2.0

package genainormalizer

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/oteltest"
	"github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/ptracetest"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"
)

// startCollector starts the DT Collector with config_examples/genainormalizer-openinference.yaml,
// adapted to run locally. It returns the port of the OTLP gRPC receiver and the sink the
// Collector exports to.
func startCollector(t *testing.T) (int, *consumertest.TracesSink) {
	sink, sinkPorts := oteltest.StartTracesSink(t)
	grpcPort := testutil.GetAvailablePort(t)
	httpPort := testutil.GetAvailablePort(t)
	healthPort := testutil.GetAvailablePort(t)

	cfg := testutil.LoadConfigExample(t, "genainormalizer-openinference.yaml").
		Replace("OTLP gRPC receiver port", func(s string) string { return testutil.ReplaceOtlpGrpcReceiverPort(s, grpcPort) }).
		Replace("OTLP HTTP receiver port", func(s string) string { return testutil.ReplaceOtlpHttpReceiverPort(s, httpPort) }).
		Replace("health_check port", func(s string) string { return testutil.ReplaceHealthCheckPort(s, healthPort) }).
		Replace("exporter endpoint", func(s string) string { return testutil.ReplaceDynatraceExporterEndpoint(s, sinkPorts.Http) }).
		Replace("sending_queue and retry_on_failure", func(s string) string { return testutil.DisableSendingQueueAndRetry(s, "otlp_http") }).
		Replace("internal metrics", testutil.DisableInternalMetrics)

	testutil.StartCollector(t, testutil.CollectorParams{
		Name:      "genainormalizer",
		Config:    cfg.String(),
		ReadyPort: grpcPort,
	})
	return grpcPort, sink
}

// TestE2E_GenAINormalizerProcessor verifies that the genainormalizer processor
// correctly maps both OpenInference and OpenLLMetry span attributes to gen_ai.*
// OTel semantic conventions. Crafted spans are sent directly to a local Collector
// and compared against golden files.
func TestE2E_GenAINormalizerProcessor(t *testing.T) {
	grpcPort, sink := startCollector(t)
	traceClient := oteltest.NewTracesClient(t, grpcPort)

	// Send both OpenInference and OpenLLMetry spans in a single export so ordering is deterministic.
	_, err := traceClient.Export(context.Background(), ptraceotlp.NewExportRequestFromTraces(buildTestTraces()))
	require.NoError(t, err)

	oteltest.WaitForTraces(t, 1, sink)
	require.Len(t, sink.AllTraces(), 1)

	traceCompareOptions := []ptracetest.CompareTracesOption{
		ptracetest.IgnoreStartTimestamp(),
		ptracetest.IgnoreEndTimestamp(),
		ptracetest.IgnoreTraceID(),
		ptracetest.IgnoreSpanID(),
		ptracetest.IgnoreResourceSpansOrder(),
		ptracetest.IgnoreScopeSpansOrder(),
		ptracetest.IgnoreSpansOrder(),
	}

	goldenPath := filepath.Join("testdata", "e2e", "expected-traces.yaml")
	// To regenerate the golden file: uncomment the WriteTraces line, run once, then revert.
	// require.Nil(t, golden.WriteTraces(t, goldenPath, sink.AllTraces()[0]))

	expected, err := golden.ReadTraces(goldenPath)
	require.NoError(t, err)
	require.NoError(t, ptracetest.CompareTraces(expected, sink.AllTraces()[0], traceCompareOptions...))
}

// buildTestTraces returns a ptrace.Traces with two resource spans — one carrying
// OpenInference llm.* attributes and one carrying OpenLLMetry llm.* attributes —
// sent in a single export so ordering at the sink is deterministic.
func buildTestTraces() ptrace.Traces {
	td := ptrace.NewTraces()

	// OpenInference span
	rsOI := td.ResourceSpans().AppendEmpty()
	rsOI.Resource().Attributes().PutStr("service.name", "test-llm-service")
	ssOI := rsOI.ScopeSpans().AppendEmpty()
	spanOI := ssOI.Spans().AppendEmpty()
	spanOI.SetName("llm-call")
	spanOI.SetKind(ptrace.SpanKindInternal)
	spanOI.SetTraceID(pcommon.TraceID([16]byte{1}))
	spanOI.SetSpanID(pcommon.SpanID([8]byte{1}))
	attrsOI := spanOI.Attributes()
	attrsOI.PutStr("llm.model_name", "gpt-4o")
	attrsOI.PutStr("llm.provider", "openai")
	attrsOI.PutStr("openinference.span.kind", "LLM")
	attrsOI.PutInt("llm.token_count.prompt", 10)
	attrsOI.PutInt("llm.token_count.completion", 5)
	attrsOI.PutStr("llm.input_messages.0.message.role", "user")
	attrsOI.PutStr("llm.input_messages.0.message.content", "What is the weather in Paris?")
	attrsOI.PutStr("llm.output_messages.0.message.role", "assistant")
	attrsOI.PutStr("llm.output_messages.0.message.content", "The weather in Paris is sunny.")
	attrsOI.PutStr("agent.name", "weather-agent")
	attrsOI.PutStr("session.id", "session-abc123")

	// OpenLLMetry span
	rsOL := td.ResourceSpans().AppendEmpty()
	rsOL.Resource().Attributes().PutStr("service.name", "test-llm-service")
	ssOL := rsOL.ScopeSpans().AppendEmpty()
	spanOL := ssOL.Spans().AppendEmpty()
	spanOL.SetName("llm-call")
	spanOL.SetKind(ptrace.SpanKindInternal)
	spanOL.SetTraceID(pcommon.TraceID([16]byte{2}))
	spanOL.SetSpanID(pcommon.SpanID([8]byte{2}))
	attrsOL := spanOL.Attributes()
	attrsOL.PutStr("llm.request.model", "gpt-4o")
	attrsOL.PutStr("llm.response.model", "gpt-4o")
	// OpenLLMetry sets gen_ai.system directly; no normalizer mapping exists for provider.name
	attrsOL.PutStr("gen_ai.system", "openai")
	attrsOL.PutStr("llm.request.type", "chat")
	attrsOL.PutInt("llm.usage.prompt_tokens", 10)
	attrsOL.PutInt("llm.usage.completion_tokens", 5)
	attrsOL.PutDouble("llm.request.temperature", 0.7)
	attrsOL.PutStr("traceloop.entity.input", `{"messages":[{"role":"user","content":"What is the weather in Paris?"}]}`)
	attrsOL.PutStr("traceloop.entity.output", `{"choices":[{"message":{"role":"assistant","content":"The weather in Paris is sunny."}}]}`)
	attrsOL.PutStr("traceloop.entity.name", "weather-agent")

	return td
}
