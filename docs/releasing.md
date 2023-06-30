This document describes the steps to release a new version of the OpenTelemetry Collector for Dynatrace.

# Making a production release

The way a production release is made is by creating and pushing a git tag which starts with the letter `v`.
The tag name MUST be a semver-compliant version string.


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
