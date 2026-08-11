// Copyright Dynatrace LLC
// SPDX-License-Identifier: Apache-2.0

//go:build e2e

package bearertokenauth

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/k8stest"
	oteltest "github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/oteltest"
	"github.com/google/uuid"
	otelk8stest "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/xk8stest"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
)

// TestE2E_BearerTokenAuth tests the WIF Phase I direct token presentation flow:
// a DT collector (sender) reads a projected service-account token from a file and
// attaches it as a Bearer credential on the OTLP/HTTP exporter, while a contrib
// collector (verifier) validates the token via the oidcauthextension before
// forwarding telemetry to the test sink.
//
// NOTE: the configs in testdata/ are intentionally NOT placed in config_examples/
// until WIF is generally available to customers.
func TestE2E_BearerTokenAuth(t *testing.T) {
	testDir := filepath.Join("testdata")

	kubeconfigPath := k8stest.TestKubeConfig
	if kubeConfigFromEnv := os.Getenv(k8stest.KubeConfigEnvVar); kubeConfigFromEnv != "" {
		kubeconfigPath = kubeConfigFromEnv
	}

	k8sClient, err := otelk8stest.NewK8sClient(kubeconfigPath)
	require.NoError(t, err)

	// Create the namespace specific for the test.
	nsFile := filepath.Join(testDir, "namespace.yaml")
	buf, err := os.ReadFile(nsFile)
	require.NoErrorf(t, err, "failed to read namespace object file %s", nsFile)
	nsObj, err := otelk8stest.CreateObject(k8sClient, buf)
	require.NoErrorf(t, err, "failed to create k8s namespace from file %s", nsFile)

	testNs := nsObj.GetName()
	defer func() {
		require.NoErrorf(t, otelk8stest.DeleteObject(k8sClient, nsObj), "failed to delete namespace %s", testNs)
	}()

	// Start the local OTLP sink that the verifier collector will export data to.
	tracesConsumer := new(consumertest.TracesSink)
	shutdownSinks := oteltest.StartUpSinks(t, oteltest.ReceiverSinks{
		Traces: []*oteltest.TraceSinkConfig{
			{
				Consumer: tracesConsumer,
			},
		},
	})
	defer shutdownSinks()

	host := otelk8stest.HostEndpoint(t)

	// Deploy the verifier first so its service name is known before the sender config is rendered.
	verifierTestID := uuid.NewString()[:8]
	verifierSvcEndpoint := fmt.Sprintf("http://otelcol-%s.%s:8080", verifierTestID, testNs)

	// Load the verifier config (oidcauthextension validating the token; exports to test sink).
	verifierConfigPath := filepath.Join(testDir, "verifier-config.yaml")
	verifierConfig, err := k8stest.GetCollectorConfig(verifierConfigPath, k8stest.ConfigTemplate{
		Host: host,
	})
	require.NoErrorf(t, err, "failed to read verifier config from %s", verifierConfigPath)

	verifierObjs := otelk8stest.CreateCollectorObjects(
		t,
		k8sClient,
		verifierTestID,
		filepath.Join(testDir, "collector-verifier"),
		map[string]string{
			"CollectorConfig": verifierConfig,
		},
		host,
	)
	defer func() {
		for _, obj := range verifierObjs {
			require.NoErrorf(t, otelk8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
		}
	}()

	// Load the sender config, overriding the exporter endpoint with the verifier's service address.
	senderTestID := uuid.NewString()[:8]
	senderConfigPath := filepath.Join(testDir, "sender-config.yaml")
	endpointOverlay := fmt.Sprintf("exporters:\n  otlphttp:\n    endpoint: %s\n    tls:\n      insecure: true\n", verifierSvcEndpoint)
	senderConfig, err := k8stest.GetCollectorConfig(senderConfigPath, k8stest.ConfigTemplate{
		Host:      host,
		Templates: []string{endpointOverlay},
	})
	require.NoErrorf(t, err, "failed to read sender config from %s", senderConfigPath)

	senderObjs := otelk8stest.CreateCollectorObjects(
		t,
		k8sClient,
		senderTestID,
		filepath.Join(testDir, "collector-sender"),
		map[string]string{
			"ContainerRegistry": os.Getenv("CONTAINER_REGISTRY"),
			"CollectorConfig":   senderConfig,
		},
		host,
	)
	defer func() {
		for _, obj := range senderObjs {
			require.NoErrorf(t, otelk8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
		}
	}()

	// Send traces through the two-collector chain via telemetrygen.
	createTeleOpts := &otelk8stest.TelemetrygenCreateOpts{
		ManifestsDir: filepath.Join(testDir, "telemetrygen"),
		TestID:       senderTestID,
		OtlpEndpoint: fmt.Sprintf("otelcol-%s.%s:4317", senderTestID, testNs),
		DataTypes:    []string{"traces"},
	}
	telemetryGenObjs, telemetryGenObjInfos := otelk8stest.CreateTelemetryGenObjects(t, k8sClient, createTeleOpts)
	defer func() {
		for _, obj := range telemetryGenObjs {
			require.NoErrorf(t, otelk8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
		}
	}()

	for _, info := range telemetryGenObjInfos {
		otelk8stest.WaitForTelemetryGenToStart(t, k8sClient, info.Namespace, info.PodLabelSelectors, info.Workload, info.DataType)
	}

	// If bearer token auth worked end-to-end, traces will arrive at the sink.
	wantEntries := 5
	oteltest.WaitForTraces(t, wantEntries, tracesConsumer)
}

// TestE2E_BearerTokenAuthReceiver tests the receiver-side bearer token auth:
// the DT collector protects its OTLP/gRPC ingestion endpoint with
// bearertokenauthextension (file-based token), and only telemetrygen
// presenting the correct Bearer token can deliver traces through it.
// The config under test is config_examples/bearertokenauth-receiver.yaml.
func TestE2E_BearerTokenAuthReceiver(t *testing.T) {
	testDir := filepath.Join("testdata")
	configExamplesDir := "../../../../config_examples"

	kubeconfigPath := k8stest.TestKubeConfig
	if kubeConfigFromEnv := os.Getenv(k8stest.KubeConfigEnvVar); kubeConfigFromEnv != "" {
		kubeconfigPath = kubeConfigFromEnv
	}

	k8sClient, err := otelk8stest.NewK8sClient(kubeconfigPath)
	require.NoError(t, err)

	// Create the namespace specific for the test.
	nsFile := filepath.Join(testDir, "namespace.yaml")
	buf, err := os.ReadFile(nsFile)
	require.NoErrorf(t, err, "failed to read namespace object file %s", nsFile)
	nsObj, err := otelk8stest.CreateObject(k8sClient, buf)
	require.NoErrorf(t, err, "failed to create k8s namespace from file %s", nsFile)

	testNs := nsObj.GetName()
	defer func() {
		require.NoErrorf(t, otelk8stest.DeleteObject(k8sClient, nsObj), "failed to delete namespace %s", testNs)
	}()

	// Start the local OTLP sink the receiver collector will forward data to.
	tracesConsumer := new(consumertest.TracesSink)
	shutdownSinks := oteltest.StartUpSinks(t, oteltest.ReceiverSinks{
		Traces: []*oteltest.TraceSinkConfig{
			{
				Consumer: tracesConsumer,
			},
		},
	})
	defer shutdownSinks()

	host := otelk8stest.HostEndpoint(t)

	// Load the receiver config from config_examples.
	// GetCollectorConfig replaces ${env:DT_ENDPOINT} → http://<HOST>:4318 (the test sink)
	// and ${env:DT_API_TOKEN} → "" (not needed for the sink).
	// The overlay strips the DT-specific Authorization header and disables retry so
	// export failures surface immediately during the test.
	receiverConfigPath := filepath.Join(configExamplesDir, "bearertokenauth-receiver.yaml")
	exporterOverlay := "exporters:\n  otlp_http:\n    headers: {}\n    retry_on_failure:\n      enabled: false\n    sending_queue:\n      enabled: false\n"
	receiverConfig, err := k8stest.GetCollectorConfig(receiverConfigPath, k8stest.ConfigTemplate{
		Host:      host,
		Templates: []string{exporterOverlay},
	})
	require.NoErrorf(t, err, "failed to read receiver config from %s", receiverConfigPath)

	receiverTestID := uuid.NewString()[:8]
	receiverObjs := otelk8stest.CreateCollectorObjects(
		t,
		k8sClient,
		receiverTestID,
		filepath.Join(testDir, "collector-receiver"),
		map[string]string{
			"ContainerRegistry": os.Getenv("CONTAINER_REGISTRY"),
			"CollectorConfig":   receiverConfig,
		},
		host,
	)
	defer func() {
		for _, obj := range receiverObjs {
			require.NoErrorf(t, otelk8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
		}
	}()

	// telemetrygen sends traces with the correct Bearer token in the Authorization header.
	// The token value matches what is stored in the collector-receiver auth-token ConfigMap.
	createTeleOpts := &otelk8stest.TelemetrygenCreateOpts{
		ManifestsDir: filepath.Join(testDir, "telemetrygen-receiver"),
		TestID:       receiverTestID,
		OtlpEndpoint: fmt.Sprintf("otelcol-%s.%s:4317", receiverTestID, testNs),
		DataTypes:    []string{"traces"},
	}
	telemetryGenObjs, telemetryGenObjInfos := otelk8stest.CreateTelemetryGenObjects(t, k8sClient, createTeleOpts)
	defer func() {
		for _, obj := range telemetryGenObjs {
			require.NoErrorf(t, otelk8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
		}
	}()

	for _, info := range telemetryGenObjInfos {
		otelk8stest.WaitForTelemetryGenToStart(t, k8sClient, info.Namespace, info.PodLabelSelectors, info.Workload, info.DataType)
	}

	// Traces arrive only if the Bearer token was accepted by the receiver.
	wantEntries := 5
	oteltest.WaitForTraces(t, wantEntries, tracesConsumer)
}

// TestE2E_OIDCAuthRejectsInvalidTokens verifies the negative cases of the WIF
// agent→gateway flow: the oidcauthextension on the gateway must reject tokens
// that are missing, carry a wrong audience, or are syntactically invalid.
// In all three cases zero traces must reach the downstream sink.
func TestE2E_OIDCAuthRejectsInvalidTokens(t *testing.T) {
	testDir := filepath.Join("testdata")

	kubeconfigPath := k8stest.TestKubeConfig
	if kubeConfigFromEnv := os.Getenv(k8stest.KubeConfigEnvVar); kubeConfigFromEnv != "" {
		kubeconfigPath = kubeConfigFromEnv
	}

	k8sClient, err := otelk8stest.NewK8sClient(kubeconfigPath)
	require.NoError(t, err)

	nsFile := filepath.Join(testDir, "namespace.yaml")
	buf, err := os.ReadFile(nsFile)
	require.NoErrorf(t, err, "failed to read namespace object file %s", nsFile)
	nsObj, err := otelk8stest.CreateObject(k8sClient, buf)
	require.NoErrorf(t, err, "failed to create k8s namespace from file %s", nsFile)

	testNs := nsObj.GetName()
	defer func() {
		require.NoErrorf(t, otelk8stest.DeleteObject(k8sClient, nsObj), "failed to delete namespace %s", testNs)
	}()

	// Sink that the verifier forwards accepted telemetry to.
	// We assert it stays empty for the duration of the test.
	tracesConsumer := new(consumertest.TracesSink)
	shutdownSinks := oteltest.StartUpSinks(t, oteltest.ReceiverSinks{
		Traces: []*oteltest.TraceSinkConfig{{Consumer: tracesConsumer}},
	})
	defer shutdownSinks()

	host := otelk8stest.HostEndpoint(t)

	// Deploy the verifier (oidcauthextension, audience: dynatrace-wif-test).
	verifierTestID := uuid.NewString()[:8]
	verifierSvcEndpoint := fmt.Sprintf("http://otelcol-%s.%s:8080", verifierTestID, testNs)

	verifierConfig, err := k8stest.GetCollectorConfig(
		filepath.Join(testDir, "verifier-config.yaml"),
		k8stest.ConfigTemplate{Host: host},
	)
	require.NoErrorf(t, err, "failed to read verifier config")

	verifierObjs := otelk8stest.CreateCollectorObjects(
		t, k8sClient, verifierTestID,
		filepath.Join(testDir, "collector-verifier"),
		map[string]string{"CollectorConfig": verifierConfig},
		host,
	)
	defer func() {
		for _, obj := range verifierObjs {
			require.NoErrorf(t, otelk8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
		}
	}()

	// Overlay that redirects each bad sender's exporter to the verifier.
	endpointOverlay := fmt.Sprintf(
		"exporters:\n  otlphttp:\n    endpoint: %s\n    tls:\n      insecure: true\n",
		verifierSvcEndpoint,
	)

	// Three negative cases.  Each uses a distinct rejection path in oidcauthextension:
	//   wrong-audience  — valid cluster-issued JWT, but aud != "dynatrace-wif-test"
	//   invalid-token   — static string that is not a parseable JWT (no issuer extraction)
	//   no-token        — no Authorization header at all
	cases := []struct {
		name            string
		configPath      string
		saTokenAudience string
	}{
		{
			name:            "wrong-audience",
			configPath:      filepath.Join(testDir, "sender-wrong-audience-config.yaml"),
			saTokenAudience: "wrong-audience-wif-test",
		},
		{
			name:            "invalid-token",
			configPath:      filepath.Join(testDir, "sender-invalid-token-config.yaml"),
			saTokenAudience: "unused",
		},
		{
			name:            "no-token",
			configPath:      filepath.Join(testDir, "sender-no-token-config.yaml"),
			saTokenAudience: "unused",
		},
	}

	type senderInfo struct {
		name   string
		testID string
	}
	var senderInfos []senderInfo

	var allTelemetryGenObjInfos []*otelk8stest.TelemetrygenObjInfo
	for _, tc := range cases {
		senderTestID := uuid.NewString()[:8]
		senderInfos = append(senderInfos, senderInfo{name: tc.name, testID: senderTestID})

		senderConfig, err := k8stest.GetCollectorConfig(tc.configPath, k8stest.ConfigTemplate{
			Host:      host,
			Templates: []string{endpointOverlay},
		})
		require.NoErrorf(t, err, "failed to read sender config for case %s", tc.name)

		senderObjs := otelk8stest.CreateCollectorObjects(
			t, k8sClient, senderTestID,
			filepath.Join(testDir, "collector-sender-negtest"),
			map[string]string{
				"ContainerRegistry": os.Getenv("CONTAINER_REGISTRY"),
				"CollectorConfig":   senderConfig,
				"SATokenAudience":   tc.saTokenAudience,
			},
			host,
		)
		defer func() {
			for _, obj := range senderObjs {
				require.NoErrorf(t, otelk8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
			}
		}()

		teleOpts := &otelk8stest.TelemetrygenCreateOpts{
			ManifestsDir: filepath.Join(testDir, "telemetrygen"),
			TestID:       senderTestID,
			OtlpEndpoint: fmt.Sprintf("otelcol-%s.%s:4317", senderTestID, testNs),
			DataTypes:    []string{"traces"},
		}
		teleObjs, teleInfos := otelk8stest.CreateTelemetryGenObjects(t, k8sClient, teleOpts)
		allTelemetryGenObjInfos = append(allTelemetryGenObjInfos, teleInfos...)
		defer func() {
			for _, obj := range teleObjs {
				require.NoErrorf(t, otelk8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
			}
		}()
	}

	// Wait for all telemetrygen pods to confirm they are running and attempting to send.
	for _, info := range allTelemetryGenObjInfos {
		otelk8stest.WaitForTelemetryGenToStart(t, k8sClient, info.Namespace, info.PodLabelSelectors, info.Workload, info.DataType)
	}

	// Assert the verifier rejected every request: the sink must stay empty.
	require.Never(t,
		func() bool { return len(tracesConsumer.AllTraces()) > 0 },
		90*time.Second, 5*time.Second,
		"oidcauthextension must reject all invalid tokens; no traces should reach the sink",
	)

	// Confirm the verifier actually received and logged a rejection for each
	// negative case — guards against a false pass where senders never sent anything.
	verifierLogs := otelk8stest.FetchPodLogs(t, k8sClient, testNs, map[string]any{
		"app.kubernetes.io/name":     "opentelemetry-collector",
		"app.kubernetes.io/instance": "otelcol-" + verifierTestID,
	})
	require.Contains(t, verifierLogs, "Authentication failed: missing or empty header",
		"verifier must log rejection for missing Authorization header (no-token case)")
	require.Contains(t, verifierLogs, "Authentication failed: could not parse issuer from token",
		"verifier must log rejection for non-JWT bearer value (invalid-token case)")
	require.Contains(t, verifierLogs, "Authentication failed: token verification failed",
		"verifier must log rejection for wrong-audience token")

	// Confirm each sender actually reached the verifier and received a 401:
	// exporterhelper logs "Permanent error" for non-retryable HTTP responses.
	for _, si := range senderInfos {
		senderLogs := otelk8stest.FetchPodLogs(t, k8sClient, testNs, map[string]any{
			"app.kubernetes.io/name":     "opentelemetry-collector",
			"app.kubernetes.io/instance": "otelcol-" + si.testID,
		})
		require.Contains(t, senderLogs, "Permanent error",
			"sender %q must log a permanent export error after 401 from verifier", si.name)
	}
}
