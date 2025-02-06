#!/bin/sh

# Copyright The OpenTelemetry Authors
# Copyright Dynatrace LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#       http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

if command -v systemctl >/dev/null 2>&1; then
    systemctl daemon-reload
    systemctl enable dynatrace-otel-collector.service
    if [ -f /etc/dynatrace-otel-collector/config.yaml ]; then
        systemctl restart dynatrace-otel-collector.service
    else
      echo "Collector installed, but no config.yaml was found, skipping startup..."
    fi
fi
