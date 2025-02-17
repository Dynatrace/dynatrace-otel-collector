package jaeger

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/Dynatrace/dynatrace-otel-collector/internal/data-ingest-cli/receiver"
	otlpreceiver "github.com/Dynatrace/dynatrace-otel-collector/internal/data-ingest-cli/receiver/otlp"
	"github.com/Dynatrace/dynatrace-otel-collector/internal/data-ingest-cli/receiver/otlphttp"
	"github.com/Dynatrace/dynatrace-otel-collector/internal/data-ingest-cli/sender/jaeger"
	"github.com/jaegertracing/jaeger/model"
)

type Config struct {
	InputFile    string
	CollectorURL string
	SignalType   string
	OutputFile   string
	ReceiverPort int
	ReceiverType string
}

type Cmd struct {
	sender     jaeger.Sender
	receiver   receiver.Receiver
	signalType string
	inputFile  string
}

func New(p Config) (*Cmd, error) {
	c := &Cmd{
		signalType: p.SignalType,
		inputFile:  p.InputFile,
	}

	sender, err := jaeger.New(p.CollectorURL)
	if err != nil {
		return nil, err
	}

	c.sender = sender

	if p.ReceiverPort > 0 && p.OutputFile != "" {
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
		return fmt.Errorf("jaeger sender support only traces signal type, used: %s", c.signalType)
	}
}

type data struct {
	Data []*model.Batch `json:",omitempty"`
}

func (c *Cmd) sendTraces(ctx context.Context) error {
	fileContent, err := os.ReadFile(c.inputFile)
	if err != nil {
		return fmt.Errorf("could not read file content: %s", err.Error())
	}

	content := data{}
	if err := json.Unmarshal(fileContent, &content); err != nil {
		return fmt.Errorf("could not unmarshall data: %s", err.Error())
	}

	// Assert that "data" is a slice of interfaces
	// data, ok := content["data"].([]interface{})
	// if !ok {
	// 	return fmt.Errorf("error: data is not an array")
	// }

	// // Access the first item in the "data" array
	// if len(data) == 0 {
	// 	return fmt.Errorf("error: data array is empty")
	// }

	// firstItem := data[0].(map[string]interface{})

	// // Marshal the first item if needed
	// firstItemJSON, err := json.Marshal(firstItem)
	// if err != nil {
	// 	return fmt.Errorf("could not marshal first item: %s", err.Error())
	// }

	// trace := &model.Batch{}
	// err = trace.Unmarshal(firstItemJSON)
	// //err = json.Unmarshal(firstItemJSON, trace)
	// if err != nil {
	// 	return fmt.Errorf("could not unmarshal traces: %s", err.Error())
	// }

	return c.sender.SendTraces(ctx, content.Data[0])
}

// func jaegerSpanToTraces(span *jaegerproto.Span) (ptrace.Traces, error) {
// 	batch := jaegerproto.Batch{
// 		Spans:   []*jaegerproto.Span{span},
// 		Process: span.Process,
// 	}
// 	return pkgjaeger.ProtoToTraces([]*jaegerproto.Batch{&batch})
// }

// func readTracesFromFile(filename string) ptrace.Traces {
// 	data, err := ioutil.ReadFile(filename)
// 	if err != nil {
// 		log.Fatalf("failed to read file: %v", err)
// 	}

// 	var traces []Trace
// 	err = json.Unmarshal(data, &traces)
// 	if err != nil {
// 		log.Fatalf("failed to unmarshal JSON: %v", err)
// 	}

// 	otelTraces := ptrace.NewTraces()
// 	rs := otelTraces.ResourceSpans().AppendEmpty()
// 	ils := rs.ScopeSpans().AppendEmpty()

// 	for _, t := range traces {
// 		span := ils.Spans().AppendEmpty()
// 		span.SetTraceID(ptrace.TraceID(t.TraceID))
// 		span.SetSpanID(ptrace.SpanID(t.SpanID))
// 		span.SetName(t.OperationName)
// 		startTime, _ := time.Parse(time.RFC3339, t.StartTime)
// 		span.SetStartTimestamp(ptrace.Timestamp(startTime.UnixNano()))
// 		span.SetEndTimestamp(ptrace.Timestamp(startTime.Add(time.Duration(t.Duration) * time.Microsecond).UnixNano()))

// 		for k, v := range t.Attributes {
// 			span.Attributes().PutStr(k, v)
// 		}
// 	}

// 	return otelTraces
// }
