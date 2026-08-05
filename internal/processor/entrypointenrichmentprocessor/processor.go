// Copyright Dynatrace LLC
// SPDX-License-Identifier: Apache-2.0

package entrypointenrichmentprocessor

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor"
)

type entrypointEnrichmentProcessor struct {
	buf *Buffer
	cfg *Config
}

func newProcessor(cfg *Config, set processor.Settings, next consumer.Traces) (*entrypointEnrichmentProcessor, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	buf := newBuffer(cfg, next, set.Logger)
	return &entrypointEnrichmentProcessor{buf: buf, cfg: cfg}, nil
}

func (p *entrypointEnrichmentProcessor) Start(_ context.Context, _ component.Host) error {
	return nil
}

func (p *entrypointEnrichmentProcessor) Shutdown(ctx context.Context) error {
	return p.buf.Shutdown(ctx)
}

func (p *entrypointEnrichmentProcessor) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: true}
}

func (p *entrypointEnrichmentProcessor) ConsumeTraces(_ context.Context, td ptrace.Traces) error {
	for i := 0; i < td.ResourceSpans().Len(); i++ {
		rs := td.ResourceSpans().At(i)
		for j := 0; j < rs.ScopeSpans().Len(); j++ {
			ss := rs.ScopeSpans().At(j)
			for k := 0; k < ss.Spans().Len(); k++ {
				p.buf.Insert(rs.Resource(), ss.Scope(), ss.Spans().At(k))
			}
		}
	}
	return nil
}
