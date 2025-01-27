package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/Dynatrace/dynatrace-otel-collector/internal/data-ingest-cli/commands/otlpjson"
	"log"
)

func main() {
	// Define the CLI arguments
	inputFile := flag.String("input-file", "", "Path to the input file containing input data")
	collectorURL := flag.String("collector-url", "", "URL of the OpenTelemetry collector")
	outputFile := flag.String("output-file", "", "Path to the file where received OTLP data will be stored")
	inputFormat := flag.String("input-format", "otlp-json", "Input format (options: 'otlp-json', 'syslog', 'systemd')")
	otlpSignalType := flag.String("otlp-signal-type", "", "OTLP signal type (options: 'logs', 'traces', 'metrics')")
	receiverPort := flag.Int("receiver-port", 0, "OTLP Receiver port. If set, the tool will open a grpc server on the specified port to receive data and store it in an output file")

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

	switch *inputFormat {
	case "otlp-json":
		cmd, err := otlpjson.New(otlpjson.Config{
			InputFile:    *inputFile,
			CollectorURL: *collectorURL,
			SignalType:   *otlpSignalType,
			OutputFile:   *outputFile,
			ReceiverPort: *receiverPort,
		})
		if err != nil {
			log.Fatalf("could not execute command: %s", err.Error())
		}
		if err := cmd.Do(context.Background()); err != nil {
			log.Fatalf("could not execute command: %s", err.Error())
		}
	case "syslog":
		// Handle reading from syslog and sending to collector
		fmt.Println("Reading from syslog and sending to collector...")
	case "systemd":
		log.Println("Reading from systemd and sending to collector...")
	default:
		log.Fatalf("Unknown input format: %s", *inputFormat)
	}

	if *outputFile != "" {
		fmt.Println("Receiving OTLP data and storing it in file...")
	}
}
