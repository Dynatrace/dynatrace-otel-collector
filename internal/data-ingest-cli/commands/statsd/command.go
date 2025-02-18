package statsd

import (
	"context"
	"fmt"
	"os"

	"github.com/Dynatrace/dynatrace-otel-collector/internal/data-ingest-cli/receiver"
	otlpreceiver "github.com/Dynatrace/dynatrace-otel-collector/internal/data-ingest-cli/receiver/otlp"
	"github.com/Dynatrace/dynatrace-otel-collector/internal/data-ingest-cli/receiver/otlphttp"
	"github.com/Dynatrace/dynatrace-otel-collector/internal/data-ingest-cli/sender/statsd"
)

type Config struct {
	ReceiveData     bool
	InputFile       string
	CollectorURL    string
	SignalType      string
	OutputFile      string
	ReceiverPort    int
	Protocol        string
	ReceiverType    string
	ReceiverTimeout int
}

type Cmd struct {
	sender     statsd.Sender
	receiver   receiver.Receiver
	signalType string
	inputFile  string
}

func New(p Config) (*Cmd, error) {
	if p.SignalType != "metrics" {
		return nil, fmt.Errorf("only 'metrics' signal type is supported for statsd")
	}

	c := &Cmd{
		signalType: p.SignalType,
		inputFile:  p.InputFile,
	}

	sender, err := statsd.New(p.CollectorURL, p.Protocol)
	if err != nil {
		return nil, err
	}
	c.sender = sender

	if p.ReceiveData && p.ReceiverPort > 0 && p.OutputFile != "" {
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

func (c *Cmd) Do(ctx context.Context) error {
	if c.receiver != nil {
		if err := c.receiver.Start(); err != nil {
			return err
		}
		defer c.receiver.Stop()
	}

	return c.sendMetrics(ctx)
}

func (c *Cmd) sendMetrics(ctx context.Context) error {
	if c.sender == nil {
		return nil
	}
	fileContent, err := os.ReadFile(c.inputFile)
	if err != nil {
		return err
	}

	return c.sender.SendMetrics(ctx, fileContent)
}
