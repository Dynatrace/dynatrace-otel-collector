# OTLP Data Ingest Tool

## Overview

The primary purpose of this CLI Tool is to assist in reproducing support cases and troubleshooting issues within the OpenTelemetry collector pipeline.
It allows users to accurately replicate conditions under which issues occur by reading data in different formats and sending it to the collector.
Additionally, the tool can receive OTLP data via an endpoint and store it in a JSON file for detailed inspection and analysis,
making it easier to diagnose and resolve problems related to OTLP data handling.

*Note:* This is a developer tool used mainly for debugging, and not intended for being used within a production environment.

## Features

- Read data from a file: The tool can read data from a specified file containing data in different formats and send it to an OpenTelemetry collector.
 Supported formats are:
  - OTLP Json
  - Statsd
  - Syslog

- Receive OTLP data via an endpoint: The tool can receive OTLP data through an endpoint and store the received payload in a specified file in OTLP Json format.

- Send data read from the JSON file to an OTLP endpoint via gRPC.

- Send statsd data read from a plain/text file and send it to the Collector endpoint via the chosen protocol.

## Building

To build the CLI, execute the following command:

```shell
go build -o data-ingest
```

## Usage

The tool accepts the following input parameters:

- `--input-file`: The name of the input file to read data from.
- `--input-format`: The input format of the ingested data (options: `otlp-json`, `syslog`, `statsd`).
- `--collector-url`: The URL of the OpenTelemetry collector.
- `--output-file`: The file in which to store the received data.
- `--receiver-port`: The port of the OTLP receiver created to act as a sink for the collector.
- `--receiver-type`: The type of receiver created to act as a sink for the collector (options: `http`, `grpc`). Please not, that when using the `http` option with Collector's `otlphttp exporter`, you need to disable the compression on the exporter, as no decompression is supported.
- `--statsd-protocol`: Statsd protocol to send metrics (options: 'udp', 'udp4', 'udp6', 'tcp', 'tcp4', 'tcp6', 'unixgram').

## Example Commands

1. Send OTLP JSON data to a collector:

```shell
./data-ingest --input-file=./commands/otlpjson/testdata/traces.json --input-format=otlp-json --otlp-signal-type=traces --collector-url=localhost:4317 --output-file=received_traces.json --receiver-port=4319 --receiver-type=http
```

1. Send statsd data to a collector:

```shell
./data-ingest --input-file=./commands/statsd/testdata/metrics.txt --input-format=statsd --collector-url=localhost:8125 --output-file=received_metrics.json --receiver-port=4319 --statsd-protocol=udp --otlp-signal-type=metrics --receiver-type=http
```
