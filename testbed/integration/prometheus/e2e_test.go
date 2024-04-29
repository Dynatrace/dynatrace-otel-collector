package k8senrichment

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/otlpreceiver"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/Dynatrace/dynatrace-otel-collector/internal/k8stest"
)

const (
	testKubeConfig = "/home/grassi/.kube/config"
)

// TestE2E_PrometheusNodeExporter tests the "Scrape data from Prometheus" use case
// See: https://docs.dynatrace.com/docs/shortlink/otel-collector-cases-prometheus
func TestE2E_PrometheusNodeExporter(t *testing.T) {
	testDir := filepath.Join("testdata")

	k8sClient, err := k8stest.NewK8sClient(testKubeConfig)
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
	installPrometheusNodeExporter()

	metricsConsumer := new(consumertest.MetricsSink)
	shutdownSinks := startUpSinks(t, metricsConsumer)
	defer shutdownSinks()

	testID := uuid.NewString()[:8]
	collectorObjs := k8stest.CreateCollectorObjects(t, k8sClient, testID, filepath.Join(testDir, "collector"))
	defer func() {
		for _, obj := range collectorObjs {
			require.NoErrorf(t, k8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
		}
	}()

	wantEntries := 2 // Minimal number of metric requests to wait for.
	waitForData(t, wantEntries, metricsConsumer)

	expectedMetrics := []string{
		"otelcol_process_memory_rss", "scrape_duration_seconds", "scrape_samples_post_metric_relabeling",
	}

	scanForServiceMetrics(t, metricsConsumer, "opentelemetry-collector", expectedMetrics)
}

func installPrometheusNodeExporter() error {
	testDir := filepath.Join("testdata", "prometheus")
	nsFile := filepath.Join(testDir, "install.sh")
	cmd, err := exec.Command("/bin/bash", nsFile).Output()

	if err != nil {
		return fmt.Errorf("Failed to install Prometheus node exporter Helm chart %s", err)
	}

	fmt.Print(string(cmd))

	return nil
}

func scanForServiceMetrics(t *testing.T, ms *consumertest.MetricsSink, expectedService string,
	expectedMetrics []string) {

	for _, r := range ms.AllMetrics() {
		for i := 0; i < r.ResourceMetrics().Len(); i++ {
			resource := r.ResourceMetrics().At(i).Resource()
			service, exist := resource.Attributes().Get("service.name")
			assert.Equal(t, true, exist, "resource does not have the 'service.name' attribute")
			if service.AsString() != expectedService {
				continue
			}

			sm := r.ResourceMetrics().At(i).ScopeMetrics().At(0).Metrics()
			assert.NoError(t, assertExpectedMetrics(expectedMetrics, sm))
			return
		}
	}
	t.Fatalf("no metric found for service %s", expectedService)
}

func assertExpectedMetrics(expectedMetrics []string, sm pmetric.MetricSlice) error {
	var actualMetrics []string
	for i := 0; i < sm.Len(); i++ {
		actualMetrics = append(actualMetrics, sm.At(i).Name())
	}

	for _, m := range expectedMetrics {
		if !slices.Contains(actualMetrics, m) {
			return fmt.Errorf("Metric: %s not found", m)
		}
	}
	return nil
}

func startUpSinks(t *testing.T, mc *consumertest.MetricsSink) func() {
	f := otlpreceiver.NewFactory()
	cfg := f.CreateDefaultConfig().(*otlpreceiver.Config)

	rcvr, err := f.CreateMetricsReceiver(context.Background(), receivertest.NewNopCreateSettings(), cfg, mc)
	require.NoError(t, err, "failed creating metrics receiver")

	require.NoError(t, rcvr.Start(context.Background(), componenttest.NewNopHost()))
	return func() {
		assert.NoError(t, rcvr.Shutdown(context.Background()))
	}
}

func waitForData(t *testing.T, entriesNum int, mc *consumertest.MetricsSink) {
	timeoutMinutes := 5
	require.Eventuallyf(t, func() bool {
		return len(mc.AllMetrics()) > entriesNum
	}, time.Duration(timeoutMinutes)*time.Minute, 1*time.Second,
		"failed to receive %d entries,  received %d metrics in %d minutes", entriesNum,
		len(mc.AllMetrics()), timeoutMinutes)
}
