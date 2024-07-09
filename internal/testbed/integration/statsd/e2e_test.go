package statsd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
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
	equal = iota
	regex
	exist
	testKubeConfig   = "/tmp/kube-config-collector-e2e-testing"
	kubeConfigEnvVar = "KUBECONFIG"
	uidRe            = "[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}"
)

// TestE2E_StatsdReceiver tests the "Ingest data from Statsd" use case
// See: https://docs.dynatrace.com/docs/extend-dynatrace/opentelemetry/collector/use-cases/statsd
func TestE2E_StatsdReceiver(t *testing.T) {
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

	metricsConsumer := new(consumertest.MetricsSink)
	shutdownSinks := startUpSinks(t, metricsConsumer)
	defer shutdownSinks()

	// create collector
	testID := uuid.NewString()[:8]
	collectorObjs := k8stest.CreateCollectorObjects(t, k8sClient, testID, filepath.Join(testDir, "collector"))

	// create job
	jobFile := filepath.Join(testDir, "statsd", "job.yaml")
	buf, err = os.ReadFile(jobFile)
	require.NoErrorf(t, err, "failed to read job object file %s", jobFile)
	jobObj, err := k8stest.CreateObject(k8sClient, buf)
	require.NoErrorf(t, err, "failed to create k8s job from file %s", nsFile)

	defer func() {
		for _, obj := range append(collectorObjs, jobObj) {
			require.NoErrorf(t, k8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
		}
	}()

	wantEntries := 1
	waitForData(t, wantEntries, metricsConsumer)

	scanForServiceMetrics(t, metricsConsumer)
}

func startUpSinks(t *testing.T, mc *consumertest.MetricsSink) func() {
	f := otlpreceiver.NewFactory()
	cfg := f.CreateDefaultConfig().(*otlpreceiver.Config)

	cfg.GRPC.NetAddr.Endpoint = "0.0.0.0:4317"
	cfg.HTTP.Endpoint = "0.0.0.0:4318"

	rcvr, err := f.CreateMetricsReceiver(context.Background(), receivertest.NewNopSettings(), cfg, mc)
	require.NoError(t, err, "failed creating metrics receiver")

	require.NoError(t, rcvr.Start(context.Background(), componenttest.NewNopHost()))
	return func() {
		assert.NoError(t, rcvr.Shutdown(context.Background()))
	}
}

func waitForData(t *testing.T, entriesNum int, mc *consumertest.MetricsSink) {
	timeoutMinutes := 1
	require.Eventuallyf(t, func() bool {
		return len(mc.AllMetrics()) >= entriesNum
	}, time.Duration(timeoutMinutes)*time.Minute, 1*time.Second,
		"failed to receive %d entries,  received %d metrics in %d minutes", entriesNum,
		len(mc.AllMetrics()), timeoutMinutes)
}

func scanForServiceMetrics(t *testing.T, ms *consumertest.MetricsSink) {
	for _, r := range ms.AllMetrics() {
		for i := 0; i < r.ResourceMetrics().Len(); i++ {
			sm := r.ResourceMetrics().At(i).ScopeMetrics().At(0).Metrics()
			assert.NoError(t, assertExpectedMetrics(sm))
			return
		}
	}
	t.Fatalf("no metric found")
}

func assertExpectedMetrics(sm pmetric.MetricSlice) error {
	expectedName := "test.metric"
	expectedVal := 42.0
	expectedAttrKey := "myKey"
	expectedAttrVal := "myVal"
	for i := 0; i < sm.Len(); i++ {
		if sm.At(i).Name() == expectedName {
			datapoint := sm.At(i).Gauge().DataPoints().At(0)
			if datapoint.DoubleValue() != expectedVal {
				return fmt.Errorf("Expected metric value %f, received %f", expectedVal, datapoint.DoubleValue())
			}
			val, ok := datapoint.Attributes().Get(expectedAttrKey)
			if !ok {
				return fmt.Errorf("Expected metric attribute not found")
			}
			if val.Str() != expectedAttrVal {
				return fmt.Errorf("Expected metric attribute value %s not found, got %s", expectedAttrVal, val.Str())
			}
			return nil
		}
	}
	return fmt.Errorf("Expected metric not found")
}
