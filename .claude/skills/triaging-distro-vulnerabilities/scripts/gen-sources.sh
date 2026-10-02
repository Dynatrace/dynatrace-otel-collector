#!/usr/bin/env bash
# Generate the real go.mod/go.sum of a released collector version, without compiling.
# The repo has no root go.mod: ocb generates it from manifest.yaml.
#
# usage: bash gen-sources.sh <tag> <workdir>
# prints the path of the generated module directory on the last line.
set -euo pipefail

tag=${1:?usage: gen-sources.sh <tag e.g. v0.56.0> <workdir>}
work=${2:?usage: gen-sources.sh <tag> <workdir>}

repo=$(git rev-parse --show-toplevel)
src="$work/src-$tag"
mkdir -p "$work"

if [ ! -d "$src" ]; then
  git -C "$repo" worktree add --detach "$src" "$tag" >&2
fi

# The builder version is pinned per release in internal/tools/go.mod.
bver=$(awk '/go.opentelemetry.io\/collector\/cmd\/builder/{print $2; exit}' "$src/internal/tools/go.mod")
[ -n "$bver" ] || { echo "cannot find pinned builder version in $src/internal/tools/go.mod" >&2; exit 1; }
echo "builder $bver" >&2

GOBIN="$work/bin-$bver" go install "go.opentelemetry.io/collector/cmd/builder@$bver" >&2

out=$(awk '/output_path:/{print $2; exit}' "$src/manifest.yaml")
(cd "$src" && "$work/bin-$bver/builder" --skip-compilation --config manifest.yaml >&2)

echo "$src/${out#./}"
