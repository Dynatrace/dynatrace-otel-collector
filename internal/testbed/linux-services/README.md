# Build and test deb/rpm/apk packages

## Prerequisites

Tools:

- [Go](https://go.dev/)
- [GoReleaser](https://goreleaser.com/)
- [Podman](https://podman.io/)
- make

## How to build and test

To build the Collector Linux packages, a few steps are required:

- Run `make snapshot` to build the necessary release assets with all architectures and packaging types into the `dist` folder
- To start the package tests,
  run: `./internal/testbed/linux-services/package-tests.sh ./dist/dynatrace-otel-collector_*_Linux_x86-64.<deb|rpm>`

