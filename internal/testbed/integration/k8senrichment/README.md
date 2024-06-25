# Enrich from Kubernetes

This is the e2e test for the Collector use-case:
[Enrich from Kubernetes](https://docs.dynatrace.com/docs/shortlink/otel-collector-cases-k8s-enrich).

## Requirements to run the tests

- Docker or Podman
- Kind

The tests require a running Kind k8s cluster. During the tests,
a Dynatrace distribution of the OpenTelemetry Collector is deployed
on the k8s cluster with configurations as per the Dynatrace documentation page.

Traces are generated and sent to the Collector, which then
exports to the test where the k8s attributes are asserted on the
received traces.

## Running the tests locally

To execute the tests locally, follow these steps:

### Start a `kind` cluster

To create a new cluster, execute the following command:

```shell
kind create cluster
```
### Build the collector binary

This is done using the 
```shell
make build
```
command. This will build the collector distro, and place the built binary 
into the `bin` directory of your local copy of the repository.

**NOTE:** When using an M1 mac, the `make build` command will build the `arm64` binary, which
will not be able to run as a container on a `kind` cluster.
For now, as workaround you will need to use the `make build-all` command to build all binaries and then copy the
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

After the above steps are completed, the E2E tests can be run.

Note that the tests will, by default, use the following `kubeconfig` path: `/tmp/kube-config-collector-e2e-testing`.
This path can be modified by setting the `KUBECONFIG` environment variable (in case you have a local kind cluster with the 
kube config located in `~/.kube.config`).
Also, if you are using `podman`, the collector image will be prefixed with `localhost/` within the local
`kind` registry. In this case, you will need to set the `CONTAINER_REGISTRY` to `localhost/`.
When using `docker`, setting the `CONTAINER_REGISTRY` env var is not required.
Below are the commands to execute the `k8senrichment` e2e test:

** Using podman:**
```shell
cd internal/testbed/integration/k8senrichment
KUBECONFIG="/Users/my-user/.kube/config" CONTAINER_REGISTRY="localhost/" go test -v --tags=e2e
```

** Using docker:**
```shell
cd internal/testbed/integration/k8senrichment
KUBECONFIG="/Users/my-user/.kube/config" go test -v --tags=e2e
```


