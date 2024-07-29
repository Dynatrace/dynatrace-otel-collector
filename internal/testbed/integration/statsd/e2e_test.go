//go:build e2e

package statsd

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/k8stest"
	"github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/oteltest"
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
	shutdownSinks := oteltest.StartUpSinks(t, oteltest.ReceiverSinks{Metrics: metricsConsumer})
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

	oteltest.WaitForMetrics(t, 2, metricsConsumer)

	scanForServiceMetrics(t, metricsConsumer)
}

func scanForServiceMetrics(t *testing.T, ms *consumertest.MetricsSink) {
	for _, r := range ms.AllMetrics() {
		for i := 0; i < r.ResourceMetrics().Len(); i++ {
			sm := r.ResourceMetrics().At(i).ScopeMetrics().At(0).Metrics()
			assert.NoError(t, assertExpectedMetrics(sm))
		}
	}
}

func assertExpectedMetrics(sm pmetric.MetricSlice) error {
	expectedGaugeName := "test.metric"
	expectedGaugeVal := 42.0
	expectedGaugeAttrKey := "myKey"
	expectedgaugeAttrVal := "myVal"
	expectedTimerName := "timerMetric"
	expectedTimerCount := uint64(10)
	expectedTimerSum := 3200.0
	expectedTimerMin := 320.0
	expectedTimerMax := 320.0
	expectedTimerAttrKey := "timerKey"
	expectedTimerAttrVal := "timerVal"
	for i := 0; i < sm.Len(); i++ {
		if sm.At(i).Name() == expectedGaugeName {
			datapoint := sm.At(i).Gauge().DataPoints().At(0)
			if datapoint.DoubleValue() != expectedGaugeVal {
				return fmt.Errorf("Expected metric value %f, received %f", expectedGaugeVal, datapoint.DoubleValue())
			}
			val, ok := datapoint.Attributes().Get(expectedGaugeAttrKey)
			if !ok {
				return fmt.Errorf("Expected metric attribute not found")
			}
			if val.Str() != expectedgaugeAttrVal {
				return fmt.Errorf("Expected metric attribute value %s not found, got %s", expectedgaugeAttrVal, val.Str())
			}
		} else if sm.At(i).Name() == expectedTimerName {
			datapoint := sm.At(i).ExponentialHistogram().DataPoints().At(0)
			if datapoint.Count() != expectedTimerCount {
				return fmt.Errorf("Expected timer metric count %d, received %d", expectedTimerCount, datapoint.Count())
			}
			if datapoint.Max() != expectedTimerMax {
				return fmt.Errorf("Expected timer metric max %f, received %f", expectedTimerMax, datapoint.Max())
			}
			if datapoint.Min() != expectedTimerMin {
				return fmt.Errorf("Expected timer metric min %f, received %f", expectedTimerMin, datapoint.Min())
			}
			if datapoint.Sum() != expectedTimerSum {
				return fmt.Errorf("Expected timer metric sum %f, received %f", expectedTimerSum, datapoint.Sum())
			}
			val, ok := datapoint.Attributes().Get(expectedTimerAttrKey)
			if !ok {
				return fmt.Errorf("Expected timer metric attribute not found")
			}
			if val.Str() != expectedTimerAttrVal {
				return fmt.Errorf("Expected timer metric attribute value %s not found, got %s", expectedTimerAttrVal, val.Str())
			}
		} else {
			return fmt.Errorf("Unexpected metric received: %s", sm.At(i).Name())
		}
	}
	return nil
}
