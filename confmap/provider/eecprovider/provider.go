// Copyright The OpenTelemetry Authors
// Copyright Dynatrace LLC
// SPDX-License-Identifier: Apache-2.0

// This file was copied from the configurablehttpprovider package and modified
// to support configuration options through query parameter-encoded URL fragments.
// Additionally, support for two schemes (HTTP/HTTPS) was dropped in favor of
// configuring TLS through an option on the provider.

package eecprovider // import "github.com/Dynatrace/dynatrace-otel-collector/confmap/provider/eecprovider"

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/collector/confmap"
)

type SchemeType string

const (
	EECScheme SchemeType = "eec"
)

const ApiTokenHeader = "Api-Token"

type provider struct {
	caCertPath         string // Used for tests
	insecureSkipVerify bool   // Used for tests

	// The provider will close this channel to signal to all watchers that
	// the provider is shutting down and they should stop.
	shutdown chan struct{}

	// Should not be necessary, but protects the watcherCancelMap from
	// data races caused by concurrent access.
	watcherMapMux *sync.Mutex

	// Keeps track of ongoing watcher functions and cancels existing runs
	// if the same URL is requested again.
	watcherCancelMap map[string]func()
}

var _ confmap.Provider = (*provider)(nil)

// NewFactory returns a factory for a confmap.Provider that reads the configuration from an https server.
//
// This Provider supports "eec" scheme.
//
// One example for an HTTPS URI is eec://localhost:3333/getConfig
func NewFactory() confmap.ProviderFactory {
	return confmap.NewProviderFactory(newProvider)
}

func newProvider(_ confmap.ProviderSettings) confmap.Provider {
	shutdown := make(chan struct{})
	return &provider{shutdown: shutdown, watcherMapMux: &sync.Mutex{}, watcherCancelMap: map[string]func(){}}
}

// Create the client based on the type of scheme that was selected.
func (p *provider) createClient(insecure bool) (*http.Client, error) {
	if insecure {
		return &http.Client{}, nil
	}

	pool, err := x509.SystemCertPool()

	if err != nil {
		return nil, fmt.Errorf("unable to create a cert pool: %w", err)
	}

	if p.caCertPath != "" {
		cert, err := os.ReadFile(filepath.Clean(p.caCertPath))

		if err != nil {
			return nil, fmt.Errorf("unable to read CA from %q URI: %w", p.caCertPath, err)
		}

		if ok := pool.AppendCertsFromPEM(cert); !ok {
			return nil, fmt.Errorf("unable to add CA from uri: %s into the cert pool", p.caCertPath)
		}
	}

	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: p.insecureSkipVerify,
				RootCAs:            pool,
			},
		},
	}, nil
}

func (p *provider) Retrieve(ctx context.Context, uri string, watcherFunc confmap.WatcherFunc) (*confmap.Retrieved, error) {
	if !strings.HasPrefix(uri, string(EECScheme)+":") {
		return nil, fmt.Errorf("%q uri is not supported by %q provider", uri, string(EECScheme))
	}

	parsedUrl, err := url.Parse(uri)
	if err != nil {
		return nil, err
	}

	params, err := url.ParseQuery(parsedUrl.Fragment)
	if err != nil {
		return nil, err
	}
	cfg, err := parseConfig(params)
	if err != nil {
		return nil, err
	}
	// Fragments will only be used to configure this provider,
	// so remove them from the URI.
	parsedUrl.Fragment = ""

	client, err := p.createClient(cfg.insecure)

	if err != nil {
		return nil, fmt.Errorf("unable to configure http transport layer: %w", err)
	}

	if cfg.insecure {
		parsedUrl.Scheme = "http"
	} else {
		parsedUrl.Scheme = "https"
	}

	newContextBoundRequest := func(ctx context.Context) (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsedUrl.String(), nil)
		if err != nil {
			return nil, err
		}
		if cfg.authToken != "" {
			req.Header.Add(ApiTokenHeader, cfg.authToken)
		}
		return req, nil
	}

	reqCtx, cancel := context.WithTimeoutCause(ctx, 3*time.Second, errors.New("request to EEC timed out"))
	defer cancel()
	req, err := newContextBoundRequest(reqCtx)
	if err != nil {
		return nil, err
	}

	body, err := p.getConfigBytes(client, req)
	if err != nil {
		return nil, err
	}

	// If the Collector has not provided a watcherFunc, or if
	// the `refresh-interval` parameter was deliberately set to 0,
	// we assume that polling for config updates has been disabled.
	if watcherFunc != nil && cfg.refreshInterval.Nanoseconds() != 0 {
		// If someone has requested the same URL twice, we want to ensure
		// we are only watching it once. Currently upstream resolves the
		// configuration twice (once for the config, once for the confmap),
		// so we need to at a minimum protect against this case.
		watcherCtx, cancel := context.WithCancel(ctx)
		p.watcherMapMux.Lock()
		url := req.URL.String()
		if _, ok := p.watcherCancelMap[url]; ok {
			p.watcherCancelMap[url]()
		}
		p.watcherCancelMap[url] = func() {
			cancel()
		}
		p.watcherMapMux.Unlock()

		watcher := &watcher{
			shutdown: p.shutdown,
			getConfigBytes: func(ctx context.Context) ([]byte, error) {
				req, err := newContextBoundRequest(ctx)
				if err != nil {
					return nil, err
				}
				return p.getConfigBytes(client, req)
			},
			refreshInterval: cfg.refreshInterval,
			watcherFunc:     watcherFunc,
			configHash:      sha256.Sum256(body),
		}

		go watcher.watchForChanges(watcherCtx)
	}

	return NewRetrievedFromYAML(body)
}

func (p *provider) Scheme() string {
	return string(EECScheme)
}

func (p *provider) Shutdown(context.Context) error {
	close(p.shutdown)

	return nil
}

func (*provider) getConfigBytes(client *http.Client, req *http.Request) ([]byte, error) {
	// send a HTTP GET request
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("unable to download the file via HTTP GET for uri %q: %w ", req.URL.String(), err)
	}
	defer resp.Body.Close()

	// check the HTTP status code
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to load resource from uri %q. status code: %d", req.RequestURI, resp.StatusCode)
	}

	// read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("fail to read the response body from uri %q: %w", req.RequestURI, err)
	}

	return body, nil
}
