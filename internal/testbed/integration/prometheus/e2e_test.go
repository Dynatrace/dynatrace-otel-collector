//go:build e2e

package prometheus

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/k8stest"
	oteltest "github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/oteltest"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
)

// TestE2E_PrometheusNodeExporter tests the "Scrape data from Prometheus" use case
// See: https://docs.dynatrace.com/docs/shortlink/otel-collector-cases-prometheus
func TestE2E_PrometheusNodeExporter(t *testing.T) {
	testDir := filepath.Join("testdata")

	k8sClient, err := k8stest.NewK8sClient()
	require.NoError(t, err)

	// Create the namespace specific for the test
	nsFile := filepath.Join(testDir, "namespace.yaml")
	buf, err := os.ReadFile(nsFile)
	require.NoErrorf(t, err, "failed to read namespace object file %s", nsFile)
	nsObj, err := k8stest.CreateObject(k8sClient, buf)
	require.NoErrorf(t, err, "failed to create k8s namespace from file %s", nsFile)

	testNs := nsObj.GetName()
	defer func() {
		require.NoErrorf(t, k8stest.DeleteObject(k8sClient, nsObj), "failed to delete namespace %s", testNs)
	}()

	// Install Prometheus Node exporter
	err = installPrometheusNodeExporter()
	require.NoErrorf(t, err, "failed to install Prometheus node exporter")

	metricsConsumer := new(consumertest.MetricsSink)
	shutdownSinks := oteltest.StartUpSinks(t, oteltest.ReceiverSinks{
		Metrics: &oteltest.MetricSinkConfig{
			Consumer: metricsConsumer,
		},
	})
	defer shutdownSinks()

	testID := uuid.NewString()[:8]
	collectorObjs := k8stest.CreateCollectorObjects(t, k8sClient, testID, filepath.Join(testDir, "collector"))
	defer func() {
		for _, obj := range collectorObjs {
			require.NoErrorf(t, k8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
		}
	}()

	wantEntries := 2 // Minimal number of metric requests to wait for.
	oteltest.WaitForMetrics(t, wantEntries, metricsConsumer)

	expectedColMetrics := []string{
		"otelcol_process_memory_rss", "scrape_duration_seconds", "scrape_samples_post_metric_relabeling",
	}
	oteltest.ScanForServiceMetrics(t, metricsConsumer, "opentelemetry-collector", expectedColMetrics)

	expectedPromMetrics := []string{
		"node_procs_running", "node_memory_MemAvailable_bytes",
	}
	oteltest.ScanForServiceMetrics(t, metricsConsumer, "node-exporter", expectedPromMetrics)
}

func installPrometheusNodeExporter() error {
	testDir := filepath.Join("testdata", "prometheus")
	nsFile := filepath.Join(testDir, "install.sh")
	cmd, err := exec.Command("/bin/bash", nsFile).Output()

	if err != nil {
		return fmt.Errorf("Failed to install Prometheus node exporter Helm chart %s", err)
	}

	// This is useful because it will print the output of
	// the Helm commands (from install.sh), showing that the Prometheus Node Exporter is running.
	fmt.Print(string(cmd))

	return nil
}
