# changelog-generator

A Go tool that automates the generation of upstream changelog sections in the Dynatrace OTel Collector
distribution's `CHANGELOG.md`.

It extracts structured `.chloggen/*.yaml` entry files from upstream "prepare release" PRs in the
OpenTelemetry Collector Core and Contrib repositories, filters them to the components included in this
distribution, and renders them into the existing changelog format.

## Usage

```sh
# Dry-run: print generated markdown to stdout without modifying any files
GITHUB_TOKEN=ghp_xxx ./bin/changelog-generator -dry-run \
  https://github.com/open-telemetry/opentelemetry-collector/pull/14515 \
  https://github.com/open-telemetry/opentelemetry-collector-contrib/pull/45836

# For real: insert the upstream section into CHANGELOG.md
GITHUB_TOKEN=ghp_xxx ./bin/changelog-generator \
  https://github.com/open-telemetry/opentelemetry-collector/pull/14515 \
  https://github.com/open-telemetry/opentelemetry-collector-contrib/pull/45836

# Multi-version upgrade (provide all four PR URLs)
GITHUB_TOKEN=ghp_xxx ./bin/changelog-generator \
  https://github.com/open-telemetry/opentelemetry-collector/pull/14400 \
  https://github.com/open-telemetry/opentelemetry-collector-contrib/pull/45700 \
  https://github.com/open-telemetry/opentelemetry-collector/pull/14515 \
  https://github.com/open-telemetry/opentelemetry-collector-contrib/pull/45836
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-manifest` | `manifest.yaml` | Path to `manifest.yaml` |
| `-config` | `internal/changelog-generator/config.yaml` | Path to allow/denylist config |
| `-changelog` | `CHANGELOG.md` | Path to `CHANGELOG.md` |
| `-dry-run` | `false` | Print to stdout without modifying files |

## Building

```sh
# From the repo root
go build -o ./bin/changelog-generator ./internal/changelog-generator
```

## How It Works

1. **Parses `manifest.yaml`** to extract the set of upstream component IDs in the distribution
   (e.g. `receiver/filelog`, `processor/batch`).
2. **Fetches `.chloggen/*.yaml` files** from the base commit of each upstream "prepare release" PR
   via the GitHub Contents API. The entries exist at the *base* commit because the PR deletes them.
3. **Parses each entry** (structured YAML with `change_type`, `component`, `note`, `issues`,
   `subtext`, `change_logs`). Entries with `change_logs: [api]` only are skipped.
4. **Filters entries** against the component set, plus the allowlist/denylist from `config.yaml`.
5. **Renders filtered entries** into formatted markdown:
   - Breaking changes appear at the top level (outside `<details>`).
   - Enhancements, bug fixes, deprecations, and new components go inside a `<details>` block.
   - Core entries appear before contrib entries within each section.
6. **Inserts** the generated section into `CHANGELOG.md` between the
   `<!-- next version -->` and `<!-- previous-version -->` markers, preserving any
   distro-specific entries added by `chloggen`.

## Configuration (`config.yaml`)

The config file lets you extend the filter beyond what is in `manifest.yaml`:

```yaml
# Always include entries for these component identifiers.
# Supports exact match and prefix match (e.g. "pkg/stanza" also matches "pkg/stanza/something").
allowlist:
  - "pkg/ottl"
  - "pkg/stanza"
  - "all"          # entries with component "all" affect every component

# Always exclude entries for these component identifiers.
# Supports glob suffix (*) for prefix matching.
# Denylist takes precedence over both allowlist and manifest components.
denylist:
  - "internal/*"
  - "cmd/*"
```

### Adding/removing components

- To **always include** a shared package (e.g. `pkg/newpackage`): add it to `allowlist`.
- To **exclude** a specific component even if it is in `manifest.yaml`: add it to `denylist`.
- The default config at `internal/changelog-generator/config.yaml` covers the commonly relevant
  shared packages.

## Component Name Mapping

The tool derives upstream component IDs from `manifest.yaml` gomod paths:

| `manifest.yaml` gomod | Upstream `component` ID |
|---|---|
| `go.opentelemetry.io/collector/receiver/otlpreceiver` | `receiver/otlp` |
| `.../receiver/filelogreceiver` | `receiver/filelog` |
| `.../processor/resourcedetectionprocessor` | `processor/resourcedetection` |
| `.../extension/storage/filestorage` | `extension/filestorage` |
| `.../connector/spanmetricsconnector` | `connector/spanmetrics` |

The mapping strips the component-type suffix from the last path segment
(e.g. `filelogreceiver` → `filelog` by stripping `receiver`). If the last segment
does not end with the type suffix, it is used as-is (e.g. `filestorage`).

## GitHub Actions Workflow

The tool is also available as a manually-triggered GitHub Actions workflow at
`.github/workflows/upstream-changelog.yml`. The workflow:

1. Builds the tool and runs it against the supplied PR URLs.
2. Bumps `dist.version` in `manifest.yaml` (auto-increments minor if not specified).
3. Bumps `OTEL_UPSTREAM_VERSION` in `Makefile`.
4. Opens a PR labelled `Skip Changelog` for human review.

### Triggering the workflow

```
Actions → "Generate Upstream Changelog" → Run workflow

Inputs:
  core_pr_url    (required)  https://github.com/open-telemetry/opentelemetry-collector/pull/NNNN
  contrib_pr_url (required)  https://github.com/open-telemetry/opentelemetry-collector-contrib/pull/NNNN
  extra_pr_urls  (optional)  Comma-separated additional PR URLs for multi-version upgrades
  dist_version   (optional)  e.g. 0.46.0 — auto-bumps minor version if omitted
```

## Running Tests

```sh
cd internal/changelog-generator
go test ./...
go test -v -run TestParseManifest
go test -v -run TestFilterEntries
go test -v -run TestGenerateChangelog
```

## Troubleshooting

**`GITHUB_TOKEN` not set / 403 errors**
Set `GITHUB_TOKEN` to a personal access token (or use `gh auth token` to get one):
```sh
export GITHUB_TOKEN=$(gh auth token)
```

**"no upstream versions found"**
Check that the PR URLs point to actual "prepare release" PRs with titles like
`[chore] Prepare release 0.145.0`. The version is extracted from the PR title.

**Entry not appearing in output**
Run with `-dry-run` and check stderr for `info:` lines showing how many entries
were fetched and filtered. If a component is missing, add it to the `allowlist`
in `config.yaml`.

