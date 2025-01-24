package main

import (
	"flag"
	"fmt"
	"log"
)

func main() {
	// Define the CLI arguments
	inputFile := flag.String("input-file", "", "Path to the input file containing OTLP JSON data (required if input-source is 'file')")
	collectorURL := flag.String("collector-url", "", "URL of the OpenTelemetry collector")
	outputFile := flag.String("output-file", "", "Path to the file where received OTLP data will be stored")
	inputFormat := flag.String("input-format", "otlp-json", "Input format (options: 'file', 'syslog', 'systemd')")

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

	switch *inputFormat {
	case "file":
		// Handle reading from file and sending to collector
		fmt.Println("Reading from file and sending to collector...")
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
