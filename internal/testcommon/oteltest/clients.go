// Copyright Dynatrace LLC
// SPDX-License-Identifier: Apache-2.0

package oteltest

import (
	"net"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/plog/plogotlp"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
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
