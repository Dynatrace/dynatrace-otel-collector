#!/bin/bash

# Copyright The OpenTelemetry Authors
# Copyright Dynatrace LLC
# SPDX-License-Identifier: Apache-2.0

set -euov pipefail

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
REPO_DIR="$( cd "$SCRIPT_DIR/../../../../" && pwd )"
export REPO_DIR
PKG_PATH="${1:-}"
ARCH="${2:-}"
DISTRO="dynatrace-otel-collector"

SERVICE_NAME=$DISTRO
PROCESS_NAME=$DISTRO

# shellcheck source=scripts/package-tests/common.sh
source "$SCRIPT_DIR"/common.sh

if [[ -z "$PKG_PATH" || -z "$ARCH" ]]; then
    echo "usage: ${BASH_SOURCE[0]} DEB_OR_RPM_PATH ARCHITECTURE" >&2
    exit 1
fi

if [[ ! -f "$PKG_PATH" ]]; then
    echo "$PKG_PATH not found!" >&2
    exit 1
fi

# translate distro x86 arch to normal name
translated_arch=""
if [[ "$ARCH" == "x86_64" ]]; then
  translated_arch="amd64"
else
  translated_arch="$ARCH"
fi

pkg_base="$( basename "$PKG_PATH" )"
pkg_type="${pkg_base##*.}"
if [[ ! "$pkg_type" =~ ^(deb|rpm)$ ]]; then
    echo "$PKG_PATH not supported!" >&2
    exit 1
fi
image_name="otelcontribcol-$pkg_type-test"
container_name="$image_name"
container_exec="podman exec $container_name"

trap 'podman rm -fv $container_name >/dev/null 2>&1 || true' EXIT

podman build -t "$image_name" --arch "$translated_arch" -f "$SCRIPT_DIR/Dockerfile.test.$pkg_type" "$SCRIPT_DIR"
podman rm -fv "$container_name" >/dev/null 2>&1 || true

# test install
podman run --name "$container_name" --arch "$translated_arch" -d "$image_name"

# ensure that the system is up and running by checking if systemctl is running
sleep 5
$container_exec systemctl is-system-running --wait
$container_exec systemctl --failed

podman_cp "$container_name" internal/testbed/linux-services/config.test.yaml /etc/dynatrace-otel-collector/config.yaml
install_pkg "$container_name" "$PKG_PATH"

# ensure service has started and still running after 5 seconds
sleep 4
echo "Checking $SERVICE_NAME service status ..."
$container_exec systemctl --no-pager status "$SERVICE_NAME"

echo "Checking $PROCESS_NAME process ..."
$container_exec pgrep -f -a -u otel "$PROCESS_NAME"

# test uninstall
uninstall_pkg "$container_name" "$pkg_type" "$DISTRO"

echo "Checking $SERVICE_NAME service status after uninstall ..."
if $container_exec systemctl --no-pager status "$SERVICE_NAME"; then
    echo "$SERVICE_NAME service still running after uninstall" >&2
    exit 1
fi
echo "$SERVICE_NAME service successfully stopped after uninstall"

echo "Checking $SERVICE_NAME service existence after uninstall ..."
if $container_exec systemctl list-unit-files --all | grep "$SERVICE_NAME"; then
    echo "$SERVICE_NAME service still exists after uninstall" >&2
    exit 1
fi
echo "$SERVICE_NAME service successfully removed after uninstall"

echo "Checking $PROCESS_NAME process after uninstall ..."
if $container_exec pgrep -f "$PROCESS_NAME"; then
    echo "$PROCESS_NAME process still running after uninstall"
    exit 1
fi
echo "$PROCESS_NAME process successfully killed after uninstall"
