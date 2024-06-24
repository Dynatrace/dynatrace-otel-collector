// Copyright Dynatrace LLC
// SPDX-License-Identifier: Apache-2.0

package eecprovider // import "github.com/Dynatrace/dynatrace-otel-collector/confmap/provider/eecprovider"

import (
	"net/url"
	"os"
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestConfigParsing(t *testing.T) {
	t.Parallel()

	token := "mytoken"
	tmpDir := t.TempDir()
	file, err := os.Create(path.Join(tmpDir, "token.key"))
	require.NoError(t, err)
	file.Write([]byte(token))

	tokenEnv := "CONFMAP_AUTH"
	envToken := "myenvtoken"
	os.Setenv(tokenEnv, envToken)

	tests := []struct {
		name           string
		params         url.Values
		expectedConfig *config
		shouldError    bool
	}{
		{
			name: "Parse all mutually-compatible fields",
			params: map[string][]string{
				AuthFile:        {file.Name()},
				RefreshInterval: {"1h"},
				Timeout:         {"50m"},
				Insecure:        {"true"},
			},
			expectedConfig: &config{
				authToken:       token,
				refreshInterval: time.Hour,
				timeout:         50 * time.Minute,
				insecure:        true,
			},
		},
		{
			name: "Parse token from env var",
			params: map[string][]string{
				AuthEnv:         {"CONFMAP_AUTH"},
				RefreshInterval: {"1h"},
			},
			expectedConfig: &config{
				authToken:       envToken,
				refreshInterval: time.Hour,
				timeout:         8 * time.Second,
			},
		},
		{
			name: "Allow unsetting refresh-interval",
			params: map[string][]string{
				RefreshInterval: {"0h"},
			},
			expectedConfig: &config{
				refreshInterval: 0,
				timeout:         8 * time.Second,
			},
		},
		{
			name:   "Allow not setting an auth token",
			params: map[string][]string{},
			expectedConfig: &config{
				refreshInterval: 10 * time.Second,
				timeout:         8 * time.Second,
			},
		},
		{
			name:   "Allow not setting a timeout",
			params: map[string][]string{},
			expectedConfig: &config{
				refreshInterval: 10 * time.Second,
				timeout:         8 * time.Second,
			},
		},
		{
			name: "Error when setting an invalid refresh-interval",
			params: map[string][]string{
				RefreshInterval: {"notavalidinterval"},
			},
			shouldError: true,
		},
		{
			name: "Error when setting both auth-file and auth-env",
			params: map[string][]string{
				AuthFile: {"myfile"},
				AuthEnv:  {"MY_ENV"},
			},
			shouldError: true,
		},
		{
			name: "Error when the auth token file doesn't exist",
			params: map[string][]string{
				AuthFile: {"doesnotexist"},
			},
			shouldError: true,
		},
		{
			name: "Error when the auth token env var isn't set",
			params: map[string][]string{
				AuthEnv: {"ENV_VAR_NOT_SET"},
			},
			shouldError: true,
		},
		{
			name: "Error when an invalid value is given to insecure",
			params: map[string][]string{
				Insecure: {"notvalid"},
			},
			shouldError: true,
		},
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			cfg, err := parseConfig(tt.params)

			if tt.expectedConfig != nil {
				require.Equal(t, *tt.expectedConfig, cfg)
			}

			if tt.shouldError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
