#!/bin/sh

# Copyright The OpenTelemetry Authors
# Copyright Dynatrace LLC
# SPDX-License-Identifier: Apache-2.0

if [ "$1" != "1" ]; then
    if command -v systemctl >/dev/null 2>&1; then
        systemctl stop dynatrace-otel-collector.service
        systemctl disable dynatrace-otel-collector.service
    fi
fi
