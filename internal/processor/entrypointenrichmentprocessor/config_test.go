// Copyright Dynatrace LLC
// SPDX-License-Identifier: Apache-2.0

package entrypointenrichmentprocessor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr string
	}{
		{
			name: "happy path",
			cfg: Config{
				WaitDuration:        100 * time.Millisecond,
				FallbackDuration:    5 * time.Second,
				NumTraces:           1000,
				AttributesToPromote: []string{`^dt\.feature_flag\.result\..+$`},
			},
		},
		{
			name: "fallback less than wait",
			cfg: Config{
				WaitDuration:     2 * time.Second,
				FallbackDuration: 1 * time.Second,
				NumTraces:        1000,
			},
			wantErr: "fallback_duration",
		},
		{
			name: "num_traces zero",
			cfg: Config{
				WaitDuration:     100 * time.Millisecond,
				FallbackDuration: 5 * time.Second,
				NumTraces:        0,
			},
			wantErr: "num_traces",
		},
		{
			name: "invalid regex",
			cfg: Config{
				WaitDuration:        100 * time.Millisecond,
				FallbackDuration:    5 * time.Second,
				NumTraces:           1000,
				AttributesToPromote: []string{`[invalid`},
			},
			wantErr: "invalid regex",
		},
		{
			name: "zero wait_duration allowed",
			cfg: Config{
				WaitDuration:     0,
				FallbackDuration: 5 * time.Second,
				NumTraces:        1000,
			},
		},
		{
			name: "empty attributes_to_promote valid",
			cfg: Config{
				WaitDuration:     100 * time.Millisecond,
				FallbackDuration: 5 * time.Second,
				NumTraces:        1000,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			} else {
				require.NoError(t, err)
				assert.Len(t, tt.cfg.compiledPatterns, len(tt.cfg.AttributesToPromote))
			}
		})
	}
}
