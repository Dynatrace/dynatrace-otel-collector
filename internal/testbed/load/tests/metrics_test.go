// Copyright The OpenTelemetry Authors
// Copyright Dynatrace LLC
// SPDX-License-Identifier: Apache-2.0

package loadtest

// This file contains Test functions which initiate the tests. The tests can be either
// coded in this file or use scenarios from perf_scenarios.go.

import (
	"testing"

	"github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
)

var (
	metricProcessors = map[string]string{
		"batch": `
  batch:
    send_batch_max_size: 1000
    timeout: 10s
    send_batch_size : 800
`,
		"memory_limiter": `
  memory_limiter:
    check_interval: 1s
    limit_percentage: 100
`,
	}
)

func TestMetric10kDPS(t *testing.T) {
	metricCount := 10_000

	tests := []struct {
		name                string
		sender              testbed.DataSender
		receiver            testbed.DataReceiver
		processors          map[string]string
		extendedLoadOptions ExtendedLoadOptions
	}{
		{
			name:     "OTLP",
			sender:   testbed.NewOTLPMetricDataSender(testbed.DefaultHost, testutil.GetAvailablePort(t)),
			receiver: testbed.NewOTLPDataReceiver(testutil.GetAvailablePort(t)),
			extendedLoadOptions: ExtendedLoadOptions{
				loadOptions: &testbed.LoadOptions{
					DataItemsPerSecond: metricCount,
					ItemsPerBatch:      1000,
					Parallel:           1,
				},
				resourceSpec: testbed.ResourceSpec{
					ExpectedMaxCPU: 60,
					ExpectedMaxRAM: 120,
				},
				attrCount:       25,
				attrSizeByte:    20,
				attrKeySizeByte: 100,
			},
			processors: metricProcessors,
		},
		{
			name:     "OTLP-HTTP",
			sender:   testbed.NewOTLPHTTPMetricDataSender(testbed.DefaultHost, testutil.GetAvailablePort(t)),
			receiver: testbed.NewOTLPHTTPDataReceiver(testutil.GetAvailablePort(t)),
			extendedLoadOptions: ExtendedLoadOptions{
				loadOptions: &testbed.LoadOptions{
					DataItemsPerSecond: metricCount,
					ItemsPerBatch:      1000,
					Parallel:           1,
				},
				resourceSpec: testbed.ResourceSpec{
					ExpectedMaxCPU: 60,
					ExpectedMaxRAM: 105,
				},
				attrCount:       25,
				attrSizeByte:    20,
				attrKeySizeByte: 100,
			},
			processors: metricProcessors,
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

func TestMetric100kDPS(t *testing.T) {
	tests := []struct {
		name                string
		sender              testbed.DataSender
		receiver            testbed.DataReceiver
		extendedLoadOptions ExtendedLoadOptions
		resourceSpec        testbed.ResourceSpec
		processors          map[string]string
	}{
		{
			name:     "OTLP",
			sender:   testbed.NewOTLPMetricDataSender(testbed.DefaultHost, testutil.GetAvailablePort(t)),
			receiver: testbed.NewOTLPDataReceiver(testutil.GetAvailablePort(t)),
			extendedLoadOptions: ExtendedLoadOptions{
				loadOptions: &testbed.LoadOptions{
					DataItemsPerSecond: 100_000,
					ItemsPerBatch:      100,
					Parallel:           1,
				},
				resourceSpec: testbed.ResourceSpec{
					ExpectedMaxCPU: 70,
					ExpectedMaxRAM: 120,
				},
				attrCount:       25,
				attrSizeByte:    20,
				attrKeySizeByte: 100,
			},
			processors: metricProcessors,
		},
		{
			name:     "OTLP-HTTP",
			sender:   testbed.NewOTLPHTTPMetricDataSender(testbed.DefaultHost, testutil.GetAvailablePort(t)),
			receiver: testbed.NewOTLPHTTPDataReceiver(testutil.GetAvailablePort(t)),
			extendedLoadOptions: ExtendedLoadOptions{
				loadOptions: &testbed.LoadOptions{
					DataItemsPerSecond: 100_000,
					ItemsPerBatch:      100,
					Parallel:           1,
				},
				resourceSpec: testbed.ResourceSpec{
					ExpectedMaxCPU: 99,
					ExpectedMaxRAM: 100,
				},
				attrCount:       25,
				attrSizeByte:    20,
				attrKeySizeByte: 100,
			},
			processors: metricProcessors,
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
