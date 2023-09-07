# How to Contribute

We'd love to accept your patches and contributions to this project. There are
just a few small guidelines you need to follow.

## Contributor License Agreement

Contributions to this project must be accompanied by a Contributor License
Agreement. You (or your employer) retain the copyright to your contribution;
this simply gives us permission to use and redistribute your contributions as
part of the project.

You generally only need to submit a CLA once, so if you've already submitted one
(even if it was for a different project), you probably don't need to do it
again.

## Code Reviews

All submissions, including submissions by project members, require review. We
use GitHub pull requests for this purpose. Consult
[GitHub Help](https://help.github.com/articles/about-pull-requests/) for more
information on using pull requests.

## Development

The following section describes how setup your environment and build the collector distribution.

### Prerequisites

You will need the `make` command which usually comes with some package such as `build-essential`.
You will also need to install an appropriate version of `go`.
Look at the current [`go.mod`](./go.mod) file to see the minimum `go` version.
All other required tools are installed automatically by `make`.

### Installing build tools

Go-based tools and their versions are controlled by [`internal/tools`](./internal/tools/).
In order to manually install tools you may run `make install-tools`, but `make` will also install them as-needed during the build process if they are not installed.
In order to remove them run `make clean-tools` or `make clean-all`.

### Building the collector

The command `make` or `make build` (the default target) will build the collector for your operating system and CPU architecture. The resulting binary will be `bin/dynatrace-otel-collector`.

#### Cross-compiling other architectures

The command `make snapshot` will build all supported OS and architecture versions of the collector.
The resulting binaries will be in the `dist` directory.

### Testing the collector

The command `make test` will run all collector tests.
It will build the collector first if required.

### Cleaning

Most of the time `make` will automatically detect when files need to be rebuilt, however sometimes you want to manually force a clean build.
The following commands delete generated files and compiled binaries:

- `make clean` removes most generated files except for the build tools.
- `make clean-tools` removes all build tools
- `make clean-all` removes all generated files and build tools

### Updating collector components

The file [`manifest.yaml`](./manifest.yaml) describes all components in the collector distribution.
See https://github.com/open-telemetry/opentelemetry-collector/tree/main/cmd/builder#configuration for details on the format of this file.
