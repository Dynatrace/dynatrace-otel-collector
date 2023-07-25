This document describes the steps to release a new version of the OpenTelemetry Collector for Dynatrace.

# Collector release prerequisites

1. Ensure the desired collector version is specified.
   
The collector version is controlled by the `dist.otelcol_version` property of [`manifest.yaml`](../manifest.yaml).
Usually this will be the latest collector release version.
This is the upstream collector version used to build the collector, not the version released from this repository.

2. Ensure the build is using the latest collector builder.

The collector builder version is controlled by the [`internal/tools/go.mod`](../internal/tools/go.mod) file.
In order to bump the version, run `go get go.opentelemetry.io/collector/cmd/builder` from the `internal/tools` directory.
The collector builder must be the same version or later than the desired collector version from step 1.

3. Ensure the manifest contains all desired collector components.

Collector components are controlled by the same `manifest.yaml`.
It is recommended that the component versions depend on the same version of the collector that is chosen in step 1.
For components in the upstream collector repos, most of the time this means they will be the same version as the collector version.

4. Set the release version

The release version is controlled by `dist.version` property of the `manifest.yaml`.
The version should be a semver-compliant string, for example `1.0.0`.

# Making a production release

The way a production release is made is by creating and pushing a git tag which starts with the letter `v`.
The tag name MUST be a semver-compliant version string with the letter "v" prepended, for example `v1.0.0`.


1. Identify the git ref you want to release.
   Usually this will be `main` (`refs/heads/main`).
2. Check out the ref locally.

```sh
git fetch origin
git checkout main
git reset --hard origin/main
```

3. Ensure the `dist.version` property of [`manifest.yaml`](../manifest.yaml) is the desired new semver-compliant version.
   If not, you will need to update it and go back to step 1.
4. Create a git tag which matches the `dist.version` property of `manifest.yaml` exactly except that it is preceded by the letter `v`.

```sh
git tag v0.0.1
```

5. Push your new tag to github

```sh
git push --tags
```

Once you have completed the above steps, the [Build and Release](../.github/workflows/release.yaml) workflow will use [goreleaser](https://goreleaser.com) to create a new draft release on GitHub. When it completes, the release will be visible to users with the required permissions at [Releases](https://github.com/Dynatrace/dynatrace-otel-collector/releases). The changelog and title for the release is created automatically, but may be modified. When you are happy with the state of the release, publish the release and it will become publicly visible.
