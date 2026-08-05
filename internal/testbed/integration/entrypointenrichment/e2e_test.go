// Copyright Dynatrace LLC
// SPDX-License-Identifier: Apache-2.0

package entrypointenrichment

import (
	"context"
	"crypto/rand"
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
  otlp_grpc:
    endpoint: localhost:%d
    tls:
      insecure: true

service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [entrypoint_enrichment]
      exporters: [otlp_grpc]
`, receiverPort, sinkPort)

	// Optionally inject DT exporter (uncomment to send to Dynatrace):
	var err error
	cfg, err = applyDynatraceExporter(cfg)
	require.NoError(t, err)

	col := testbed.NewChildProcessCollector(testbed.WithAgentExePath(collectorExecPath))
	cleanup, err := col.PrepareConfig(t, cfg)
	require.NoError(t, err)
	t.Cleanup(cleanup)

	logPath := t.TempDir() + "/col.log"
	err = col.Start(testbed.StartParams{
		Name:        "dynatrace-otel-collector",
		LogFilePath: logPath,
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
	assertServiceRoot(t, tracesSink, "checkout-service", map[string]string{
		"dt.feature_flag.result.pricing_v2": "on",
		"dt.local_root":                     "true",
	})
	assertServiceRoot(t, tracesSink, "order-service", map[string]string{
		"dt.feature_flag.result.checkout_flow": "variant_b",
		"dt.feature_flag.result.recs_algo":     "collab",
		"dt.local_root":                        "true",
	})
	assertServiceRoot(t, tracesSink, "shipping-service", map[string]string{
		"dt.feature_flag.result.shipping_calc": "fast",
		"dt.local_root":                        "true",
	})

	// Assert no cross-contamination: checkout-service root should NOT have order-service's flags.
	assertServiceRootLacks(t, tracesSink, "checkout-service", []string{
		"dt.feature_flag.result.checkout_flow",
		"dt.feature_flag.result.recs_algo",
	})

	// b, err := os.Open(logPath)
	// require.NoError(t, err)
	// defer b.Close()
	// logContents, err := io.ReadAll(b)
	// require.NoError(t, err)
	// t.Logf("collector log:\n%s", string(logContents))
}

func newTraceID() pcommon.TraceID {
	var id pcommon.TraceID
	_, _ = rand.Read(id[:])
	return id
}

func newSpanID() pcommon.SpanID {
	var id pcommon.SpanID
	_, _ = rand.Read(id[:])
	return id
}

// buildDistributedTrace constructs a 3-service distributed trace simulating an
// HTTP checkout → order → shipping call chain, with feature-flag attributes on
// descendant spans that should be promoted to each service's local root.
//
// Timeline (ms from t0):
//
//	Service A (checkout-service):  0–42 ms  POST /api/v1/checkout
//	  price.calculate              2–7 ms   (ff: pricing_v2=on)
//	  cart.validate                7–10 ms
//	  POST order-service           10–40 ms (CLIENT)
//	Service B (order-service):     12–38 ms POST /api/v1/orders
//	  checkout.process             14–19 ms (ff: checkout_flow=variant_b)
//	  recommendations.apply        19–24 ms (ff: recs_algo=collab)
//	  inventory.check              24–28 ms
//	  POST shipping-service        28–36 ms (CLIENT)
//	Service C (shipping-service):  30–34 ms POST /api/v1/shipments
//	  rate.calculate               31–33 ms (ff: shipping_calc=fast)
//	  address.validate             31–33 ms
func buildDistributedTrace() ptrace.Traces {
	td := ptrace.NewTraces()
	traceID := newTraceID()

	t0 := time.Now()
	ms := func(n int) pcommon.Timestamp {
		return pcommon.NewTimestampFromTime(t0.Add(time.Duration(n) * time.Millisecond))
	}

	// ---- Service A: checkout-service ----
	// Receives the original user request; no parent → always a local root.
	aRootID := newSpanID()
	aCalcID := newSpanID()
	aValidID := newSpanID()
	aHopID := newSpanID()

	rsA := td.ResourceSpans().AppendEmpty()
	rsA.Resource().Attributes().PutStr("service.name", "checkout-service")
	rsA.Resource().Attributes().PutStr("service.version", "1.4.2")
	rsA.Resource().Attributes().PutStr("telemetry.sdk.name", "opentelemetry")
	rsA.Resource().Attributes().PutStr("telemetry.sdk.language", "go")
	rsA.Resource().Attributes().PutStr("deployment.environment.name", "production")
	ssA := rsA.ScopeSpans().AppendEmpty()
	ssA.Scope().SetName("github.com/example/checkout-service")
	ssA.Scope().SetVersion("1.4.2")

	aRoot := ssA.Spans().AppendEmpty()
	aRoot.SetTraceID(traceID)
	aRoot.SetSpanID(aRootID)
	aRoot.SetName("POST /api/v1/checkout")
	aRoot.SetKind(ptrace.SpanKindServer)
	aRoot.SetStartTimestamp(ms(0))
	aRoot.SetEndTimestamp(ms(42))
	aRoot.Status().SetCode(ptrace.StatusCodeOk)
	aRoot.Attributes().PutStr("http.request.method", "POST")
	aRoot.Attributes().PutStr("url.path", "/api/v1/checkout")
	aRoot.Attributes().PutStr("http.route", "/api/v1/checkout")
	aRoot.Attributes().PutInt("http.response.status_code", 200)
	aRoot.Attributes().PutStr("network.protocol.version", "1.1")
	aRoot.Attributes().PutStr("server.address", "checkout-service.internal")
	aRoot.Attributes().PutInt("server.port", 8080)
	aRoot.Attributes().PutStr("client.address", "10.0.1.55")

	// price.calculate evaluates the pricing_v2 feature flag.
	aCalc := ssA.Spans().AppendEmpty()
	aCalc.SetTraceID(traceID)
	aCalc.SetSpanID(aCalcID)
	aCalc.SetParentSpanID(aRootID)
	aCalc.SetName("price.calculate")
	aCalc.SetKind(ptrace.SpanKindInternal)
	aCalc.SetFlags(0x100) // HAS_IS_REMOTE=1, IS_REMOTE=0 → local child
	aCalc.SetStartTimestamp(ms(2))
	aCalc.SetEndTimestamp(ms(7))
	aCalc.Status().SetCode(ptrace.StatusCodeOk)
	aCalc.Attributes().PutStr("price.strategy", "tiered")
	aCalc.Attributes().PutStr("dt.feature_flag.result.pricing_v2", "on")

	// cart.validate checks stock and coupon validity.
	aValid := ssA.Spans().AppendEmpty()
	aValid.SetTraceID(traceID)
	aValid.SetSpanID(aValidID)
	aValid.SetParentSpanID(aRootID)
	aValid.SetName("cart.validate")
	aValid.SetKind(ptrace.SpanKindInternal)
	aValid.SetFlags(0x100)
	aValid.SetStartTimestamp(ms(7))
	aValid.SetEndTimestamp(ms(10))
	aValid.Status().SetCode(ptrace.StatusCodeOk)
	aValid.Attributes().PutInt("cart.item_count", 3)
	aValid.Attributes().PutBool("cart.coupon_applied", true)

	// CLIENT span that carries the trace context to service B.
	aHop := ssA.Spans().AppendEmpty()
	aHop.SetTraceID(traceID)
	aHop.SetSpanID(aHopID)
	aHop.SetParentSpanID(aRootID)
	aHop.SetName("POST order-service")
	aHop.SetKind(ptrace.SpanKindClient)
	aHop.SetFlags(0x100)
	aHop.SetStartTimestamp(ms(10))
	aHop.SetEndTimestamp(ms(40))
	aHop.Status().SetCode(ptrace.StatusCodeOk)
	aHop.Attributes().PutStr("http.request.method", "POST")
	aHop.Attributes().PutStr("url.full", "http://order-service.internal:8081/api/v1/orders")
	aHop.Attributes().PutInt("http.response.status_code", 201)
	aHop.Attributes().PutStr("server.address", "order-service.internal")
	aHop.Attributes().PutInt("server.port", 8081)
	aHop.Attributes().PutStr("network.protocol.version", "1.1")

	// ---- Service B: order-service ----
	// Receives from A via HTTP; flags=0x300 (HAS_IS_REMOTE + IS_REMOTE) → local root.
	bRootID := newSpanID()
	bCheckoutID := newSpanID()
	bRecsID := newSpanID()
	bInventoryID := newSpanID()
	bHopID := newSpanID()

	rsB := td.ResourceSpans().AppendEmpty()
	rsB.Resource().Attributes().PutStr("service.name", "order-service")
	rsB.Resource().Attributes().PutStr("service.version", "2.1.0")
	rsB.Resource().Attributes().PutStr("telemetry.sdk.name", "opentelemetry")
	rsB.Resource().Attributes().PutStr("telemetry.sdk.language", "java")
	rsB.Resource().Attributes().PutStr("deployment.environment.name", "production")
	ssB := rsB.ScopeSpans().AppendEmpty()
	ssB.Scope().SetName("io.example.order-service")
	ssB.Scope().SetVersion("2.1.0")

	bRoot := ssB.Spans().AppendEmpty()
	bRoot.SetTraceID(traceID)
	bRoot.SetSpanID(bRootID)
	bRoot.SetParentSpanID(aHopID)
	bRoot.SetName("POST /api/v1/orders")
	bRoot.SetKind(ptrace.SpanKindServer)
	bRoot.SetFlags(0x300) // HAS_IS_REMOTE=1, IS_REMOTE=1 → remote parent → local root
	bRoot.SetStartTimestamp(ms(12))
	bRoot.SetEndTimestamp(ms(38))
	bRoot.Status().SetCode(ptrace.StatusCodeOk)
	bRoot.Attributes().PutStr("http.request.method", "POST")
	bRoot.Attributes().PutStr("url.path", "/api/v1/orders")
	bRoot.Attributes().PutStr("http.route", "/api/v1/orders")
	bRoot.Attributes().PutInt("http.response.status_code", 201)
	bRoot.Attributes().PutStr("network.protocol.version", "1.1")
	bRoot.Attributes().PutStr("server.address", "order-service.internal")
	bRoot.Attributes().PutInt("server.port", 8081)

	// checkout.process evaluates the checkout_flow feature flag.
	bCheckout := ssB.Spans().AppendEmpty()
	bCheckout.SetTraceID(traceID)
	bCheckout.SetSpanID(bCheckoutID)
	bCheckout.SetParentSpanID(bRootID)
	bCheckout.SetName("checkout.process")
	bCheckout.SetKind(ptrace.SpanKindInternal)
	bCheckout.SetFlags(0x100)
	bCheckout.SetStartTimestamp(ms(14))
	bCheckout.SetEndTimestamp(ms(19))
	bCheckout.Status().SetCode(ptrace.StatusCodeOk)
	bCheckout.Attributes().PutStr("order.id", "ord-8821")
	bCheckout.Attributes().PutStr("dt.feature_flag.result.checkout_flow", "variant_b")

	// recommendations.apply selects the recommendation algorithm.
	bRecs := ssB.Spans().AppendEmpty()
	bRecs.SetTraceID(traceID)
	bRecs.SetSpanID(bRecsID)
	bRecs.SetParentSpanID(bRootID)
	bRecs.SetName("recommendations.apply")
	bRecs.SetKind(ptrace.SpanKindInternal)
	bRecs.SetFlags(0x100)
	bRecs.SetStartTimestamp(ms(19))
	bRecs.SetEndTimestamp(ms(24))
	bRecs.Status().SetCode(ptrace.StatusCodeOk)
	bRecs.Attributes().PutInt("recommendations.count", 5)
	bRecs.Attributes().PutStr("dt.feature_flag.result.recs_algo", "collab")

	// inventory.check verifies stock levels.
	bInventory := ssB.Spans().AppendEmpty()
	bInventory.SetTraceID(traceID)
	bInventory.SetSpanID(bInventoryID)
	bInventory.SetParentSpanID(bRootID)
	bInventory.SetName("inventory.check")
	bInventory.SetKind(ptrace.SpanKindInternal)
	bInventory.SetFlags(0x100)
	bInventory.SetStartTimestamp(ms(24))
	bInventory.SetEndTimestamp(ms(28))
	bInventory.Status().SetCode(ptrace.StatusCodeOk)
	bInventory.Attributes().PutBool("inventory.all_available", true)

	// CLIENT span that carries the trace context to service C.
	bHop := ssB.Spans().AppendEmpty()
	bHop.SetTraceID(traceID)
	bHop.SetSpanID(bHopID)
	bHop.SetParentSpanID(bRootID)
	bHop.SetName("POST shipping-service")
	bHop.SetKind(ptrace.SpanKindClient)
	bHop.SetFlags(0x100)
	bHop.SetStartTimestamp(ms(28))
	bHop.SetEndTimestamp(ms(36))
	bHop.Status().SetCode(ptrace.StatusCodeOk)
	bHop.Attributes().PutStr("http.request.method", "POST")
	bHop.Attributes().PutStr("url.full", "http://shipping-service.internal:8082/api/v1/shipments")
	bHop.Attributes().PutInt("http.response.status_code", 200)
	bHop.Attributes().PutStr("server.address", "shipping-service.internal")
	bHop.Attributes().PutInt("server.port", 8082)
	bHop.Attributes().PutStr("network.protocol.version", "1.1")

	// ---- Service C: shipping-service ----
	// Receives from B via HTTP; flags=0x300 → local root.
	cRootID := newSpanID()
	cRateID := newSpanID()
	cAddrID := newSpanID()

	rsC := td.ResourceSpans().AppendEmpty()
	rsC.Resource().Attributes().PutStr("service.name", "shipping-service")
	rsC.Resource().Attributes().PutStr("service.version", "3.0.1")
	rsC.Resource().Attributes().PutStr("telemetry.sdk.name", "opentelemetry")
	rsC.Resource().Attributes().PutStr("telemetry.sdk.language", "python")
	rsC.Resource().Attributes().PutStr("deployment.environment.name", "production")
	ssC := rsC.ScopeSpans().AppendEmpty()
	ssC.Scope().SetName("example.shipping_service")
	ssC.Scope().SetVersion("3.0.1")

	cRoot := ssC.Spans().AppendEmpty()
	cRoot.SetTraceID(traceID)
	cRoot.SetSpanID(cRootID)
	cRoot.SetParentSpanID(bHopID)
	cRoot.SetName("POST /api/v1/shipments")
	cRoot.SetKind(ptrace.SpanKindServer)
	cRoot.SetFlags(0x300)
	cRoot.SetStartTimestamp(ms(30))
	cRoot.SetEndTimestamp(ms(34))
	cRoot.Status().SetCode(ptrace.StatusCodeOk)
	cRoot.Attributes().PutStr("http.request.method", "POST")
	cRoot.Attributes().PutStr("url.path", "/api/v1/shipments")
	cRoot.Attributes().PutStr("http.route", "/api/v1/shipments")
	cRoot.Attributes().PutInt("http.response.status_code", 200)
	cRoot.Attributes().PutStr("network.protocol.version", "1.1")
	cRoot.Attributes().PutStr("server.address", "shipping-service.internal")
	cRoot.Attributes().PutInt("server.port", 8082)

	// rate.calculate selects the shipping rate via the shipping_calc feature flag.
	cRate := ssC.Spans().AppendEmpty()
	cRate.SetTraceID(traceID)
	cRate.SetSpanID(cRateID)
	cRate.SetParentSpanID(cRootID)
	cRate.SetName("rate.calculate")
	cRate.SetKind(ptrace.SpanKindInternal)
	cRate.SetFlags(0x100)
	cRate.SetStartTimestamp(ms(31))
	cRate.SetEndTimestamp(ms(33))
	cRate.Status().SetCode(ptrace.StatusCodeOk)
	cRate.Attributes().PutStr("shipping.carrier", "fedex")
	cRate.Attributes().PutStr("dt.feature_flag.result.shipping_calc", "fast")

	// address.validate confirms the destination address.
	cAddr := ssC.Spans().AppendEmpty()
	cAddr.SetTraceID(traceID)
	cAddr.SetSpanID(cAddrID)
	cAddr.SetParentSpanID(cRootID)
	cAddr.SetName("address.validate")
	cAddr.SetKind(ptrace.SpanKindInternal)
	cAddr.SetFlags(0x100)
	cAddr.SetStartTimestamp(ms(31))
	cAddr.SetEndTimestamp(ms(33))
	cAddr.Status().SetCode(ptrace.StatusCodeOk)
	cAddr.Attributes().PutStr("address.country", "US")
	cAddr.Attributes().PutBool("address.validated", true)

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
  otlp_http/dynatrace:
    endpoint: ${env:DT_ENDPOINT}
    headers:
      Authorization: "Api-Token ${env:DT_API_TOKEN}"
`
	cfg = strings.Replace(cfg, "\nexporters:", "\nexporters:"+dtExporterBlock, 1)
	cfg = strings.Replace(cfg, "exporters: [otlp_grpc]", "exporters: [otlp_grpc, otlp_http/dynatrace]", 1)
	return cfg, nil
}
