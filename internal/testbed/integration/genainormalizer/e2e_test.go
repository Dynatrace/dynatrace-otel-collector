// Copyright Dynatrace LLC
// SPDX-License-Identifier: Apache-2.0

//go:build e2e

package genainormalizer

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/k8stest"
	oteltest "github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/oteltest"
	"github.com/google/uuid"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/ptracetest"
	otelk8stest "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/xk8stest"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const collectorGRPCPort = "5317"

// TestE2E_GenAINormalizerProcessor verifies that the genainormalizer processor
// correctly maps both OpenInference and OpenLLMetry span attributes to gen_ai.*
// OTel semantic conventions. Crafted spans are sent directly to the collector
// via kubectl port-forward and compared against golden files.
func TestE2E_GenAINormalizerProcessor(t *testing.T) {
	testDir := filepath.Join("testdata")

	kubeconfigPath := k8stest.TestKubeConfig
	if kubeConfigFromEnv := os.Getenv(k8stest.KubeConfigEnvVar); kubeConfigFromEnv != "" {
		kubeconfigPath = kubeConfigFromEnv
	}

	k8sClient, err := otelk8stest.NewK8sClient(kubeconfigPath)
	require.NoError(t, err)

	nsFile := filepath.Join(testDir, "namespace.yaml")
	buf, err := os.ReadFile(nsFile)
	require.NoErrorf(t, err, "failed to read namespace object file %s", nsFile)
	nsObj, err := otelk8stest.CreateObject(k8sClient, buf)
	require.NoErrorf(t, err, "failed to create k8s namespace from file %s", nsFile)

	testNs := nsObj.GetName()
	defer func() {
		require.NoErrorf(t, otelk8stest.DeleteObject(k8sClient, nsObj), "failed to delete namespace %s", testNs)
	}()

	tracesConsumer := new(consumertest.TracesSink)
	shutdownSinks := oteltest.StartUpSinks(t, oteltest.ReceiverSinks{
		Traces: []*oteltest.TraceSinkConfig{
			{
				Consumer: tracesConsumer,
			},
		},
	})
	defer shutdownSinks()

	testID := uuid.NewString()[:8]
	host := otelk8stest.HostEndpoint(t)

	collectorConfigPath := filepath.Join("../../../../config_examples", "genainormalizer-openinference.yaml")
	collectorConfig, err := k8stest.GetCollectorConfig(collectorConfigPath, k8stest.ConfigTemplate{
		Host: host,
	})
	require.NoErrorf(t, err, "Failed to read collector config from file %s", collectorConfigPath)

	collectorObjs := otelk8stest.CreateCollectorObjects(
		t,
		k8sClient,
		testID,
		filepath.Join(testDir, "collector"),
		map[string]string{
			"ContainerRegistry": os.Getenv("CONTAINER_REGISTRY"),
			"CollectorConfig":   collectorConfig,
		},
		host,
	)
	defer func() {
		for _, obj := range collectorObjs {
			require.NoErrorf(t, otelk8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
		}
	}()

	// Port-forward the collector's gRPC receiver to localhost so the test can send spans directly.
	collectorSvc := fmt.Sprintf("svc/otelcol-%s", testID)
	pfCmd := exec.Command("kubectl", "--kubeconfig", kubeconfigPath,
		"port-forward", "-n", testNs,
		collectorSvc,
		collectorGRPCPort+":4317",
	)
	require.NoError(t, pfCmd.Start())
	t.Cleanup(func() {
		if pfCmd.Process != nil {
			_ = pfCmd.Process.Kill()
			_ = pfCmd.Wait()
		}
	})

	// Wait for port-forward to be ready.
	require.Eventually(t, func() bool {
		conn, err := net.DialTimeout("tcp", "localhost:"+collectorGRPCPort, time.Second)
		if err == nil {
			conn.Close()
			return true
		}
		return false
	}, 30*time.Second, 500*time.Millisecond, "port-forward to collector did not become ready")

	grpcConn, err := grpc.NewClient("localhost:"+collectorGRPCPort,
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer grpcConn.Close()

	traceClient := ptraceotlp.NewGRPCClient(grpcConn)

	// Send both OpenInference and OpenLLMetry spans in a single export so ordering is deterministic.
	_, err = traceClient.Export(context.Background(), ptraceotlp.NewExportRequestFromTraces(buildTestTraces()))
	require.NoError(t, err)

	oteltest.WaitForTraces(t, 0, tracesConsumer)

	traceCompareOptions := []ptracetest.CompareTracesOption{
		ptracetest.IgnoreStartTimestamp(),
		ptracetest.IgnoreEndTimestamp(),
		ptracetest.IgnoreTraceID(),
		ptracetest.IgnoreSpanID(),
		ptracetest.IgnoreResourceSpansOrder(),
		ptracetest.IgnoreScopeSpansOrder(),
		ptracetest.IgnoreSpansOrder(),
	}

	// To regenerate the golden file: uncomment the WriteTraces line, run once, then revert.
	// require.Nil(t, golden.WriteTraces(t, filepath.Join(testDir, "e2e", "expected-traces.yaml"), tracesConsumer.AllTraces()[0]))

	expected, err := golden.ReadTraces(filepath.Join(testDir, "e2e", "expected-traces.yaml"))
	require.NoError(t, err)
	require.NoError(t, ptracetest.CompareTraces(expected, tracesConsumer.AllTraces()[0], traceCompareOptions...))
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
