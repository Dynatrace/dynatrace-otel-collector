package integration

import (
	"fmt"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

var _ testbed.TestCaseValidator = &MetricsValidator{}

type MetricsValidator struct {
	expectedMetrics []pmetric.Metrics
	t               *testing.T
}

func NewMetricValidator(t *testing.T, expectedMetrics []pmetric.Metrics) *MetricsValidator {
	return &MetricsValidator{
		expectedMetrics: expectedMetrics,
		t:               t,
	}
}

func (v *MetricsValidator) Validate(tc *testbed.TestCase) {
	assertExpectedMetricsAreInReceived(v.t, v.expectedMetrics, tc.MockBackend.ReceivedMetrics)
}

func (v *MetricsValidator) RecordResults(tc *testbed.TestCase) {
}

func assertExpectedMetricsAreInReceived(t *testing.T, expected, actual []pmetric.Metrics) {
	expectedMap := make(map[string]pmetric.Metrics)
	populateMetricsMap(expectedMap, expected)

	for _, td := range actual {
		resourceMetrics := td.ResourceMetrics()
		for i := 0; i < resourceMetrics.Len(); i++ {
			scopeMetrics := resourceMetrics.At(i).ScopeMetrics()
			for j := 0; j < scopeMetrics.Len(); j++ {
				metrics := scopeMetrics.At(j).Metrics()
				for k := 0; k < metrics.Len(); k++ {
					actualMetric := metrics.At(k)
					hasEntry := assert.Contains(t,
						expectedMap,
						actualMetric.Name(),
						fmt.Sprintf("Metric with name : %q not found among expected metrics", actualMetric.Name()))

					// avoid panic due to expectedMap being nil
					if !hasEntry {
						return
					}
					assert.Nil(t, pmetrictest.CompareMetrics(expectedMap[actualMetric.Name()], td), pmetrictest.IgnoreDatapointAttributesOrder())
				}
			}
		}
	}
}

func populateMetricsMap(metricsMap map[string]pmetric.Metrics, tds []pmetric.Metrics) {
	for _, td := range tds {
		resourceMetrics := td.ResourceMetrics()
		for i := 0; i < resourceMetrics.Len(); i++ {
			scopeMetrics := resourceMetrics.At(i).ScopeMetrics()
			for j := 0; j < scopeMetrics.Len(); j++ {
				metrics := scopeMetrics.At(j).Metrics()
				for k := 0; k < metrics.Len(); k++ {
					metric := metrics.At(k)
					key := metric.Name()
					metricsMap[key] = td
				}
			}
		}
	}
}
