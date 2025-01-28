package otlp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/plog/plogotlp"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"
	"google.golang.org/grpc"
	_ "google.golang.org/grpc/encoding/gzip"
	"log"
	"net"
	"os"
	"sync"
	"time"
)

type Config struct {
	Port       int
	OutputFile string
}

type OTLPReceiver struct {
	server *grpc.Server
	config Config

	receivedDataChan chan struct{}

	wg sync.WaitGroup
}

func NewOTLPReceiver(c Config) (*OTLPReceiver, error) {
	grpcServer := grpc.NewServer()

	return &OTLPReceiver{
		server:           grpcServer,
		config:           c,
		wg:               sync.WaitGroup{},
		receivedDataChan: make(chan struct{}),
	}, nil
}

func (r *OTLPReceiver) Start() error {
	ptraceotlp.RegisterGRPCServer(r.server, &traceService{
		receivedDataChan: r.receivedDataChan,
		outputFile:       r.config.OutputFile,
	})

	pmetricotlp.RegisterGRPCServer(r.server, &metricsService{
		receivedDataChan: r.receivedDataChan,
		outputFile:       r.config.OutputFile,
	})

	plogotlp.RegisterGRPCServer(r.server, &logsService{
		receivedDataChan: r.receivedDataChan,
		outputFile:       r.config.OutputFile,
	})

	lis, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", r.config.Port))
	if err != nil {
		return err
	}

	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		if err := r.server.Serve(lis); err != nil {
			fmt.Println(err.Error())
		}
	}()
	return nil
}

func (r *OTLPReceiver) Stop() {

	select {
	case <-r.receivedDataChan:
	case <-time.After(10 * time.Second):
	}
	if r.server != nil {
		r.server.GracefulStop()
	}
	r.wg.Wait()
}

type traceService struct {
	ptraceotlp.UnimplementedGRPCServer
	outputFile       string
	receivedDataChan chan struct{}
}

func (t *traceService) Export(_ context.Context, req ptraceotlp.ExportRequest) (ptraceotlp.ExportResponse, error) {
	log.Println("Received traces")
	traceMarshaler := &ptrace.JSONMarshaler{}
	traces, err := traceMarshaler.MarshalTraces(req.Traces())
	if err != nil {
		log.Printf("Could not marshal traces: %v\n", err)
		return ptraceotlp.NewExportResponse(), nil
	}

	writeToFile(t.outputFile, traces)

	t.receivedDataChan <- struct{}{}
	return ptraceotlp.NewExportResponse(), nil
}

type metricsService struct {
	pmetricotlp.UnimplementedGRPCServer
	outputFile       string
	receivedDataChan chan struct{}
}

func (m *metricsService) Export(_ context.Context, req pmetricotlp.ExportRequest) (pmetricotlp.ExportResponse, error) {
	log.Println("Received metrics")
	metricsMarshaler := &pmetric.JSONMarshaler{}
	metrics, err := metricsMarshaler.MarshalMetrics(req.Metrics())
	if err != nil {
		log.Printf("Could not marshal metrics: %v\n", err)
		return pmetricotlp.NewExportResponse(), nil
	}

	writeToFile(m.outputFile, metrics)

	m.receivedDataChan <- struct{}{}
	return pmetricotlp.NewExportResponse(), nil
}

type logsService struct {
	plogotlp.UnimplementedGRPCServer
	outputFile       string
	receivedDataChan chan struct{}
}

func (l *logsService) Export(_ context.Context, req plogotlp.ExportRequest) (plogotlp.ExportResponse, error) {
	log.Println("Received logs")
	logsMarshaler := &plog.JSONMarshaler{}
	logs, err := logsMarshaler.MarshalLogs(req.Logs())
	if err != nil {
		log.Printf("Could not marshal metrics: %v\n", err)
		return plogotlp.NewExportResponse(), nil
	}

	writeToFile(l.outputFile, logs)

	l.receivedDataChan <- struct{}{}
	return plogotlp.NewExportResponse(), nil
}

func writeToFile(dest string, raw []byte) {
	var prettyJSON bytes.Buffer
	if err := json.Indent(&prettyJSON, raw, "", "\t"); err != nil {
		log.Printf("Could not format JSON string: %v\n", err)
	}
	if err := os.WriteFile(dest, prettyJSON.Bytes(), os.ModePerm); err != nil {
		log.Printf("Could not write received data to file: %v\n", err)
	}
	log.Printf("Stored received data in %s\n", dest)
}
