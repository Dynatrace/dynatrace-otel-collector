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
	Data dataItem `json:"spans,omitempty"`
}

type dataItem struct {
	Batch model.Batch
}

func (c *Cmd) sendTraces(ctx context.Context) error {
	fileContent, err := os.ReadFile(c.inputFile)
	if err != nil {
		return fmt.Errorf("could not read file content: %s", err.Error())
	}

	content := data{}

	if err := json.Unmarshal(fileContent, &content); err != nil {
		return err
	}

	data := content.Data.Batch

	bytes, err := content.Data.Batch.Marshal()
	if err != nil {
		return fmt.Errorf("could not marshal traces: %s", err.Error())
	}

	//Unmarshal the file content into an array of raw messages
	// var rawMessages []json.RawMessage
	// err = json.Unmarshal(fileContent, &rawMessages)
	// if err != nil {
	// 	return fmt.Errorf("could not unmarshal file content: %s", err.Error())
	// }

	//Iterate through the array and unmarshal each trace into a model.Batch
	// var batches []*model.Batch
	// for _, rawMessage := range rawMessages {
	trace := &model.Batch{}
	//err = jsonpb.Unmarshal(bytes.NewReader(fileContent), trace)
	err = trace.Unmarshal([]byte(data))
	if err != nil {
		return fmt.Errorf("could not unmarshal traces: %s", err.Error())
	}
	// 	batches = append(batches, trace)
	// }

	//Unmarshal the file content into an array of raw messages
	// var rawMessages []json.RawMessage
	// err = json.Unmarshal(fileContent, &rawMessages)
	// if err != nil {
	// 	return fmt.Errorf("could not unmarshal file content: %s", err.Error())
	//}

	//Iterate through the array and unmarshal each trace into a model.Batch
	// var batches []*model.Batch
	// for _, rawMessage := range rawMessages {
	// 	trace := &model.Batch{}
	// 	err = json.Unmarshal(rawMessage, trace)
	// 	//err = trace.Unmarshal(rawMessage)
	// 	if err != nil {
	// 		return fmt.Errorf("could not unmarshal trace: %s", err.Error())
	// 	}
	// 	batches = append(batches, trace)
	// }

	// span := &jaegerproto.Span{}
	// err = jsonpb.Unmarshal(bytes.NewReader(fileContent), span)
	// if err != nil {
	// 	return fmt.Errorf("could not unmarshal trace: %s", err.Error())
	// }

	// tracestt, err := jaegerSpanToTraces(span)
	// if err != nil {
	// 	return fmt.Errorf("could not unmffffarshal trace: %s", err.Error())
	// }

	return c.sender.SendTraces(ctx, trace)
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
