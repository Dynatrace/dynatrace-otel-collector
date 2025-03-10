# Set up debug environment with Netflow receiver

The primary purpose of setting up a debug environment with the NetFlow receiver is to inspect the behavior of the NetFlow receiver and the entire collector while feeding it with data using the [nflow-generator](https://github.com/nerdalert/nflow-generator) application. This debug environment setup substitutes the standard setup using the CLI debug tool to emit NetFlow data.

A known limitation of the `nflow-generator` tool is that it can only send `netFlow v5` data and hence misses the `netflow v9`, `netflow IPFIX` and `sflow v5` formats, which are currently supported by the [OTel Netflow receiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/netflowreceiver).

## Prerequisities

- Podman v5

## Set up

1. Retrieve your local IP address

Depending on your OS, you should retrieve your local IP address so Netflow generator running in a Podman container can send data to your collector running locally. This is convenient because you can run your collector via an IDE and therefore have more debug options available.

On macOS or Linux, you can display your network interfaces via:

```shell
ifconfig
```

You should search for `en0` (macOS) or `enp0` (Linux), but it can differ on every system.

Afterwards, store your IP address in an environment variable:

```shell
export LOCAL_IP=<your_ip_address>
```

1. Start collecor

You can start the collector from locally build binary, IDE or official release. You can use the [example config](./config.yaml).

```shell
./collector --config $(pwd)/commands/netflow/config.yaml
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

1. Start up Netflow generator using podman.

```shell
podman run -it --rm \
  networkstatic/nflow-generator \
  -t $LOCAL_IP -p 2055 -c 16
```

The tool will generate `netflow v5` data and send it to the collector, which will then forward it to the receiver sink. The received data will be stored in `received.json` file.
