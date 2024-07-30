module github.com/Dynatrace/dynatrace-otel-collector/internal/testbed

go 1.22.0

toolchain go1.22.5

require (
	github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon v0.0.0
	github.com/google/uuid v1.6.0
	github.com/open-telemetry/opentelemetry-collector-contrib/testbed v0.106.0
	github.com/stretchr/testify v1.9.0
	go.opentelemetry.io/collector/component v0.106.0
	go.opentelemetry.io/collector/consumer v0.106.0
	go.opentelemetry.io/collector/featuregate v1.12.0
	go.opentelemetry.io/collector/pdata v1.12.0
	go.opentelemetry.io/collector/semconv v0.106.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/DataDog/datadog-agent/pkg/trace/exportable v0.0.0-20201016145401-4646cf596b02 // indirect
	github.com/Microsoft/go-winio v0.6.1 // indirect
	github.com/alecthomas/participle/v2 v2.1.1 // indirect
	github.com/apache/thrift v0.20.0 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cenkalti/backoff/v4 v4.3.0 // indirect
	github.com/census-instrumentation/opencensus-proto v0.4.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/distribution/reference v0.5.0 // indirect
	github.com/docker/docker v27.1.0+incompatible // indirect
	github.com/docker/go-connections v0.5.0 // indirect
	github.com/docker/go-units v0.5.0 // indirect
	github.com/emicklei/go-restful/v3 v3.11.0 // indirect
	github.com/expr-lang/expr v1.16.9 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/fluent/fluent-logger-golang v1.9.0 // indirect
	github.com/fsnotify/fsnotify v1.7.0 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/go-openapi/jsonpointer v0.20.2 // indirect
	github.com/go-openapi/jsonreference v0.20.4 // indirect
	github.com/go-openapi/swag v0.22.9 // indirect
	github.com/go-viper/mapstructure/v2 v2.0.0 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/goccy/go-json v0.10.3 // indirect
	github.com/gogo/googleapis v1.4.1 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/google/gnostic-models v0.6.8 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/gorilla/mux v1.8.1 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.20.0 // indirect
	github.com/hashicorp/go-version v1.7.0 // indirect
	github.com/hashicorp/golang-lru v1.0.2 // indirect
	github.com/hashicorp/golang-lru/v2 v2.0.7 // indirect
	github.com/iancoleman/strcase v0.3.0 // indirect
	github.com/imdario/mergo v0.3.16 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/jaegertracing/jaeger v1.59.0 // indirect
	github.com/jonboulle/clockwork v0.4.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/jpillora/backoff v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/compress v1.17.9 // indirect
	github.com/knadh/koanf v1.5.0 // indirect
	github.com/knadh/koanf/v2 v2.1.1 // indirect
	github.com/leodido/go-syslog/v4 v4.1.0 // indirect
	github.com/leodido/ragel-machinery v0.0.0-20190525184631-5f46317e436b // indirect
	github.com/lightstep/go-expohisto v1.0.0 // indirect
	github.com/lufia/plan9stats v0.0.0-20240408141607-282e7b5d6b74 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/moby/docker-image-spec v1.3.1 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/mostynb/go-grpc-compression v1.2.3 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/connector/routingconnector v0.106.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/connector/spanmetricsconnector v0.106.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/carbonexporter v0.106.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/opencensusexporter v0.106.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusexporter v0.106.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sapmexporter v0.106.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter v0.106.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/syslogexporter v0.106.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/zipkinexporter v0.106.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/common v0.106.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal v0.106.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/sharedcomponent v0.106.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk v0.106.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/batchperresourceattr v0.106.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/experimentalmetricmetadata v0.106.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden v0.106.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl v0.106.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil v0.106.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/resourcetotelemetry v0.106.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza v0.106.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/jaeger v0.106.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/opencensus v0.106.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheus v0.106.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/signalfx v0.106.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/zipkin v0.106.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/jaegerreceiver v0.106.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/opencensusreceiver v0.106.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/syslogreceiver v0.106.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/zipkinreceiver v0.106.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/testbed/mockdatasenders/mockdatadogagentexporter v0.106.0 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.1.0 // indirect
	github.com/openzipkin/zipkin-go v0.4.3 // indirect
	github.com/philhofer/fwd v1.1.3-0.20240612014219-fbbf4953d986 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/power-devops/perfstat v0.0.0-20240221224432-82ca36839d55 // indirect
	github.com/prometheus/client_golang v1.19.1 // indirect
	github.com/prometheus/client_model v0.6.1 // indirect
	github.com/prometheus/common v0.55.0 // indirect
	github.com/prometheus/procfs v0.15.1 // indirect
	github.com/rs/cors v1.11.0 // indirect
	github.com/shirou/gopsutil/v4 v4.24.6 // indirect
	github.com/shoenig/go-m1cpu v0.1.6 // indirect
	github.com/signalfx/com_signalfx_metrics_protobuf v0.0.3 // indirect
	github.com/signalfx/sapm-proto v0.14.0 // indirect
	github.com/soheilhy/cmux v0.1.5 // indirect
	github.com/spf13/cobra v1.8.1 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/tilinna/clock v1.1.0 // indirect
	github.com/tinylib/msgp v1.2.0 // indirect
	github.com/tklauser/go-sysconf v0.3.14 // indirect
	github.com/tklauser/numcpus v0.8.0 // indirect
	github.com/valyala/fastjson v1.6.4 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	go.opencensus.io v0.24.0 // indirect
	go.opentelemetry.io/collector v0.106.0 // indirect
	go.opentelemetry.io/collector/client v0.106.0 // indirect
	go.opentelemetry.io/collector/config/configauth v0.106.0 // indirect
	go.opentelemetry.io/collector/config/configcompression v1.12.0 // indirect
	go.opentelemetry.io/collector/config/configgrpc v0.106.0 // indirect
	go.opentelemetry.io/collector/config/confighttp v0.106.0 // indirect
	go.opentelemetry.io/collector/config/confignet v0.106.0 // indirect
	go.opentelemetry.io/collector/config/configopaque v1.12.0 // indirect
	go.opentelemetry.io/collector/config/configretry v1.12.0 // indirect
	go.opentelemetry.io/collector/config/configtelemetry v0.106.0 // indirect
	go.opentelemetry.io/collector/config/configtls v1.12.0 // indirect
	go.opentelemetry.io/collector/config/internal v0.106.0 // indirect
	go.opentelemetry.io/collector/confmap v0.106.0 // indirect
	go.opentelemetry.io/collector/confmap/provider/fileprovider v0.106.0 // indirect
	go.opentelemetry.io/collector/connector v0.106.0 // indirect
	go.opentelemetry.io/collector/consumer/consumerprofiles v0.106.0 // indirect
	go.opentelemetry.io/collector/consumer/consumertest v0.106.0 // indirect
	go.opentelemetry.io/collector/exporter v0.106.0 // indirect
	go.opentelemetry.io/collector/exporter/debugexporter v0.106.0 // indirect
	go.opentelemetry.io/collector/exporter/otlpexporter v0.106.0 // indirect
	go.opentelemetry.io/collector/exporter/otlphttpexporter v0.106.0 // indirect
	go.opentelemetry.io/collector/extension v0.106.0 // indirect
	go.opentelemetry.io/collector/extension/auth v0.106.0 // indirect
	go.opentelemetry.io/collector/extension/ballastextension v0.106.0 // indirect
	go.opentelemetry.io/collector/extension/zpagesextension v0.106.0 // indirect
	go.opentelemetry.io/collector/internal/globalgates v0.106.0 // indirect
	go.opentelemetry.io/collector/otelcol v0.106.0 // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.106.0 // indirect
	go.opentelemetry.io/collector/processor v0.106.0 // indirect
	go.opentelemetry.io/collector/processor/batchprocessor v0.106.0 // indirect
	go.opentelemetry.io/collector/processor/memorylimiterprocessor v0.106.0 // indirect
	go.opentelemetry.io/collector/receiver v0.106.0 // indirect
	go.opentelemetry.io/collector/receiver/otlpreceiver v0.106.0 // indirect
	go.opentelemetry.io/collector/service v0.106.0 // indirect
	go.opentelemetry.io/contrib/config v0.8.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.53.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.53.0 // indirect
	go.opentelemetry.io/contrib/propagators/b3 v1.28.0 // indirect
	go.opentelemetry.io/contrib/zpages v0.53.0 // indirect
	go.opentelemetry.io/otel v1.28.0 // indirect
	go.opentelemetry.io/otel/bridge/opencensus v1.28.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp v0.4.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc v1.28.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp v1.28.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.28.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.28.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp v1.28.0 // indirect
	go.opentelemetry.io/otel/exporters/prometheus v0.50.0 // indirect
	go.opentelemetry.io/otel/exporters/stdout/stdoutmetric v1.28.0 // indirect
	go.opentelemetry.io/otel/exporters/stdout/stdouttrace v1.28.0 // indirect
	go.opentelemetry.io/otel/log v0.4.0 // indirect
	go.opentelemetry.io/otel/metric v1.28.0 // indirect
	go.opentelemetry.io/otel/sdk v1.28.0 // indirect
	go.opentelemetry.io/otel/sdk/log v0.4.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.28.0 // indirect
	go.opentelemetry.io/otel/trace v1.28.0 // indirect
	go.opentelemetry.io/proto/otlp v1.3.1 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.27.0 // indirect
	golang.org/x/exp v0.0.0-20240506185415-9bf2ced13842 // indirect
	golang.org/x/mod v0.18.0 // indirect
	golang.org/x/net v0.27.0 // indirect
	golang.org/x/oauth2 v0.21.0 // indirect
	golang.org/x/sync v0.7.0 // indirect
	golang.org/x/sys v0.22.0 // indirect
	golang.org/x/term v0.22.0 // indirect
	golang.org/x/text v0.16.0 // indirect
	golang.org/x/time v0.5.0 // indirect
	golang.org/x/tools v0.22.0 // indirect
	gonum.org/v1/gonum v0.15.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20240701130421-f6361c86f094 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240701130421-f6361c86f094 // indirect
	google.golang.org/grpc v1.65.0 // indirect
	google.golang.org/protobuf v1.34.2 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	k8s.io/api v0.30.3 // indirect
	k8s.io/apimachinery v0.30.3 // indirect
	k8s.io/client-go v0.30.3 // indirect
	k8s.io/klog/v2 v2.120.1 // indirect
	k8s.io/kube-openapi v0.0.0-20240228011516-70dd3763d340 // indirect
	k8s.io/utils v0.0.0-20230726121419-3b25d923346b // indirect
	sigs.k8s.io/json v0.0.0-20221116044647-bc3834ca7abd // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.4.1 // indirect
	sigs.k8s.io/yaml v1.3.0 // indirect
)

replace cloud.google.com/go => cloud.google.com/go v0.115.0

replace github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon v0.0.0 => ../testcommon
