//go:build e2e

package resource_detection

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

func TestE2E_ResourceDetectionProcessor(t *testing.T) {
	testDir := filepath.Join("testdata")
	configExamplesDir := "../../../../config_examples"

	kubeconfigPath := k8stest.TestKubeConfig
	if kubeConfigFromEnv := os.Getenv(k8stest.KubeConfigEnvVar); kubeConfigFromEnv != "" {
		kubeconfigPath = kubeConfigFromEnv
	}

	k8sClient, err := otelk8stest.NewK8sClient(kubeconfigPath)
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
	collectorConfigPath := path.Join(configExamplesDir, "resource-detection.yaml")
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

	createTeleOpts := &otelk8stest.TelemetrygenCreateOpts{
		ManifestsDir: filepath.Join(testDir, "telemetrygen"),
		TestID:       testID,
		OtlpEndpoint: fmt.Sprintf("otelcol-%s.%s:4317", testID, testNs),
		DataTypes:    []string{"traces"},
	}
	telemetryGenObjs, telemetryGenObjInfos := otelk8stest.CreateTelemetryGenObjects(t, k8sClient, createTeleOpts)
	defer func() {
		for _, obj := range append(collectorObjs, telemetryGenObjs...) {
			require.NoErrorf(t, otelk8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
		}
	}()

	for _, info := range telemetryGenObjInfos {
		otelk8stest.WaitForTelemetryGenToStart(t, k8sClient, info.Namespace, info.PodLabelSelectors, info.Workload, info.DataType)
	}

	wantEntries := 30 // Minimal number of traces to wait for.
	oteltest.WaitForTraces(t, wantEntries, tracesConsumer)

	tcs := []struct {
		name    string
		service string
		attrs   map[string]oteltest.ExpectedValue
	}{
		{
			name:    "traces-job",
			service: "test-traces-job",
			attrs: map[string]oteltest.ExpectedValue{
				"dt.entity.host": oteltest.NewExpectedValue(oteltest.AttributeMatchTypeEqual, "my-host-from-properties"),
				"host.name":      oteltest.NewExpectedValue(oteltest.AttributeMatchTypeEqual, "my-host-from-properties"),
			},
		},
		{
			name:    "traces-statefulset",
			service: "test-traces-statefulset",
			attrs: map[string]oteltest.ExpectedValue{
				"dt.entity.host": oteltest.NewExpectedValue(oteltest.AttributeMatchTypeEqual, "my-host-from-properties"),
				"host.name":      oteltest.NewExpectedValue(oteltest.AttributeMatchTypeEqual, "my-host-not-overridden"),
			},
		},
		{
			name:    "traces-deployment",
			service: "test-traces-deployment",
			attrs: map[string]oteltest.ExpectedValue{
				"dt.entity.host": oteltest.NewExpectedValue(oteltest.AttributeMatchTypeEqual, "my-host-from-properties"),
				"host.name":      oteltest.NewExpectedValue(oteltest.AttributeMatchTypeEqual, "my-host-from-properties"),
			},
		},
		{
			name:    "traces-daemonset",
			service: "test-traces-daemonset",
			attrs: map[string]oteltest.ExpectedValue{
				"dt.entity.host": oteltest.NewExpectedValue(oteltest.AttributeMatchTypeEqual, "my-host-from-properties"),
				"host.name":      oteltest.NewExpectedValue(oteltest.AttributeMatchTypeEqual, "my-host-from-properties"),
			},
		},
	}

	for _, tc := range tcs {
		oteltest.ScanTracesForAttributes(t, tracesConsumer, tc.service, tc.attrs, nil)
	}

}
