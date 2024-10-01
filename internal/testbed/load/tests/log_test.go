package loadtest

import (
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/datasenders"

	"github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
)

func TestLog10kDPS(t *testing.T) {
	tests := []struct {
		name        string
		sender      testbed.DataSender
		receiver    testbed.DataReceiver
		loadOptions ExtendedLoadOptions
		extensions  map[string]string
	}{
		{
			name:     "OTLP",
			sender:   testbed.NewOTLPLogsDataSender(testbed.DefaultHost, testutil.GetAvailablePort(t)),
			receiver: testbed.NewOTLPDataReceiver(testutil.GetAvailablePort(t)),
			loadOptions: ExtendedLoadOptions{
				loadOptions: &testbed.LoadOptions{
					DataItemsPerSecond: 10_000,
					ItemsPerBatch:      100,
					Parallel:           1,
				},
				resourceSpec: testbed.ResourceSpec{
					ExpectedMaxCPU: 30,
					ExpectedMaxRAM: 120,
				},
				attrCount:       0,
				attrSizeByte:    0,
				attrKeySizeByte: 0,
			},
		},
	}

	processors := map[string]string{}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			GenericScenario(
				t,
				test.sender,
				test.receiver,
				performanceResultsSummary,
				processors,
				nil,
				test.loadOptions,
			)
		})
	}
}

func TestLogSyslog(t *testing.T) {
	tests := []struct {
		name                string
		sender              testbed.DataSender
		receiver            testbed.DataReceiver
		resourceSpec        testbed.ResourceSpec
		extensions          map[string]string
		extendedLoadOptions ExtendedLoadOptions
	}{
		{
			name:     "syslog-10kDPS-batch-1",
			sender:   datasenders.NewSyslogWriter("tcp", testbed.DefaultHost, testutil.GetAvailablePort(t), 1),
			receiver: testbed.NewOTLPHTTPDataReceiver(testutil.GetAvailablePort(t)),
			resourceSpec: testbed.ResourceSpec{
				ExpectedMaxCPU: 90,
				ExpectedMaxRAM: 150,
			},
			extendedLoadOptions: ExtendedLoadOptions{
				loadOptions: &testbed.LoadOptions{
					DataItemsPerSecond: 10_000,
					ItemsPerBatch:      1,
					Parallel:           1,
				},
				resourceSpec: testbed.ResourceSpec{
					ExpectedMaxCPU: 130,
					ExpectedMaxRAM: 130,
				},
				attrCount:       25,
				attrSizeByte:    20,
				attrKeySizeByte: 100,
			},
		},
		{
			name:     "syslog-10kDPS-batch-100",
			sender:   datasenders.NewSyslogWriter("tcp", testbed.DefaultHost, testutil.GetAvailablePort(t), 100),
			receiver: testbed.NewOTLPDataReceiver(testutil.GetAvailablePort(t)),
			extendedLoadOptions: ExtendedLoadOptions{
				loadOptions: &testbed.LoadOptions{
					DataItemsPerSecond: 10_000,
					ItemsPerBatch:      100,
					Parallel:           1,
				},
				resourceSpec: testbed.ResourceSpec{
					ExpectedMaxCPU: 130,
					ExpectedMaxRAM: 120,
				},
				attrCount:       25,
				attrSizeByte:    20,
				attrKeySizeByte: 100,
			},
		},
		{
			name:     "syslog-70kDPS-batch-1",
			sender:   datasenders.NewSyslogWriter("tcp", testbed.DefaultHost, testutil.GetAvailablePort(t), 1),
			receiver: testbed.NewOTLPHTTPDataReceiver(testutil.GetAvailablePort(t)),
			resourceSpec: testbed.ResourceSpec{
				ExpectedMaxCPU: 90,
				ExpectedMaxRAM: 150,
			},
			extendedLoadOptions: ExtendedLoadOptions{
				loadOptions: &testbed.LoadOptions{
					DataItemsPerSecond: 70_000,
					ItemsPerBatch:      1,
					Parallel:           1,
				},
				resourceSpec: testbed.ResourceSpec{
					ExpectedMaxCPU: 130,
					ExpectedMaxRAM: 120,
				},
				attrCount:       25,
				attrSizeByte:    20,
				attrKeySizeByte: 100,
			},
		},
		{
			name:     "syslog-70kDPS-batch-100",
			sender:   datasenders.NewSyslogWriter("tcp", testbed.DefaultHost, testutil.GetAvailablePort(t), 100),
			receiver: testbed.NewOTLPDataReceiver(testutil.GetAvailablePort(t)),
			extendedLoadOptions: ExtendedLoadOptions{
				loadOptions: &testbed.LoadOptions{
					DataItemsPerSecond: 70_000,
					ItemsPerBatch:      100,
					Parallel:           1,
				},
				resourceSpec: testbed.ResourceSpec{
					ExpectedMaxCPU: 130,
					ExpectedMaxRAM: 120,
				},
				attrCount:       25,
				attrSizeByte:    20,
				attrKeySizeByte: 100,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			GenericScenario(
				t,
				test.sender,
				test.receiver,
				performanceResultsSummary,
				defaultProcessors,
				nil,
				test.extendedLoadOptions,
			)
		})
	}
}
