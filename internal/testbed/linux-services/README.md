# Build and test deb/rpm/apk packages

## Prerequisites

Tools:

- [Go](https://go.dev/)
- [GoReleaser](https://goreleaser.com/)
- [Podman](https://podman.io/)
- make

## How to build and test

To build the Collector Linux packages, a few steps are required:

- Run `make snapshot` to build the necessary release assets with all architectures and packaging types into the `dist`
  folder
- Check the filename of the Linux package you want to test in the `dist` folder and fill out the placeholders below
  accordingly
- To start the package tests,
  run: `make package-test PACKAGE_PATH=./dist/dynatrace-otel-collector_<version>_Linux_<arch>.<deb|rpm>`
