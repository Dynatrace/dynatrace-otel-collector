package otlpjson

import (
	"context"
	"fmt"
	otlpreceiver "github.com/Dynatrace/dynatrace-otel-collector/internal/data-ingest-cli/receiver/otlp"
	"github.com/Dynatrace/dynatrace-otel-collector/internal/data-ingest-cli/sender/otlp"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"os"
)

type Config struct {
	InputFile    string
	CollectorURL string
	SignalType   string
	OutputFile   string
	ReceiverPort int
}

type Cmd struct {
	sender     otlp.Sender
	receiver   *otlpreceiver.OTLPReceiver
	signalType string
	inputFile  string
}

func New(p Config) (*Cmd, error) {
	c := &Cmd{
		signalType: p.SignalType,
		inputFile:  p.InputFile,
	}

	sender, err := otlp.New(p.CollectorURL)
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
	switch c.signalType {
	case "metrics":
		return c.sendMetrics(ctx)
	case "logs":
		return c.sendLogs(ctx)
	case "trace", "traces":
		return c.sendTraces(ctx)
	default:
		return fmt.Errorf("unknown signal type '%s'. Must be one of [traces,logs,metrics]", c.signalType)
	}
}

func (c *Cmd) sendMetrics(ctx context.Context) error {
	fileContent, err := os.ReadFile(c.inputFile)
	if err != nil {
		return err
	}
	metricsUnmarshaler := &pmetric.JSONUnmarshaler{}
	metrics, err := metricsUnmarshaler.UnmarshalMetrics(fileContent)
	if err != nil {
		return err
	}
	return c.sender.SendMetrics(ctx, metrics)
}

func (c *Cmd) sendLogs(ctx context.Context) error {
	fileContent, err := os.ReadFile(c.inputFile)
	if err != nil {
		return err
	}
	logsUnmarshaler := &plog.JSONUnmarshaler{}
	logs, err := logsUnmarshaler.UnmarshalLogs(fileContent)
	if err != nil {
		return err
	}
	return c.sender.SendLogs(ctx, logs)
}

func (c *Cmd) sendTraces(ctx context.Context) error {
	fileContent, err := os.ReadFile(c.inputFile)
	if err != nil {
		return err
	}
	traceUnmarshaler := &ptrace.JSONUnmarshaler{}
	traces, err := traceUnmarshaler.UnmarshalTraces(fileContent)

	if err != nil {
		return err
	}

	return c.sender.SendTraces(ctx, traces)
}
