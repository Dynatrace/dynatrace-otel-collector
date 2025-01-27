# OTLP Data Ingest Tool

## Overview

The primary purpose of this CLI Tool is to assist in reproducing support cases and troubleshooting issues within the OpenTelemetry collector pipeline.
It allows users to accurately replicate conditions under which issues occur by reading data in different formats and sending it to the collector.
Additionally, the tool can receive OTLP data via an endpoint and store it in a JSON file for detailed inspection and analysis,
making it easier to diagnose and resolve problems related to OTLP data handling.

*Note:* This is a developer tool used mainly for debugging, and not intended for being used within a production environment.

## Features

- Read OTLP JSON data from a file: The tool can read OTLP data from a specified file containing data in different formats and send it to an OpenTelemetry collector.
 Supported formats are:
  - OTLP Json
  - Systemd
  - Syslog

- Receive OTLP data via an endpoint: The tool can receive OTLP data through an endpoint and store the received payload in a specified file in OTLP Json format.

## Usage

The tool accepts the following input parameters:

- `--input-file`: The name of the input file to read data from.
- `--input-format`: The input format of the ingested data (options: `otlpjson`, `syslog`, `systemd`). 
- `--collector-url`: The URL of the OpenTelemetry collector.
- `--output-file`: The file in which to store the received data.

## Example Commands

1. Send OTLP JSON data to a collector:

```
otlp-data-ingest --input-file=data.json --input-format=otlpjson --collector-url=http://collector.example.com:4317 --output-file=received.json
```
