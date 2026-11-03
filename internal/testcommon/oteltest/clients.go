// Copyright Dynatrace LLC
// SPDX-License-Identifier: Apache-2.0

package oteltest

import (
	"bytes"
	"net"
	"net/http"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/plog/plogotlp"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// newGRPCConn connects to an insecure gRPC server on localhost until the test ends.
func newGRPCConn(t *testing.T, port int) *grpc.ClientConn {
	conn, err := grpc.NewClient(net.JoinHostPort("localhost", strconv.Itoa(port)), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

// NewTracesClient returns an OTLP/gRPC traces client for a receiver on localhost.
func NewTracesClient(t *testing.T, port int) ptraceotlp.GRPCClient {
	return ptraceotlp.NewGRPCClient(newGRPCConn(t, port))
}

// NewMetricsClient returns an OTLP/gRPC metrics client for a receiver on localhost.
func NewMetricsClient(t *testing.T, port int) pmetricotlp.GRPCClient {
	return pmetricotlp.NewGRPCClient(newGRPCConn(t, port))
}

// NewLogsClient returns an OTLP/gRPC logs client for a receiver on localhost.
func NewLogsClient(t *testing.T, port int) plogotlp.GRPCClient {
	return plogotlp.NewGRPCClient(newGRPCConn(t, port))
}

// exportHTTP POSTs a protobuf encoded OTLP request to a receiver on localhost and requires a 200 response.
func exportHTTP(t *testing.T, port int, path string, body []byte, err error) {
	t.Helper()
	require.NoError(t, err)
	url := "http://" + net.JoinHostPort("localhost", strconv.Itoa(port)) + path
	resp, err := http.Post(url, "application/x-protobuf", bytes.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode, "unexpected response from %s", url)
}

// ExportTracesHTTP sends traces to the OTLP/HTTP receiver on localhost:port.
func ExportTracesHTTP(t *testing.T, port int, td ptrace.Traces) {
	t.Helper()
	b, err := ptraceotlp.NewExportRequestFromTraces(td).MarshalProto()
	exportHTTP(t, port, "/v1/traces", b, err)
}

// ExportMetricsHTTP sends metrics to the OTLP/HTTP receiver on localhost:port.
func ExportMetricsHTTP(t *testing.T, port int, md pmetric.Metrics) {
	t.Helper()
	b, err := pmetricotlp.NewExportRequestFromMetrics(md).MarshalProto()
	exportHTTP(t, port, "/v1/metrics", b, err)
}

// ExportLogsHTTP sends logs to the OTLP/HTTP receiver on localhost:port.
func ExportLogsHTTP(t *testing.T, port int, ld plog.Logs) {
	t.Helper()
	b, err := plogotlp.NewExportRequestFromLogs(ld).MarshalProto()
	exportHTTP(t, port, "/v1/logs", b, err)
}
