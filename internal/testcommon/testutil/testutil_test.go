// Copyright Dynatrace LLC
// SPDX-License-Identifier: Apache-2.0

package testutil

import (
	"net"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func TestGetAvailableLocalAddress(t *testing.T) {
	addr := GetAvailableLocalAddress(t)

	// Endpoint should be free.
	ln0, err := net.Listen("tcp", addr)
	require.NoError(t, err)
	require.NotNil(t, ln0)
	t.Cleanup(func() {
		require.NoError(t, ln0.Close())
	})

	// Ensure that the endpoint wasn't something like ":0" by checking that a second listener will fail.
	ln1, err := net.Listen("tcp", addr)
	require.Error(t, err)
	require.Nil(t, ln1)
}
func TestGetAvailableLocalUDPAddress(t *testing.T) {
	addr := GetAvailableLocalNetworkAddress(t, "udp")
	// Endpoint should be free.
	ln0, err := net.ListenPacket("udp", addr)
	require.NoError(t, err)
	require.NotNil(t, ln0)
	t.Cleanup(func() {
		require.NoError(t, ln0.Close())
	})

	// Ensure that the endpoint wasn't something like ":0" by checking that a second listener will fail.
	ln1, err := net.ListenPacket("udp", addr)
	require.Error(t, err)
	require.Nil(t, ln1)
}

func TestCreateExclusionsList(t *testing.T) {
	// Test two examples of typical output from "netsh interface ipv4 show excludedportrange protocol=tcp"
	emptyExclusionsText := `

Protocol tcp Port Exclusion Ranges

Start Port    End Port      
----------    --------      

* - Administered port exclusions.`

	exclusionsText := `

Start Port    End Port
----------    --------
     49697       49796
     49797       49896

* - Administered port exclusions.
`
	exclusions := createExclusionsList(t, exclusionsText)
	require.Len(t, exclusions, 2)

	emptyExclusions := createExclusionsList(t, emptyExclusionsText)
	require.Empty(t, emptyExclusions)
}

func TestDeduplicateLogs(t *testing.T) {
	logs := plog.NewLogs()
	for i, body := range []string{"a", "a", "b", "a"} {
		rl := logs.ResourceLogs().AppendEmpty()
		rl.Resource().Attributes().PutStr("service.name", "svc")
		lr := rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
		lr.Body().SetStr(body)
		lr.SetTimestamp(pcommon.Timestamp(i + 1))
		lr.SetObservedTimestamp(pcommon.Timestamp(i + 1))
	}

	DeduplicateLogs(logs)

	require.Equal(t, 2, logs.ResourceLogs().Len())
	first := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	require.Equal(t, "a", first.Body().Str())
	require.Equal(t, pcommon.Timestamp(1), first.Timestamp())
	require.Equal(t, "b", logs.ResourceLogs().At(1).ScopeLogs().At(0).LogRecords().At(0).Body().Str())
}

func TestDeduplicateTraces(t *testing.T) {
	traces := ptrace.NewTraces()
	for i, name := range []string{"a", "b", "a", "b", "b"} {
		rs := traces.ResourceSpans().AppendEmpty()
		rs.Resource().Attributes().PutStr("service.name", "svc")
		span := rs.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
		span.SetName(name)
		span.SetStartTimestamp(pcommon.Timestamp(i + 1))
		span.SetEndTimestamp(pcommon.Timestamp(i + 2))
		span.SetTraceID(pcommon.TraceID{byte(i + 1)})
		span.SetSpanID(pcommon.SpanID{byte(i + 1)})
		span.SetParentSpanID(pcommon.SpanID{byte(i + 2)})
	}

	DeduplicateTraces(traces)

	require.Equal(t, 2, traces.ResourceSpans().Len())
	first := traces.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)
	require.Equal(t, "a", first.Name())
	require.Equal(t, pcommon.SpanID{1}, first.SpanID())
	require.Equal(t, "b", traces.ResourceSpans().At(1).ScopeSpans().At(0).Spans().At(0).Name())
}
