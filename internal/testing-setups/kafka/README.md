# Kafka test environment

This is a demo environment for checking how the Kafka receiver and exporter
interface with a real Kafka cluster. The files in this directory are meant for
quick scaffolding, and are intended to be edited as desired when using the setup
to troubleshoot an issue.

## Prerequisites

You will need the following tools to use this setup:

* Some kind of containerization environment
* kind to run Kubernetes
* Helm to install the Collector Helm chart
* Golang
* Ideally k9s or a similar tool to view the Collector's logs

## Usage

To start the test environment, run:

```shell
./up.sh
```

This will create a kind cluster, install the Strimzi Kafka Operator, and
instantiate a Kafka cluster. It will also start a Collector that receives OTLP
on port 30000, sends it to the cluster, then listens on the same topic and
prints the data in the debug logs.

To send data to the Collector, run:

```shell
./send-data.sh
```

Use `Ctrl+C` to end telemetry generation.

This will use telemetrygen (obtained through `go tool`) to send data to the
Collector. After a short period of time, you should see the telemetry appear in
the Collector container's logs.

To shut down the setup, run:

```shell
./down.sh
```
