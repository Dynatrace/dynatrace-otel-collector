package zipkin

import (
	"context"
	"fmt"
	"os"

	"github.com/Dynatrace/dynatrace-otel-collector/internal/data-ingest-cli/receiver"
	otlpreceiver "github.com/Dynatrace/dynatrace-otel-collector/internal/data-ingest-cli/receiver/otlp"
	"github.com/Dynatrace/dynatrace-otel-collector/internal/data-ingest-cli/receiver/otlphttp"
	"github.com/Dynatrace/dynatrace-otel-collector/internal/data-ingest-cli/sender/zipkin"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/zipkin/zipkinv1"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/zipkin/zipkinv2"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

type Config struct {
	SendData      bool
	ReceiveData   bool
	InputFile     string
	CollectorURL  string
	SignalType    string
	OutputFile    string
	ReceiverPort  int
	ReceiverType  string
	ZipkinVersion string
}

type Cmd struct {
	sender        zipkin.Sender
	receiver      receiver.Receiver
	signalType    string
	inputFile     string
	zipkinVersion string
}

func New(p Config) (*Cmd, error) {
	c := &Cmd{
		signalType:    p.SignalType,
		inputFile:     p.InputFile,
		zipkinVersion: p.ZipkinVersion,
	}

	if p.SendData {
		sender, err := zipkin.New(p.CollectorURL)
		if err != nil {
			return nil, err
		}
		c.sender = sender
	}

	if p.ReceiveData && p.ReceiverPort > 0 && p.OutputFile != "" {
		switch p.ReceiverType {
		case "grpc":
			c.receiver = otlpreceiver.NewOTLPReceiver(otlpreceiver.Config{
				Port:       p.ReceiverPort,
				OutputFile: p.OutputFile,
			})
		case "http":
			c.receiver = otlphttp.NewOTLPHTTPReceiver(otlphttp.Config{
				Port:       p.ReceiverPort,
				OutputFile: p.OutputFile,
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
	switch c.signalType {
	case "trace", "traces":
		return c.sendTraces(ctx)
	default:
		return fmt.Errorf("zipkin sender support only traces signal type, used: %s", c.signalType)
	}
}

func (c *Cmd) sendTraces(ctx context.Context) error {
	if c.sender == nil {
		return nil
	}
	fileContent, err := os.ReadFile(c.inputFile)
	if err != nil {
		return fmt.Errorf("could not read file content: %s", err.Error())
	}

	var traceUnmarshaler ptrace.Unmarshaler
	switch c.zipkinVersion {
	case "v1":
		traceUnmarshaler = zipkinv1.NewJSONTracesUnmarshaler(false)
	case "v2":
		traceUnmarshaler = zipkinv2.NewJSONTracesUnmarshaler(false)
	default:
		return fmt.Errorf("unsupported zipkin trace version: %s", c.zipkinVersion)
	}

	traces, err := traceUnmarshaler.UnmarshalTraces(fileContent)
	if err != nil {
		return fmt.Errorf("could not unmarshall traces: %s", err.Error())
	}

	return c.sender.SendTraces(ctx, traces)
}
