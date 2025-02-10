package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/Dynatrace/dynatrace-otel-collector/internal/data-ingest-cli/commands/fluent"
	"log"

	"github.com/Dynatrace/dynatrace-otel-collector/internal/data-ingest-cli/commands/otlpjson"
	"github.com/Dynatrace/dynatrace-otel-collector/internal/data-ingest-cli/commands/statsd"
	"github.com/Dynatrace/dynatrace-otel-collector/internal/data-ingest-cli/commands/syslog"
	"github.com/Dynatrace/dynatrace-otel-collector/internal/data-ingest-cli/commands/zipkin"
)

func main() {
	// Define the CLI arguments
	inputFile := flag.String("input-file", "", "Path to the input file containing input data")
	collectorURL := flag.String("collector-url", "localhost:4317", "URL of the OpenTelemetry collector")
	outputFile := flag.String("output-file", "", "Path to the file where received OTLP data will be stored")
	inputFormat := flag.String("input-format", "otlp-json", "Input format (options: 'otlp-json', 'syslog', 'statsd')")
	statsdProtocol := flag.String("statsd-protocol", "udp4", "Statsd protocol to send metrics (options: 'udp', 'udp4', 'udp6', 'tcp', 'tcp4', 'tcp6', 'unixgram')")
	otlpSignalType := flag.String("otlp-signal-type", "", "OTLP signal type (options: 'logs', 'traces', 'metrics')")
	syslogTransport := flag.String("syslog-transport", "tcp", "Syslog network transport (options: 'udp', 'tcp')")
	receiverPort := flag.Int("receiver-port", 0, "OTLP Receiver port. If set, the tool will open a grpc server on the specified port to receive data and store it in an output file")
	receiverType := flag.String("receiver-type", "http", "The type of receiver created to act as a sink for the collector (options: `http`, `grpc`)")
	zipkinVersion := flag.String("zipkin-version", "v2", "The version of zipkin traces (options: `v1`, `v2`)")

	// Parse the CLI arguments
	flag.Parse()

	// Validate required arguments
	if *collectorURL == "" {
		log.Fatal("collector-url is required")
	}

	fmt.Println("Input File:", *inputFile)
	fmt.Println("Collector URL:", *collectorURL)
	fmt.Println("Output File:", *outputFile)
	fmt.Println("Input Format:", *inputFormat)
	fmt.Println("OTLP Signal Type:", *otlpSignalType)
	fmt.Println("Statsd protocol:", *statsdProtocol)
	fmt.Println("Syslog transport:", *syslogTransport)
	fmt.Println("Receiver type:", *receiverType)

	switch *inputFormat {
	case "otlp-json":
		fmt.Println("Reading otlpjson data and sending it to collector...")
		cmd, err := otlpjson.New(otlpjson.Config{
			InputFile:    *inputFile,
			CollectorURL: *collectorURL,
			SignalType:   *otlpSignalType,
			OutputFile:   *outputFile,
			ReceiverPort: *receiverPort,
			ReceiverType: *receiverType,
		})
		if err != nil {
			log.Fatalf("could not create otlp-json sender: %s", err.Error())
		}
		if err := cmd.Do(context.Background()); err != nil {
			log.Fatalf("could not execute command: %s", err.Error())
		}
	case "syslog":
		fmt.Println("Reading syslog data and sending it to collector...")
		cmd, err := syslog.New(syslog.Config{
			InputFile:    *inputFile,
			CollectorURL: *collectorURL,
			Transport:    *syslogTransport,
			OutputFile:   *outputFile,
			ReceiverPort: *receiverPort,
			ReceiverType: *receiverType,
		})
		if err != nil {
			log.Fatalf("could not create syslog sender: %s", err.Error())
		}
		if err := cmd.Do(context.Background()); err != nil {
			log.Fatalf("could not execute command: %s", err.Error())
		}
	case "statsd":
		log.Println("Reading from statsd and sending to collector...")
		cmd, err := statsd.New(statsd.Config{
			InputFile:    *inputFile,
			CollectorURL: *collectorURL,
			SignalType:   *otlpSignalType,
			OutputFile:   *outputFile,
			ReceiverPort: *receiverPort,
			Protocol:     *statsdProtocol,
			ReceiverType: *receiverType,
		})
		if err != nil {
			log.Fatalf("could not create statsd sender: %s", err.Error())
		}
		if err := cmd.Do(context.Background()); err != nil {
			log.Fatalf("could not execute command: %s", err.Error())
		}
	case "zipkin":
		log.Println("Reading from zipkin and sending to collector...")
		cmd, err := zipkin.New(zipkin.Config{
			InputFile:     *inputFile,
			CollectorURL:  *collectorURL,
			SignalType:    *otlpSignalType,
			OutputFile:    *outputFile,
			ReceiverPort:  *receiverPort,
			ReceiverType:  *receiverType,
			ZipkinVersion: *zipkinVersion,
		})
		if err != nil {
			log.Fatalf("could not create zipkin sender: %s", err.Error())
		}
		if err := cmd.Do(context.Background()); err != nil {
			log.Fatalf("could not execute command: %s", err.Error())
		}
	case "fluent":
		log.Println("Reading from fluent and sending to collector...")
		cmd, err := fluent.New(fluent.Config{
			InputFile:    *inputFile,
			CollectorURL: *collectorURL,
			OutputFile:   *outputFile,
			ReceiverPort: *receiverPort,
			ReceiverType: *receiverType,
		})
		if err != nil {
			log.Fatalf("could not execute command: %s", err.Error())
		}
		if err := cmd.Do(context.Background()); err != nil {
			log.Fatalf("could not execute command: %s", err.Error())
		}
	default:
		log.Fatalf("Unknown input format: %s", *inputFormat)
	}
}
