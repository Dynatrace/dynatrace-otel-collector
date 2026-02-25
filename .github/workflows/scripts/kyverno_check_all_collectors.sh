#!/usr/bin/env bash
set -euo pipefail

# Usage:
#   kyverno_check_all_collectors_simple.sh [INTEGRATION_ROOT] [POLICIES_DIR] [OUT_BASE]
#
# Defaults assume this repo layout:
# - integrations: internal/testbed/integration
# - policies:     .github/workflows/kyverno/policies
# - output:       $RUNNER_TEMP/rendered-collectors-simple (or /tmp/..)

INTEGRATION_ROOT="${1:-internal/testbed/integration}"
POLICIES_DIR="${2:-.github/workflows/kyverno/policies}"
OUT_BASE="${3:-${RUNNER_TEMP:-/tmp}/rendered-collectors-simple}"

echo "INTEGRATION_ROOT=$INTEGRATION_ROOT"
echo "POLICIES_DIR=$POLICIES_DIR"
echo "OUT_BASE=$OUT_BASE"

command -v kyverno >/dev/null 2>&1 || { echo "kyverno not found in PATH"; exit 2; }

rm -rf "$OUT_BASE"
mkdir -p "$OUT_BASE"

# --- find collector template dirs ---
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

# --- preprocess (simple placeholder substitution) ---
preprocess_dir() {
  local in_dir="$1"
  local out_dir="$2"

  mkdir -p "$out_dir"
  cp -R "$in_dir"/. "$out_dir"/

  find "$out_dir" -type f \( -name '*.yaml' -o -name '*.yml' \) -print0 |
    xargs -0 perl -0777 -pi -e '
      s/\{\{\s*\.Name\s*\}\}/otelcol-ci/g;
      s/\{\{\s*\.Namespace\s*\}\}/e2e/g;
      s/\{\{\s*\.TestID\s*\}\}/ci/g;
      s/\{\{\s*\.HostEndpoint\s*\}\}/http:\/\/example.invalid/g;
      s/\{\{\s*\.ContainerRegistry\s*\}\}/dynatrace/g;
      s/\{\{\s*\.K8sCluster\s*\}\}/ci/g;
      s/\{\{\s*\.CollectorConfig\s*\}\}/receivers: {}\\nexporters: {}\\nservice: { pipelines: {} }\\n/g;
    '

  # Fail if anything template-like remains
  if grep -R --line-number "{{" "$out_dir" >/dev/null 2>&1; then
    echo "ERROR: Unhandled template expressions remain in $in_dir (preprocessed at $out_dir)."
    echo "First occurrences:"
    grep -R --line-number "{{" "$out_dir" | head -n 50
    exit 1
  fi
}

for d in "${COLLECTOR_DIRS[@]}"; do
  safe="${d#"$INTEGRATION_ROOT"/}"
  safe="${safe//\//_}"
  out="$OUT_BASE/$safe"
  preprocess_dir "$d" "$out"
done

# --- collect only workload YAMLs (Deployment/DaemonSet/StatefulSet) ---
RES_ARGS=()
while IFS= read -r f; do
  kind="$(grep -m1 '^[[:space:]]*kind:' "$f" | awk '{print $2}' || true)"
  case "$kind" in
    Deployment|DaemonSet|StatefulSet)
      RES_ARGS+=("-r" "$f")
      ;;
  esac
done < <(find "$OUT_BASE" -type f \( -name '*.yaml' -o -name '*.yml' \) | sort)

if [ "${#RES_ARGS[@]}" -eq 0 ]; then
  echo "No workload YAMLs found after preprocessing."
  exit 1
fi

echo "Kyverno will validate workload YAMLs:"
for ((i=1; i<${#RES_ARGS[@]}; i+=2)); do
  echo " - ${RES_ARGS[i]}"
done

# --- run kyverno ---
kyverno apply "$POLICIES_DIR"/*.yaml "${RES_ARGS[@]}"
