package loadtest

import (
	"testing"

	"github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
)

var processors = map[string]string{
	"batch": `
  batch:
    send_batch_max_size: 1000
    timeout: 10s
    send_batch_size : 800
`,
}

func TestTrace10kSPS(t *testing.T) {
	limitProcessors := map[string]string{
		"memory_limiter": `
  memory_limiter:
    check_interval: 1s
    limit_percentage: 100
`,
	}

	limitProcessors["batch"] = processors["batch"]

	tests := []struct {
		name         string
		sender       testbed.DataSender
		receiver     testbed.DataReceiver
		resourceSpec testbed.ResourceSpec
		processors   map[string]string
	}{
		{
			"OTLP-gRPC",
			testbed.NewOTLPTraceDataSender(testbed.DefaultHost, testutil.GetAvailablePort(t)),
			testbed.NewOTLPDataReceiver(testutil.GetAvailablePort(t)),
			testbed.ResourceSpec{
				ExpectedMaxCPU: 30,
				ExpectedMaxRAM: 120,
			},
			processors,
		},
		{
			"OTLP-HTTP",
			testbed.NewOTLPHTTPTraceDataSender(testbed.DefaultHost, testutil.GetAvailablePort(t), ""),
			testbed.NewOTLPHTTPDataReceiver(testutil.GetAvailablePort(t)),
			testbed.ResourceSpec{
				ExpectedMaxCPU: 30,
				ExpectedMaxRAM: 120,
			},
			processors,
		},
		{
			"OTLP-gRPC-memory-limiter",
			testbed.NewOTLPTraceDataSender(testbed.DefaultHost, testutil.GetAvailablePort(t)),
			testbed.NewOTLPDataReceiver(testutil.GetAvailablePort(t)),
			testbed.ResourceSpec{
				ExpectedMaxCPU: 30,
				ExpectedMaxRAM: 120,
			},
			limitProcessors,
		},
		{
			"OTLP-HTTP-memory-limiter",
			testbed.NewOTLPHTTPTraceDataSender(testbed.DefaultHost, testutil.GetAvailablePort(t), ""),
			testbed.NewOTLPHTTPDataReceiver(testutil.GetAvailablePort(t)),
			testbed.ResourceSpec{
				ExpectedMaxCPU: 30,
				ExpectedMaxRAM: 120,
			},
			limitProcessors,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ScenarioItemsPerSecond(
				10_000,
				t,
				test.sender,
				test.receiver,
				test.resourceSpec,
				performanceResultsSummary,
				test.processors,
				nil,
			)
		})
	}
}

func TestTrace100kSPS(t *testing.T) {
	tests := []struct {
		name         string
		sender       testbed.DataSender
		receiver     testbed.DataReceiver
		resourceSpec testbed.ResourceSpec
	}{
		{
			"OTLP-gRPC",
			testbed.NewOTLPTraceDataSender(testbed.DefaultHost, testutil.GetAvailablePort(t)),
			testbed.NewOTLPDataReceiver(testutil.GetAvailablePort(t)),
			testbed.ResourceSpec{
				ExpectedMaxCPU: 80,
				ExpectedMaxRAM: 120,
			},
		},
		{
			"OTLP-HTTP",
			testbed.NewOTLPHTTPTraceDataSender(testbed.DefaultHost, testutil.GetAvailablePort(t), ""),
			testbed.NewOTLPHTTPDataReceiver(testutil.GetAvailablePort(t)),
			testbed.ResourceSpec{
				ExpectedMaxCPU: 80,
				ExpectedMaxRAM: 120,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ScenarioItemsPerSecond(
				100_000,
				t,
				test.sender,
				test.receiver,
				test.resourceSpec,
				performanceResultsSummary,
				processors,
				nil,
			)
		})
	}
}
