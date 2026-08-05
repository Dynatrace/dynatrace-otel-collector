// Copyright Dynatrace LLC
// SPDX-License-Identifier: Apache-2.0

package entrypointenrichmentprocessor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFactory(t *testing.T) {
	f := NewFactory()
	require.NotNil(t, f)
	assert.Equal(t, Type, f.Type())
	cfg := f.CreateDefaultConfig()
	require.NotNil(t, cfg)
	c, ok := cfg.(*Config)
	require.True(t, ok)
	assert.Equal(t, ModeFlagsWithKindFallback, c.LocalRootDetection)
	assert.Equal(t, "dt.local_root", c.LocalRootMarkerAttribute)
	assert.Equal(t, 1000000, c.NumTraces)
}
