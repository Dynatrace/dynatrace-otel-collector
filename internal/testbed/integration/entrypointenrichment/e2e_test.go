// Copyright Dynatrace LLC
// SPDX-License-Identifier: Apache-2.0

package entrypointenrichment

import (
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	oteltest "github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/oteltest"
	"github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// collectorExecPath is relative to this test file's directory.
const collectorExecPath = "../../../../bin/dynatrace-otel-collector"

func TestE2E_EntrypointEnrichment(t *testing.T) {
	receiverPort := testutil.GetAvailablePort(t)
	sinkPort := testutil.GetAvailablePort(t)
	sinkHTTPPort := testutil.GetAvailablePort(t)

	// Set up the output sink (OTLP receiver that collects spans for assertions).
	tracesSink := new(consumertest.TracesSink)
	shutdownSinks := oteltest.StartUpSinks(t, oteltest.ReceiverSinks{
		Traces: []*oteltest.TraceSinkConfig{
			{
				Consumer: tracesSink,
				Ports: &oteltest.ReceiverPorts{
					Grpc: sinkPort,
					Http: sinkHTTPPort,
				},
			},
		},
	})
	defer shutdownSinks()

	// Build the inline collector config.
	cfg := fmt.Sprintf(`
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:%d

processors:
  entrypoint_enrichment:
    wait_duration: 100ms
    fallback_duration: 2s
    num_traces: 1000000
    local_root_detection: flags_with_kind_fallback
    attributes_to_promote:
      - "^dt\\.feature_flag\\.result\\..+$"
    local_root_marker_attribute: "dt.local_root"

exporters:
  otlp:
    endpoint: localhost:%d
    tls:
      insecure: true

service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [entrypoint_enrichment]
      exporters: [otlp]
`, receiverPort, sinkPort)

	// Optionally inject DT exporter (uncomment to send to Dynatrace):
	// var err error
	// cfg, err = applyDynatraceExporter(cfg)
	// require.NoError(t, err)

	col := testbed.NewChildProcessCollector(testbed.WithAgentExePath(collectorExecPath))
	cleanup, err := col.PrepareConfig(t, cfg)
	require.NoError(t, err)
	t.Cleanup(cleanup)

	err = col.Start(testbed.StartParams{
		Name:        "dynatrace-otel-collector",
		LogFilePath: t.TempDir() + "/col.log",
	})
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = col.Stop() })

	// Wait for the collector's OTLP gRPC receiver to be ready.
	require.Eventually(t, func() bool {
		conn, err := net.DialTimeout("tcp", "localhost:"+strconv.Itoa(receiverPort), time.Second)
		if err == nil {
			conn.Close()
			return true
		}
		return false
	}, 15*time.Second, 100*time.Millisecond, "collector OTLP receiver did not become ready")

	// Send traces via gRPC OTLP.
	conn, err := grpc.NewClient(
		"localhost:"+strconv.Itoa(receiverPort),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	defer conn.Close()

	client := ptraceotlp.NewGRPCClient(conn)
	_, err = client.Export(context.Background(), ptraceotlp.NewExportRequestFromTraces(buildDistributedTrace()))
	require.NoError(t, err)

	// Wait for all spans to arrive at the sink.
	// 4 (service-a) + 5 (service-b) + 3 (service-c) = 12 spans total.
	expectedSpanCount := 12
	require.Eventually(t, func() bool {
		return tracesSink.SpanCount() >= expectedSpanCount
	}, 10*time.Second, 100*time.Millisecond,
		"expected %d spans, got %d", expectedSpanCount, tracesSink.SpanCount())

	// Assert each service's local root has its FF attributes promoted.
	assertServiceRoot(t, tracesSink, "service-a", map[string]string{
		"dt.feature_flag.result.pricing_v2": "on",
		"dt.local_root":                     "true",
	})
	assertServiceRoot(t, tracesSink, "service-b", map[string]string{
		"dt.feature_flag.result.checkout_flow": "variant_b",
		"dt.feature_flag.result.recs_algo":     "collab",
		"dt.local_root":                        "true",
	})
	assertServiceRoot(t, tracesSink, "service-c", map[string]string{
		"dt.feature_flag.result.shipping_calc": "fast",
		"dt.local_root":                        "true",
	})

	// Assert no cross-contamination: service-a root should NOT have service-b's flags.
	assertServiceRootLacks(t, tracesSink, "service-a", []string{
		"dt.feature_flag.result.checkout_flow",
		"dt.feature_flag.result.recs_algo",
	})
}

// buildDistributedTrace constructs a 3-service distributed trace with feature-flag
// attributes on descendant spans that should be promoted to each service's local root.
func buildDistributedTrace() ptrace.Traces {
	td := ptrace.NewTraces()
	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})

	// ---- Service A ----
	// SERVER root (empty parent → always local root)
	// 2 INTERNAL children (one with FF attr)
	// 1 CLIENT hop to B
	aRootID := pcommon.SpanID([8]byte{1})
	aInternalFFID := pcommon.SpanID([8]byte{2})
	aInternalID := pcommon.SpanID([8]byte{3})
	aHopID := pcommon.SpanID([8]byte{4})

	rsA := td.ResourceSpans().AppendEmpty()
	rsA.Resource().Attributes().PutStr("service.name", "service-a")
	ssA := rsA.ScopeSpans().AppendEmpty()

	aRoot := ssA.Spans().AppendEmpty()
	aRoot.SetTraceID(traceID)
	aRoot.SetSpanID(aRootID)
	aRoot.SetKind(ptrace.SpanKindServer)
	// empty parent → always local root

	aInternalFF := ssA.Spans().AppendEmpty()
	aInternalFF.SetTraceID(traceID)
	aInternalFF.SetSpanID(aInternalFFID)
	aInternalFF.SetParentSpanID(aRootID)
	aInternalFF.SetKind(ptrace.SpanKindInternal)
	aInternalFF.SetFlags(0x100) // HAS_IS_REMOTE=1, IS_REMOTE=0 → local child
	aInternalFF.Attributes().PutStr("dt.feature_flag.result.pricing_v2", "on")

	aInternal := ssA.Spans().AppendEmpty()
	aInternal.SetTraceID(traceID)
	aInternal.SetSpanID(aInternalID)
	aInternal.SetParentSpanID(aRootID)
	aInternal.SetKind(ptrace.SpanKindInternal)
	aInternal.SetFlags(0x100)

	aHop := ssA.Spans().AppendEmpty()
	aHop.SetTraceID(traceID)
	aHop.SetSpanID(aHopID)
	aHop.SetParentSpanID(aRootID)
	aHop.SetKind(ptrace.SpanKindClient)
	aHop.SetFlags(0x100)

	// ---- Service B ----
	// SERVER root (remote parent = A's CLIENT hop, flags=0x300 → local root)
	// 3 INTERNAL children (2 with FF attrs)
	// 1 CLIENT hop to C
	bRootID := pcommon.SpanID([8]byte{10})
	bFF1ID := pcommon.SpanID([8]byte{11})
	bFF2ID := pcommon.SpanID([8]byte{12})
	bInternalID := pcommon.SpanID([8]byte{13})
	bHopID := pcommon.SpanID([8]byte{14})

	rsB := td.ResourceSpans().AppendEmpty()
	rsB.Resource().Attributes().PutStr("service.name", "service-b")
	ssB := rsB.ScopeSpans().AppendEmpty()

	bRoot := ssB.Spans().AppendEmpty()
	bRoot.SetTraceID(traceID)
	bRoot.SetSpanID(bRootID)
	bRoot.SetParentSpanID(aHopID)
	bRoot.SetKind(ptrace.SpanKindServer)
	bRoot.SetFlags(0x300) // HAS_IS_REMOTE=1, IS_REMOTE=1 → remote parent → local root

	bFF1 := ssB.Spans().AppendEmpty()
	bFF1.SetTraceID(traceID)
	bFF1.SetSpanID(bFF1ID)
	bFF1.SetParentSpanID(bRootID)
	bFF1.SetKind(ptrace.SpanKindInternal)
	bFF1.SetFlags(0x100)
	bFF1.Attributes().PutStr("dt.feature_flag.result.checkout_flow", "variant_b")

	bFF2 := ssB.Spans().AppendEmpty()
	bFF2.SetTraceID(traceID)
	bFF2.SetSpanID(bFF2ID)
	bFF2.SetParentSpanID(bRootID)
	bFF2.SetKind(ptrace.SpanKindInternal)
	bFF2.SetFlags(0x100)
	bFF2.Attributes().PutStr("dt.feature_flag.result.recs_algo", "collab")

	bInternal := ssB.Spans().AppendEmpty()
	bInternal.SetTraceID(traceID)
	bInternal.SetSpanID(bInternalID)
	bInternal.SetParentSpanID(bRootID)
	bInternal.SetKind(ptrace.SpanKindInternal)
	bInternal.SetFlags(0x100)

	bHop := ssB.Spans().AppendEmpty()
	bHop.SetTraceID(traceID)
	bHop.SetSpanID(bHopID)
	bHop.SetParentSpanID(bRootID)
	bHop.SetKind(ptrace.SpanKindClient)
	bHop.SetFlags(0x100)

	// ---- Service C ----
	// SERVER root (remote parent = B's CLIENT hop, flags=0x300 → local root)
	// 2 INTERNAL children (one with FF attr)
	cRootID := pcommon.SpanID([8]byte{20})
	cFFID := pcommon.SpanID([8]byte{21})
	cInternalID := pcommon.SpanID([8]byte{22})

	rsC := td.ResourceSpans().AppendEmpty()
	rsC.Resource().Attributes().PutStr("service.name", "service-c")
	ssC := rsC.ScopeSpans().AppendEmpty()

	cRoot := ssC.Spans().AppendEmpty()
	cRoot.SetTraceID(traceID)
	cRoot.SetSpanID(cRootID)
	cRoot.SetParentSpanID(bHopID)
	cRoot.SetKind(ptrace.SpanKindServer)
	cRoot.SetFlags(0x300)

	cFF := ssC.Spans().AppendEmpty()
	cFF.SetTraceID(traceID)
	cFF.SetSpanID(cFFID)
	cFF.SetParentSpanID(cRootID)
	cFF.SetKind(ptrace.SpanKindInternal)
	cFF.SetFlags(0x100)
	cFF.Attributes().PutStr("dt.feature_flag.result.shipping_calc", "fast")

	cInternal := ssC.Spans().AppendEmpty()
	cInternal.SetTraceID(traceID)
	cInternal.SetSpanID(cInternalID)
	cInternal.SetParentSpanID(cRootID)
	cInternal.SetKind(ptrace.SpanKindInternal)
	cInternal.SetFlags(0x100)

	return td
}

// assertServiceRoot finds the SERVER span for the given service and asserts
// that it has all expectedAttrs.
func assertServiceRoot(t *testing.T, sink *consumertest.TracesSink, serviceName string, expectedAttrs map[string]string) {
	t.Helper()
	for _, traces := range sink.AllTraces() {
		for i := 0; i < traces.ResourceSpans().Len(); i++ {
			rs := traces.ResourceSpans().At(i)
			svcName, ok := rs.Resource().Attributes().Get("service.name")
			if !ok || svcName.AsString() != serviceName {
				continue
			}
			for j := 0; j < rs.ScopeSpans().Len(); j++ {
				for k := 0; k < rs.ScopeSpans().At(j).Spans().Len(); k++ {
					span := rs.ScopeSpans().At(j).Spans().At(k)
					if span.Kind() != ptrace.SpanKindServer {
						continue
					}
					for key, wantVal := range expectedAttrs {
						v, exists := span.Attributes().Get(key)
						assert.True(t, exists, "service %s root missing attribute %s", serviceName, key)
						if exists {
							assert.Equal(t, wantVal, v.AsString(), "service %s attr %s", serviceName, key)
						}
					}
					return
				}
			}
		}
	}
	t.Errorf("no SERVER span found for service %s", serviceName)
}

// assertServiceRootLacks finds the SERVER span for the given service and asserts
// that none of the forbiddenKeys are present.
func assertServiceRootLacks(t *testing.T, sink *consumertest.TracesSink, serviceName string, forbiddenKeys []string) {
	t.Helper()
	for _, traces := range sink.AllTraces() {
		for i := 0; i < traces.ResourceSpans().Len(); i++ {
			rs := traces.ResourceSpans().At(i)
			svcName, ok := rs.Resource().Attributes().Get("service.name")
			if !ok || svcName.AsString() != serviceName {
				continue
			}
			for j := 0; j < rs.ScopeSpans().Len(); j++ {
				for k := 0; k < rs.ScopeSpans().At(j).Spans().Len(); k++ {
					span := rs.ScopeSpans().At(j).Spans().At(k)
					if span.Kind() != ptrace.SpanKindServer {
						continue
					}
					for _, key := range forbiddenKeys {
						_, exists := span.Attributes().Get(key)
						assert.False(t, exists, "service %s root should NOT have attribute %s", serviceName, key)
					}
					return
				}
			}
		}
	}
}

// applyDynatraceExporter injects an otlphttp exporter targeting the Dynatrace
// endpoint into the collector config string and adds it to the traces pipeline.
// Requires DT_ENDPOINT and DT_API_TOKEN to be set in the environment; the
// spawned collector inherits the test process's environment so ${env:...}
// references resolve correctly at runtime.
//
//nolint:unused
func applyDynatraceExporter(cfg string) (string, error) {
	if os.Getenv("DT_ENDPOINT") == "" || os.Getenv("DT_API_TOKEN") == "" {
		return "", fmt.Errorf("DT_ENDPOINT and DT_API_TOKEN must both be set")
	}
	dtExporterBlock := `
  otlphttp/dynatrace:
    endpoint: ${env:DT_ENDPOINT}
    headers:
      Authorization: "Api-Token ${env:DT_API_TOKEN}"
`
	cfg = strings.Replace(cfg, "\nexporters:", "\nexporters:"+dtExporterBlock, 1)
	cfg = strings.Replace(cfg, "exporters: [otlp]", "exporters: [otlp, otlphttp/dynatrace]", 1)
	return cfg, nil
}
