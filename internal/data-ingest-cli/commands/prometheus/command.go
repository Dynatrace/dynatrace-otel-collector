package prometheus

import (
	"context"
	"fmt"
	"net/http"
	"path"

	"github.com/Dynatrace/dynatrace-otel-collector/internal/data-ingest-cli/receiver"
	otlpreceiver "github.com/Dynatrace/dynatrace-otel-collector/internal/data-ingest-cli/receiver/otlp"
	"github.com/Dynatrace/dynatrace-otel-collector/internal/data-ingest-cli/receiver/otlphttp"
)

type Config struct {
	ReceiveData     bool
	InputFile       string
	OutputFile      string
	ServerPort      int
	ReceiverPort    int
	ReceiverType    string
	ReceiverTimeout int
}

type Cmd struct {
	receiver  receiver.Receiver
	inputFile string
	port      int
}

func New(cfg Config) (*Cmd, error) {
	c := &Cmd{
		inputFile: cfg.InputFile,
		port:      cfg.ServerPort,
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

func (c *Cmd) Do(ctx context.Context) error {
	if c.receiver != nil {
		if err := c.receiver.Start(); err != nil {
			return err
		}
		defer c.receiver.Stop()
	}

	endpoint := fmt.Sprintf("localhost:%d", c.port)
	dir := path.Dir(c.inputFile)
	fmt.Printf("Serving metrics at %s from %s\n", endpoint, dir)
	fmt.Println("Send an interrupt to stop")
	_ = http.ListenAndServe(endpoint, http.FileServer(http.Dir(dir)))

	return nil
}
