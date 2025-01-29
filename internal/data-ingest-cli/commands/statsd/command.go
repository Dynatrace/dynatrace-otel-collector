package statsd

import (
	"context"
	"fmt"
	"os"

	otlpreceiver "github.com/Dynatrace/dynatrace-otel-collector/internal/data-ingest-cli/receiver/otlp"
	"github.com/Dynatrace/dynatrace-otel-collector/internal/data-ingest-cli/sender/statsd"
)

type Config struct {
	InputFile    string
	CollectorURL string
	SignalType   string
	OutputFile   string
	ReceiverPort int
	Protocol     string
}

type Cmd struct {
	sender     statsd.Sender
	receiver   *otlpreceiver.OTLPReceiver
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

	if p.ReceiverPort > 0 && p.OutputFile != "" {
		receiver, err := otlpreceiver.NewOTLPReceiver(otlpreceiver.Config{
			Port:       p.ReceiverPort,
			OutputFile: p.OutputFile,
		})
		if err != nil {
			return nil, err
		}

		c.receiver = receiver
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
	fileContent, err := os.ReadFile(c.inputFile)
	if err != nil {
		return err
	}

	return c.sender.SendMetrics(ctx, fileContent)
}
