package receive

import (
	"context"
	"fmt"

	"github.com/Dynatrace/dynatrace-otel-collector/internal/data-ingest-cli/receiver"
	otlpreceiver "github.com/Dynatrace/dynatrace-otel-collector/internal/data-ingest-cli/receiver/otlp"
	"github.com/Dynatrace/dynatrace-otel-collector/internal/data-ingest-cli/receiver/otlphttp"
)

type Config struct {
	OutputFile      string
	ReceiverPort    int
	ReceiverType    string
	ReceiverTimeout int
}

type Cmd struct {
	receiver receiver.Receiver
}

func New(p Config) (*Cmd, error) {
	c := &Cmd{}

	if p.ReceiverPort > 0 && p.OutputFile != "" {
		switch p.ReceiverType {
		case "grpc":
			c.receiver = otlpreceiver.NewOTLPReceiver(otlpreceiver.Config{
				Port:       p.ReceiverPort,
				OutputFile: p.OutputFile,
				Timeout:    p.ReceiverTimeout,
			})
		case "http":
			c.receiver = otlphttp.NewOTLPHTTPReceiver(otlphttp.Config{
				Port:       p.ReceiverPort,
				OutputFile: p.OutputFile,
				Timeout:    p.ReceiverTimeout,
			})
		default:
			return nil, fmt.Errorf("invalid receiver type %s", p.ReceiverType)
		}
	}

	return c, nil
}

func (c *Cmd) Do(_ context.Context) error {
	if c.receiver == nil {
		return fmt.Errorf("no receiver has been set up")
	}
	if err := c.receiver.Start(); err != nil {
		return err
	}
	defer c.receiver.Stop()
	return nil
}
