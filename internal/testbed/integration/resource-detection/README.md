# Resource detection E2E test

Tests the `dynatrace` detector of the resource detection processor with
[`config_examples/resource-detection.yaml`](../../../../config_examples/resource-detection.yaml).

The detector reads `/var/lib/dynatrace/enrichment/dt_host_metadata.properties`, a path that
can't be configured. To control that file without touching the host, the test runs the
Collector in a container with the file copied in (using
[testcontainers](https://golang.testcontainers.org/)). The Collector exports to a sink in the
test process, reached through `host.docker.internal`.

## Running

Docker is required. From the repository root, build the Collector and the image the test uses:

```sh
make build
docker build -t dynatrace-otel-collector:e2e-test -f Dockerfile bin
```

Then run the test:

```sh
cd internal/testbed/integration/resource-detection
go test -v
```

`make build` must produce a Linux binary, because it runs in the container. On macOS, build
it for Linux (for example with `GOOS=linux`) before building the image.

Rebuild the image after changing the Collector; the test uses whatever
`dynatrace-otel-collector:e2e-test` is available locally. If it is missing, the test fails with
the build command.

## Notes

- On Linux, the test adds `host.docker.internal:host-gateway` to the container. Podman uses
  `host.containers.internal` instead, which the test doesn't handle.
- In CI this runs in the `resource-detection-e2e-test` job of `.github/workflows/e2e.yaml`,
  using the image built by the `docker-build` job.
