// Copyright Dynatrace LLC
// SPDX-License-Identifier: Apache-2.0

package otlphttp

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/Dynatrace/dynatrace-otel-collector/internal/data-ingest-cli/receiver"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/plog/plogotlp"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"
)

type Config struct {
	Port       int
	OutputFile string
	Timeout    int
}

type OTLPHTTPReceiver struct {
	config           Config
	receivedDataChan chan struct{}
}

func NewOTLPHTTPReceiver(c Config) *OTLPHTTPReceiver {
	return &OTLPHTTPReceiver{
		config:           c,
		receivedDataChan: make(chan struct{}),
	}
}

func (r *OTLPHTTPReceiver) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/traces", r.handleTraces)
	mux.HandleFunc("/v1/metrics", r.handleMetrics)
	mux.HandleFunc("/v1/logs", r.handleLogs)

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", r.config.Port))
	if err != nil {
		return fmt.Errorf("could not listen on %d: %w", r.config.Port, err)
	}

	server := &http.Server{Handler: mux}
	go func() {
		if err := server.Serve(lis); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP server error: %v\n", err)
		}
	}()
	return nil
}

func (r *OTLPHTTPReceiver) Stop() {
	select {
	case <-r.receivedDataChan:
	case <-time.After(time.Duration(r.config.Timeout) * time.Second):
	}
}

func (r *OTLPHTTPReceiver) handleTraces(w http.ResponseWriter, req *http.Request) {
	log.Println("Received traces")
	body, err, status := r.readRequest(req)
	if err != nil {
		http.Error(w, err.Error(), status)
		log.Fatalln(err)
		return
	}

	unmarshaler := ptrace.ProtoUnmarshaler{}
	traces, err := unmarshaler.UnmarshalTraces(body)
	if err != nil {
		log.Println("Failed to unmarshal traces to proto, checking JSON...")
		unmarshaler := ptrace.JSONUnmarshaler{}
		traces, err = unmarshaler.UnmarshalTraces(body)
		if err != nil {
			http.Error(w, "Failed to unmarshal traces", http.StatusBadRequest)
			log.Fatalln("Failed to unmarshal traces to JSON and proto")
			return
		}
	}

	tracesMarshaler := &ptrace.JSONMarshaler{}
	data, err := tracesMarshaler.MarshalTraces(traces)
	if err != nil {
		err := fmt.Errorf("Failed to marshal traces to JSON %s", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Fatalln(err)
		return
	}

	receiver.WriteToFile(r.config.OutputFile, data)

	resp := ptraceotlp.NewExportResponse()
	msg, err := resp.MarshalProto()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/x-protobuf")
	if _, err := w.Write(msg); err != nil {
		log.Fatalln(err)
	}

	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
	r.receivedDataChan <- struct{}{}
}

func (r *OTLPHTTPReceiver) handleMetrics(w http.ResponseWriter, req *http.Request) {
	log.Println("Received metrics")
	body, err, status := r.readRequest(req)
	if err != nil {
		http.Error(w, err.Error(), status)
		log.Fatalln(err)
		return
	}

	unmarshaler := pmetric.ProtoUnmarshaler{}
	metrics, err := unmarshaler.UnmarshalMetrics(body)
	if err != nil {
		log.Println("Failed to unmarshal metrics to proto, checking JSON...")
		unmarshaler := pmetric.JSONUnmarshaler{}
		metrics, err = unmarshaler.UnmarshalMetrics(body)
		if err != nil {
			http.Error(w, "Failed to unmarshal metrics", http.StatusBadRequest)
			log.Fatalln("Failed to unmarshal metrics to JSON and proto")
			return
		}
	}

	metricsMarshaler := &pmetric.JSONMarshaler{}
	data, err := metricsMarshaler.MarshalMetrics(metrics)
	if err != nil {
		err := fmt.Errorf("Failed to marshal metrics to JSON %s", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Fatalln(err)
		return
	}

	receiver.WriteToFile(r.config.OutputFile, data)

	resp := pmetricotlp.NewExportResponse()
	msg, err := resp.MarshalProto()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/x-protobuf")
	if _, err := w.Write(msg); err != nil {
		log.Fatalln(err)
	}

	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
	r.receivedDataChan <- struct{}{}
}

func (r *OTLPHTTPReceiver) handleLogs(w http.ResponseWriter, req *http.Request) {
	log.Println("Received logs")
	body, err, status := r.readRequest(req)
	if err != nil {
		http.Error(w, err.Error(), status)
		log.Fatalln(err)
		return
	}

	unmarshaler := plog.ProtoUnmarshaler{}
	logs, err := unmarshaler.UnmarshalLogs(body)
	if err != nil {
		log.Println("Failed to unmarshal logs to proto, checking JSON...")
		unmarshaler := plog.JSONUnmarshaler{}
		logs, err = unmarshaler.UnmarshalLogs(body)
		if err != nil {
			http.Error(w, "Failed to unmarshal logs", http.StatusBadRequest)
			log.Fatalln("Failed to unmarshal logs to JSON and proto")
			return
		}
	}

	logsMarshaler := &plog.JSONMarshaler{}
	data, err := logsMarshaler.MarshalLogs(logs)
	if err != nil {
		err := fmt.Errorf("Failed to marshal logs to JSON %s", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Fatalln(err)
		return
	}

	receiver.WriteToFile(r.config.OutputFile, data)

	resp := plogotlp.NewExportResponse()
	msg, err := resp.MarshalProto()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/x-protobuf")
	if _, err := w.Write(msg); err != nil {
		log.Fatalln(err)
	}

	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
	r.receivedDataChan <- struct{}{}
}

func (r *OTLPHTTPReceiver) readRequest(req *http.Request) ([]byte, error, int) {
	if req.Method != http.MethodPost {
		err := fmt.Errorf("Invalid request method %s", req.Method)
		return nil, err, http.StatusMethodNotAllowed
	}

	defer req.Body.Close()
	body, err := io.ReadAll(req.Body)
	if err != nil {
		err := fmt.Errorf("Failed to read request body %s", err.Error())
		return nil, err, http.StatusMethodNotAllowed
	}

	return body, nil, 0
}
