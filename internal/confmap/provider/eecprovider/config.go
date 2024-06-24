// Copyright Dynatrace LLC
// SPDX-License-Identifier: Apache-2.0

package eecprovider // import "github.com/Dynatrace/dynatrace-otel-collector/confmap/provider/eecprovider/eecprovider"

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"time"
)

const (
	RefreshInterval = "refresh-interval"
	Timeout         = "timeout"
	AuthFile        = "auth-file"
	AuthEnv         = "auth-env"
	Insecure        = "insecure"
)

type config struct {
	// A time duration that defines how frequently the provider should check the given URL for updates.
	refreshInterval time.Duration

	// The maximum time allowed for a single request to the server.
	timeout time.Duration

	// Token passed as the value for the header in authHeader.
	authToken string

	// Whether to use HTTP instead of HTTPS
	insecure bool
}

func parseConfig(params url.Values) (config, error) {
	cfg := config{
		refreshInterval: 10 * time.Second,
		timeout:         8 * time.Second,
	}
	var err error

	if params.Has(RefreshInterval) {
		cfg.refreshInterval, err = time.ParseDuration(params.Get(RefreshInterval))
		if err != nil {
			return cfg, err
		}
	}

	if params.Has(Timeout) {
		cfg.timeout, err = time.ParseDuration(params.Get(Timeout))
		if err != nil {
			return cfg, err
		}
	}

	if params.Has(AuthFile) && params.Has(AuthEnv) {
		return cfg, errors.New("cannot pass both auth-file and auth-env")
	}

	if params.Has(AuthFile) {
		by, err := os.ReadFile(params.Get(AuthFile))
		if err != nil {
			return cfg, err
		}

		cfg.authToken = string(by)
	}

	if params.Has(AuthEnv) {
		env := os.Getenv(params.Get(AuthEnv))

		if env == "" {
			return cfg, fmt.Errorf("auth token environment variable %q is not set", params.Get(AuthEnv))
		}

		cfg.authToken = env
	}

	if params.Has(Insecure) {
		insecureParam := params.Get(Insecure)
		if insecureParam == "true" {
			cfg.insecure = true
		} else if insecureParam != "false" {
			return cfg, fmt.Errorf("valid values for %q are {true, false}; got %q", Insecure, insecureParam)
		}
	}

	return cfg, nil
}
