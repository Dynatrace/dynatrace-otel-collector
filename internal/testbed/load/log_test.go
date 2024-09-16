// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package tests contains test cases. To run the tests go to tests directory and run:
// RUN_TESTBED=1 go test -v

package tests

import (
	"testing"

	"github.com/Dynatrace/dynatrace-otel-collector/internal/testbed/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/datasenders"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
)

func TestLog10kDPS(t *testing.T) {
	testbed.GlobalConfig.DefaultAgentExeRelativeFile = "../../../bin/dynatrace-otel-collector"
	tests := []struct {
		name         string
		sender       testbed.DataSender
		receiver     testbed.DataReceiver
		resourceSpec testbed.ResourceSpec
		extensions   map[string]string
	}{
		{
			name:     "syslog-batch-1",
			sender:   datasenders.NewSyslogWriter("tcp", testbed.DefaultHost, testutil.GetAvailablePort(t), 1),
			receiver: testbed.NewOTLPHTTPDataReceiver(testutil.GetAvailablePort(t)),
			resourceSpec: testbed.ResourceSpec{
				ExpectedMaxCPU: 90,
				ExpectedMaxRAM: 150,
			},
		},
		{
			name:     "syslog-batch-100",
			sender:   datasenders.NewSyslogWriter("tcp", testbed.DefaultHost, testutil.GetAvailablePort(t), 100),
			receiver: testbed.NewOTLPDataReceiver(testutil.GetAvailablePort(t)),
			resourceSpec: testbed.ResourceSpec{
				ExpectedMaxCPU: 90,
				ExpectedMaxRAM: 150,
			},
		},
	}

	processors := map[string]string{}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			Scenario70kItemsPerSecond(
				t,
				test.sender,
				test.receiver,
				test.resourceSpec,
				performanceResultsSummary,
				processors,
				test.extensions,
			)
		})
	}
}

func TestLogOtlpSendingQueue(t *testing.T) {
	otlpreceiver10 := testbed.NewOTLPDataReceiver(testutil.GetAvailablePort(t))
	otlpreceiver10.WithRetry(`
    retry_on_failure:
      enabled: true
`)
	otlpreceiver10.WithQueue(`
    sending_queue:
      enabled: true
      queue_size: 10
`)
	t.Run("OTLP-sending-queue-full", func(t *testing.T) {
		ScenarioSendingQueuesFull(
			t,
			testbed.NewOTLPLogsDataSender(testbed.DefaultHost, testutil.GetAvailablePort(t)),
			otlpreceiver10,
			testbed.LoadOptions{
				DataItemsPerSecond: 100,
				ItemsPerBatch:      10,
				Parallel:           1,
			},
			testbed.ResourceSpec{
				ExpectedMaxCPU: 80,
				ExpectedMaxRAM: 120,
			}, 10,
			performanceResultsSummary,
			nil,
			nil)
	})

	otlpreceiver100 := testbed.NewOTLPDataReceiver(testutil.GetAvailablePort(t))
	otlpreceiver100.WithRetry(`
    retry_on_failure:
      enabled: true
`)
	otlpreceiver10.WithQueue(`
    sending_queue:
      enabled: true
      queue_size: 100
`)
	t.Run("OTLP-sending-queue-not-full", func(t *testing.T) {
		ScenarioSendingQueuesNotFull(
			t,
			testbed.NewOTLPLogsDataSender(testbed.DefaultHost, testutil.GetAvailablePort(t)),
			otlpreceiver100,
			testbed.LoadOptions{
				DataItemsPerSecond: 100,
				ItemsPerBatch:      10,
				Parallel:           1,
			},
			testbed.ResourceSpec{
				ExpectedMaxCPU: 80,
				ExpectedMaxRAM: 120,
			}, 10,
			performanceResultsSummary,
			nil,
			nil)
	})

}
