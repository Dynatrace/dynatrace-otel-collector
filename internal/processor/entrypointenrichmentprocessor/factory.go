// Copyright Dynatrace LLC
// SPDX-License-Identifier: Apache-2.0

package entrypointenrichmentprocessor

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"
)

// Type is the component type identifier.
var Type = component.MustNewType("entrypoint_enrichment")

// NewFactory creates a new processor factory.
func NewFactory() processor.Factory {
	return processor.NewFactory(
		Type,
		createDefaultConfig,
		processor.WithTraces(createTracesProcessor, component.StabilityLevelDevelopment),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		WaitDuration:             500 * time.Millisecond,
		FallbackDuration:         5 * time.Second,
		NumTraces:                1000000,
		AttributesToPromote:      []string{`^dt\.feature_flag\.result\..+$`},
		LocalRootMarkerAttribute: "dt.local_root",
	}
}

func createTracesProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	next consumer.Traces,
) (processor.Traces, error) {
	return newProcessor(cfg.(*Config), set, next)
}
