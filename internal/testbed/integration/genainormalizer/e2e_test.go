//go:build e2e

package genainormalizer

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/k8stest"
	oteltest "github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/oteltest"
	"github.com/google/uuid"
	otelk8stest "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/xk8stest"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
)

// TestE2E_GenAINormalizerProcessor_OpenInference verifies that the genainormalizer
// processor correctly maps OpenInference span attributes to gen_ai.* semantic conventions.
// It covers both openai-openinference and aws-bedrock-openinference attribute shapes,
// which use the same llm.* attribute names but different model values.
func TestE2E_GenAINormalizerProcessor_OpenInference(t *testing.T) {
	testDir := filepath.Join("testdata")
	configExamplesDir := "../../../../config_examples"

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

	collectorConfigPath := path.Join(configExamplesDir, "genainormalizer-openinference.yaml")
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

	testAppObjs := otelk8stest.CreateCollectorObjects(
		t,
		k8sClient,
		testID,
		filepath.Join(testDir, "testapp"),
		map[string]string{
			"ContainerRegistry": os.Getenv("CONTAINER_REGISTRY"),
			"CollectorEndpoint": fmt.Sprintf("http://otelcol-%s.%s:4318", testID, testNs),
		},
		host,
	)

	defer func() {
		for _, obj := range append(collectorObjs, testAppObjs...) {
			require.NoErrorf(t, otelk8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
		}
	}()

	// Wait for normalised traces from the test app. The app emits spans for both
	// gpt-4o (OpenAI shape) and anthropic.claude-3-sonnet-20240229-v1:0 (Bedrock shape).
	const wantEntries = 10
	oteltest.WaitForTraces(t, wantEntries, tracesConsumer)

	oteltest.ScanTracesForAttributes(
		t,
		tracesConsumer,
		"test-genainormalizer-openinference",
		map[string]oteltest.ExpectedValue{
			"service.name": oteltest.NewExpectedValue(oteltest.AttributeMatchTypeEqual, "test-genainormalizer-openinference"),
		},
		[]map[string]oteltest.ExpectedValue{
			// OpenAI OpenInference shape: llm.model_name=gpt-4o → gen_ai.request.model=gpt-4o
			{
				"gen_ai.request.model":  oteltest.NewExpectedValue(oteltest.AttributeMatchTypeEqual, "gpt-4o"),
				"gen_ai.response.model": oteltest.NewExpectedValue(oteltest.AttributeMatchTypeEqual, "gpt-4o"),
			},
			// Bedrock OpenInference shape: llm.model_name=anthropic.claude-... → gen_ai.request.model=anthropic.claude-...
			{
				"gen_ai.request.model":  oteltest.NewExpectedValue(oteltest.AttributeMatchTypeEqual, "anthropic.claude-3-sonnet-20240229-v1:0"),
				"gen_ai.response.model": oteltest.NewExpectedValue(oteltest.AttributeMatchTypeEqual, "anthropic.claude-3-sonnet-20240229-v1:0"),
			},
		},
	)
}
