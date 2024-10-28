package integration

import (
	"fmt"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

var _ testbed.TestCaseValidator = &MetricsValidator{}

func WithHiddenMetricsValidationErrorMessages() func(validator *MetricsValidator) {
	return func(v *MetricsValidator) {
		v.hideValidationErrorMessage = true
	}
}

type MetricsValidator struct {
	expectedMetrics            []pmetric.Metrics
	t                          *testing.T
	hideValidationErrorMessage bool
}

func NewMetricValidator(t *testing.T, expectedMetrics []pmetric.Metrics, opts ...func(validator *MetricsValidator)) *MetricsValidator {
	v := &MetricsValidator{
		expectedMetrics: expectedMetrics,
		t:               t,
	}

	for _, opt := range opts {
		opt(v)
	}

	return v
}

func (v *MetricsValidator) Validate(tc *testbed.TestCase) {
	assertExpectedMetricsAreInReceived(v.t, v.expectedMetrics, tc.MockBackend.ReceivedMetrics, v.hideValidationErrorMessage)
}

func (v *MetricsValidator) RecordResults(tc *testbed.TestCase) {
}

func assertExpectedMetricsAreInReceived(t *testing.T, expected, actual []pmetric.Metrics, hideError bool) {
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
					require.Contains(t,
						expectedMap,
						actualMetric.Name(),
						fmt.Sprintf("Metric with name : %q not found among expected metrics", actualMetric.Name()))

					err := pmetrictest.CompareMetrics(
						expectedMap[actualMetric.Name()],
						td,
						pmetrictest.IgnoreDatapointAttributesOrder(),
						pmetrictest.IgnoreStartTimestamp(),
					)

					// if hideError is set, the actual error message is not logged. This is for testing scenarios where we validate the
					// redaction of API Tokens in the attributes of the received data items.
					// If the redaction is not applied correctly, the original values would otherwise be visible in the logs,
					// which in turn might lead to false positive security alerts (the sample tokens are in an allow list, but just to be on the safe side).
					if hideError && err != nil {
						t.Error("Received metrics did not match expected metrics")
					} else {
						require.NoError(
							t,
							err,
						)
					}
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
