# Set up debug environment with jaeger receiver

The primary purpose of setting up a debug environment with the Jaeger receiver is to inspect the behavior of the Jaeger receiver and the entire collector while feeding it with data using the [HotROD](https://github.com/jaegertracing/jaeger/blob/main/examples/hotrod/README.md) application. This debug environment setup substitutes the standard setup using the CLI debug tool to emit traces. Creating a Jaeger receiver sender is more complex due to a known limitation in the Jaeger model, which, when implemented, requires a manual implementation of the JSON to Protobuf converter. This adds a burden on future maintenance and does not guarantee a bug-free behavior on the debug CLI side. More information about the limitation can be found [here](https://github.com/golang/protobuf/issues/698).

Additionally, the current standard of sending traces from Jaeger to the collector is via the `otlp receiver`. Therefore, the `jaeger receiver` is currently used less, supporting the idea of avoiding significant time investments to support this directly in the debug CLI tool.

Due to the mentioned complexity, we can use [HotROD](https://github.com/jaegertracing/jaeger/blob/main/examples/hotrod/README.md) to send traces to the Jaeger receiver configured as part of the collector.

With this setup, you should be able to access the HotROD UI to generate traces, which will be sent to the collector and then from the collector to the debug CLI, which acts as a sink and stores the received traces to a file.

## Prerequisities

- Podman v5

## Set up

1. Retrieve you local IP address

Depending on your OS, you should retrieve your local IP address so HotROD running in a Podman container can send traces to your collector running locally. This is convenient because you can run your collector via an IDE and therefore have more debug options available.

On macOS or Linux, you can display your network interfaces via:

```shell
ifconfig
```

You should search for `en0` (MacOS) or `eth0` (Linux), but it can differ on every system.

Afterwards, store your IP address in an environment variable:

```shell
export LOCAL_IP=<your_ip_address>
```

1. Start up HotROD using podman.

It will send traces to the url configured via `OTEL_EXPORTER_JAEGER_ENDPOINT`. The endpoint here respresents the address the Jaeger receiver is receiving data.

HotROD uses the `thrift_http` protocol to send traces to the collector.

```shell
podman run --rm -it \
  -p 8080-8083:8080-8083 \
  -e OTEL_EXPORTER_JAEGER_ENDPOINT=http://$LOCAL_IP:14268/api/traces \
  jaegertracing/example-hotrod:1.48.0 \
  all
```

**Note:**

Keep in mind that you need to use the HotROD `v1.48.0`, as the newest version does not support this setup.

1. Start collecor

You can start the collector from locally build binary, IDE or official release. You can use the [example config](./config.yaml) with the `thrift_http` protocol enabled.

```shell
./collector --config $(pwd)/commands/jaeger/config.yaml
```

1. Build and start debug CLI tool to receive traces

To build the CLI, execute the following command:

```shell
go build -o data-ingest
```

To start it in the `receiver mode`

```shell
./data-ingest --receive --output-file received.json --receiver-port 4319 --receiver-type http
```

## Run

You can access `http://localhost:8080` and click on one of the trace generators. Afterwards you can see in the logs of the HotROD that the traces are being generated and sent to the collector. If you have set up a debug exporter in the collector (set up by default in the [example config](./config.yaml)),
you should be able to see those traces also in the `received.json` file, as the traces are being exported via `otlp_http` exporter to the debug CLI tool and stored by it in the file.
