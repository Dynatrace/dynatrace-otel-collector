#!/usr/bin/env bash
# PRESENT level: which manifest components (and the core collector) link a vulnerable package.
# A component counts as present if the vulnerable package is in `go list -deps` of its import path.
#
# usage: (cd <generated-module-dir> && [PRESENT_ONLY=1] bash attribute-components.sh <vulnerable-package-import-path> <manifest.yaml>)
#   PRESENT_ONLY=1 prints only the core line and the components that link the package.
set -euo pipefail

pkg=${1:?vulnerable package import path}
manifest=${2:?path to manifest.yaml}

printf '%-12s %-9s %s\n' KIND PRESENT COMPONENT

present() { go list -deps "$1" 2>/dev/null | grep -qx "$pkg"; }

# Core collector: the service/otelcol layer every distro links regardless of components.
if present go.opentelemetry.io/collector/otelcol; then c=yes; else c=no; fi
printf '%-12s %-9s %s\n' core "$c" go.opentelemetry.io/collector/otelcol

# section name comes from the top-level key, component path from "- gomod: <path> <version>"
awk '/^[a-z_]+:/{sec=$1; sub(":","",sec)} $2=="gomod:"{print sec, $3}' "$manifest" |
while read -r kind path; do
  if present "$path"; then r=yes; else r=no; fi
  [ -n "${PRESENT_ONLY:-}" ] && [ "$r" = no ] && continue
  printf '%-12s %-9s %s\n' "$kind" "$r" "$path"
done
