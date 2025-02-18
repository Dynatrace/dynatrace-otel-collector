podman run --rm -it \
  -p 8080-8083:8080-8083 \
  -e OTEL_EXPORTER_JAEGER_ENDPOINT=http://172.23.214.27:14267/api/traces \
  jaegertracing/example-hotrod:1.48.0 \
  all

ipconfig getifaddr en0

//OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:6831 go run ./examples/hotrod/main.go all

  podman run --rm -it \
  -p 14268:14268 \
  -p 8080-8083:8080-8083 \
  -e OTEL_EXPORTER_JAEGER_ENDPOINT=http://0.0.0.0:14268 \
  jaegertracing/example-hotrod:1.48.0 \
  all