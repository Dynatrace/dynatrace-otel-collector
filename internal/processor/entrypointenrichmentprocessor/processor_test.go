// Copyright Dynatrace LLC
// SPDX-License-Identifier: Apache-2.0

package entrypointenrichmentprocessor

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor/processortest"
)

func TestProcessor_Smoke(t *testing.T) {
	sink := new(consumertest.TracesSink)
	cfg := &Config{
		WaitDuration:             10 * time.Millisecond,
		FallbackDuration:         200 * time.Millisecond,
		NumTraces:                1000,
		AttributesToPromote:      []string{`^dt\.feature_flag\.`},
		LocalRootMarkerAttribute: "dt.local_root",
	}
	set := processortest.NewNopSettings(Type)
	p, err := newProcessor(cfg, set, sink)
	require.NoError(t, err)
	require.NotNil(t, p)

	err = p.Start(context.Background(), nil)
	require.NoError(t, err)

	// Build a simple trace: root + child with FF attr.
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("service.name", "test-svc")
	ss := rs.ScopeSpans().AppendEmpty()

	traceID := pcommon.TraceID([16]byte{1})
	rootID := pcommon.SpanID([8]byte{1})
	childID := pcommon.SpanID([8]byte{2})

	root := ss.Spans().AppendEmpty()
	root.SetTraceID(traceID)
	root.SetSpanID(rootID)
	root.SetKind(ptrace.SpanKindServer)

	child := ss.Spans().AppendEmpty()
	child.SetTraceID(traceID)
	child.SetSpanID(childID)
	child.SetParentSpanID(rootID)
	child.SetKind(ptrace.SpanKindInternal)
	child.Attributes().PutStr("dt.feature_flag.result.pricing_v2", "on")

	err = p.ConsumeTraces(context.Background(), td)
	require.NoError(t, err)

	// Wait for flush.
	require.Eventually(t, func() bool {
		return sink.SpanCount() == 2
	}, 500*time.Millisecond, time.Millisecond)

	// Verify root has the promoted attribute and the marker.
	foundRoot := false
	for _, traces := range sink.AllTraces() {
		for i := 0; i < traces.ResourceSpans().Len(); i++ {
			for j := 0; j < traces.ResourceSpans().At(i).ScopeSpans().Len(); j++ {
				for k := 0; k < traces.ResourceSpans().At(i).ScopeSpans().At(j).Spans().Len(); k++ {
					sp := traces.ResourceSpans().At(i).ScopeSpans().At(j).Spans().At(k)
					if sp.SpanID() == rootID {
						foundRoot = true
						v, ok := sp.Attributes().Get("dt.feature_flag.result.pricing_v2")
						assert.True(t, ok, "FF attribute should be promoted to root")
						if ok {
							assert.Equal(t, "on", v.AsString())
						}
						m, ok := sp.Attributes().Get("dt.local_root")
						assert.True(t, ok, "marker attribute should be set on root")
						if ok {
							assert.Equal(t, "true", m.AsString())
						}
					}
				}
			}
		}
	}
	assert.True(t, foundRoot, "root span should be in output")

	err = p.Shutdown(context.Background())
	require.NoError(t, err)
}
