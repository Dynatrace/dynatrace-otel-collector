# Combined load test

This load test showcases the CPU and memory usage of the `dynatrace-collector` when accepting all
types of data (logs, metrics, traces).
[Telemetrygen](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/cmd/telemetrygen#telemetry-generator-for-opentelemetry)
is used to generate data sent into the collector via OTLP.

The generated data has the following parameters:

- 1000 traces per second (size 1.2 KB)
- 1MB logs per second
- 1000 metrics per second (single datapoint; size 1.2KB)

The test firstly sets up the [metrics-server](https://kubernetes-sigs.github.io/metrics-server/)
in order to collect CPU and memory usage.
Afterwards, `dynatrace-collector` and 3 `telemetrygen` pods (one for each data type)
are deployed.
The data from the `metrics-server` is retrieved via `metricsAPIClient` every 15 seconds for 150 seconds
and written out as part of the logs to showcase how the CPU and memory usage increases in the first 30-90 seconds
and stabilizes afterwards.

```shell
$ KUBECONFIG=/Users/ondrej.dubaj/.kube/config CONTAINER_REGISTRY="localhost/" go test -v
=== RUN   TestLoad_Combined
    e2e_test.go:36: deploying metrics-server...
    e2e_test.go:48: metrics-server deployed
    collector.go:62: waiting for collector pods to be ready
    collector.go:100: collector pods are ready
    e2e_test.go:67: deploying telemetrygen...
    e2e_test.go:78: telemetrygen deployed
    e2e_test.go:85: collecting data...
    e2e_test.go:93: ------------------------------------------------------
    e2e_test.go:94: data after 15 second:
    e2e_test.go:95: memory: 96Mi, cpu: 124m
    e2e_test.go:96: ------------------------------------------------------
    e2e_test.go:93: ------------------------------------------------------
    e2e_test.go:94: data after 30 second:
    e2e_test.go:95: memory: 117Mi, cpu: 127m
    e2e_test.go:96: ------------------------------------------------------
    e2e_test.go:93: ------------------------------------------------------
    e2e_test.go:94: data after 45 second:
    e2e_test.go:95: memory: 136Mi, cpu: 127m
    e2e_test.go:96: ------------------------------------------------------
    e2e_test.go:93: ------------------------------------------------------
    e2e_test.go:94: data after 60 second:
    e2e_test.go:95: memory: 161Mi, cpu: 127m
    e2e_test.go:96: ------------------------------------------------------
    e2e_test.go:93: ------------------------------------------------------
    e2e_test.go:94: data after 75 second:
    e2e_test.go:95: memory: 175Mi, cpu: 128m
    e2e_test.go:96: ------------------------------------------------------
    e2e_test.go:93: ------------------------------------------------------
    e2e_test.go:94: data after 90 second:
    e2e_test.go:95: memory: 203Mi, cpu: 126m
    e2e_test.go:96: ------------------------------------------------------
    e2e_test.go:93: ------------------------------------------------------
    e2e_test.go:94: data after 105 second:
    e2e_test.go:95: memory: 214Mi, cpu: 127m
    e2e_test.go:96: ------------------------------------------------------
    e2e_test.go:93: ------------------------------------------------------
    e2e_test.go:94: data after 120 second:
    e2e_test.go:95: memory: 214Mi, cpu: 127m
    e2e_test.go:96: ------------------------------------------------------
    e2e_test.go:93: ------------------------------------------------------
    e2e_test.go:94: data after 135 second:
    e2e_test.go:95: memory: 215Mi, cpu: 128m
    e2e_test.go:96: ------------------------------------------------------
    e2e_test.go:93: ------------------------------------------------------
    e2e_test.go:94: data after 150 second:
    e2e_test.go:95: memory: 218Mi, cpu: 126m
    e2e_test.go:96: ------------------------------------------------------
--- PASS: TestLoad_Combined (198.28s)
```

## Requirements to run the test

- Docker or Podman
- Kind

The test requires a running Kind k8s cluster. During the test,
a Dynatrace distribution of the OpenTelemetry Collector is deployed
on the k8s cluster with configurations as per the Dynatrace documentation page.

## Running the tests locally

To execute the tests locally, follow these steps:

### Start a `kind` cluster

To create a new cluster, execute the following command:

```shell
kind create cluster
```
### Build the collector binary

This is done using the `make build` command.
This will build the collector distro, and place the built binary 
into the `bin` directory of your local copy of the repository.

**NOTE:** When using an ARM-based Mac, the `make build` command will build the `arm64` binary, which
will not be able to run as a container on a `kind` cluster.
For now, as a workaround you will need to use the `make build-all` command to build all binaries and then copy the
`linux_amd64` binary from the `dist` directory into `bin` under the name `dynatrace-otel-collector`.

### Build the container

From the `bin` directory, use `docker` or `podman` to build the collector image, and to load the built
image into the local registry of the `kind` cluster.

Note that this process differs between `podman` and `docker`, as with `podman`, you will have to
load the image using the `image-archive` argument:

**Build and load with `podman`:**
```shell
cd bin
podman buildx build -t dynatrace-otel-collector:e2e-test -f ../Dockerfile . --load
podman save -o dynatrace-otel-collector.tar dynatrace-otel-collector:e2e-test
kind load image-archive dynatrace-otel-collector.tar --name kind
cd ..
```

**Build and load with `docker`:**
```shell
cd bin
docker buildx build -t dynatrace-otel-collector:e2e-test -f ../Dockerfile . --load
docker save -o dynatrace-otel-collector.tar dynatrace-otel-collector:e2e-test
kind load docker-image dynatrace-otel-collector:e2e-test --name kind
cd ..
```

### Running the tests

After the above steps are completed, the load test can be run.

Note that the test will, by default, use the following `kubeconfig` path: `/tmp/kube-config-collector-e2e-testing`.
This path can be modified by setting the `KUBECONFIG` environment variable (in case you have a local kind cluster with the
kube config located in `~/.kube.config`).
Also, if you are using `podman`, the collector image will be prefixed with `localhost/` within the local
`kind` registry. In this case, you will need to set the `CONTAINER_REGISTRY` to `localhost/`.
When using `docker`, setting the `CONTAINER_REGISTRY` env var is not required.
Below are the commands to execute the `combinedload` e2e test:

** Using podman:**
```shell
cd internal/testbed/integration/combinedload
KUBECONFIG="~/.kube/config" CONTAINER_REGISTRY="localhost/" go test -v
```

** Using docker:**
```shell
cd internal/testbed/integration/combinedload
KUBECONFIG="/Users/my-user/.kube/config" go test -v
```
