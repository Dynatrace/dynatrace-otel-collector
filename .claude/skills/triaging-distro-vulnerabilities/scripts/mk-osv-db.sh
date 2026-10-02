#!/usr/bin/env bash
# Build a local Go vulnerability DB with one hand-written entry, for advisories
# that vuln.go.dev does not know yet (BDSA ids, fresh GHSAs without a Go DB entry).
# govulncheck then does the symbol-level call-graph analysis against it.
#
# usage: bash mk-osv-db.sh <outdir> <id> <aliases,comma> <module> <fixed-without-v> <symbols,comma> [pkg-import-path]
#   symbols use govulncheck notation: Func  or  Type.Method
# then:  govulncheck -db file://<outdir> -show verbose ./...   (run in the generated module dir)
set -euo pipefail

out=${1:?outdir}; id=${2:?id}; aliases=${3:?aliases}; mod=${4:?module}
fixed=${5:?fixed version without leading v}; syms=${6:?symbols}
pkg=${7:-$mod}
now=$(date -u +%Y-%m-%dT%H:%M:%SZ)

mkdir -p "$out/index" "$out/ID"

jq -n --arg id "$id" --arg now "$now" --arg aliases "$aliases" --arg mod "$mod" \
      --arg fixed "$fixed" --arg syms "$syms" --arg pkg "$pkg" '
{schema_version:"1.3.0", id:$id, modified:$now, published:$now,
 aliases:($aliases|split(",")), summary:("local entry for " + $aliases),
 affected:[{package:{name:$mod, ecosystem:"Go"},
   ranges:[{type:"SEMVER", events:[{introduced:"0"},{fixed:$fixed}]}],
   ecosystem_specific:{imports:[{path:$pkg, symbols:($syms|split(","))}]}}]}' \
  > "$out/ID/$id.json"

jq -n --arg now "$now" '{modified:$now}' > "$out/index/db.json"
jq -n --arg now "$now" --arg id "$id" --arg mod "$mod" --arg fixed "$fixed" \
  '[{path:$mod, vulns:[{id:$id, modified:$now, fixed:$fixed}]}]' > "$out/index/modules.json"
jq -n --arg now "$now" --arg id "$id" --arg aliases "$aliases" \
  '[{id:$id, modified:$now, aliases:($aliases|split(","))}]' > "$out/index/vulns.json"

echo "local DB ready: file://$out"
