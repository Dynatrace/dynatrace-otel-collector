// Copyright The OpenTelemetry Authors
// Copyright Dynatrace LLC
// SPDX-License-Identifier: Apache-2.0

// Tests were added to this file to test the features added to the provider.
// Existing tests were adapted to handle changes to the provider.

package eecprovider

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/confmaptest"
)

func newEECProvider(set confmap.ProviderSettings) *provider {
	return NewFactory().Create(set).(*provider)
}

func answerGet(w http.ResponseWriter, _ *http.Request) {
	f, err := os.ReadFile("./testdata/otel-config.yaml")
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		_, innerErr := w.Write([]byte("Cannot find the config file"))
		if innerErr != nil {
			fmt.Println("Write failed: ", innerErr)
		}
		return
	}
	w.WriteHeader(http.StatusOK)
	_, err = w.Write(f)
	if err != nil {
		fmt.Println("Write failed: ", err)
	}
}

// Generate a self signed certificate specific for the tests. Based on
// https://go.dev/src/crypto/tls/generate_cert.go
func generateCertificate(hostname string) (cert string, key string, err error) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)

	if err != nil {
		return "", "", fmt.Errorf("Failed to generate private key: %w", err)
	}

	keyUsage := x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment | x509.KeyUsageCertSign
	notBefore := time.Now()
	notAfter := notBefore.Add(time.Hour * 12)
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)

	if err != nil {
		return "", "", fmt.Errorf("Failed to generate serial number: %w", err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Httpprovider Co"},
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              keyUsage,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
		DNSNames:              []string{hostname},
	}
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)

	if err != nil {
		return "", "", fmt.Errorf("Failed to create certificate: %w", err)
	}

	certOut, err := os.CreateTemp("", "cert*.pem")
	if err != nil {
		return "", "", fmt.Errorf("Failed to open cert.pem for writing: %w", err)
	}

	if err = pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		return "", "", fmt.Errorf("Failed to write data to cert.pem: %w", err)
	}

	if err = certOut.Close(); err != nil {
		return "", "", fmt.Errorf("Error closing cert.pem: %w", err)
	}

	keyOut, err := os.CreateTemp("", "key*.pem")

	if err != nil {
		return "", "", fmt.Errorf("Failed to open key.pem for writing: %w", err)
	}

	privBytes, err := x509.MarshalPKCS8PrivateKey(priv)

	if err != nil {
		return "", "", fmt.Errorf("Unable to marshal private key: %w", err)
	}

	if err := pem.Encode(keyOut, &pem.Block{Type: "PRIVATE KEY", Bytes: privBytes}); err != nil {
		return "", "", fmt.Errorf("Failed to write data to key.pem: %w", err)
	}

	if err := keyOut.Close(); err != nil {
		return "", "", fmt.Errorf("Error closing key.pem: %w", err)
	}

	return certOut.Name(), keyOut.Name(), nil
}

func TestFunctionalityDownloadFileHTTP(t *testing.T) {
	ep := newEECProvider(confmaptest.NewNopProviderSettings())
	ts := httptest.NewServer(http.HandlerFunc(answerGet))
	defer ts.Close()
	_, err := ep.Retrieve(context.Background(), makeInsecure(t, ts.URL), nil)
	assert.NoError(t, err)
	assert.NoError(t, ep.Shutdown(context.Background()))
}

func TestFunctionalityDownloadFileHTTPS(t *testing.T) {
	certPath, keyPath, err := generateCertificate("localhost")
	assert.NoError(t, err)

	invalidCert, err := os.CreateTemp("", "cert*.crt")
	assert.NoError(t, err)
	_, err = invalidCert.Write([]byte{0, 1, 2})
	assert.NoError(t, err)

	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	assert.NoError(t, err)
	ts := httptest.NewUnstartedServer(http.HandlerFunc(answerGet))
	ts.TLS = &tls.Config{Certificates: []tls.Certificate{cert}}
	ts.StartTLS()

	defer os.Remove(certPath)
	defer os.Remove(keyPath)
	defer os.Remove(invalidCert.Name())
	defer ts.Close()

	tests := []struct {
		testName               string
		certPath               string
		hostName               string
		useCertificate         bool
		skipHostnameValidation bool
		shouldError            bool
	}{
		{
			testName:               "Test valid certificate and name",
			certPath:               certPath,
			hostName:               "localhost",
			useCertificate:         true,
			skipHostnameValidation: false,
			shouldError:            false,
		},
		{
			testName:               "Test valid certificate with invalid name",
			certPath:               certPath,
			hostName:               "127.0.0.1",
			useCertificate:         true,
			skipHostnameValidation: false,
			shouldError:            true,
		},
		{
			testName:               "Test valid certificate with invalid name, skip validation",
			certPath:               certPath,
			hostName:               "127.0.0.1",
			useCertificate:         true,
			skipHostnameValidation: true,
			shouldError:            false,
		},
		{
			testName:               "Test no certificate should fail",
			certPath:               certPath,
			hostName:               "localhost",
			useCertificate:         false,
			skipHostnameValidation: false,
			shouldError:            true,
		},
		{
			testName:               "Test invalid cert",
			certPath:               invalidCert.Name(),
			hostName:               "localhost",
			useCertificate:         true,
			skipHostnameValidation: false,
			shouldError:            true,
		},
		{
			testName:               "Test no cert",
			certPath:               "no_certificate",
			hostName:               "localhost",
			useCertificate:         true,
			skipHostnameValidation: false,
			shouldError:            true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.testName, func(t *testing.T) {
			ep := newEECProvider(confmaptest.NewNopProviderSettings())
			// Parse url of the test server to get the port number.
			tsURL, err := url.Parse(ts.URL)
			require.NoError(t, err)
			if tt.useCertificate {
				ep.caCertPath = tt.certPath
			}
			ep.insecureSkipVerify = tt.skipHostnameValidation
			_, err = ep.Retrieve(context.Background(), fmt.Sprintf("eec://%s:%s", tt.hostName, tsURL.Port()), nil)
			if tt.shouldError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestUnsupportedScheme(t *testing.T) {
	ep := newProvider(confmaptest.NewNopProviderSettings())
	_, err := ep.Retrieve(context.Background(), "https://...", nil)
	assert.Error(t, err)
	assert.NoError(t, ep.Shutdown(context.Background()))
}

func TestEmptyURI(t *testing.T) {
	ep := newProvider(confmaptest.NewNopProviderSettings())
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer ts.Close()
	_, err := ep.Retrieve(context.Background(), makeInsecure(t, ts.URL), nil)
	require.Error(t, err)
	require.NoError(t, ep.Shutdown(context.Background()))
}

func TestRetrieveFromShutdownServer(t *testing.T) {
	ep := newProvider(confmaptest.NewNopProviderSettings())
	ts := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	ts.Close()
	_, err := ep.Retrieve(context.Background(), makeInsecure(t, ts.URL), nil)
	assert.Error(t, err)
	require.NoError(t, ep.Shutdown(context.Background()))
}

func TestNonExistent(t *testing.T) {
	ep := newProvider(confmaptest.NewNopProviderSettings())
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()
	_, err := ep.Retrieve(context.Background(), makeInsecure(t, ts.URL), nil)
	assert.Error(t, err)
	require.NoError(t, ep.Shutdown(context.Background()))
}

func TestInvalidYAML(t *testing.T) {
	ep := newProvider(confmaptest.NewNopProviderSettings())
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte("wrong : ["))
		if err != nil {
			fmt.Println("Write failed: ", err)
		}
	}))
	defer ts.Close()
	_, err := ep.Retrieve(context.Background(), makeInsecure(t, ts.URL), nil)
	assert.Error(t, err)
	require.NoError(t, ep.Shutdown(context.Background()))
}

func TestScheme(t *testing.T) {
	ep := newProvider(confmaptest.NewNopProviderSettings())
	assert.Equal(t, "eec", ep.Scheme())
	require.NoError(t, ep.Shutdown(context.Background()))
}

func TestValidateProviderScheme(t *testing.T) {
	assert.NoError(t, confmaptest.ValidateProviderScheme(newProvider(confmaptest.NewNopProviderSettings())))
}

func TestInvalidTransport(t *testing.T) {
	ep := newProvider(confmaptest.NewNopProviderSettings())

	_, err := ep.Retrieve(context.Background(), "foo://..", nil)
	assert.Error(t, err)
}

func TestNoReloadIfConfigIsSame(t *testing.T) {
	count := &atomic.Uint32{}
	answerWithCount := func(w http.ResponseWriter, _ *http.Request) {
		f, err := os.ReadFile("./testdata/otel-config.yaml")
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			_, innerErr := w.Write([]byte("Cannot find the config file"))
			if innerErr != nil {
				fmt.Println("Write failed: ", innerErr)
			}
			return
		}
		w.WriteHeader(http.StatusOK)
		_, err = w.Write(f)
		if err != nil {
			fmt.Println("Write failed: ", err)
		}
		count.Add(1)
	}
	called := &atomic.Bool{}
	watcherFunc := func(_ *confmap.ChangeEvent) {
		called.Store(true)
	}
	ep := newEECProvider(confmaptest.NewNopProviderSettings())
	ts := httptest.NewServer(http.HandlerFunc(answerWithCount))
	defer ts.Close()
	uri, err := url.Parse(makeInsecure(t, ts.URL))
	require.NoError(t, err)
	params, err := url.ParseQuery(uri.Fragment)
	require.NoError(t, err)
	params.Set(RefreshInterval, "10ms")
	uri.Fragment = params.Encode()

	_, err = ep.Retrieve(context.Background(), uri.String(), watcherFunc)
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		return count.Load() > 2
	}, time.Second*3, time.Millisecond*50)
	require.False(t, called.Load())
	assert.NoError(t, ep.Shutdown(context.Background()))
}

func TestReloadIfConfigChanges(t *testing.T) {
	count := &atomic.Uint32{}
	answerWithCount := func(w http.ResponseWriter, _ *http.Request) {
		configFile := "./testdata/otel-config.yaml"
		if count.Load() > 2 {
			configFile = "./testdata/otel-config-updated.yaml"
		}
		f, err := os.ReadFile(configFile)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			_, innerErr := w.Write([]byte("Cannot find the config file"))
			if innerErr != nil {
				fmt.Println("Write failed: ", innerErr)
			}
			return
		}
		w.WriteHeader(http.StatusOK)
		_, err = w.Write(f)
		if err != nil {
			fmt.Println("Write failed: ", err)
		}
		count.Add(1)
	}
	called := &atomic.Bool{}
	watcherFunc := func(_ *confmap.ChangeEvent) {
		if called.Load() {
			require.FailNow(t, "Reloaded more than once")
		}
		called.Store(true)
	}
	ep := newEECProvider(confmaptest.NewNopProviderSettings())
	ts := httptest.NewServer(http.HandlerFunc(answerWithCount))
	defer ts.Close()
	uri, err := url.Parse(makeInsecure(t, ts.URL))
	require.NoError(t, err)
	params, err := url.ParseQuery(uri.Fragment)
	require.NoError(t, err)
	params.Set(RefreshInterval, "10ms")
	uri.Fragment = params.Encode()

	// Call Retrieve twice to verify that watcherFunc isn't registered twice
	// then later called twice when the config changes.
	_, err = ep.Retrieve(context.Background(), uri.String(), watcherFunc)
	require.NoError(t, err)
	_, err = ep.Retrieve(context.Background(), uri.String(), watcherFunc)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return count.Load() > 2
	}, time.Second*3, time.Millisecond*50)
	require.True(t, called.Load())
	assert.NoError(t, ep.Shutdown(context.Background()))
}

func TestContinuesRetryingOnRefreshError(t *testing.T) {
	count := &atomic.Uint32{}
	answerWithCount := func(w http.ResponseWriter, _ *http.Request) {
		if count.Load() > 2 {
			w.WriteHeader(http.StatusInternalServerError)
			_, err := w.Write(nil)
			if err != nil {
				fmt.Println("Write failed: ", err)
			}
			count.Add(1)
			return
		}

		f, err := os.ReadFile("./testdata/otel-config.yaml")
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			_, innerErr := w.Write([]byte("Cannot find the config file"))
			if innerErr != nil {
				fmt.Println("Write failed: ", innerErr)
			}
			return
		}
		w.WriteHeader(http.StatusOK)
		_, err = w.Write(f)
		if err != nil {
			fmt.Println("Write failed: ", err)
		}
		count.Add(1)
	}
	called := &atomic.Bool{}
	watcherFunc := func(_ *confmap.ChangeEvent) {
		called.Store(true)
	}
	ep := newEECProvider(confmaptest.NewNopProviderSettings())
	ts := httptest.NewServer(http.HandlerFunc(answerWithCount))
	defer ts.Close()
	uri, err := url.Parse(makeInsecure(t, ts.URL))
	require.NoError(t, err)
	params, err := url.ParseQuery(uri.Fragment)
	require.NoError(t, err)
	params.Set(RefreshInterval, "10ms")
	uri.Fragment = params.Encode()

	_, err = ep.Retrieve(context.Background(), uri.String(), watcherFunc)
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		return count.Load() > 5
	}, time.Second*3, time.Millisecond*20)
	require.False(t, called.Load())
	assert.NoError(t, ep.Shutdown(context.Background()))
}

func TestFragmentConfiguration(t *testing.T) {
	wg := &sync.WaitGroup{}
	refreshInterval := "10h"

	token := "mytoken"
	tmpDir := t.TempDir()
	file, err := os.Create(path.Join(tmpDir, "token.key"))
	require.NoError(t, err)
	file.Write([]byte(token))

	answerWithConfig := func(w http.ResponseWriter, req *http.Request) {
		f, err := os.ReadFile("./testdata/otel-config.yaml")
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			_, innerErr := w.Write([]byte("Cannot find the config file"))
			if innerErr != nil {
				fmt.Println("Write failed: ", innerErr)
			}
			return
		}
		w.WriteHeader(http.StatusOK)
		_, err = w.Write(f)
		if err != nil {
			fmt.Println("Write failed: ", err)
		}

		require.Equal(t, token, req.Header.Get(ApiTokenHeader))

		wg.Done()
	}
	called := &atomic.Bool{}
	watcherFunc := func(_ *confmap.ChangeEvent) {
		called.Store(true)
	}
	ep := newEECProvider(confmaptest.NewNopProviderSettings())
	ts := httptest.NewServer(http.HandlerFunc(answerWithConfig))
	defer ts.Close()
	uri, err := url.Parse(makeInsecure(t, ts.URL))
	require.NoError(t, err)
	params, err := url.ParseQuery(uri.Fragment)
	require.NoError(t, err)
	params.Set(AuthFile, file.Name())
	params.Set(RefreshInterval, refreshInterval)
	uri.Fragment = params.Encode()

	wg.Add(1)
	_, err = ep.Retrieve(context.Background(), uri.String(), watcherFunc)
	require.NoError(t, err)
	wg.Wait()
	assert.NoError(t, ep.Shutdown(context.Background()))
}

func makeInsecure(t *testing.T, URL string) string {
	parsedURL, err := url.Parse(URL)
	require.NoError(t, err)
	parsedURL.Scheme = "eec"
	params, err := url.ParseQuery(parsedURL.Fragment)
	require.NoError(t, err)
	params.Set(Insecure, "true")
	parsedURL.Fragment = params.Encode()
	return parsedURL.String()
}
