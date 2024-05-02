package configurablehttpprovider // import "github.com/Dynatrace/dynatrace-otel-collector/confmap/provider/eecprovider/internal/configurablehttpprovider"

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"time"
)

type config struct {
	// A time duration that defines how frequently the provider should check the given URL for updates.
	refreshInterval time.Duration

	// A header that will be used to authenticate the provider with the EEC.
	// We will always use Api-Token (and this can be assumed as a default if auth-file is passed),
	// but upstream will want to support configurable headers and we should consider
	// offering this option should upstream determine the option is required.
	authHeader string

	// Token passed as the value for the header in authHeader.
	authToken string
}

func parseConfig(params url.Values) (config, error) {
	cfg := config{
		refreshInterval: time.Second * 10,
		authHeader:      "Api-Token",
	}
	var err error

	if params.Has(RefreshInterval) {
		cfg.refreshInterval, err = time.ParseDuration(params.Get(RefreshInterval))
		if err != nil {
			return cfg, err
		}
	}

	if params.Has(AuthHeader) {
		cfg.authHeader = params.Get(AuthHeader)
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

	return cfg, nil
}
