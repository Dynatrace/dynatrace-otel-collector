#!/usr/bin/env bash
set -euo pipefail

INTEGRATION_ROOT="${1:-internal/testbed/integration}"
POLICIES_DIR="${2:-.github/workflows/kyverno/policies}"
OUT_BASE="${3:-${RUNNER_TEMP:-/tmp}/rendered-collectors}"

echo "INTEGRATION_ROOT=$INTEGRATION_ROOT"
echo "POLICIES_DIR=$POLICIES_DIR"
echo "OUT_BASE=$OUT_BASE"

command -v go >/dev/null 2>&1 || { echo "go not found"; exit 2; }
command -v kyverno >/dev/null 2>&1 || { echo "kyverno not found"; exit 2; }

rm -rf "$OUT_BASE"
mkdir -p "$OUT_BASE"

# Minimal template data. Keep CollectorConfig simple; we won't pass configmaps to Kyverno anyway.
DATA_JSON='{
  "Name":"otelcol-ci",
  "TestID":"ci",
  "HostEndpoint":"http://example.invalid",
  "ContainerRegistry":"dynatrace",
  "CollectorConfig":"receivers: {}\\nexporters: {}\\nservice: { pipelines: {} }\\n",
  "K8sCluster":"ci"
}'

# Find all collector* directories under integration root testdata
COLLECTOR_DIRS=()
while IFS= read -r d; do
  COLLECTOR_DIRS+=("$d")
done < <(find "$INTEGRATION_ROOT" -type d -path "*/testdata/*" -name 'collector*' | sort)

if [ "${#COLLECTOR_DIRS[@]}" -eq 0 ]; then
  echo "No collector* directories found under: $INTEGRATION_ROOT"
  exit 1
fi

echo "Found collector template dirs: ${#COLLECTOR_DIRS[@]}"
printf ' - %s\n' "${COLLECTOR_DIRS[@]}"

# Render each collector dir into a unique output folder
for d in "${COLLECTOR_DIRS[@]}"; do
  safe="${d#"$INTEGRATION_ROOT"/}"
  safe="${safe//\//_}"
  out="$OUT_BASE/$safe"
  mkdir -p "$out"

  go run .github/tools/render/main.go -in "$d" -out "$out" -data "$DATA_JSON"
done

# Build Kyverno resource args from workload YAMLs only
RES_ARGS=()
while IFS= read -r f; do
  # Determine kind cheaply (no full YAML parse needed)
  kind="$(grep -m1 '^[[:space:]]*kind:' "$f" | awk '{print $2}' || true)"
  case "$kind" in
    Deployment|DaemonSet|StatefulSet)
      RES_ARGS+=("-r" "$f")
      ;;
  esac
done < <(find "$OUT_BASE" -type f \( -name '*.yaml' -o -name '*.yml' \) | sort)

if [ "${#RES_ARGS[@]}" -eq 0 ]; then
  echo "No rendered workload YAMLs (Deployment/DaemonSet/StatefulSet) found under: $OUT_BASE"
  exit 1
fi

echo "Kyverno will validate workload YAMLs:"
for ((i=1; i<${#RES_ARGS[@]}; i+=2)); do
  echo " - ${RES_ARGS[i]}"
done

# Apply policies (pass policies as files + resources as repeated -r)
set +e
kyverno apply "$POLICIES_DIR"/*.yaml "${RES_ARGS[@]}"
rc=$?
set -e

if [ "$rc" -ne 0 ]; then
  echo "Kyverno failed. Dumping first 120 lines of workload YAMLs for debugging:"
  for ((i=1; i<${#RES_ARGS[@]}; i+=2)); do
    f="${RES_ARGS[i]}"
    echo "===== $f ====="
    nl -ba "$f" | sed -n '1,120p'
  done
  exit "$rc"
fi

echo "Kyverno validation passed."
