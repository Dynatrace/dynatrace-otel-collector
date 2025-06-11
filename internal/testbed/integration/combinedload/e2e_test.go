//go:build e2e

package combinedload

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/k8stest"
	oteltest "github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/oteltest"
	"github.com/google/uuid"
	otelk8stest "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/xk8stest"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
)

func TestLoad_Combined(t *testing.T) {
	testDir := filepath.Join("testdata")

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
	// defer func() {
	// 	require.NoErrorf(t, otelk8stest.DeleteObject(k8sClient, nsObj), "failed to delete namespace %s", testNs)
	// }()

	tracesConsumer := new(consumertest.TracesSink)
	metricsConsumer := new(consumertest.MetricsSink)
	logsConsumer := new(consumertest.LogsSink)
	_ = oteltest.StartUpSinks(t, oteltest.ReceiverSinks{
		Traces: &oteltest.TraceSinkConfig{
			Consumer: tracesConsumer,
			Ports: &oteltest.ReceiverPorts{
				Grpc: 4327,
				Http: 4427,
			},
		},
		Metrics: &oteltest.MetricSinkConfig{
			Consumer: metricsConsumer,
			Ports: &oteltest.ReceiverPorts{
				Grpc: 4328,
				Http: 4428,
			},
		},
		Logs: &oteltest.LogSinkConfig{
			Consumer: logsConsumer,
			Ports: &oteltest.ReceiverPorts{
				Grpc: 4329,
				Http: 4429,
			},
		},
	})
	// defer func() {
	// 	// give some more time to the collector to finish exporting before stopping the sinks
	// 	// so we do not have any dropped data after the test is finished
	// 	time.Sleep(10 * time.Second)
	// 	shutdownSinks()
	// }()

	// start up metrics-server
	t.Log("deploying metrics-server...")
	err = k8stest.PerformOperationOnYAMLFiles(k8sClient, filepath.Join(testDir, "metrics-server"), otelk8stest.CreateObject)
	require.NoErrorf(t, err, "failed to create k8s metrics server")

	// defer func() {
	// 	err = k8stest.PerformOperationOnYAMLFiles(k8sClient, filepath.Join(testDir, "metrics-server"), k8stest.DeleteObjectFromManifest)
	// 	require.NoErrorf(t, err, "failed to delete k8s metrics server")
	// }()

	// wait for metrics server to be ready
	err = k8stest.WaitForDeploymentPods(k8sClient.DynamicClient, "kube-system", "metrics-server", 2*time.Minute)
	require.NoErrorf(t, err, "failed to rollout k8s metrics server")
	t.Log("metrics-server deployed")

	// get metrics client
	metricsClientSet, err := k8stest.NewMetricsClientSet()
	require.NoError(t, err)

	testID := uuid.NewString()[:8]
	_ = otelk8stest.CreateCollectorObjects(t, k8sClient, testID, filepath.Join(testDir, "collector"), map[string]string{
		"ContainerRegistry": os.Getenv("CONTAINER_REGISTRY"),
	}, "")

	createTeleOpts := &otelk8stest.TelemetrygenCreateOpts{
		ManifestsDir: filepath.Join(testDir, "telemetrygen"),
		TestID:       testID,
		OtlpEndpoint: fmt.Sprintf("otelcol-%s.%s:4317", testID, testNs),
		DataTypes:    []string{""},
	}

	_, telemetryGenObjInfos := otelk8stest.CreateTelemetryGenObjects(t, k8sClient, createTeleOpts)
	// defer func() {
	// 	for _, obj := range append(collectorObjs, telemetryGenObjs...) {
	// 		require.NoErrorf(t, otelk8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
	// 	}
	// }()

	for _, info := range telemetryGenObjInfos {
		otelk8stest.WaitForTelemetryGenToStart(t, k8sClient, info.Namespace, info.PodLabelSelectors, info.Workload, info.DataType)
	}

	otelColPodName, err := k8stest.GetPodNameByLabels(k8sClient.DynamicClient, testNs, map[string]string{
		"app.kubernetes.io/name": "opentelemetry-collector",
	})
	require.NoError(t, err)

	t.Log("collecting data...")
	ctx, _ := context.WithTimeout(context.Background(), 151*time.Second)
	//defer cancel()
	interval := 15 * time.Second
	ticker := time.NewTicker(interval)
	i := 0
	for {
		select {
		case <-ticker.C:
			i += 1
			//fetch metrics data
			cpu, mem, err := k8stest.FetchPodMetrics(metricsClientSet, testNs, otelColPodName)
			require.NoError(t, err)

			t.Log("------------------------------------------------------")
			t.Logf("data after %d seconds:", i*int(interval.Seconds()))
			t.Logf("memory: %s, cpu: %s", mem, cpu)
			t.Log("------------------------------------------------------")
		case <-ctx.Done():
			return
		}
	}
}
