#!/bin/sh

# Copyright The OpenTelemetry Authors
# Copyright Dynatrace LLC
# SPDX-License-Identifier: Apache-2.0

getent passwd otel >/dev/null || useradd --system --user-group --no-create-home --shell /sbin/nologin otel
