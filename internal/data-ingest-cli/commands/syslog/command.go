package syslog

import (
	"context"
	"fmt"
	"os"

	"github.com/Dynatrace/dynatrace-otel-collector/internal/data-ingest-cli/receiver"
	otlpreceiver "github.com/Dynatrace/dynatrace-otel-collector/internal/data-ingest-cli/receiver/otlp"
	"github.com/Dynatrace/dynatrace-otel-collector/internal/data-ingest-cli/receiver/otlphttp"
	"github.com/Dynatrace/dynatrace-otel-collector/internal/data-ingest-cli/sender/syslog"
)

type Config struct {
	InputFile    string
	CollectorURL string
	Transport    string
	OutputFile   string
	ReceiverPort int
	ReceiverType string
}

type Cmd struct {
	cfg      Config
	receiver receiver.Receiver
}

func New(cfg Config) (*Cmd, error) {
	if cfg.Transport != "tcp" && cfg.Transport != "udp" {
		return nil, fmt.Errorf("invalid transport: %q", cfg.Transport)
	}

	c := &Cmd{
		cfg: cfg,
	}

	if cfg.ReceiverPort > 0 && cfg.OutputFile != "" {
		switch cfg.ReceiverType {
		case "grpc":
			c.receiver = otlpreceiver.NewOTLPReceiver(otlpreceiver.Config{
				Port:       cfg.ReceiverPort,
				OutputFile: cfg.OutputFile,
			})
		case "http":
			c.receiver = otlphttp.NewOTLPHTTPReceiver(otlphttp.Config{
				Port:       cfg.ReceiverPort,
				OutputFile: cfg.OutputFile,
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
	return c.sendLogs(ctx)
}

func (c *Cmd) sendLogs(ctx context.Context) error {
	fileContent, err := os.ReadFile(c.cfg.InputFile)
	if err != nil {
		return err
	}

	sender, err := syslog.Connect(ctx, &syslog.Config{
		Endpoint:  c.cfg.CollectorURL,
		Transport: c.cfg.Transport,
	})
	if err != nil {
		return err
	}

	return sender.Write(ctx, string(fileContent))
}
