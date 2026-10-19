// Copyright Dynatrace LLC
// SPDX-License-Identifier: Apache-2.0

//go:build e2e

package featureflags

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/oteltest"
	"github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	collectorExecPath = "../../../../bin/dynatrace-otel-collector"
	configPath        = "../../../../config_examples/feature_flag_enrichment.yaml"
	featureGatesArg   = "--feature-gates=service.profilesSupport,ottl.functions.enableLambda"
	testWaitDuration  = "1s"
)

// prepareConfig reads the example config and adapts it for the test. Each replacement is
// required to change the config so that edits to the example can't silently break the test.
func prepareConfig(t *testing.T, receiverPort, exporterPort int) string {
	raw, err := os.ReadFile(configPath)
	require.NoError(t, err)
	cfg := string(raw)

	replace := func(name string, f func(string) string) {
		out := f(cfg)
		require.NotEqual(t, cfg, out, "config replacement %q had no effect", name)
		cfg = out
	}
	replace("receiver port", func(s string) string { return testutil.ReplaceOtlpGrpcReceiverPort(s, receiverPort) })
	replace("exporter endpoint", func(s string) string { return testutil.ReplaceDynatraceExporterEndpoint(s, exporterPort) })
	replace("api token", func(s string) string { return strings.Replace(s, "${env:DT_API_TOKEN}", "", 1) })
	replace("wait_duration", func(s string) string {
		return strings.Replace(s, "wait_duration: 60s", "wait_duration: "+testWaitDuration, 1)
	})
	replace("sending_queue", func(s string) string {
		return strings.Replace(s, "  otlp_http:\n    endpoint:", "  otlp_http:\n    sending_queue:\n      enabled: false\n    endpoint:", 1)
	})
	// The Collector's internal metrics server binds the fixed port 8888, which would clash with
	// other Collectors on the machine (or a leftover from an aborted run).
	replace("internal telemetry", func(s string) string {
		return strings.Replace(s, "service:\n  pipelines:", "service:\n  telemetry:\n    metrics:\n      level: none\n  pipelines:", 1)
	})
	return cfg
}

func startCollector(t *testing.T, cfg string, receiverPort int) {
	col := testbed.NewChildProcessCollector(testbed.WithAgentExePath(collectorExecPath))
	cleanupCfg, err := col.PrepareConfig(t, cfg)
	require.NoError(t, err)
	t.Cleanup(cleanupCfg)

	logPath := filepath.Join(t.TempDir(), "collector.log")
	require.NoError(t, col.Start(testbed.StartParams{
		Name:        "feature-flags",
		LogFilePath: logPath,
		CmdArgs:     []string{featureGatesArg},
	}))
	t.Cleanup(func() {
		_, err := col.Stop()
		assert.NoError(t, err)
		if t.Failed() {
			if b, err := os.ReadFile(logPath); err == nil {
				t.Logf("collector log:\n%s", b)
			}
		}
	})

	addr := net.JoinHostPort("localhost", strconv.Itoa(receiverPort))
	require.Eventually(t, func() bool {
		conn, err := net.DialTimeout("tcp", addr, time.Second)
		if err != nil {
			return false
		}
		_ = conn.Close()
		return true
	}, 30*time.Second, 200*time.Millisecond, "collector did not start listening on %s", addr)
}

type spanKey struct {
	trace [16]byte
	span  [8]byte
}

func totalSpans(all []ptrace.Traces) int {
	n := 0
	for _, td := range all {
		n += td.SpanCount()
	}
	return n
}

// indexSpans indexes all spans of the payloads by (trace ID, span ID).
func indexSpans(payloads []ptrace.Traces) map[spanKey]ptrace.Span {
	out := map[spanKey]ptrace.Span{}
	for _, td := range payloads {
		for _, rs := range td.ResourceSpans().All() {
			for _, ss := range rs.ScopeSpans().All() {
				for _, sp := range ss.Spans().All() {
					out[spanKey{sp.TraceID(), sp.SpanID()}] = sp
				}
			}
		}
	}
	return out
}

func spanNames(td ptrace.Traces) []string {
	var names []string
	for _, rs := range td.ResourceSpans().All() {
		svc, _ := rs.Resource().Attributes().Get("service.name")
		for _, ss := range rs.ScopeSpans().All() {
			for _, sp := range ss.Spans().All() {
				names = append(names, svc.Str()+"/"+sp.Name())
			}
		}
	}
	return names
}

func TestFeatureFlagEnrichment(t *testing.T) {
	topos := topologies()

	for modeIdx, mode := range payloadModes() {
		t.Run(mode.name, func(t *testing.T) {
			runMode(t, modeIdx, mode, topos)
		})
	}
}

func runMode(t *testing.T, modeIdx int, mode payloadMode, topos []topology) {
	receiverPort := testutil.GetAvailablePort(t)
	exporterPort := testutil.GetAvailablePort(t)
	sinkGrpcPort := testutil.GetAvailablePort(t)

	sink := new(consumertest.TracesSink)
	shutdown := oteltest.StartUpSinks(t, oteltest.ReceiverSinks{
		Traces: []*oteltest.TraceSinkConfig{{
			Consumer: sink,
			Ports:    &oteltest.ReceiverPorts{Grpc: sinkGrpcPort, Http: exporterPort},
		}},
	})
	t.Cleanup(shutdown)

	startCollector(t, prepareConfig(t, receiverPort, exporterPort), receiverPort)

	conn, err := grpc.NewClient(fmt.Sprintf("localhost:%d", receiverPort), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	client := ptraceotlp.NewGRPCClient(conn)

	var sent []ptrace.Traces
	for _, items := range mode.split(topos) {
		sent = append(sent, buildTraces(modeIdx, items))
	}
	sentCount := totalSpans(sent)
	for _, td := range sent {
		_, err := client.Export(context.Background(), ptraceotlp.NewExportRequestFromTraces(td))
		require.NoError(t, err)
	}

	require.Eventually(t, func() bool {
		return totalSpans(sink.AllTraces()) >= sentCount
	}, 60*time.Second, 500*time.Millisecond, "expected %d spans, got %d", sentCount, totalSpans(sink.AllTraces()))

	received := sink.AllTraces()

	t.Run("grouping", func(t *testing.T) { checkGrouping(t, modeIdx, topos, received) })
	t.Run("enrichment", func(t *testing.T) { checkEnrichment(t, modeIdx, topos, sent, received) })
}

func checkGrouping(t *testing.T, modeIdx int, topos []topology, received []ptrace.Traces) {
	wantCalls := 0
	for _, topo := range topos {
		wantCalls += len(topo.calls)
	}
	require.Len(t, received, wantCalls)

	matched := make([][]bool, len(topos))
	for i, topo := range topos {
		matched[i] = make([]bool, len(topo.calls))
	}

	for _, td := range received {
		names := spanNames(td)
		var (
			traceIDs = map[[16]byte]struct{}{}
			services = map[string]struct{}{}
			ids      []byte
			topoIdxs = map[int]struct{}{}
		)
		for _, rs := range td.ResourceSpans().All() {
			svc, _ := rs.Resource().Attributes().Get("service.name")
			services[svc.Str()] = struct{}{}
			for _, ss := range rs.ScopeSpans().All() {
				for _, sp := range ss.Spans().All() {
					traceIDs[sp.TraceID()] = struct{}{}
					sid := sp.SpanID()
					ids = append(ids, sid[7])
					topoIdxs[int(sid[6])-1] = struct{}{}
				}
			}
		}
		if !assert.Len(t, traceIDs, 1, "payload spans more than one trace: %v", names) ||
			!assert.Len(t, services, 1, "payload spans more than one service: %v", names) ||
			!assert.Len(t, topoIdxs, 1, "payload spans more than one topology: %v", names) {
			continue
		}
		var topoIdx int
		for k := range topoIdxs {
			topoIdx = k
		}
		for tid := range traceIDs {
			assert.Equal(t, [16]byte(traceID(modeIdx, topoIdx)), tid, "unexpected trace ID for %v", names)
		}
		slices.Sort(ids)
		found := false
		for ci, call := range topos[topoIdx].calls {
			want := slices.Clone(call)
			slices.Sort(want)
			if !matched[topoIdx][ci] && slices.Equal(ids, want) {
				matched[topoIdx][ci] = true
				found = true
				break
			}
		}
		assert.True(t, found, "payload in topology %s matches no expected call (span ids %v): %v", topos[topoIdx].name, ids, names)
	}

	for ti, topo := range topos {
		for ci, ok := range matched[ti] {
			assert.True(t, ok, "expected call %v of topology %s was never received", topo.calls[ci], topo.name)
		}
	}
}

func checkEnrichment(t *testing.T, modeIdx int, topos []topology, sent, received []ptrace.Traces) {
	sentIdx := indexSpans(sent)
	gotIdx := indexSpans(received)

	for ti, topo := range topos {
		t.Run(topo.name, func(t *testing.T) {
			for _, s := range topo.spans {
				k := spanKey{traceID(modeIdx, ti), spanID(ti, s.id)}
				got, ok := gotIdx[k]
				require.True(t, ok, "span %q (id %d) not received", s.name, s.id)
				orig, ok := sentIdx[k]
				require.True(t, ok)

				want := map[string]any{"test.span.name": s.name}
				for key, v := range s.wantFF {
					want[ffAttrPrefix+key] = v
				}
				assert.Equal(t, want, got.Attributes().AsRaw(), "attributes of span %q (id %d)", s.name, s.id)

				assert.Equal(t, orig.Kind(), got.Kind(), "kind of span %q", s.name)
				assert.Equal(t, orig.ParentSpanID(), got.ParentSpanID(), "parent of span %q", s.name)

				if assert.Equal(t, orig.Events().Len(), got.Events().Len(), "event count of span %q", s.name) {
					for i := range orig.Events().Len() {
						assert.Equal(t, orig.Events().At(i).Name(), got.Events().At(i).Name(), "event name of span %q", s.name)
						assert.Equal(t, orig.Events().At(i).Attributes().AsRaw(), got.Events().At(i).Attributes().AsRaw(),
							"event attributes of span %q", s.name)
					}
				}
			}
		})
	}
}
