//go:build e2e

package prometheus_large_scale

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/k8stest"
	oteltest "github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/oteltest"
	otelk8stest "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/xk8stest"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
)

const (
	testNamespace   = "otel-ta"
	selfmonGrpcPort = 4327
	selfmonHttpPort = 4328
)

// TestE2E_PrometheusLargeScale deploys the full tiered prometheus-large-scale
// setup (allocator + tier1-scraper + tier2-gateway + selfmon-scraper + avalanche)
// on a pre-existing Kind cluster and verifies that metrics flow through both the
// normal data path (avalanche → scraper → gateway → sink) and the self-monitoring
// path (selfmon-scraper → sink).
func TestE2E_PrometheusLargeScale(t *testing.T) {
	configDir, err := filepath.Abs("../../../../config_examples/prometheus-large-scale")
	require.NoError(t, err)

	kubeconfigPath := k8stest.KubeconfigFromEnvOrDefault()
	_, err = otelk8stest.NewK8sClient(kubeconfigPath)
	require.NoError(t, err)

	host := otelk8stest.HostEndpoint(t)
	containerRegistry := os.Getenv("CONTAINER_REGISTRY")

	// Sink 1: normal data from tier2-gateway (default ports 4317/4318)
	normalConsumer := new(consumertest.MetricsSink)
	// Sink 2: selfmon data from selfmon-scraper (ports 4327/4328)
	selfmonConsumer := new(consumertest.MetricsSink)

	shutdownSinks := oteltest.StartUpSinks(t, oteltest.ReceiverSinks{
		Metrics: []*oteltest.MetricSinkConfig{
			{
				Consumer: normalConsumer,
			},
			{
				Consumer: selfmonConsumer,
				Ports:    &oteltest.ReceiverPorts{Grpc: selfmonGrpcPort, Http: selfmonHttpPort},
			},
		},
	})
	defer shutdownSinks()

	// Register teardown first so it always runs, even if setup fails
	t.Cleanup(func() {
		if err := runTeardown(); err != nil {
			t.Logf("teardown warning: %v", err)
		}
	})

	// Deploy all components
	err = runSetup(configDir, host, containerRegistry)
	require.NoError(t, err, "failed to set up prometheus-large-scale components")

	// Wait for metrics on both sinks
	wantEntries := 5

	t.Log("Waiting for normal metrics from tier2-gateway...")
	oteltest.WaitForMetrics(t, wantEntries, normalConsumer)

	t.Log("Waiting for selfmon metrics from selfmon-scraper...")
	oteltest.WaitForMetrics(t, wantEntries, selfmonConsumer)

	// Verify expected scrape metadata metrics arrive on the normal data sink.
	// Use retry logic: not all metrics may appear in the first batch.
	expectedDataMetrics := []string{
		"up",
		"scrape_duration_seconds",
		"scrape_samples_scraped",
		"scrape_series_added",
		"scrape_samples_post_metric_relabeling",
	}
	requireMetricsEventually(t, normalConsumer, expectedDataMetrics, 5*time.Minute)

	require.Greater(t, len(selfmonConsumer.AllMetrics()), 0, "expected selfmon metrics from selfmon-scraper")
}

// requireMetricsEventually polls the sink until all expectedMetrics are found
// across any resource/scope, retrying for up to timeout.
func requireMetricsEventually(t *testing.T, sink *consumertest.MetricsSink, expectedMetrics []string, timeout time.Duration) {
	t.Helper()
	require.Eventuallyf(t, func() bool {
		found := make(map[string]bool, len(expectedMetrics))
		for _, md := range sink.AllMetrics() {
			for i := 0; i < md.ResourceMetrics().Len(); i++ {
				rm := md.ResourceMetrics().At(i)
				for j := 0; j < rm.ScopeMetrics().Len(); j++ {
					sm := rm.ScopeMetrics().At(j)
					for k := 0; k < sm.Metrics().Len(); k++ {
						found[sm.Metrics().At(k).Name()] = true
					}
				}
			}
		}
		for _, name := range expectedMetrics {
			if !found[name] {
				return false
			}
		}
		return true
	}, timeout, 2*time.Second,
		"not all expected metrics found in normal data sink within %v, want: %v", timeout, expectedMetrics)
}

func runSetup(configDir, host, containerRegistry string) error {
	scriptPath := filepath.Join("testdata", "scripts", "setup.sh")
	cmd := exec.Command("/bin/bash", scriptPath)
	cmd.Env = append(os.Environ(),
		"NAMESPACE="+testNamespace,
		"HOST="+host,
		"CONTAINER_REGISTRY="+containerRegistry,
		"CONFIG_DIR="+configDir,
		fmt.Sprintf("SELFMON_HTTP_PORT=%d", selfmonHttpPort),
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func runTeardown() error {
	scriptPath := filepath.Join("testdata", "scripts", "teardown.sh")
	cmd := exec.Command("/bin/bash", scriptPath)
	cmd.Env = append(os.Environ(),
		"NAMESPACE="+testNamespace,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
