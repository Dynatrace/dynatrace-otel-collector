package loadtest

import (
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
)

var (
	performanceResultsSummary testbed.TestResultsSummary = &testbed.PerformanceResults{}
)

// TestMain is used to initiate setup, execution and tear down of testbed.
func TestMain(m *testing.M) {
	testbed.DoTestMain(m, performanceResultsSummary)
}
