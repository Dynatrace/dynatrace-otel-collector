# changelog-generator

A Go tool that fills the upstream changelog section in `CHANGELOG.md` during a release.

`make chlog-update` (via `chloggen`) generates a new version section with placeholder comments.
This tool fetches the upstream `.chloggen/*.yaml` entry files from the upstream "prepare release" PRs,
filters them to components included in this distribution, and replaces those placeholders with real content.

## Usage

```sh
# Upstream release: fill placeholders with content from the upstream PRs
GITHUB_TOKEN=$(gh auth token) ./bin/changelog-generator \
  https://github.com/open-telemetry/opentelemetry-collector/pull/14515 \
  https://github.com/open-telemetry/opentelemetry-collector-contrib/pull/45836

# Multi-version upgrade: pass all upstream PR URLs
GITHUB_TOKEN=$(gh auth token) ./bin/changelog-generator \
  https://github.com/open-telemetry/opentelemetry-collector/pull/14515 \
  https://github.com/open-telemetry/opentelemetry-collector-contrib/pull/45836

# Distro-only release (no upstream bump): removes the upstream placeholder section entirely
GITHUB_TOKEN=$(gh auth token) ./bin/changelog-generator

# Dry-run: print the generated pieces to stdout without modifying any files
GITHUB_TOKEN=$(gh auth token) ./bin/changelog-generator -dry-run \
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
cd internal/changelog-generator && go build -o ./../../bin/changelog-generator . && cd ../../
```

## How It Works

1. **`make chlog-update VERSION="vX.Y.Z"`** runs `chloggen`, which consumes distro `.chloggen/*.yaml` entries and
   writes a new version section into `CHANGELOG.md` using `summary.tmpl`. The template includes
   named placeholder comments (`<!-- upstream-version -->`, `<!-- upstream-collector-versions -->`,
   `<!-- upstream-breaking-changes -->`, `<!-- upstream-other-changes -->`) wrapped in
   `<!-- upstream-start -->`/`<!-- upstream-end -->` boundary markers.

2. **This tool** then fills those placeholders:
   - Reads `manifest.yaml` to build the set of included upstream component IDs.
   - For each supplied PR URL, fetches `.chloggen/*.yaml` files from the PR's **base commit** via
     the GitHub Contents API (entries live at the base because the PR deletes them on merge), and
     reads `versions.yaml` from the **head commit** to determine the upstream release version.
   - Filters entries against the component set and the allow/denylist from `config.yaml`.
   - Renders filtered entries into the four placeholder slots and removes the boundary markers.
   - When no PR URLs are provided, removes the entire `<!-- upstream-start -->`…`<!-- upstream-end -->` block.

## Configuration (`config.yaml`)

```yaml
# Always include entries for these component identifiers (exact or prefix match).
allowlist:
  - "pkg/ottl"
  - "all"   # entries with component "all" affect every component

# Always exclude. Denylist takes precedence over allowlist and manifest components.
# Supports glob suffix (*) for prefix matching.
denylist:
  - "internal/*"
  - "cmd/*"
```

- **To add a shared package**: add it to `allowlist` (e.g. `pkg/newpackage`).
- **To suppress a component** even if it's in `manifest.yaml`: add it to `denylist`.

## Component Name Mapping

Component IDs are derived from `manifest.yaml` gomod paths:

| `manifest.yaml` gomod | Upstream `component` ID |
|---|---|
| `go.opentelemetry.io/collector/receiver/otlpreceiver` | `receiver/otlp` |
| `.../receiver/filelogreceiver` | `receiver/filelog` |
| `.../processor/resourcedetectionprocessor` | `processor/resourcedetection` |
| `.../extension/storage/filestorage` | `extension/filestorage` |

The type suffix is stripped from the last path segment (e.g. `filelogreceiver` → `filelog`).

## GitHub Actions Workflow

The tool is wrapped by `.github/workflows/upstream-changelog.yml`, which:

1. Runs `make chlog-update` to consume distro entries and write the scaffold.
2. Runs this tool to fill the upstream placeholders.
3. Bumps `dist.version` in `manifest.yaml` and `OTEL_UPSTREAM_VERSION` in `Makefile`.
4. Opens a PR labelled `Skip Changelog` for human review.

```
Actions → "Generate Upstream Changelog" → Run workflow

Inputs:
  core_pr_url    (required)  upstream core "prepare release" PR URL
  contrib_pr_url (required)  upstream contrib "prepare release" PR URL
  extra_pr_urls  (optional)  comma-separated additional PR URLs for multi-version upgrades
  dist_version   (optional)  e.g. 0.46.0 — auto-bumps minor version if omitted
```

## Running Tests

```sh
cd internal/changelog-generator
go test ./...
```

## Troubleshooting

**`GITHUB_TOKEN` not set / 403 errors**
```sh
export GITHUB_TOKEN=$(gh auth token)
```

**"no valid upstream versions found"**
Each PR must have a `versions.yaml` at its head commit with a valid semver. The tool reads the
version from `module-sets.beta.version` (core) or `module-sets.contrib-base.version` (contrib).

**Entry not appearing in output**
Run with `-dry-run` and check the `info:` lines on stderr. If a component is missing, add it to
`allowlist` in `config.yaml`.

