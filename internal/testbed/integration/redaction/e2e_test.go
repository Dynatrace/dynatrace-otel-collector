//go:build e2e

package redaction

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/k8stest"
	oteltest "github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/oteltest"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
)

func TestE2E_RedactionProcessor(t *testing.T) {
	testDir := filepath.Join("testdata")

	k8sClient, err := k8stest.NewK8sClient()
	require.NoError(t, err)

	// Create the namespace specific for the test
	nsFile := filepath.Join(testDir, "namespace.yaml")
	buf, err := os.ReadFile(nsFile)
	require.NoErrorf(t, err, "failed to read namespace object file %s", nsFile)
	nsObj, err := k8stest.CreateObject(k8sClient, buf)
	require.NoErrorf(t, err, "failed to create k8s namespace from file %s", nsFile)

	testNs := nsObj.GetName()
	defer func() {
		require.NoErrorf(t, k8stest.DeleteObject(k8sClient, nsObj), "failed to delete namespace %s", testNs)
	}()

	tracesConsumer := new(consumertest.TracesSink)
	shutdownSinks := oteltest.StartUpSinks(t, oteltest.ReceiverSinks{
		Traces: &oteltest.TraceSinkConfig{
			Consumer: tracesConsumer,
		},
	})
	defer shutdownSinks()

	testID := uuid.NewString()[:8]
	collectorObjs := k8stest.CreateCollectorObjects(t, k8sClient, testID, filepath.Join(testDir, "collector"))

	createTeleOpts := &k8stest.TelemetrygenCreateOpts{
		ManifestsDir: filepath.Join(testDir, "telemetrygen"),
		TestID:       testID,
		OtlpEndpoint: fmt.Sprintf("otelcol-%s.%s:4317", testID, testNs),
		DataTypes:    []string{"traces"},
	}
	telemetryGenObjs, telemetryGenObjInfos := k8stest.CreateTelemetryGenObjects(t, k8sClient, createTeleOpts)
	defer func() {
		for _, obj := range append(collectorObjs, telemetryGenObjs...) {
			require.NoErrorf(t, k8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
		}
	}()

	for _, info := range telemetryGenObjInfos {
		k8stest.WaitForTelemetryGenToStart(t, k8sClient, info.Namespace, info.PodLabelSelectors, info.Workload, info.DataType)
	}

	wantEntries := 30 // Minimal number of traces to wait for.
	oteltest.WaitForTraces(t, wantEntries, tracesConsumer)

	tcs := []struct {
		name    string
		service string
		attrs   map[string]oteltest.ExpectedValue
	}{
		{
			name:    "traces-job",
			service: "test-traces-job",
			attrs: map[string]oteltest.ExpectedValue{
				"service.name":             oteltest.NewExpectedValue(oteltest.AttributeMatchTypeEqual, "test-traces-job"),
				"k8s.pod.name":             oteltest.NewExpectedValue(oteltest.AttributeMatchTypeRegex, "telemetrygen-"+testID+"-traces-job-[a-z0-9]*"),
				"k8s.pod.uid":              oteltest.NewExpectedValue(oteltest.AttributeMatchTypeEqual, "****"),
				"k8s.job.name":             oteltest.NewExpectedValue(oteltest.AttributeMatchTypeEqual, "telemetrygen-"+testID+"-traces-job"),
				"k8s.namespace.name":       oteltest.NewExpectedValue(oteltest.AttributeMatchTypeEqual, testNs),
				"k8s.cluster.uid":          oteltest.NewExpectedValue(oteltest.AttributeMatchTypeEqual, "****"),
				"redaction.redacted.count": oteltest.NewExpectedValue(oteltest.AttributeMatchTypeEqual, "2"),
				"redaction.masked.count":   oteltest.NewExpectedValue(oteltest.AttributeMatchTypeEqual, "2"),
				"redaction.ignored.count":  oteltest.NewExpectedValue(oteltest.AttributeMatchTypeEqual, "2"),
			},
		},
		{
			name:    "traces-statefulset",
			service: "test-traces-statefulset",
			attrs: map[string]oteltest.ExpectedValue{
				"service.name":             oteltest.NewExpectedValue(oteltest.AttributeMatchTypeEqual, "test-traces-statefulset"),
				"k8s.pod.name":             oteltest.NewExpectedValue(oteltest.AttributeMatchTypeEqual, "telemetrygen-"+testID+"-traces-statefulset-0"),
				"k8s.pod.uid":              oteltest.NewExpectedValue(oteltest.AttributeMatchTypeEqual, "****"),
				"k8s.statefulset.name":     oteltest.NewExpectedValue(oteltest.AttributeMatchTypeEqual, "telemetrygen-"+testID+"-traces-statefulset"),
				"k8s.namespace.name":       oteltest.NewExpectedValue(oteltest.AttributeMatchTypeEqual, testNs),
				"k8s.cluster.uid":          oteltest.NewExpectedValue(oteltest.AttributeMatchTypeEqual, "****"),
				"redaction.redacted.count": oteltest.NewExpectedValue(oteltest.AttributeMatchTypeEqual, "2"),
				"redaction.masked.count":   oteltest.NewExpectedValue(oteltest.AttributeMatchTypeEqual, "2"),
				"redaction.ignored.count":  oteltest.NewExpectedValue(oteltest.AttributeMatchTypeEqual, "2"),
			},
		},
		{
			name:    "traces-deployment",
			service: "test-traces-deployment",
			attrs: map[string]oteltest.ExpectedValue{
				"service.name":             oteltest.NewExpectedValue(oteltest.AttributeMatchTypeEqual, "test-traces-deployment"),
				"k8s.pod.name":             oteltest.NewExpectedValue(oteltest.AttributeMatchTypeRegex, "telemetrygen-"+testID+"-traces-deployment-[a-z0-9]*-[a-z0-9]*"),
				"k8s.pod.uid":              oteltest.NewExpectedValue(oteltest.AttributeMatchTypeEqual, "****"),
				"k8s.deployment.name":      oteltest.NewExpectedValue(oteltest.AttributeMatchTypeEqual, "telemetrygen-"+testID+"-traces-deployment"),
				"k8s.namespace.name":       oteltest.NewExpectedValue(oteltest.AttributeMatchTypeEqual, testNs),
				"k8s.cluster.uid":          oteltest.NewExpectedValue(oteltest.AttributeMatchTypeEqual, "****"),
				"redaction.redacted.count": oteltest.NewExpectedValue(oteltest.AttributeMatchTypeEqual, "2"),
				"redaction.masked.count":   oteltest.NewExpectedValue(oteltest.AttributeMatchTypeEqual, "2"),
				"redaction.ignored.count":  oteltest.NewExpectedValue(oteltest.AttributeMatchTypeEqual, "2"),
			},
		},
		{
			name:    "traces-daemonset",
			service: "test-traces-daemonset",
			attrs: map[string]oteltest.ExpectedValue{
				"service.name":             oteltest.NewExpectedValue(oteltest.AttributeMatchTypeEqual, "test-traces-daemonset"),
				"k8s.pod.name":             oteltest.NewExpectedValue(oteltest.AttributeMatchTypeRegex, "telemetrygen-"+testID+"-traces-daemonset-[a-z0-9]*"),
				"k8s.pod.uid":              oteltest.NewExpectedValue(oteltest.AttributeMatchTypeEqual, "****"),
				"k8s.daemonset.name":       oteltest.NewExpectedValue(oteltest.AttributeMatchTypeEqual, "telemetrygen-"+testID+"-traces-daemonset"),
				"k8s.namespace.name":       oteltest.NewExpectedValue(oteltest.AttributeMatchTypeEqual, testNs),
				"k8s.cluster.uid":          oteltest.NewExpectedValue(oteltest.AttributeMatchTypeEqual, "****"),
				"redaction.redacted.count": oteltest.NewExpectedValue(oteltest.AttributeMatchTypeEqual, "2"),
				"redaction.masked.count":   oteltest.NewExpectedValue(oteltest.AttributeMatchTypeEqual, "2"),
				"redaction.ignored.count":  oteltest.NewExpectedValue(oteltest.AttributeMatchTypeEqual, "2"),
			},
		},
	}

	for _, tc := range tcs {
		oteltest.ScanTracesForAttributes(t, tracesConsumer, tc.service, tc.attrs, nil)
	}

}