//go:build e2e

package zipkin

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/k8stest"
	"github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/oteltest"
	"github.com/google/uuid"
	otelk8stest "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/xk8stest"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
)

// TestE2E_ZipkinReceiver tests the "Ingest data from Zipkin" use case
// See: https://docs.dynatrace.com/docs/extend-dynatrace/opentelemetry/collector/use-cases/zipkin
func TestE2E_ZipkinReceiver(t *testing.T) {
	testDir := filepath.Join("testdata")
	configExamplesDir := "../../../../config_examples"

	k8sClient, err := otelk8stest.NewK8sClient(k8stest.TestKubeConfig)
	require.NoError(t, err)

	// Create the namespace specific for the test
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
		Traces: &oteltest.TraceSinkConfig{
			Consumer: tracesConsumer,
		},
	})
	defer shutdownSinks()

	testID := uuid.NewString()[:8]
	collectorConfigPath := path.Join(configExamplesDir, "zipkin.yaml")
	host := otelk8stest.HostEndpoint(t)
	collectorConfig, err := k8stest.GetCollectorConfig(collectorConfigPath, host)
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
	createZipkinOpts := &k8stest.ZipkinAppCreateOpts{
		ManifestsDir: filepath.Join(testDir, "zipkin"),
		TestID:       testID,
		OtlpEndpoint: fmt.Sprintf("otelcol-%s.%s:4317", testID, testNs),
	}

	zipkinObjs, zipkinObjInfos := k8stest.CreateZipkinAppObjects(t, k8sClient, createZipkinOpts)
	defer func() {
		for _, obj := range append(collectorObjs, zipkinObjs...) {
			require.NoErrorf(t, otelk8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
		}
	}()

	for _, info := range zipkinObjInfos {
		k8stest.WaitForZipkinAppToStart(t, k8sClient, info.Namespace, info.PodLabelSelectors)
	}

	wantEntries := 30 // Minimal number of traces to wait for.
	oteltest.WaitForTraces(t, wantEntries, tracesConsumer)

	tcs := []struct {
		name           string
		service        string
		attrs          map[string]oteltest.ExpectedValue
		scopeSpanAttrs []map[string]oteltest.ExpectedValue
	}{
		{
			name:    "frontend-traces",
			service: "frontend",
			scopeSpanAttrs: []map[string]oteltest.ExpectedValue{
				{
					"http.method": oteltest.NewExpectedValue(oteltest.AttributeMatchTypeEqual, "GET"),
					"http.path":   oteltest.NewExpectedValue(oteltest.AttributeMatchTypeEqual, "/"),
				},
				{
					"http.method":  oteltest.NewExpectedValue(oteltest.AttributeMatchTypeEqual, "GET"),
					"http.path":    oteltest.NewExpectedValue(oteltest.AttributeMatchTypeEqual, "/api"),
					"peer.service": oteltest.NewExpectedValue(oteltest.AttributeMatchTypeEqual, "backend"),
				},
			},
		},
		{
			name:    "backend-traces",
			service: "backend",
			scopeSpanAttrs: []map[string]oteltest.ExpectedValue{
				{
					"http.method": oteltest.NewExpectedValue(oteltest.AttributeMatchTypeEqual, "GET"),
					"http.path":   oteltest.NewExpectedValue(oteltest.AttributeMatchTypeEqual, "/api"),
				},
			},
		},
	}

	for _, tc := range tcs {
		oteltest.ScanTracesForAttributes(t, tracesConsumer, tc.service, tc.attrs, tc.scopeSpanAttrs)
	}
}
