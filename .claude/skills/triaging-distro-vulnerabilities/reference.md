# Reference: triaging distro vulnerabilities

## How the collector is bundled and shipped

- `manifest.yaml` lists components (receivers, exporters, extensions, processors, connectors, providers) plus `replaces`.
- CI runs `ocb` (`go.opentelemetry.io/collector/cmd/builder`) to generate `main.go`, `components.go`, `go.mod`, `go.sum` into `./build`, then compiles. The builder version is pinned per release in `internal/tools/go.mod`.
- Releases (GitHub, `Dynatrace/dynatrace-otel-collector`) attach per-platform archives, `.deb`/`.rpm`/`.apk`, CycloneDX SBOMs (`*-sbom.cdx.json`) and `checksums.txt`. A container image `ghcr.io/dynatrace/dynatrace-otel-collector/dynatrace-otel-collector:<version>` is published too.
- Asset name for the scan: `dynatrace-otel-collector_<version>_Linux_x86_64.tar.gz`.
- Modules under `internal/` (tools, testbed, testcommon, and others) are build and test support. They can show up in repo-wide scanners but do not ship. Say so in the report when a finding points at them.
- The release binary embeds its module list, readable with `go version -m <binary>`, on any host OS. SBOM and binary should agree. Verify the archive checksum against `checksums.txt`.

## Tool behavior that costs time

- **`govulncheck` knows only vuln.go.dev.** BDSA ids and advisories without a GHSA/CVE entry in the Go DB give "No vulnerabilities found". Use `mk-osv-db.sh` to supply an entry.
- **Symbol notation** in a local entry: `Func` or `Type.Method`. Verify the list against the fix commit, not only the advisory text.
- **`-show verbose` on source mode first prints every scanned module.** Read only the `=== Symbol Results ===`, `=== Package Results ===` sections.
- **Result wording:** "your code is affected by 0 vulnerabilities ... 1 in packages you import" means present, not called. Binary mode counts every symbol compiled in, so it reports more than source mode. Report both and explain the difference.
- **Local entry needs `details` and `database_specific.url`.** Without them `govulncheck -show verbose` panics (nil deref in `x/vuln` `text.go`, `TextHandler.vulnerability`) and dumps a huge goroutine trace. `mk-osv-db.sh` now writes both. If a panic still shows, rerun with `-format json` and filter: `jq -c 'select(.finding!=null)|.finding|{fixed_version,trace:[.trace[]|{package,function}]}'`. Only findings with a `function` are symbol-level.
- **A local entry only finds the symbols you list.** "Zero symbols called" is only as good as that list. The bug often sits in an internal package, not the one with the public entry points. Read the fix commit, list the entry points and the internal functions, and pass the extra package as a `<pkg> <symbols>` pair to `mk-osv-db.sh`. Confirm the package is linked with `go list -deps ./... | grep <pkg>`.
- **Set `WORK` and the scripts dir once.** Scripts `cd` nowhere, but the shell cwd resets after each call. Use absolute paths.
- **Use `PRESENT_ONLY=1` with `attribute-components.sh`** to print only the components that link the package. The full list is about 35 rows.
- **No ticket id in the request?** Name the report after the advisory id, for example `GHSA-xxxx.md`, and ask for the ticket id.
- **`gen-sources.sh` leaves a git worktree** at `<workdir>/src-<tag>`. Remove it when done: `git worktree remove --force <path>`.
- **macOS has no `timeout`.** Run long commands without it, or use the harness timeout.
- **Callgraph size:** `callgraph -algo=rta .` on the full distro writes millions of lines. Save to a file and query with grep or a BFS script, never print it.

## Telling real call edges from RTA noise

RTA (rapid type analysis) over-approximates: a call to any interface method is linked to every instantiated type with that method, and `reflect.Value.Call` is linked to every function.

- Read the path hop by hop from `main`. Closures such as `cobra.Command.preRun -> some closure -> library X` are artifacts when the middle hops share no real data flow.
- An edge from a JSON/YAML library's `Unmarshaler` dispatch to the vulnerable `UnmarshalJSON` only matters if some code actually decodes into the vulnerable type. Grep non-test code in every linked package for the type (including type aliases re-exported by other packages).
- Direct calls to the entry points from packages linked in only as library dependencies (for example validation or client libraries pulled in by a receiver) matter only if their own callers are reachable. Run a BFS from `main` to the specific function and read the result.
- Read the vulnerable function's own source at the vulnerable version. The trigger is often narrower than the advisory title, for example "only when the target implements an ordered-map interface".
- `crypto/...` or `bufio` calling a library closure is always noise.

Create the call graph file once per release and query it with the script. Never print the graph.

```
callgraph -algo=rta -format='{{.Caller}} --> {{.Callee}}' . > $WORK/callgraph.txt
python3 scripts/callgraph-path.py $WORK/callgraph.txt <target-function>...
```

`callgraph` is `golang.org/x/tools/cmd/callgraph`. Run it in the generated module dir. Target names use the callgraph's own notation, for example `(*pkg/path.Type).Method`.

## Exploitable: questions to answer

1. Which input does the advisory need (network request, file, config, spec upload)?
2. Which collector component receives that input, and is it enabled in the default configs and examples shipped in `config_examples/` and docs?
3. Can an unauthenticated or lower-trust party supply the input?
4. Does the effect matter for a collector (crash of the whole process is high impact for an agent, so availability findings count)?
5. Is a config-level mitigation available (disable the component, limit body size)?

Mark the answer as "human-confirmed" with the name of the person who confirmed it. Do not mark it from static analysis alone.

## Report template

```markdown
# <ticket> - <advisory ids> - <module>

## Advisory
Module, vulnerable range, fixed version, vulnerable symbols, trigger. Link.

## Version table
| Release | Go | Module version | In range |
(output of scan-releases.sh, last 15 releases, first fixed release in bold)

## Per component (vulnerable release: <tag>)
| Component | Present | Reachable | Exploitable |
Core collector line first. Only list components that are Present.

## Evidence
Commands run and the lines that decided each level. Call path or the absence of one.

## Verdict
One of: not affected / affected not exploitable / affected / hotfix. Fixed in: <release>.

## Caveats
Reflection and RTA limits, assumptions, items needing human confirmation.

## Out of scope
Downstream pins (for example remoteplugin) are not assessed. If the ticket names a module version that does not appear in any release, say so and ask the reporter for the manifest path.
```
