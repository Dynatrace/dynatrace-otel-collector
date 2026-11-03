// Copyright Dynatrace LLC
// SPDX-License-Identifier: Apache-2.0

package resourcedetection

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/oteltest"
	"github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"
)

// image is built by the docker-build CI job; locally build it from the repository root with
// docker build -t dynatrace-otel-collector:e2e-test -f Dockerfile bin
const image = "dynatrace-otel-collector:e2e-test"

const (
	enrichmentFile = "/var/lib/dynatrace/enrichment/dt_host_metadata.properties"

	enrichmentContent = `dt.entity.host=my-host-from-properties
host.name=my-host-from-properties
dt.entity.host_group=my-host-group-from-properties
dt.foo=bar
dt.smartscape.host=my-smartscape-host-from-properties
not-a-property
`
)

// startCollector starts the DT Collector image with config_examples/resource-detection.yaml.
// If enrichment is not empty, it is the content of the dt_host_metadata.properties file in the
// container. It returns the host port of the OTLP/gRPC receiver and the sink the Collector exports to.
func startCollector(t *testing.T, enrichment string) (int, *consumertest.TracesSink) {
	sink, sinkPorts := oteltest.StartTracesSink(t)

	// The Collector runs in a container: it listens on all interfaces and reaches the sink
	// on the host through host.docker.internal.
	cfg := testutil.LoadConfigExample(t, "resource-detection.yaml").
		ReplaceString("${env:MY_POD_IP}", "0.0.0.0").
		Replace("exporter endpoint", func(s string) string { return testutil.ReplaceDynatraceExporterEndpoint(s, sinkPorts.Http) }).
		ReplaceString("http://0.0.0.0:", "http://host.docker.internal:").
		Replace("sending_queue and retry_on_failure", func(s string) string { return testutil.DisableSendingQueueAndRetry(s, "otlp_http") })

	files := []testcontainers.ContainerFile{{
		Reader:            strings.NewReader(cfg.String()),
		ContainerFilePath: "/etc/otelcol/config.yaml",
		FileMode:          0o644,
	}}
	if enrichment != "" {
		files = append(files, testcontainers.ContainerFile{
			Reader:            strings.NewReader(enrichment),
			ContainerFilePath: enrichmentFile,
			FileMode:          0o644,
		})
	}

	// Detection runs in the processor's Start, so once the Collector is ready it is done.
	// The image has no shell, so wait for the log line instead of a port check.
	ctr, err := testcontainers.Run(t.Context(), image,
		testcontainers.WithExposedPorts("4317/tcp"),
		testcontainers.WithFiles(files...),
		testcontainers.WithHostConfigModifier(func(hc *container.HostConfig) {
			hc.ExtraHosts = append(hc.ExtraHosts, "host.docker.internal:host-gateway")
		}),
		testcontainers.WithWaitStrategy(wait.ForLog("Everything is ready").WithStartupTimeout(time.Minute)),
	)
	testcontainers.CleanupContainer(t, ctr)
	require.NoError(t, err, "failed to start %s; build it with: docker build -t %s -f Dockerfile bin", image, image)

	port, err := ctr.MappedPort(t.Context(), "4317/tcp")
	require.NoError(t, err)
	return int(port.Num()), sink
}

// exportResources sends one span per resource, in a single export.
func exportResources(t *testing.T, port int, resources map[string]map[string]any) {
	td := ptrace.NewTraces()
	for service, attrs := range resources {
		rs := td.ResourceSpans().AppendEmpty()
		require.NoError(t, rs.Resource().Attributes().FromRaw(attrs))
		rs.Resource().Attributes().PutStr("service.name", service)
		span := rs.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
		span.SetName(service)
		span.SetTraceID([16]byte{1})
		span.SetSpanID([8]byte{1})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err := oteltest.NewTracesClient(t, port).Export(ctx, ptraceotlp.NewExportRequestFromTraces(td))
	require.NoError(t, err)
}

// receivedResources returns the resource attributes of the received traces, by service.name.
func receivedResources(t *testing.T, sink *consumertest.TracesSink, want int) map[string]map[string]any {
	oteltest.WaitForTraces(t, 1, sink)
	got := map[string]map[string]any{}
	for _, td := range sink.AllTraces() {
		for i := range td.ResourceSpans().Len() {
			attrs := td.ResourceSpans().At(i).Resource().Attributes().AsRaw()
			service, _ := attrs["service.name"].(string)
			require.NotContains(t, got, service, "duplicate resource")
			got[service] = attrs
		}
	}
	require.Len(t, got, want)
	require.Equal(t, want, sink.SpanCount())
	return got
}

// TestE2E_ResourceDetectionProcessor tests the dynatrace detector of the resource detection
// processor: it adds the host properties from dt_host_metadata.properties to resources and,
// with override: false, keeps existing values. The config under test is
// config_examples/resource-detection.yaml.
//
// The detector reads a hard-coded path, so the Collector runs in a container with that file
// copied in. This needs Docker and the image (see image).
func TestE2E_ResourceDetectionProcessor(t *testing.T) {
	t.Run("file present", func(t *testing.T) {
		port, sink := startCollector(t, enrichmentContent)

		exportResources(t, port, map[string]map[string]any{
			"rd-none-set": {},
			"rd-all-set": {
				"host.name":          "my-host-not-overridden",
				"dt.entity.host":     "my-host-not-overridden",
				"dt.smartscape.host": "my-smartscape-host-not-overridden",
			},
			"rd-host-name-set": {"host.name": "my-host-not-overridden"},
		})

		got := receivedResources(t, sink, 3)
		assert.Equal(t, map[string]any{
			"service.name":       "rd-none-set",
			"dt.entity.host":     "my-host-from-properties",
			"host.name":          "my-host-from-properties",
			"dt.smartscape.host": "my-smartscape-host-from-properties",
		}, got["rd-none-set"])
		assert.Equal(t, map[string]any{
			"service.name":       "rd-all-set",
			"host.name":          "my-host-not-overridden",
			"dt.entity.host":     "my-host-not-overridden",
			"dt.smartscape.host": "my-smartscape-host-not-overridden",
		}, got["rd-all-set"])
		// override: false applies per key.
		assert.Equal(t, map[string]any{
			"service.name":       "rd-host-name-set",
			"host.name":          "my-host-not-overridden",
			"dt.entity.host":     "my-host-from-properties",
			"dt.smartscape.host": "my-smartscape-host-from-properties",
		}, got["rd-host-name-set"])
	})

	// Without OneAgent the Collector starts and passes data through unchanged.
	t.Run("file absent", func(t *testing.T) {
		port, sink := startCollector(t, "")

		exportResources(t, port, map[string]map[string]any{"rd-none-set": {}})

		got := receivedResources(t, sink, 1)
		assert.Equal(t, map[string]any{"service.name": "rd-none-set"}, got["rd-none-set"])
	})
}
