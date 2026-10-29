// Copyright Dynatrace LLC
// SPDX-License-Identifier: Apache-2.0

package bearertokenauth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"encoding/pem"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/oteltest"
	"github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/testutil"
	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/coreos/go-oidc/v3/oidc/oidctest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// audience is the audience configured in config_examples/oidcauth-receiver.yaml.
const audience = "my-collector"

// startCollector starts the DT Collector with an example config whose OTLP receiver is protected
// by an auth extension. edit adapts the config to the test on top of the common local overrides.
// It returns the port of the OTLP/gRPC receiver and the sink the Collector exports to.
func startCollector(t *testing.T, example string, edit func(*testutil.Config)) (int, *consumertest.TracesSink) {
	sink, sinkPorts := oteltest.StartTracesSink(t)
	grpcPort := testutil.GetAvailablePort(t)
	httpPort := testutil.GetAvailablePort(t)
	healthPort := testutil.GetAvailablePort(t)

	cfg := testutil.LoadConfigExample(t, example).
		Replace("grpc receiver port", func(s string) string { return testutil.ReplaceOtlpGrpcReceiverPort(s, grpcPort) }).
		Replace("http receiver port", func(s string) string { return testutil.ReplaceOtlpHttpReceiverPort(s, httpPort) }).
		Replace("health_check port", func(s string) string { return testutil.ReplaceHealthCheckPort(s, healthPort) }).
		Replace("exporter endpoint", func(s string) string { return testutil.ReplaceDynatraceExporterEndpoint(s, sinkPorts.Http) }).
		Replace("sending_queue and retry_on_failure", func(s string) string { return testutil.DisableSendingQueueAndRetry(s, "otlp_http") }).
		Replace("internal metrics", testutil.DisableInternalMetrics)
	edit(cfg)

	testutil.StartCollector(t, testutil.CollectorParams{
		Name:      strings.TrimSuffix(example, ".yaml"),
		Config:    cfg.String(),
		ReadyPort: grpcPort,
	})
	return grpcPort, sink
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

// TestE2E_BearerTokenAuthReceiver tests the receiver-side bearer token auth: the DT
// Collector protects its OTLP/gRPC ingestion endpoint with bearertokenauthextension
// (file-based token), and only clients presenting the correct Bearer token can deliver traces.
// The config under test is config_examples/bearertokenauth-receiver.yaml.
func TestE2E_BearerTokenAuthReceiver(t *testing.T) {
	const token = "s3cr3t-token"
	tokenFile := filepath.Join(t.TempDir(), "auth-token")
	require.NoError(t, os.WriteFile(tokenFile, []byte(token), 0o600))

	grpcPort, sink := startCollector(t, "bearertokenauth-receiver.yaml", func(c *testutil.Config) {
		c.ReplaceString("/etc/otelcol/auth-token", tokenFile)
	})
	client := oteltest.NewTracesClient(t, grpcPort)

	t.Run("valid", func(t *testing.T) {
		require.NoError(t, export(client, "Bearer "+token))
		oteltest.WaitForTraces(t, 1, sink)
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

// keyID identifies the signing key of the mock OIDC issuer.
const keyID = "test-key"

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

	handler := &oidctest.Server{PublicKeys: []oidctest.PublicKey{
		{PublicKey: &iss.key.PublicKey, KeyID: keyID, Algorithm: oidc.RS256},
	}}
	iss.server = httptest.NewTLSServer(handler)
	t.Cleanup(iss.server.Close)
	handler.SetIssuer(iss.server.URL)

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
	now := time.Now()
	payload, err := json.Marshal(map[string]any{
		"iss": opts.issuer,
		"aud": opts.audience,
		"sub": "test-subject",
		"iat": now.Unix(),
		"exp": now.Add(opts.expires).Unix(),
	})
	require.NoError(t, err)
	return oidctest.SignIDToken(key, keyID, oidc.RS256, string(payload))
}

// TestE2E_OIDCAuthRejectsInvalidTokens verifies that the oidcauthextension of the DT Collector
// accepts a valid token and rejects tokens that are missing, malformed, have a wrong audience,
// are expired, are signed with an unknown key or come from a different issuer.
// The issuer is mocked; the Kubernetes API server as issuer is covered by k8sbearertokenauth.
// The config under test is config_examples/oidcauth-receiver.yaml.
func TestE2E_OIDCAuthRejectsInvalidTokens(t *testing.T) {
	issuer := newOIDCIssuer(t)

	grpcPort, sink := startCollector(t, "oidcauth-receiver.yaml", func(c *testutil.Config) {
		c.ReplaceString("https://kubernetes.default.svc.cluster.local", issuer.url()).
			ReplaceString("/var/run/secrets/kubernetes.io/serviceaccount/ca.crt", issuer.caPath)
	})
	client := oteltest.NewTracesClient(t, grpcPort)

	// The positive control proves that the setup works, so that the rejections below can't be false passes.
	t.Run("valid", func(t *testing.T) {
		require.NoError(t, export(client, "Bearer "+issuer.mint(t, nil)))
		oteltest.WaitForTraces(t, 1, sink)
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
	assert.Equal(t, 1, sink.SpanCount(), "rejected requests must not reach the sink")
}
