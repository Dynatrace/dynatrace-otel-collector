package fluent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strconv"

	"github.com/Dynatrace/dynatrace-otel-collector/internal/data-ingest-cli/receiver"
	otlpreceiver "github.com/Dynatrace/dynatrace-otel-collector/internal/data-ingest-cli/receiver/otlp"
	"github.com/Dynatrace/dynatrace-otel-collector/internal/data-ingest-cli/receiver/otlphttp"
	"github.com/Dynatrace/dynatrace-otel-collector/internal/data-ingest-cli/sender/fluent"
)

var errMissingFluentProperties = fmt.Errorf("test data must be a json object containing a 'tag' and 'message' property")

type Config struct {
	SendData     bool
	ReceiveData  bool
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
	sender   fluent.Sender
}

func New(cfg Config) (*Cmd, error) {
	c := &Cmd{
		cfg: cfg,
	}

	if cfg.ReceiveData && cfg.ReceiverPort > 0 && cfg.OutputFile != "" {
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

	if cfg.SendData {
		collectorURL, err := url.Parse(cfg.CollectorURL)
		if err != nil {
			return nil, err
		}
		port, err := strconv.Atoi(collectorURL.Port())
		if err != nil {
			return nil, err
		}
		sender, err := fluent.New(collectorURL.Hostname(), port)
		if err != nil {
			return nil, err
		}
		c.sender = sender
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
	return c.sendLogs()
}

func (c *Cmd) sendLogs() error {
	if c.sender == nil {
		return nil
	}
	fileContent, err := os.ReadFile(c.cfg.InputFile)
	if err != nil {
		return err
	}

	content := map[string]any{}

	if err := json.Unmarshal(fileContent, &content); err != nil {
		return err
	}

	tag, ok := content["tag"].(string)
	if !ok {
		return errMissingFluentProperties
	}

	msg, ok := content["message"].(map[string]interface{})
	if !ok {
		return errMissingFluentProperties
	}

	return c.sender.Write(tag, msg)
}
