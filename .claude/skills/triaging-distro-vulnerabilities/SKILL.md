---
name: triaging-distro-vulnerabilities
description: Use when a Jira security ticket, scanner finding (Black Duck BDSA, Snyk, Trivy), or advisory (CVE, GHSA) names a vulnerable Go dependency and you must say which Dynatrace OTel Collector releases and components are affected, reachable, or exploitable
model: sonnet
effort: medium
---

# Triaging distro vulnerabilities

## Overview

We own the distribution, not its consumers. The answer is about **released dt-collector versions and the manifest components inside them**. Do not investigate remoteplugin or other downstream repos. Report which releases are affected and the first fixed release; downstream owners map that onto their pins.

The repo has **no root `go.mod`**. `ocb` generates it from `manifest.yaml`, so grepping `go.mod` files in the repo proves nothing about what ships. Evidence comes from the released binary and from the generated module.

## Verdict model, per component and for the core

| Level | Meaning | Evidence |
|---|---|---|
| **Present** | Vulnerable package is linked into the binary | `go version -m` on the release binary, `attribute-components.sh` |
| **Reachable** | A call path from `main` reaches a vulnerable symbol | `govulncheck` source mode, every hop read by a human |
| **Exploitable** | Reachable, attacker can control the input, and the advisory's preconditions hold in the shipped default config | Human judgment. Mark it as such in the report. |

Write "core collector" as its own line. It is affected only if `go.opentelemetry.io/collector/otelcol` links the package. Otherwise name the components.

## Steps

All scripts live in `scripts/` next to this file. Run them with `bash`. Set `WORK` to a scratch directory outside the repo.

1. **Read the advisory.** Get ID, aliases, affected module, vulnerable range, **fixed version**, vulnerable symbols and the trigger condition. For a GHSA, use `gh api /repos/<org>/<repo>/security-advisories/<GHSA>`. Check osv.dev for aliases. A BDSA id alone is unknown to `govulncheck`.
2. **Version table.** `bash scan-releases.sh <exact-module-path> 15` shows the module version in the last 15 release binaries. Mark each as inside or outside the vulnerable range. This alone often settles whole releases as not affected, and gives the first fixed release.
3. **Present.** For one vulnerable release, `bash gen-sources.sh <tag> $WORK` and print the module dir. Then, inside it, `go mod why -m <module>` and `bash attribute-components.sh <vulnerable-package> <path to manifest.yaml>`. The package path matters, the module can be linked while the package is not.
4. **Reachable.** Check whether `govulncheck ./...` already knows the advisory. If not, `bash mk-osv-db.sh` builds a local entry from the advisory's fixed version and symbols, then run `govulncheck -db file://<dir> -show verbose ./...`. Prefer source mode. Binary mode only shows symbols compiled in, with no call stacks.
5. **Cross-check and 6. Exploitable: only if step 4 reports a vulnerable symbol called, or you cannot rule it out.** If `govulncheck` reports the package imported with zero symbols called, state that and skip to step 7. Otherwise dispatch the `vuln-reachability-analyst` agent (Agent tool, `subagent_type: vuln-reachability-analyst`). It carries its own model and effort. Pass it the advisory facts, the generated module dir, and the `govulncheck` output. It runs `callgraph -algo=rta`, searches from `main`, judges each hop and the trigger condition, and returns a verdict. Its exploitability answers still need human confirmation. `reference.md` explains real edges versus RTA noise.
7. **Report.** Write `$WORK/reports/<ticket>.md` from the template in `reference.md`. Draft the Jira comment in plain prose. Never post it without the user's say-so.

## Decision

| Result | Action |
|---|---|
| Version outside vulnerable range | Not affected. Cite the binary's `go version -m` line. |
| Present, not reachable | Affected version, not exploitable (VEX `vulnerable_code_not_in_execute_path`). Fix on the next routine bump. |
| Reachable, not exploitable in default config | Affected. Fix within the severity SLA. Document the config mitigation. |
| Reachable and exploitable | Affected. Hotfix release, tell downstream the first fixed version. |

## Common mistakes

- Treating "No vulnerabilities found" as proof. Check that the Go DB has an entry for the advisory.
- Greping repo `go.mod` files. Test and tooling modules show up there and are not shipped.
- Checking the root `swag`-style module and not the submodule that the advisory names.
- Trusting RTA edges. Interface dispatch and `reflect.Value.Call` produce false paths.
- Stopping at "function reachable". Check the type the advisory needs (for example an ordered-map type) is ever used.
- Scanning only one platform's binary. The module list is the same across platforms for one release.

Further detail, the report template and tool gotchas: `reference.md`.
