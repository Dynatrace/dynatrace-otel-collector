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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const collectorGRPCPort = "5317"

// TestE2E_GenAINormalizerProcessor_OpenInference verifies that the genainormalizer
// processor correctly maps OpenInference span attributes to gen_ai.* semantic conventions.
// The test sends crafted spans with OpenInference-style attributes directly to the
// collector via kubectl port-forward, then compares the normalized output against a
// golden file.
func TestE2E_GenAINormalizerProcessor_OpenInference(t *testing.T) {
	testDir := filepath.Join("testdata")
	expectedTracesFile := filepath.Join(testDir, "e2e", "expected-traces.yaml")

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

	collectorConfigPath := filepath.Join(testDir, "config.yaml")
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

	// Send crafted spans with OpenInference attributes to the collector.
	grpcConn, err := grpc.NewClient("localhost:"+collectorGRPCPort,
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer grpcConn.Close()

	traceClient := ptraceotlp.NewGRPCClient(grpcConn)
	traces := buildOpenInferenceTraces()
	_, err = traceClient.Export(context.Background(), ptraceotlp.NewExportRequestFromTraces(traces))
	require.NoError(t, err)

	oteltest.WaitForTraces(t, 0, tracesConsumer)

	// To regenerate the golden file: uncomment the WriteTraces line, run once, then revert.
	require.Nil(t, golden.WriteTraces(t, expectedTracesFile, tracesConsumer.AllTraces()[0]))

	expectedTraces, err := golden.ReadTraces(expectedTracesFile)
	require.NoError(t, err)

	traceCompareOptions := []ptracetest.CompareTracesOption{
		ptracetest.IgnoreStartTimestamp(),
		ptracetest.IgnoreEndTimestamp(),
		ptracetest.IgnoreTraceID(),
		ptracetest.IgnoreSpanID(),
		ptracetest.IgnoreResourceSpansOrder(),
		ptracetest.IgnoreScopeSpansOrder(),
		ptracetest.IgnoreSpansOrder(),
	}

	const (
		compareTimeout = 3 * time.Minute
		compareTick    = 5 * time.Second
	)

	require.EventuallyWithT(t, func(tt *assert.CollectT) {
		assert.NoError(tt, ptracetest.CompareTraces(expectedTraces, tracesConsumer.AllTraces()[0], traceCompareOptions...))
	}, compareTimeout, compareTick)
}

// buildOpenInferenceTraces returns a ptrace.Traces with one span carrying
// OpenInference-style llm.* attributes. The genainormalizer processor should
// map these to gen_ai.* OTel semantic conventions.
func buildOpenInferenceTraces() ptrace.Traces {
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("service.name", "test-llm-service")

	ss := rs.ScopeSpans().AppendEmpty()
	span := ss.Spans().AppendEmpty()
	span.SetName("llm-call")
	span.SetKind(ptrace.SpanKindInternal)
	span.SetTraceID(pcommon.TraceID([16]byte{1}))
	span.SetSpanID(pcommon.SpanID([8]byte{1}))

	attrs := span.Attributes()
	attrs.PutStr("llm.model_name", "gpt-4o")
	attrs.PutStr("llm.provider", "openai")
	attrs.PutStr("openinference.span.kind", "LLM")
	attrs.PutInt("llm.token_count.prompt", 10)
	attrs.PutInt("llm.token_count.completion", 5)

	return td
}
