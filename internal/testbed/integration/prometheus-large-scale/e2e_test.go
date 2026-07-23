// Copyright Dynatrace LLC
// SPDX-License-Identifier: Apache-2.0

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
	"github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetricassert"
	otelk8stest "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/xk8stest"

	// "github.com/stretchr/testify/assert"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

const (
	testNamespace = "otel-ta"
	// Selfmon sink ports (HTTP only — exporters use otlphttp)
	selfmonScraperGrpcPort   = 4327
	selfmonScraperHttpPort   = 4328
	selfmonGatewayGrpcPort   = 4329
	selfmonGatewayHttpPort   = 4330
	selfmonAllocatorGrpcPort = 4331
	selfmonAllocatorHttpPort = 4332
)

// TestE2E_PrometheusLargeScale deploys the full tiered prometheus-large-scale
// setup (allocator + tier1-scraper + tier2-gateway + selfmon-scraper + avalanche)
// on a pre-existing Kind cluster and verifies that metrics flow through both the
// normal data path (avalanche → scraper → gateway → sink) and the self-monitoring
// path (selfmon-scraper → 3 separate sinks, one per source).
func TestE2E_PrometheusLargeScale(t *testing.T) {
	configDir, err := filepath.Abs("../../../../config_examples/prometheus-large-scale")
	require.NoError(t, err)

	helmOverrideDir, err := filepath.Abs("./testdata/helm")
	require.NoError(t, err)

	kubeconfigPath := k8stest.KubeconfigFromEnvOrDefault()
	_, err = otelk8stest.NewK8sClient(kubeconfigPath)
	require.NoError(t, err)

	host := otelk8stest.HostEndpoint(t)
	containerRegistry := os.Getenv("CONTAINER_REGISTRY")

	// Sink 1: normal data from tier2-gateway (default ports 4317/4318)
	normalConsumer := new(consumertest.MetricsSink)
	// Sink 2: selfmon data from tier1-scraper (ports 4327/4328)
	scraperSelfmonConsumer := new(consumertest.MetricsSink)
	// Sink 3: selfmon data from tier2-gateway (ports 4329/4330)
	gatewaySelfmonConsumer := new(consumertest.MetricsSink)
	// Sink 4: selfmon data from allocator (ports 4331/4332)
	allocatorSelfmonConsumer := new(consumertest.MetricsSink)

	shutdownSinks := oteltest.StartUpSinks(t, oteltest.ReceiverSinks{
		Metrics: []*oteltest.MetricSinkConfig{
			{
				Consumer: normalConsumer,
			},
			{
				Consumer: scraperSelfmonConsumer,
				Ports:    &oteltest.ReceiverPorts{Grpc: selfmonScraperGrpcPort, Http: selfmonScraperHttpPort},
			},
			{
				Consumer: gatewaySelfmonConsumer,
				Ports:    &oteltest.ReceiverPorts{Grpc: selfmonGatewayGrpcPort, Http: selfmonGatewayHttpPort},
			},
			{
				Consumer: allocatorSelfmonConsumer,
				Ports:    &oteltest.ReceiverPorts{Grpc: selfmonAllocatorGrpcPort, Http: selfmonAllocatorHttpPort},
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
	err = runSetup(configDir, helmOverrideDir, host, containerRegistry)
	require.NoError(t, err, "failed to set up prometheus-large-scale components")

	// Wait for metrics on all sinks
	wantEntries := 1

	t.Log("Waiting for scraper selfmon metrics...")
	oteltest.WaitForMetrics(t, wantEntries, scraperSelfmonConsumer)

	t.Log("Waiting for gateway selfmon metrics...")
	oteltest.WaitForMetrics(t, wantEntries, gatewaySelfmonConsumer)

	t.Log("Waiting for allocator selfmon metrics...")
	oteltest.WaitForMetrics(t, wantEntries, allocatorSelfmonConsumer)

	// Validate each selfmon source independently
	t.Log("Validating scraper selfmon metrics...")
	validateSelfmonSource(t, scraperSelfmonConsumer, "./testdata/e2e/expected-selfmon-scraper.assert.yaml")

	t.Log("Validating gateway selfmon metrics...")
	validateSelfmonSource(t, gatewaySelfmonConsumer, "./testdata/e2e/expected-selfmon-gateway.assert.yaml")

	t.Log("Validating allocator selfmon metrics...")
	validateSelfmonSource(t, allocatorSelfmonConsumer, "./testdata/e2e/expected-selfmon-allocator.assert.yaml")

	t.Log("Waiting for avalanche metrics from gateway...")
	oteltest.WaitForMetrics(t, wantEntries, normalConsumer)

	// Verify expected scrape metadata metrics arrive on the normal data sink.
	expectedDataMetrics := []string{
		"up",
		"scrape_duration_seconds",
		"scrape_samples_scraped",
		"scrape_series_added",
		"scrape_samples_post_metric_relabeling",
	}

	t.Log("Checking avalanche metrics from gateway...")
	requireMetricsEventually(t, normalConsumer, expectedDataMetrics, 5*time.Minute)
}

func validateSelfmonSource(t *testing.T, consumer *consumertest.MetricsSink, assertFile string) {
	t.Helper()

	resourceIgnoreList := []string{
		"k8s.cluster.uid",
		"k8s.node.name",
		"k8s.pod.ip",
		"k8s.pod.name",
		"k8s.pod.uid",
		"k8s.workload.uid",
		"service.version",
		"server.address",
		"server.port",
		"service.instance.id",
		"telemetry.sdk.version",
	}

	dpIgnoreList := []string{
		"k8s_pod_ip",
		"k8s_pod_name",
		"net.peer.name",
		"server.address",
		"server.port",
		"version",
		"collector_name",
		"endpoint",
		"pod_identifier",
		"status",
	}

	// To regenerate: uncomment, run the test once, re-comment.
	// actual := mergeAllMetrics(consumer.AllMetrics())
	// testutil.ReplaceAttrValsWithStar(actual, resourceIgnoreList, dpIgnoreList)
	// require.NoError(t, pmetricassert.WriteAssertionFile(t, assertFile, actual))

	require.EventuallyWithT(t, func(tt *assert.CollectT) {
		actual := mergeAllMetrics(consumer.AllMetrics())
		testutil.ReplaceAttrValsWithStar(actual, resourceIgnoreList, dpIgnoreList)
		testutil.DeduplicateResources(actual)
		err := pmetricassert.AssertMetrics(assertFile, actual)
		assert.NoError(tt, err)
	}, 3*time.Minute, 1*time.Second)
}

// mergeAllMetrics combines multiple pmetric.Metrics batches into a single one
// so that pmetricassert.AssertMetrics can compare all accumulated data at once.
func mergeAllMetrics(batches []pmetric.Metrics) pmetric.Metrics {
	merged := pmetric.NewMetrics()
	for _, batch := range batches {
		for i := 0; i < batch.ResourceMetrics().Len(); i++ {
			batch.ResourceMetrics().At(i).CopyTo(merged.ResourceMetrics().AppendEmpty())
		}
	}
	return merged
}

// requireMetricsEventually polls the sink until all expectedMetrics are found
// across any resource/scope in any batch, retrying for up to timeout.
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
		"not all expected metrics found in sink within %v, want: %v", timeout, expectedMetrics)
}

func runSetup(configDir, helmOverrideDir, host, containerRegistry string) error {
	scriptPath := filepath.Join("testdata", "scripts", "setup.sh")
	cmd := exec.Command("/bin/bash", scriptPath)
	cmd.Env = append(os.Environ(),
		"NAMESPACE="+testNamespace,
		"HOST="+host,
		"CONTAINER_REGISTRY="+containerRegistry,
		"CONFIG_DIR="+configDir,
		"HELM_OVERRIDE_DIR="+helmOverrideDir,
		fmt.Sprintf("SELFMON_SCRAPER_HTTP_PORT=%d", selfmonScraperHttpPort),
		fmt.Sprintf("SELFMON_GATEWAY_HTTP_PORT=%d", selfmonGatewayHttpPort),
		fmt.Sprintf("SELFMON_ALLOCATOR_HTTP_PORT=%d", selfmonAllocatorHttpPort),
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
