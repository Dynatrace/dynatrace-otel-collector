module github.com/Dynatrace/dynatrace-otel-collector

go 1.21

require (
	github.com/open-telemetry/opentelemetry-collector-contrib/testbed v0.98.0
	github.com/stretchr/testify v1.9.0
	go.opentelemetry.io/collector/component v0.98.0
	go.opentelemetry.io/collector/pdata v1.5.0
	go.opentelemetry.io/collector/semconv v0.98.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/DataDog/datadog-agent/pkg/trace/exportable v0.0.0-20201016145401-4646cf596b02 // indirect
	github.com/apache/thrift v0.20.0 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cenkalti/backoff/v4 v4.3.0 // indirect
	github.com/census-instrumentation/opencensus-proto v0.4.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/expr-lang/expr v1.16.3 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/fluent/fluent-logger-golang v1.9.0 // indirect
	github.com/fsnotify/fsnotify v1.7.0 // indirect
	github.com/go-logr/logr v1.4.1 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/go-viper/mapstructure/v2 v2.0.0-alpha.1 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/gogo/googleapis v1.4.1 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/gorilla/mux v1.8.1 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.19.1 // indirect
	github.com/haimrubinstein/go-syslog/v3 v3.0.0 // indirect
	github.com/hashicorp/go-version v1.6.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/influxdata/go-syslog/v3 v3.0.1-0.20230911200830-875f5bc594a4 // indirect
	github.com/jaegertracing/jaeger v1.55.0 // indirect
	github.com/jpillora/backoff v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/compress v1.17.8 // indirect
	github.com/knadh/koanf v1.5.0 // indirect
	github.com/knadh/koanf/v2 v2.1.1 // indirect
	github.com/leodido/ragel-machinery v0.0.0-20181214104525-299bdde78165 // indirect
	github.com/lufia/plan9stats v0.0.0-20220913051719-115f729f3c8c // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/mostynb/go-grpc-compression v1.2.2 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/carbonexporter v0.98.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/opencensusexporter v0.98.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusexporter v0.98.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sapmexporter v0.98.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter v0.98.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/syslogexporter v0.98.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/zipkinexporter v0.98.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/common v0.98.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal v0.98.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/sharedcomponent v0.98.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk v0.98.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/batchperresourceattr v0.98.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/experimentalmetricmetadata v0.98.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden v0.98.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil v0.98.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/resourcetotelemetry v0.98.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza v0.98.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/jaeger v0.98.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/opencensus v0.98.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheus v0.98.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/signalfx v0.98.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/zipkin v0.98.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/jaegerreceiver v0.98.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/opencensusreceiver v0.98.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/syslogreceiver v0.98.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/zipkinreceiver v0.98.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/testbed/mockdatasenders/mockdatadogagentexporter v0.98.0 // indirect
	github.com/openzipkin/zipkin-go v0.4.2 // indirect
	github.com/philhofer/fwd v1.1.2 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/power-devops/perfstat v0.0.0-20220216144756-c35f1ee13d7c // indirect
	github.com/prometheus/client_golang v1.19.0 // indirect
	github.com/prometheus/client_model v0.6.1 // indirect
	github.com/prometheus/common v0.52.2 // indirect
	github.com/prometheus/procfs v0.12.0 // indirect
	github.com/rs/cors v1.10.1 // indirect
	github.com/shirou/gopsutil/v3 v3.24.3 // indirect
	github.com/shoenig/go-m1cpu v0.1.6 // indirect
	github.com/signalfx/com_signalfx_metrics_protobuf v0.0.3 // indirect
	github.com/signalfx/sapm-proto v0.14.0 // indirect
	github.com/soheilhy/cmux v0.1.5 // indirect
	github.com/spf13/cobra v1.8.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/tinylib/msgp v1.1.9 // indirect
	github.com/tklauser/go-sysconf v0.3.12 // indirect
	github.com/tklauser/numcpus v0.6.1 // indirect
	github.com/valyala/fastjson v1.6.4 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	go.opencensus.io v0.24.0 // indirect
	go.opentelemetry.io/collector v0.98.0 // indirect
	go.opentelemetry.io/collector/config/configauth v0.98.0 // indirect
	go.opentelemetry.io/collector/config/configcompression v1.5.0 // indirect
	go.opentelemetry.io/collector/config/configgrpc v0.98.0 // indirect
	go.opentelemetry.io/collector/config/confighttp v0.98.0 // indirect
	go.opentelemetry.io/collector/config/confignet v0.98.0 // indirect
	go.opentelemetry.io/collector/config/configopaque v1.5.0 // indirect
	go.opentelemetry.io/collector/config/configretry v0.98.0 // indirect
	go.opentelemetry.io/collector/config/configtelemetry v0.98.0 // indirect
	go.opentelemetry.io/collector/config/configtls v0.98.0 // indirect
	go.opentelemetry.io/collector/config/internal v0.98.0 // indirect
	go.opentelemetry.io/collector/confmap v0.98.0 // indirect
	go.opentelemetry.io/collector/confmap/converter/expandconverter v0.98.0 // indirect
	go.opentelemetry.io/collector/confmap/provider/envprovider v0.98.0 // indirect
	go.opentelemetry.io/collector/confmap/provider/fileprovider v0.98.0 // indirect
	go.opentelemetry.io/collector/confmap/provider/httpprovider v0.98.0 // indirect
	go.opentelemetry.io/collector/confmap/provider/httpsprovider v0.98.0 // indirect
	go.opentelemetry.io/collector/confmap/provider/yamlprovider v0.98.0 // indirect
	go.opentelemetry.io/collector/connector v0.98.0 // indirect
	go.opentelemetry.io/collector/consumer v0.98.0 // indirect
	go.opentelemetry.io/collector/exporter v0.98.0 // indirect
	go.opentelemetry.io/collector/exporter/debugexporter v0.98.0 // indirect
	go.opentelemetry.io/collector/exporter/otlpexporter v0.98.0 // indirect
	go.opentelemetry.io/collector/exporter/otlphttpexporter v0.98.0 // indirect
	go.opentelemetry.io/collector/extension v0.98.0 // indirect
	go.opentelemetry.io/collector/extension/auth v0.98.0 // indirect
	go.opentelemetry.io/collector/extension/ballastextension v0.98.0 // indirect
	go.opentelemetry.io/collector/extension/zpagesextension v0.98.0 // indirect
	go.opentelemetry.io/collector/featuregate v1.5.0 // indirect
	go.opentelemetry.io/collector/otelcol v0.98.0 // indirect
	go.opentelemetry.io/collector/processor v0.98.0 // indirect
	go.opentelemetry.io/collector/processor/batchprocessor v0.98.0 // indirect
	go.opentelemetry.io/collector/processor/memorylimiterprocessor v0.98.0 // indirect
	go.opentelemetry.io/collector/receiver v0.98.0 // indirect
	go.opentelemetry.io/collector/receiver/otlpreceiver v0.98.0 // indirect
	go.opentelemetry.io/collector/service v0.98.0 // indirect
	go.opentelemetry.io/contrib/config v0.4.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.49.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.49.0 // indirect
	go.opentelemetry.io/contrib/propagators/b3 v1.25.0 // indirect
	go.opentelemetry.io/contrib/zpages v0.50.0 // indirect
	go.opentelemetry.io/otel v1.25.0 // indirect
	go.opentelemetry.io/otel/bridge/opencensus v1.25.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc v1.25.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp v1.25.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.25.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.25.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp v1.25.0 // indirect
	go.opentelemetry.io/otel/exporters/prometheus v0.47.0 // indirect
	go.opentelemetry.io/otel/exporters/stdout/stdoutmetric v1.25.0 // indirect
	go.opentelemetry.io/otel/exporters/stdout/stdouttrace v1.25.0 // indirect
	go.opentelemetry.io/otel/metric v1.25.0 // indirect
	go.opentelemetry.io/otel/sdk v1.25.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.25.0 // indirect
	go.opentelemetry.io/otel/trace v1.25.0 // indirect
	go.opentelemetry.io/proto/otlp v1.1.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.27.0 // indirect
	golang.org/x/exp v0.0.0-20240119083558-1b970713d09a // indirect
	golang.org/x/net v0.24.0 // indirect
	golang.org/x/sys v0.19.0 // indirect
	golang.org/x/text v0.14.0 // indirect
	gonum.org/v1/gonum v0.15.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20240314234333-6e1732d8331c // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240401170217-c3f982113cda // indirect
	google.golang.org/grpc v1.63.2 // indirect
	google.golang.org/protobuf v1.33.0 // indirect
)

replace cloud.google.com/go => cloud.google.com/go v0.112.2
