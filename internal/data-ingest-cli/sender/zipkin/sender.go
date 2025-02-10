package zipkin

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/zipkin/zipkinv2"
	zipkinreporter "github.com/openzipkin/zipkin-go/reporter"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

type Sender interface {
	SendTraces(ctx context.Context, traces ptrace.Traces) error
}

type ZipkinSender struct {
	url string
}

func New(url string) (*ZipkinSender, error) {
	return &ZipkinSender{
		url: url,
	}, nil
}

func (s *ZipkinSender) SendTraces(ctx context.Context, traces ptrace.Traces) error {
	log.Printf("Sending traces to %s\n", s.url)

	var translator zipkinv2.FromTranslator
	spans, err := translator.FromTraces(traces)
	if err != nil {
		return fmt.Errorf("failed to push trace data via Zipkin sender: %w", err)
	}

	serializer := zipkinreporter.JSONSerializer{}
	body, err := serializer.Serialize(spans)
	if err != nil {
		return fmt.Errorf("failed to push trace data via Zipkin sender: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to push trace data via Zipkin sender: %w", err)
	}
	req.Header.Set("Content-Type", serializer.ContentType())

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to push trace data via Zipkin sender: %w", err)
	}

	_ = resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("failed the request with status code %d", resp.StatusCode)
	}

	return nil
}
