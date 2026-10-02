#!/usr/bin/env bash
# Version of one Go module embedded in the last N released Linux x86_64 binaries.
# Reads the binary's build info (go version -m), so no go.mod and no build are needed.
#
# usage: bash scan-releases.sh <exact-module-path> [N=15]
# env:   WORK  cache dir (default: $TMPDIR/vuln-triage)
set -euo pipefail

mod=${1:?usage: scan-releases.sh <exact-module-path> [N]}
n=${2:-15}
work=${WORK:-${TMPDIR:-/tmp}/vuln-triage}
slug=Dynatrace/dynatrace-otel-collector

printf '%-10s %-10s %s\n' RELEASE GO "${mod}"
for tag in $(gh release list -R "$slug" --limit "$n" --json tagName --jq '.[].tagName'); do
  v=${tag#v}
  d="$work/rel/$tag"
  bin="$d/dynatrace-otel-collector"
  if [ ! -x "$bin" ]; then
    mkdir -p "$d"
    gh release download "$tag" -R "$slug" -D "$d" --clobber \
      -p "dynatrace-otel-collector_${v}_Linux_x86_64.tar.gz" >/dev/null 2>&1 \
      || { printf '%-10s download failed\n' "$tag"; continue; }
    tar xzf "$d"/*_Linux_x86_64.tar.gz -C "$d" dynatrace-otel-collector
  fi
  info=$(go version -m "$bin")
  gov=$(printf '%s\n' "$info" | head -1 | awk '{print $2}')
  ver=$(printf '%s\n' "$info" | awk -v m="$mod" '$1=="dep" && $2==m {print $3}')
  printf '%-10s %-10s %s\n' "$tag" "$gov" "${ver:-absent}"
done
