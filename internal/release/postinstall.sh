#!/bin/sh

# Copyright The OpenTelemetry Authors
# Copyright Dynatrace LLC
# SPDX-License-Identifier: Apache-2.0

if command -v systemctl >/dev/null 2>&1; then
    if [ -d /run/systemd/system ]; then
      systemctl daemon-reload
    fi
    systemctl enable dynatrace-otel-collector.service
    if [ -f /etc/dynatrace-otel-collector/config.yaml ]; then
      if [ -d /run/systemd/system ]; then
        systemctl restart dynatrace-otel-collector.service
      fi
    else
      echo "Collector installed, but no config.yaml was found, skipping startup..."
    fi
fi
