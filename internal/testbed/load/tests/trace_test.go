package loadtest

import (
	"testing"

	"github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
)

func TestTrace10kSPS(t *testing.T) {
	tests := []struct {
		name                string
		sender              testbed.DataSender
		receiver            testbed.DataReceiver
		extendedLoadOptions ExtendedLoadOptions
		processors          map[string]string
	}{
		{
			"OTLP-gRPC",
			testbed.NewOTLPTraceDataSender(testbed.DefaultHost, testutil.GetAvailablePort(t)),
			testbed.NewOTLPDataReceiver(testutil.GetAvailablePort(t)),
			ExtendedLoadOptions{
				loadOptions: &testbed.LoadOptions{
					DataItemsPerSecond: 10_000,
					ItemsPerBatch:      100,
					Parallel:           1,
				},
				resourceSpec: testbed.ResourceSpec{
					ExpectedMaxCPU: 30,
					ExpectedMaxRAM: 150,
				},
				attrCount:       10,
				attrSizeByte:    50,
				attrKeySizeByte: 50,
			},
			defaultProcessors,
		},
		{
			"OTLP-HTTP",
			testbed.NewOTLPHTTPTraceDataSender(testbed.DefaultHost, testutil.GetAvailablePort(t), ""),
			testbed.NewOTLPHTTPDataReceiver(testutil.GetAvailablePort(t)),
			ExtendedLoadOptions{
				loadOptions: &testbed.LoadOptions{
					DataItemsPerSecond: 10_000,
					ItemsPerBatch:      100,
					Parallel:           1,
				},
				resourceSpec: testbed.ResourceSpec{
					ExpectedMaxCPU: 30,
					ExpectedMaxRAM: 150,
				},
				attrCount:       10,
				attrSizeByte:    50,
				attrKeySizeByte: 50,
			},
			defaultProcessors,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			GenericScenario(
				t,
				test.sender,
				test.receiver,
				performanceResultsSummary,
				test.processors,
				nil,
				test.extendedLoadOptions,
			)
		})
	}
}

func TestTrace100kSPS(t *testing.T) {
	tests := []struct {
		name                string
		sender              testbed.DataSender
		receiver            testbed.DataReceiver
		extendedLoadOptions ExtendedLoadOptions
		processors          map[string]string
	}{
		{
			"OTLP-gRPC",
			testbed.NewOTLPTraceDataSender(testbed.DefaultHost, testutil.GetAvailablePort(t)),
			testbed.NewOTLPDataReceiver(testutil.GetAvailablePort(t)),
			ExtendedLoadOptions{
				loadOptions: &testbed.LoadOptions{
					DataItemsPerSecond: 100_000,
					ItemsPerBatch:      100,
					Parallel:           1,
				},
				resourceSpec: testbed.ResourceSpec{
					ExpectedMaxCPU: 90,
					ExpectedMaxRAM: 150,
				},
				attrCount:       10,
				attrSizeByte:    50,
				attrKeySizeByte: 50,
			},
			defaultProcessors,
		},
		{
			"OTLP-HTTP",
			testbed.NewOTLPHTTPTraceDataSender(testbed.DefaultHost, testutil.GetAvailablePort(t), ""),
			testbed.NewOTLPHTTPDataReceiver(testutil.GetAvailablePort(t)),
			ExtendedLoadOptions{
				loadOptions: &testbed.LoadOptions{
					DataItemsPerSecond: 100_000,
					ItemsPerBatch:      100,
					Parallel:           1,
				},
				resourceSpec: testbed.ResourceSpec{
					ExpectedMaxCPU: 110,
					ExpectedMaxRAM: 150,
				},
				attrCount:       10,
				attrSizeByte:    50,
				attrKeySizeByte: 50,
			},
			defaultProcessors,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			GenericScenario(
				t,
				test.sender,
				test.receiver,
				performanceResultsSummary,
				test.processors,
				nil,
				test.extendedLoadOptions,
			)
		})
	}
}
