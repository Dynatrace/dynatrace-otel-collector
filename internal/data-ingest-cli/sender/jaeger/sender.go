package jaeger

import (
	"context"
	"fmt"
	"log"

	"github.com/jaegertracing/jaeger/model"
	jaegerproto "github.com/jaegertracing/jaeger/proto-gen/api_v2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Sender interface {
	SendTraces(ctx context.Context, traces *model.Batch) error
}

type JaegerSender struct {
	grpcClient jaegerproto.CollectorServiceClient
}

func New(url string) (*JaegerSender, error) {
	conn, err := grpc.NewClient(url, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	return &JaegerSender{
		grpcClient: jaegerproto.NewCollectorServiceClient(conn),
	}, nil
}

func (s *JaegerSender) SendTraces(ctx context.Context, batch *model.Batch) error {
	log.Printf("Sending traces via grpc")

	_, err := s.grpcClient.PostSpans(
		ctx,
		&jaegerproto.PostSpansRequest{Batch: *batch})

	if err != nil {
		return fmt.Errorf("failed to send trace data via grpc Jaeger sender: %w", err)
	}

	return nil
}

// func sendTracesUsingHTTP(ctx context.Context, traces ptrace.Traces) {
// 	exp, err := otlptracehttp.New(ctx, otlptracehttp.WithEndpoint("http://localhost:4318"), otlptracehttp.WithInsecure())
// 	if err != nil {
// 		log.Fatalf("failed to create OTLP HTTP exporter: %v", err)
// 	}
// 	tp := trace.NewTracerProvider(
// 		trace.WithBatcher(exp),
// 		trace.WithResource(resource.NewWithAttributes("service.name", "http-tracer")),
// 	)
// 	otel.SetTracerProvider(tp)

// 	// Send traces
// 	err = tp.ForceFlush(ctx)
// 	if err != nil {
// 		log.Fatalf("failed to flush traces: %v", err)
// 	}
// }
