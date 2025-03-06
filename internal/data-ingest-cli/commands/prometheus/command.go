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
	ReceiverPort    int
	ReceiverType    string
	ReceiverTimeout int
}

type Cmd struct {
	receiver  receiver.Receiver
	inputFile string
}

func New(p Config) (*Cmd, error) {
	c := &Cmd{
		inputFile: p.InputFile,
	}

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

	dir := path.Dir(c.inputFile)
	fmt.Println("Serving metrics at localhost:9100 from " + dir)
	fmt.Println("Send an interrupt to stop")
	_ = http.ListenAndServe("localhost:9100", http.FileServer(http.Dir(dir)))

	return nil
}
