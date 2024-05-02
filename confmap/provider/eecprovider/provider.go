// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package eecprovider // import "github.com/Dynatrace/dynatrace-otel-collector/confmap/provider/eecprovider"

import (
	"github.com/Dynatrace/dynatrace-otel-collector/confmap/provider/internal/configurablehttpprovider"
	"go.opentelemetry.io/collector/confmap"
)

// NewFactory returns a factory for a confmap.Provider that reads the configuration from an https server.
//
// This Provider supports "eec" scheme.
//
// One example for an HTTPS URI is eec://localhost:3333/getConfig
func NewFactory() confmap.ProviderFactory {
	return confmap.NewProviderFactory(new)
}

func new(set confmap.ProviderSettings) confmap.Provider {
	return configurablehttpprovider.New(configurablehttpprovider.EECScheme, set)
}
