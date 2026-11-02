// Copyright Dynatrace LLC
// SPDX-License-Identifier: Apache-2.0

package zipkin

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/oteltest"
	"github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/testutil"
	"github.com/openzipkin/zipkin-go/model"
	"github.com/openzipkin/zipkin-go/proto/zipkin_proto3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// startCollector starts the DT Collector with config_examples/zipkin.yaml, adapted to run locally.
// It returns the port of the Zipkin receiver and the sink the Collector exports to.
func startCollector(t *testing.T) (int, *consumertest.TracesSink) {
	sink, sinkPorts := oteltest.StartTracesSink(t)
	zipkinPort := testutil.GetAvailablePort(t)
	healthPort := testutil.GetAvailablePort(t)

	cfg := testutil.LoadConfigExample(t, "zipkin.yaml").
		Replace("zipkin receiver port", func(s string) string { return testutil.ReplaceZipkinReceiverPort(s, zipkinPort) }).
		Replace("health_check port", func(s string) string { return testutil.ReplaceHealthCheckPort(s, healthPort) }).
		Replace("exporter endpoint", func(s string) string { return testutil.ReplaceDynatraceExporterEndpoint(s, sinkPorts.Http) }).
		Replace("sending_queue and retry_on_failure", func(s string) string { return testutil.DisableSendingQueueAndRetry(s, "otlp_http") }).
		Replace("internal metrics", testutil.DisableInternalMetrics)

	testutil.StartCollector(t, testutil.CollectorParams{
		Name:      "zipkin",
		Config:    cfg.String(),
		ReadyPort: zipkinPort,
	})
	return zipkinPort, sink
}

var (
	baseTime = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

	frontendServerID = model.ID(1)
	frontendClientID = model.ID(2)
	backendServerID  = model.ID(3)
)

// testSpans returns a trace across two services: frontend (server and client span) and backend
// (server span, failed).
func testSpans(traceID model.TraceID) []*model.SpanModel {
	frontend := func(port uint16) *model.Endpoint {
		return &model.Endpoint{ServiceName: "frontend", IPv4: net.ParseIP("10.0.0.1"), Port: port}
	}
	backend := &model.Endpoint{ServiceName: "backend", IPv4: net.ParseIP("10.0.0.2"), Port: 9000}
	parent := func(id model.ID) *model.ID { return &id }

	return []*model.SpanModel{
		{
			TraceID:       traceID,
			ID:            frontendServerID,
			Name:          "get /",
			Kind:          model.Server,
			Timestamp:     baseTime,
			Duration:      10 * time.Millisecond,
			LocalEndpoint: frontend(8081),
			Annotations:   []model.Annotation{{Timestamp: baseTime.Add(9 * time.Millisecond), Value: "wr"}},
			Tags:          map[string]string{"http.method": "GET", "http.path": "/"},
		},
		{
			TraceID:        traceID,
			ID:             frontendClientID,
			ParentID:       parent(frontendServerID),
			Name:           "get /api",
			Kind:           model.Client,
			Timestamp:      baseTime.Add(time.Millisecond),
			Duration:       7 * time.Millisecond,
			LocalEndpoint:  frontend(0),
			RemoteEndpoint: backend,
			Tags:           map[string]string{"http.method": "GET", "http.path": "/api"},
		},
		{
			TraceID:       traceID,
			ID:            backendServerID,
			ParentID:      parent(frontendClientID),
			Name:          "get /api",
			Kind:          model.Server,
			Timestamp:     baseTime.Add(2 * time.Millisecond),
			Duration:      5 * time.Millisecond,
			LocalEndpoint: backend,
			Tags:          map[string]string{"http.method": "GET", "http.path": "/api", "error": "boom"},
		},
	}
}

func pcommonTraceID(id model.TraceID) pcommon.TraceID {
	var b [16]byte
	binary.BigEndian.PutUint64(b[:8], id.High)
	binary.BigEndian.PutUint64(b[8:], id.Low)
	return b
}

func pcommonSpanID(id model.ID) pcommon.SpanID {
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], uint64(id))
	return b
}

func post(t *testing.T, port int, contentType string, body []byte) int {
	resp, err := http.Post(fmt.Sprintf("http://localhost:%d/api/v2/spans", port), contentType, bytes.NewReader(body))
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	return resp.StatusCode
}

// TestE2E_ZipkinReceiver tests the "Ingest data from Zipkin" use case
// See: https://docs.dynatrace.com/docs/shortlink/otel-collector-cases-zipkin
// The config under test is config_examples/zipkin.yaml.
func TestE2E_ZipkinReceiver(t *testing.T) {
	port, sink := startCollector(t)

	jsonBody := func(spans []*model.SpanModel) []byte {
		b, err := json.Marshal(spans)
		require.NoError(t, err)
		return b
	}
	protoBody := func(spans []*model.SpanModel) []byte {
		b, err := zipkin_proto3.SpanSerializer{}.Serialize(spans)
		require.NoError(t, err)
		return b
	}

	encodings := []struct {
		name        string
		contentType string
		encode      func([]*model.SpanModel) []byte
	}{
		{"json", "application/json", jsonBody},
		{"protobuf", "application/x-protobuf", protoBody},
	}
	for _, enc := range encodings {
		t.Run(enc.name, func(t *testing.T) {
			sink.Reset()
			traceID := model.TraceID{High: 0x0102030405060708, Low: 0x090a0b0c0d0e0f10}

			assert.Equal(t, http.StatusAccepted, post(t, port, enc.contentType, enc.encode(testSpans(traceID))))
			require.Eventually(t, func() bool { return sink.SpanCount() == 3 }, 10*time.Second, 50*time.Millisecond)

			assertSpans(t, sink.AllTraces(), pcommonTraceID(traceID))
		})
	}

	t.Run("invalid payload", func(t *testing.T) {
		sink.Reset()
		assert.Equal(t, http.StatusBadRequest, post(t, port, "application/json", []byte("{not json")))
		assert.Equal(t, 0, sink.SpanCount())
	})
}

func assertSpans(t *testing.T, traces []ptrace.Traces, traceID pcommon.TraceID) {
	// Resource attributes per service, and the spans received for each.
	resources := map[string][]ptrace.Span{}
	for _, td := range traces {
		for i := range td.ResourceSpans().Len() {
			rs := td.ResourceSpans().At(i)
			assert.Equal(t, 1, rs.Resource().Attributes().Len(), "only service.name is expected on the resource")
			name, ok := rs.Resource().Attributes().Get("service.name")
			require.True(t, ok)
			for j := range rs.ScopeSpans().Len() {
				ss := rs.ScopeSpans().At(j)
				for k := range ss.Spans().Len() {
					resources[name.Str()] = append(resources[name.Str()], ss.Spans().At(k))
				}
			}
		}
	}
	require.Len(t, resources, 2)
	require.Len(t, resources["frontend"], 2)
	require.Len(t, resources["backend"], 1)

	spans := map[pcommon.SpanID]ptrace.Span{}
	for _, ss := range resources {
		for _, s := range ss {
			spans[s.SpanID()] = s
		}
	}

	spanID := pcommonSpanID
	ts := func(d time.Duration) pcommon.Timestamp { return pcommon.NewTimestampFromTime(baseTime.Add(d)) }

	// The network attribute names depend on the pkg.translator.zipkin.EmitV1NetworkConventions
	// and pkg.translator.zipkin.DontEmitV0NetworkConventions feature gates. Exact maps are
	// compared on purpose: when a gate graduates the attributes customers get change, and this
	// test should flag it on the upgrade PR.
	expected := []struct {
		id        model.ID
		parent    pcommon.SpanID
		name      string
		kind      ptrace.SpanKind
		start     pcommon.Timestamp
		end       pcommon.Timestamp
		status    ptrace.StatusCode
		attrs     map[string]any
		eventName string
		eventTime pcommon.Timestamp
	}{
		{
			id: frontendServerID, name: "get /", kind: ptrace.SpanKindServer,
			start: ts(0), end: ts(10 * time.Millisecond), status: ptrace.StatusCodeUnset,
			attrs: map[string]any{
				"http.method":           "GET",
				"http.path":             "/",
				"network.local.address": "10.0.0.1",
				"net.host.port":         int64(8081),
			},
			eventName: "wr", eventTime: ts(9 * time.Millisecond),
		},
		{
			id: frontendClientID, parent: spanID(frontendServerID), name: "get /api", kind: ptrace.SpanKindClient,
			start: ts(time.Millisecond), end: ts(8 * time.Millisecond), status: ptrace.StatusCodeUnset,
			attrs: map[string]any{
				"http.method":           "GET",
				"http.path":             "/api",
				"network.local.address": "10.0.0.1",
				"service.peer.name":     "backend",
				"network.peer.address":  "10.0.0.2",
				"net.peer.port":         int64(9000),
			},
		},
		{
			id: backendServerID, parent: spanID(frontendClientID), name: "get /api", kind: ptrace.SpanKindServer,
			start: ts(2 * time.Millisecond), end: ts(7 * time.Millisecond), status: ptrace.StatusCodeError,
			attrs: map[string]any{
				"http.method":           "GET",
				"http.path":             "/api",
				"network.local.address": "10.0.0.2",
				"net.host.port":         int64(9000),
				// The tag is kept because its value isn't "true".
				"error": "boom",
			},
		},
	}

	for _, e := range expected {
		s, ok := spans[spanID(e.id)]
		require.Truef(t, ok, "span %v not received", e.id)
		assert.Equal(t, traceID, s.TraceID())
		assert.Equal(t, e.parent, s.ParentSpanID())
		assert.Equal(t, e.name, s.Name())
		assert.Equal(t, e.kind, s.Kind())
		assert.Equal(t, e.start, s.StartTimestamp())
		assert.Equal(t, e.end, s.EndTimestamp())
		assert.Equal(t, e.status, s.Status().Code())
		assert.Equal(t, e.attrs, s.Attributes().AsRaw())
		if e.eventName == "" {
			assert.Equal(t, 0, s.Events().Len())
			continue
		}
		require.Equal(t, 1, s.Events().Len())
		assert.Equal(t, e.eventName, s.Events().At(0).Name())
		assert.Equal(t, e.eventTime, s.Events().At(0).Timestamp())
	}
}
