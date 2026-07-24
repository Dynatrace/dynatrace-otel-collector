// Copyright Dynatrace LLC
// SPDX-License-Identifier: Apache-2.0

//go:build e2e

package bearertokenauth

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

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
