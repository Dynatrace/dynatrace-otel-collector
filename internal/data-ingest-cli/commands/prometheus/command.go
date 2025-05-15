package prometheus

import (
	"context"
	"fmt"
	"github.com/Dynatrace/dynatrace-otel-collector/internal/data-ingest-cli/receiver"
	otlpreceiver "github.com/Dynatrace/dynatrace-otel-collector/internal/data-ingest-cli/receiver/otlp"
	"github.com/Dynatrace/dynatrace-otel-collector/internal/data-ingest-cli/receiver/otlphttp"
	"net/http"
)

type Config struct {
	ReceiveData     bool
	OutputFile      string
	ServerPort      int
	ReceiverPort    int
	ReceiverType    string
	ReceiverTimeout int
	Payload         string
}

type Cmd struct {
	receiver receiver.Receiver
	port     int
	payload  string
}

func New(cfg Config) (*Cmd, error) {
	c := &Cmd{
		payload: cfg.Payload,
		port:    cfg.ServerPort,
	}

	if cfg.ReceiveData && cfg.ReceiverPort > 0 && cfg.OutputFile != "" {
		switch cfg.ReceiverType {
		case "grpc":
			c.receiver = otlpreceiver.NewOTLPReceiver(otlpreceiver.Config{
				Port:       cfg.ReceiverPort,
				OutputFile: cfg.OutputFile,
				Timeout:    cfg.ReceiverTimeout,
			})
		case "http":
			c.receiver = otlphttp.NewOTLPHTTPReceiver(otlphttp.Config{
				Port:       cfg.ReceiverPort,
				OutputFile: cfg.OutputFile,
				Timeout:    cfg.ReceiverTimeout,
			})
		default:
			return nil, fmt.Errorf("invalid receiver type %s", cfg.ReceiverType)
		}
	}

	return c, nil
}

func (c *Cmd) Do(_ context.Context) error {
	if c.receiver != nil {
		if err := c.receiver.Start(); err != nil {
			return err
		}
		defer c.receiver.Stop()
	}

	endpoint := fmt.Sprintf("localhost:%d", c.port)
	fmt.Println("Send an interrupt to stop")
	_ = http.ListenAndServe(endpoint, &promHttpHandler{payload: c.payload})

	return nil
}

type promHttpHandler struct {
	payload string
}

func (p promHttpHandler) ServeHTTP(writer http.ResponseWriter, _ *http.Request) {
	_, _ = writer.Write([]byte(p.payload))
}
