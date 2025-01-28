package otlp

import (
	"context"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/plog/plogotlp"
	collogpb "go.opentelemetry.io/collector/pdata/plog/plogotlp"
	"go.opentelemetry.io/collector/pdata/pmetric"
	colmetricpb "go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"
	coltracepb "go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"log"
)

type Sender interface {
	SendTraces(ctx context.Context, spans ptrace.Traces) error
	SendLogs(ctx context.Context, logs plog.Logs) error
	SendMetrics(ctx context.Context, metrics pmetric.Metrics) error
}

type GRPCSender struct {
	client       *grpc.ClientConn
	traceClient  ptraceotlp.GRPCClient
	logClient    collogpb.GRPCClient
	metricClient colmetricpb.GRPCClient
}

func New(url string) (*GRPCSender, error) {
	conn, err := grpc.NewClient(url, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	return &GRPCSender{
		client:       conn,
		traceClient:  coltracepb.NewGRPCClient(conn),
		logClient:    collogpb.NewGRPCClient(conn),
		metricClient: colmetricpb.NewGRPCClient(conn),
	}, nil
}

func (s *GRPCSender) SendTraces(ctx context.Context, traces ptrace.Traces) error {
	log.Printf("Sending traces to %s\n", s.client.Target())
	exportRequest := ptraceotlp.NewExportRequestFromTraces(traces)
	_, err := s.traceClient.Export(ctx, exportRequest)

	return err
}

func (s *GRPCSender) SendLogs(ctx context.Context, logs plog.Logs) error {
	log.Printf("Sending logs to %s\n", s.client.Target())
	req := plogotlp.NewExportRequestFromLogs(logs)
	_, err := s.logClient.Export(ctx, req)

	return err
}

func (s *GRPCSender) SendMetrics(ctx context.Context, metrics pmetric.Metrics) error {
	log.Printf("Sending metrics to %s\n", s.client.Target())
	req := colmetricpb.NewExportRequestFromMetrics(metrics)
	_, err := s.metricClient.Export(ctx, req)

	return err
}
