#!/usr/bin/env bash

set -euo pipefail

go tool telemetrygen metrics --rate 1000 --interval 1s --duration 6000s --metrics 1000 --otlp-insecure --otlp-http --otlp-endpoint localhost:30000
