package loadtest

import (
	"testing"

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
