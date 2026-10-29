// Copyright Dynatrace LLC
// SPDX-License-Identifier: Apache-2.0

package bearertokenauth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/oteltest"
	"github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/testutil"
	"github.com/go-jose/go-jose/v4"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	collectorExecPath = "../../../../bin/dynatrace-otel-collector"
	configExamplesDir = "../../../../config_examples"
	// audience is the audience configured in config_examples/oidcauth-receiver.yaml.
	audience = "my-collector"
)

// ports holds the ports a locally started Collector and its sink use.
type ports struct {
	grpc     int
	http     int
	health   int
	exporter int
}

// startSink starts a traces sink that the Collector under test exports to and returns
// the sink together with the ports of the Collector.
func startSink(t *testing.T) (*consumertest.TracesSink, ports) {
	p := ports{
		grpc:     testutil.GetAvailablePort(t),
		http:     testutil.GetAvailablePort(t),
		health:   testutil.GetAvailablePort(t),
		exporter: testutil.GetAvailablePort(t),
	}
	sink := new(consumertest.TracesSink)
	shutdown := oteltest.StartUpSinks(t, oteltest.ReceiverSinks{
		Traces: []*oteltest.TraceSinkConfig{{
			Consumer: sink,
			Ports:    &oteltest.ReceiverPorts{Grpc: testutil.GetAvailablePort(t), Http: p.exporter},
		}},
	})
	t.Cleanup(shutdown)
	return sink, p
}

// prepareConfig reads an example config and adapts it for the test. Each replacement is
// required to change the config so that edits to the example can't silently break the test.
// The returned replace function can be used for further, test specific adaptations.
func prepareConfig(t *testing.T, example string, p ports) (cfg *string, replace func(name string, f func(string) string)) {
	raw, err := os.ReadFile(filepath.Join(configExamplesDir, example))
	require.NoError(t, err)
	out := string(raw)
	cfg = &out

	replace = func(name string, f func(string) string) {
		res := f(*cfg)
		require.NotEqual(t, *cfg, res, "config replacement %q had no effect", name)
		*cfg = res
	}
	replace("grpc receiver port", func(s string) string { return testutil.ReplaceOtlpGrpcReceiverPort(s, p.grpc) })
	replace("http receiver port", func(s string) string { return testutil.ReplaceOtlpHttpReceiverPort(s, p.http) })
	replace("health_check port", func(s string) string {
		return strings.Replace(s, "0.0.0.0:13133", "0.0.0.0:"+strconv.Itoa(p.health), 1)
	})
	replace("exporter endpoint", func(s string) string { return testutil.ReplaceDynatraceExporterEndpoint(s, p.exporter) })
	replace("api token", func(s string) string { return strings.Replace(s, "${env:DT_API_TOKEN}", "", 1) })
	// Export results should be synchronous so that failures show up immediately.
	replace("sending_queue and retry_on_failure", func(s string) string {
		return strings.Replace(s, "  otlp_http:\n    endpoint:",
			"  otlp_http:\n    retry_on_failure:\n      enabled: false\n    sending_queue:\n      enabled: false\n    endpoint:", 1)
	})
	// The Collector's internal metrics server binds the fixed port 8888, which would clash with
	// other Collectors on the machine (or a leftover from an aborted run).
	replace("internal telemetry", func(s string) string {
		return strings.Replace(s, "service:\n", "service:\n  telemetry:\n    metrics:\n      level: none\n", 1)
	})
	return cfg, replace
}

func startCollector(t *testing.T, name, cfg string, readyPort int) {
	col := testbed.NewChildProcessCollector(testbed.WithAgentExePath(collectorExecPath))
	cleanupCfg, err := col.PrepareConfig(t, cfg)
	require.NoError(t, err)
	t.Cleanup(cleanupCfg)

	logPath := filepath.Join(t.TempDir(), "collector.log")
	require.NoError(t, col.Start(testbed.StartParams{
		Name:        name,
		LogFilePath: logPath,
	}))
	t.Cleanup(func() {
		_, err := col.Stop()
		assert.NoError(t, err)
		if t.Failed() {
			if b, err := os.ReadFile(logPath); err == nil {
				t.Logf("collector log:\n%s", b)
			}
		}
	})

	addr := net.JoinHostPort("localhost", strconv.Itoa(readyPort))
	require.Eventually(t, func() bool {
		conn, err := net.DialTimeout("tcp", addr, time.Second)
		if err != nil {
			return false
		}
		_ = conn.Close()
		return true
	}, 30*time.Second, 200*time.Millisecond, "collector did not start listening on %s", addr)
}

// newTraceClient returns an OTLP/gRPC traces client connected to the Collector's receiver.
func newTraceClient(t *testing.T, port int) ptraceotlp.GRPCClient {
	conn, err := grpc.NewClient(fmt.Sprintf("localhost:%d", port), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	return ptraceotlp.NewGRPCClient(conn)
}

// export sends a single span, setting the authorization header if it is not empty.
func export(client ptraceotlp.GRPCClient, authorization string) error {
	td := ptrace.NewTraces()
	span := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.SetName("auth-test")
	span.SetTraceID([16]byte{1})
	span.SetSpanID([8]byte{1})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if authorization != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, "authorization", authorization)
	}
	_, err := client.Export(ctx, ptraceotlp.NewExportRequestFromTraces(td))
	return err
}

func spanCount(sink *consumertest.TracesSink) int {
	n := 0
	for _, td := range sink.AllTraces() {
		n += td.SpanCount()
	}
	return n
}

func waitForSpans(t *testing.T, sink *consumertest.TracesSink, want int) {
	require.Eventually(t, func() bool {
		return spanCount(sink) >= want
	}, 30*time.Second, 200*time.Millisecond, "expected %d spans, got %d", want, spanCount(sink))
}

// TestE2E_BearerTokenAuthReceiver tests the receiver-side bearer token auth: the DT
// Collector protects its OTLP/gRPC ingestion endpoint with bearertokenauthextension
// (file-based token), and only clients presenting the correct Bearer token can deliver traces.
// The config under test is config_examples/bearertokenauth-receiver.yaml.
func TestE2E_BearerTokenAuthReceiver(t *testing.T) {
	const token = "s3cr3t-token"
	tokenFile := filepath.Join(t.TempDir(), "auth-token")
	require.NoError(t, os.WriteFile(tokenFile, []byte(token), 0o600))

	sink, p := startSink(t)
	cfg, replace := prepareConfig(t, "bearertokenauth-receiver.yaml", p)
	replace("token file", func(s string) string { return strings.Replace(s, "/etc/otelcol/auth-token", tokenFile, 1) })
	startCollector(t, "bearertokenauth-receiver", *cfg, p.grpc)

	client := newTraceClient(t, p.grpc)

	t.Run("valid", func(t *testing.T) {
		require.NoError(t, export(client, "Bearer "+token))
		waitForSpans(t, sink, 1)
	})

	rejected := []struct {
		name          string
		authorization string
	}{
		{"wrong token", "Bearer nope"},
		{"wrong scheme", "Basic " + token},
		{"missing", ""},
	}
	for _, tc := range rejected {
		t.Run(tc.name, func(t *testing.T) {
			err := export(client, tc.authorization)
			require.Error(t, err)
			assert.Equal(t, codes.Unauthenticated, status.Code(err), "unexpected error: %v", err)
		})
	}

	// The example config claims that the token is refreshed without a restart.
	t.Run("token rotation", func(t *testing.T) {
		const rotated = "rotated-token"
		require.NoError(t, os.WriteFile(tokenFile, []byte(rotated), 0o600))
		require.Eventually(t, func() bool {
			return export(client, "Bearer "+rotated) == nil
		}, 30*time.Second, 200*time.Millisecond, "rotated token was not accepted")

		err := export(client, "Bearer "+token)
		require.Error(t, err)
		assert.Equal(t, codes.Unauthenticated, status.Code(err), "unexpected error: %v", err)
	})
}

// oidcIssuer is a mock OIDC issuer serving the discovery document and the JWKS over TLS.
type oidcIssuer struct {
	server *httptest.Server
	// key is published in the JWKS, otherKey is not.
	key, otherKey *rsa.PrivateKey
	// caPath is the path of the PEM encoded server certificate.
	caPath string
}

func newOIDCIssuer(t *testing.T) *oidcIssuer {
	newKey := func() *rsa.PrivateKey {
		k, err := rsa.GenerateKey(rand.Reader, 2048)
		require.NoError(t, err)
		return k
	}
	iss := &oidcIssuer{key: newKey(), otherKey: newKey()}

	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{
			"issuer":   iss.url(),
			"jwks_uri": iss.url() + "/jwks",
		})
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, _ *http.Request) {
		jwk := jose.JSONWebKey{Key: &iss.key.PublicKey, KeyID: "test-key", Algorithm: string(jose.RS256), Use: "sig"}
		_ = json.NewEncoder(w).Encode(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{jwk}})
	})
	iss.server = httptest.NewTLSServer(mux)
	t.Cleanup(iss.server.Close)

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: iss.server.Certificate().Raw})
	iss.caPath = filepath.Join(t.TempDir(), "ca.crt")
	require.NoError(t, os.WriteFile(iss.caPath, certPEM, 0o600))
	return iss
}

func (i *oidcIssuer) url() string { return i.server.URL }

// tokenOpts adapts the claims of a minted token.
type tokenOpts struct {
	issuer   string
	audience string
	expires  time.Duration
	// unpublishedKey signs the token with a key that is not part of the JWKS.
	unpublishedKey bool
}

// mint signs a JWT. By default it is valid for the issuer and the audience of the example config.
func (i *oidcIssuer) mint(t *testing.T, mod func(*tokenOpts)) string {
	opts := tokenOpts{issuer: i.url(), audience: audience, expires: 10 * time.Minute}
	if mod != nil {
		mod(&opts)
	}

	key := i.key
	if opts.unpublishedKey {
		key = i.otherKey
	}
	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.RS256, Key: jose.JSONWebKey{Key: key, KeyID: "test-key"}},
		(&jose.SignerOptions{}).WithType("JWT"),
	)
	require.NoError(t, err)

	now := time.Now()
	payload, err := json.Marshal(map[string]any{
		"iss": opts.issuer,
		"aud": opts.audience,
		"sub": "test-subject",
		"iat": now.Unix(),
		"exp": now.Add(opts.expires).Unix(),
	})
	require.NoError(t, err)
	jws, err := signer.Sign(payload)
	require.NoError(t, err)
	token, err := jws.CompactSerialize()
	require.NoError(t, err)
	return token
}

// TestE2E_OIDCAuthRejectsInvalidTokens verifies that the oidcauthextension of the DT Collector
// accepts a valid token and rejects tokens that are missing, malformed, have a wrong audience,
// are expired, are signed with an unknown key or come from a different issuer.
// The issuer is mocked; the Kubernetes API server as issuer is covered by k8s-bearertokenauth.
// The config under test is config_examples/oidcauth-receiver.yaml.
func TestE2E_OIDCAuthRejectsInvalidTokens(t *testing.T) {
	issuer := newOIDCIssuer(t)

	sink, p := startSink(t)
	cfg, replace := prepareConfig(t, "oidcauth-receiver.yaml", p)
	replace("issuer_url", func(s string) string {
		return strings.Replace(s, "https://kubernetes.default.svc.cluster.local", issuer.url(), 1)
	})
	replace("issuer_ca_path", func(s string) string {
		return strings.Replace(s, "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt", issuer.caPath, 1)
	})
	startCollector(t, "oidcauth-receiver", *cfg, p.grpc)

	client := newTraceClient(t, p.grpc)

	// The positive control proves that the setup works, so that the rejections below can't be false passes.
	t.Run("valid", func(t *testing.T) {
		require.NoError(t, export(client, "Bearer "+issuer.mint(t, nil)))
		waitForSpans(t, sink, 1)
	})

	rejected := []struct {
		name          string
		authorization string
	}{
		{"no token", ""},
		{"not a JWT", "Bearer not.a.valid.jwt"},
		{"wrong audience", "Bearer " + issuer.mint(t, func(o *tokenOpts) { o.audience = "wrong-audience" })},
		{"expired", "Bearer " + issuer.mint(t, func(o *tokenOpts) { o.expires = -time.Hour })},
		{"unknown key", "Bearer " + issuer.mint(t, func(o *tokenOpts) { o.unpublishedKey = true })},
		{"wrong issuer", "Bearer " + issuer.mint(t, func(o *tokenOpts) { o.issuer = "https://other-issuer.example.com" })},
	}
	for _, tc := range rejected {
		t.Run(tc.name, func(t *testing.T) {
			err := export(client, tc.authorization)
			require.Error(t, err)
			assert.Equal(t, codes.Unauthenticated, status.Code(err), "unexpected error: %v", err)
		})
	}

	// Nothing but the span of the positive control may have reached the sink.
	assert.Equal(t, 1, spanCount(sink), "rejected requests must not reach the sink")
}
