package otlphttp

import (
	"fmt"
	"io"
	"log"
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
	http.HandleFunc("/v1/traces", r.handleTraces)
	http.HandleFunc("/v1/metrics", r.handleMetrics)
	http.HandleFunc("/v1/logs", r.handleLogs)

	server := &http.Server{
		Addr: fmt.Sprintf(":%d", r.config.Port),
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Could not listen on %d: %v\n", r.config.Port, err)
		}
	}()
	return nil
}

func (r *OTLPHTTPReceiver) Stop() {
	select {
	case <-r.receivedDataChan:
	case <-time.After(10 * time.Second):
	}
}

func (r *OTLPHTTPReceiver) handleTraces(w http.ResponseWriter, req *http.Request) {
	log.Println("Received metrics")
	body, err, status := r.readRequest(w, req)
	if err != nil {
		http.Error(w, err.Error(), status)
		log.Fatalln(err)
		return
	}

	req.Body.Close()

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

	w.(http.Flusher).Flush()
	r.receivedDataChan <- struct{}{}
}

func (r *OTLPHTTPReceiver) handleMetrics(w http.ResponseWriter, req *http.Request) {
	log.Println("Received metrics")
	body, err, status := r.readRequest(w, req)
	if err != nil {
		http.Error(w, err.Error(), status)
		log.Fatalln(err)
		return
	}

	req.Body.Close()

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

	w.(http.Flusher).Flush()
	r.receivedDataChan <- struct{}{}
}

func (r *OTLPHTTPReceiver) handleLogs(w http.ResponseWriter, req *http.Request) {
	log.Println("Received metrics")
	body, err, status := r.readRequest(w, req)
	if err != nil {
		http.Error(w, err.Error(), status)
		log.Fatalln(err)
		return
	}

	req.Body.Close()

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

	w.(http.Flusher).Flush()
	r.receivedDataChan <- struct{}{}
}

func (r *OTLPHTTPReceiver) readRequest(w http.ResponseWriter, req *http.Request) ([]byte, error, int) {
	if req.Method != http.MethodPost {
		err := fmt.Errorf("Invalid request method %s", req.Method)
		return nil, err, http.StatusMethodNotAllowed
	}

	body, err := io.ReadAll(req.Body)
	if err != nil {
		err := fmt.Errorf("Failed to read request body %s", err.Error())
		return nil, err, http.StatusMethodNotAllowed
	}

	defer req.Body.Close()

	return body, nil, 0
}
