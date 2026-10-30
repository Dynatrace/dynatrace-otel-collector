// Copyright Dynatrace LLC
// SPDX-License-Identifier: Apache-2.0

package netflow

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/oteltest"
	"github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/testutil"
)

// v5Header and v5Record mirror the NetFlow v5 wire format, in wire order.
type v5Header struct {
	Version, Count                               uint16
	SysUptime, UnixSecs, UnixNSecs, FlowSequence uint32
	EngineType, EngineID                         uint8
	SamplingInterval                             uint16
}

type v5Record struct {
	SrcAddr, DstAddr, NextHop   [4]byte
	Input, Output               uint16
	DPkts, DOctets, First, Last uint32
	SrcPort, DstPort            uint16
	Pad1, TCPFlags, Proto, Tos  uint8
	SrcAS, DstAS                uint16
	SrcMask, DstMask            uint8
	Pad2                        uint16
}

const (
	sysUptime = 10_000 // ms
	unixSecs  = 1_700_000_000
	sequence  = 42
	sampling  = 100
)

var (
	recordA = v5Record{
		SrcAddr: [4]byte{10, 0, 0, 1}, DstAddr: [4]byte{10, 0, 0, 2}, NextHop: [4]byte{10, 0, 0, 254},
		Input: 3, Output: 4,
		DPkts: 10, DOctets: 1500, First: 9_000, Last: 9_500,
		SrcPort: 51000, DstPort: 443,
		TCPFlags: 0x1b, Proto: 6, Tos: 0x10,
		SrcAS: 64512, DstAS: 64513,
		SrcMask: 24, DstMask: 16,
	}
	recordB = v5Record{
		SrcAddr: [4]byte{10, 0, 1, 1}, DstAddr: [4]byte{10, 0, 2, 2},
		DPkts: 1, DOctets: 80, First: 9_800, Last: 9_800,
		SrcPort: 53000, DstPort: 53,
		Proto: 17,
	}
)

// encodeV5 builds a NetFlow v5 packet with the given records.
func encodeV5(t *testing.T, records ...v5Record) []byte {
	var buf bytes.Buffer
	header := v5Header{
		Version: 5, Count: uint16(len(records)),
		SysUptime: sysUptime, UnixSecs: unixSecs, FlowSequence: sequence,
		SamplingInterval: sampling,
	}
	require.NoError(t, binary.Write(&buf, binary.BigEndian, header))
	for _, r := range records {
		require.NoError(t, binary.Write(&buf, binary.BigEndian, r))
	}
	return buf.Bytes()
}

// startCollector starts the DT Collector with config_examples/netflow.yaml. It returns the
// UDP port of the NetFlow receiver and the sink the Collector exports to.
func startCollector(t *testing.T) (int, *consumertest.LogsSink) {
	sink, sinkPorts := oteltest.StartLogsSink(t)
	udpPort := testutil.GetAvailableUDPPort(t)
	healthPort := testutil.GetAvailablePort(t)

	cfg := testutil.LoadConfigExample(t, "netflow.yaml").
		ReplaceString("port: 2055", fmt.Sprintf("port: %d", udpPort)).
		Replace("health_check port", func(s string) string { return testutil.ReplaceHealthCheckPort(s, healthPort) }).
		Replace("exporter endpoint", func(s string) string { return testutil.ReplaceDynatraceExporterEndpoint(s, sinkPorts.Http) }).
		Replace("sending_queue and retry_on_failure", func(s string) string { return testutil.DisableSendingQueueAndRetry(s, "otlp_http") }).
		Replace("internal metrics", testutil.DisableInternalMetrics)

	testutil.StartCollector(t, testutil.CollectorParams{
		Name:      "netflow",
		Config:    cfg.String(),
		ReadyPort: healthPort,
	})
	testutil.WaitForHealthCheck(t, healthPort)
	return udpPort, sink
}

// expectedAttributes returns all attributes of a record except flow.time_received.
func expectedAttributes(r v5Record, start, end time.Duration, srcAddr, dstAddr, nextHop, transport string) map[string]any {
	startNs := int64(unixSecs)*int64(time.Second) - int64(sysUptime*time.Millisecond-start)
	endNs := int64(unixSecs)*int64(time.Second) - int64(sysUptime*time.Millisecond-end)
	return map[string]any{
		"source.address":      srcAddr,
		"source.port":         int64(r.SrcPort),
		"destination.address": dstAddr,
		"destination.port":    int64(r.DstPort),
		"network.transport":   transport,
		"network.type":        "ipv4",

		"flow.io.bytes":      int64(r.DOctets),
		"flow.io.packets":    int64(r.DPkts),
		"flow.type":          "netflow_v5",
		"flow.sequence_num":  int64(sequence),
		"flow.start":         startNs,
		"flow.end":           endNs,
		"flow.sampling_rate": int64(sampling),
		// goflow2 unmaps IPv4-mapped addresses.
		"flow.sampler_address": "127.0.0.1",
		"flow.next_hop":        nextHop,
		"flow.in_if":           int64(r.Input),
		"flow.out_if":          int64(r.Output),
		"flow.tcp_flags":       int64(r.TCPFlags),
		"flow.ip_tos":          int64(r.Tos),
		"flow.src_as":          int64(r.SrcAS),
		"flow.dst_as":          int64(r.DstAS),
		"flow.src_net":         int64(r.SrcMask),
		"flow.dst_net":         int64(r.DstMask),
		"flow.src_mac":         "00:00:00:00:00:00",
		"flow.dst_mac":         "00:00:00:00:00:00",

		"flow.ip_ttl":                0,
		"flow.ip_flags":              0,
		"flow.fragment_id":           0,
		"flow.fragment_offset":       0,
		"flow.ipv6_flow_label":       0,
		"flow.icmp_type":             0,
		"flow.icmp_code":             0,
		"flow.src_vlan":              0,
		"flow.dst_vlan":              0,
		"flow.vlan_id":               0,
		"flow.next_hop_as":           0,
		"flow.forwarding_status":     0,
		"flow.observation_domain_id": 0,
		"flow.observation_point_id":  0,
	}
}

// TestE2E_NetflowReceiver tests the NetFlow receiver. A hand-built NetFlow v5 packet with two
// flows is sent to the DT Collector running config_examples/netflow.yaml, and every attribute
// and timestamp of the resulting log records is compared exactly.
func TestE2E_NetflowReceiver(t *testing.T) {
	udpPort, sink := startCollector(t)

	conn, err := net.Dial("udp", fmt.Sprintf("127.0.0.1:%d", udpPort))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	before := time.Now()
	_, err = conn.Write(encodeV5(t, recordA, recordB))
	require.NoError(t, err)

	records := func() int {
		n := 0
		for _, l := range sink.AllLogs() {
			n += l.LogRecordCount()
		}
		return n
	}
	require.Eventually(t, func() bool { return records() >= 2 }, 30*time.Second, 200*time.Millisecond,
		"expected 2 log records, got %d", records())

	allLogs := sink.AllLogs()
	require.Len(t, allLogs, 1, "both flows were sent in one packet")
	logs := allLogs[0]
	require.Equal(t, 1, logs.ResourceLogs().Len())
	rl := logs.ResourceLogs().At(0)
	require.Equal(t, 1, rl.ScopeLogs().Len())
	sl := rl.ScopeLogs().At(0)
	assert.Equal(t, map[string]any{"receiver": "netflow"}, sl.Scope().Attributes().AsRaw())
	require.Equal(t, 2, sl.LogRecords().Len())

	want := []map[string]any{
		expectedAttributes(recordA, 9_000*time.Millisecond, 9_500*time.Millisecond, "10.0.0.1", "10.0.0.2", "10.0.0.254", "tcp"),
		expectedAttributes(recordB, 9_800*time.Millisecond, 9_800*time.Millisecond, "10.0.1.1", "10.0.2.2", "0.0.0.0", "udp"),
	}
	for i, wantAttrs := range want {
		lr := sl.LogRecords().At(i)
		attrs := lr.Attributes()

		received, ok := attrs.Get("flow.time_received")
		require.True(t, ok, "record %d has no flow.time_received", i)
		receivedTs := pcommon.Timestamp(received.Int())
		assert.False(t, receivedTs.AsTime().Before(before), "record %d: received before the packet was sent", i)
		assert.False(t, receivedTs.AsTime().After(time.Now()), "record %d: received in the future", i)
		assert.Equal(t, receivedTs, lr.ObservedTimestamp(), "record %d", i)
		attrs.Remove("flow.time_received")

		// All integers in the raw map are int64.
		normalized := make(map[string]any, len(wantAttrs))
		for k, v := range wantAttrs {
			if n, isInt := v.(int); isInt {
				v = int64(n)
			}
			normalized[k] = v
		}
		assert.Equal(t, normalized, attrs.AsRaw(), "record %d", i)
		assert.Equal(t, pcommon.Timestamp(wantAttrs["flow.start"].(int64)), lr.Timestamp(), "record %d", i)
	}
}
