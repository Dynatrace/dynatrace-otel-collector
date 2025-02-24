// Copyright The OpenTelemetry Authors
// Copyright Dynatrace LLC
// SPDX-License-Identifier: Apache-2.0

package loadtest

// This file contains Test functions which initiate the tests. The tests can be either
// coded in this file or use scenarios from perf_scenarios.go.

import (
	"testing"

	datasenders2 "github.com/Dynatrace/dynatrace-otel-collector/internal/testbed/load/tests/datasenders"

	"github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/datasenders"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
)

var (
	defaultProcessors = map[string]string{
		"memory_limiter": `
  memory_limiter:
    check_interval: 1s
    limit_percentage: 100
`,
		"batch": `
  batch:
    send_batch_max_size: 1000
    timeout: 10s
    send_batch_size : 800
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
					ExpectedMaxCPU: 25,
					ExpectedMaxRAM: 150,
				},
				attrCount:       25,
				attrSizeByte:    20,
				attrKeySizeByte: 100,
			},
			processors: defaultProcessors,
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
					ExpectedMaxCPU: 25,
					ExpectedMaxRAM: 150,
				},
				attrCount:       25,
				attrSizeByte:    20,
				attrKeySizeByte: 100,
			},
			processors: defaultProcessors,
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
					ExpectedMaxRAM: 130,
				},
				attrCount:       25,
				attrSizeByte:    20,
				attrKeySizeByte: 100,
			},
			processors: defaultProcessors,
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
					ExpectedMaxCPU: 105,
					ExpectedMaxRAM: 120,
				},
				attrCount:       25,
				attrSizeByte:    20,
				attrKeySizeByte: 100,
			},
			processors: defaultProcessors,
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

func TestPrometheusMetric(t *testing.T) {
	tests := []struct {
		name                string
		sender              testbed.DataSender
		receiver            testbed.DataReceiver
		extendedLoadOptions ExtendedLoadOptions
		resourceSpec        testbed.ResourceSpec
		processors          map[string]string
	}{
		{
			name:     "Prometheus 1kDPS - 1 Prometheus Endpoint",
			sender:   datasenders.NewPrometheusDataSender(testbed.DefaultHost, testutil.GetAvailablePort(t)),
			receiver: testbed.NewOTLPDataReceiver(testutil.GetAvailablePort(t)),
			extendedLoadOptions: ExtendedLoadOptions{
				resourceSpec: testbed.ResourceSpec{
					ExpectedMaxCPU:         10,
					ExpectedMaxRAM:         140,
					MaxConsecutiveFailures: 2,
				},
				loadOptions: &testbed.LoadOptions{
					DataItemsPerSecond: 1,
					ItemsPerBatch:      1,
					Parallel:           1,
				},
				attrCount:       25,
				attrSizeByte:    20,
				attrKeySizeByte: 100,
				scrapeLoadOptions: scrapeLoadOptions{
					numberOfMetrics:            1000,
					scrapeIntervalMilliSeconds: 1000,
				},
			},
			processors: defaultProcessors,
		},
		{
			name:     "Prometheus 10kDPS - 1 Prometheus Endpoint",
			sender:   datasenders.NewPrometheusDataSender(testbed.DefaultHost, testutil.GetAvailablePort(t)),
			receiver: testbed.NewOTLPDataReceiver(testutil.GetAvailablePort(t)),
			extendedLoadOptions: ExtendedLoadOptions{
				resourceSpec: testbed.ResourceSpec{
					ExpectedMaxCPU:         50,
					ExpectedMaxRAM:         350,
					MaxConsecutiveFailures: 2,
				},
				loadOptions: &testbed.LoadOptions{
					DataItemsPerSecond: 1,
					ItemsPerBatch:      1,
					Parallel:           1,
				},
				attrCount:       25,
				attrSizeByte:    20,
				attrKeySizeByte: 100,
				scrapeLoadOptions: scrapeLoadOptions{
					numberOfMetrics:            10_000,
					scrapeIntervalMilliSeconds: 1000,
				},
			},
			processors: defaultProcessors,
		},
		{
			name:     "Prometheus 1kDPS - 5 Prometheus Endpoints",
			sender:   datasenders2.NewMultiHostPrometheusDataSender(testbed.DefaultHost, testutil.GetAvailablePorts(t, 5)),
			receiver: testbed.NewOTLPDataReceiver(testutil.GetAvailablePort(t)),
			extendedLoadOptions: ExtendedLoadOptions{
				resourceSpec: testbed.ResourceSpec{
					ExpectedMaxCPU:         150,
					ExpectedMaxRAM:         250,
					MaxConsecutiveFailures: 2,
				},
				loadOptions: &testbed.LoadOptions{
					DataItemsPerSecond: 1,
					ItemsPerBatch:      1,
					Parallel:           1,
				},
				attrCount:       25,
				attrSizeByte:    20,
				attrKeySizeByte: 100,
				scrapeLoadOptions: scrapeLoadOptions{
					numberOfMetrics:            1000,
					scrapeIntervalMilliSeconds: 1000,
				},
			},
			processors: defaultProcessors,
		},
		{
			name:     "Prometheus 1kDPS - 10 Prometheus Endpoints",
			sender:   datasenders2.NewMultiHostPrometheusDataSender(testbed.DefaultHost, testutil.GetAvailablePorts(t, 10)),
			receiver: testbed.NewOTLPDataReceiver(testutil.GetAvailablePort(t)),
			extendedLoadOptions: ExtendedLoadOptions{
				resourceSpec: testbed.ResourceSpec{
					ExpectedMaxCPU:         230,
					ExpectedMaxRAM:         550,
					MaxConsecutiveFailures: 2,
				},
				loadOptions: &testbed.LoadOptions{
					DataItemsPerSecond: 1,
					ItemsPerBatch:      1,
					Parallel:           1,
				},
				attrCount:       25,
				attrSizeByte:    20,
				attrKeySizeByte: 100,
				scrapeLoadOptions: scrapeLoadOptions{
					numberOfMetrics:            1000,
					scrapeIntervalMilliSeconds: 1000,
				},
			},
			processors: defaultProcessors,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			PullBasedSenderScenario(
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
