//go:build e2e

package k8senrichment

import (
	"fmt"
	"github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/k8s"
	oteltest "github.com/Dynatrace/dynatrace-otel-collector/internal/testcommon/otel"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"os"
	"path/filepath"
	"testing"
)

const (
	equal = iota
	regex
	exist
	uidRe = "[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}"
)

type expectedValue struct {
	mode  int
	value string
}

// TestE2E_ClusterRBAC tests the "Enrich from Kubernetes" use case
// See: https://docs.dynatrace.com/docs/shortlink/otel-collector-cases-k8s-enrich
func TestE2E_ClusterRBAC(t *testing.T) {
	testDir := filepath.Join("testdata")

	k8sClient, err := k8s.NewK8sClient()
	require.NoError(t, err)

	// Create the namespace specific for the test
	nsFile := filepath.Join(testDir, "namespace.yaml")
	buf, err := os.ReadFile(nsFile)
	require.NoErrorf(t, err, "failed to read namespace object file %s", nsFile)
	nsObj, err := k8s.CreateObject(k8sClient, buf)
	require.NoErrorf(t, err, "failed to create k8s namespace from file %s", nsFile)

	testNs := nsObj.GetName()
	defer func() {
		require.NoErrorf(t, k8s.DeleteObject(k8sClient, nsObj), "failed to delete namespace %s", testNs)
	}()

	tracesConsumer := new(consumertest.TracesSink)
	shutdownSinks := oteltest.StartUpSinks(t, oteltest.ReceiverSinks{Traces: tracesConsumer})
	defer shutdownSinks()

	testID := uuid.NewString()[:8]
	collectorObjs := k8s.CreateCollectorObjects(t, k8sClient, testID, filepath.Join(testDir, "collector"))
	createTeleOpts := &k8s.TelemetrygenCreateOpts{
		ManifestsDir: filepath.Join(testDir, "telemetrygen"),
		TestID:       testID,
		OtlpEndpoint: fmt.Sprintf("otelcol-%s.%s:4317", testID, testNs),
		DataTypes:    []string{"metrics", "logs", "traces"},
	}
	telemetryGenObjs, telemetryGenObjInfos := k8s.CreateTelemetryGenObjects(t, k8sClient, createTeleOpts)
	defer func() {
		for _, obj := range append(collectorObjs, telemetryGenObjs...) {
			require.NoErrorf(t, k8s.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
		}
	}()

	for _, info := range telemetryGenObjInfos {
		k8s.WaitForTelemetryGenToStart(t, k8sClient, info.Namespace, info.PodLabelSelectors, info.Workload, info.DataType)
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
				"k8s.pod.name":             oteltest.NewExpectedValue(oteltest.AttributeMatchTypeRegex, "telemetrygen-"+testID+"-traces-job-[a-z0-9]*"),
				"k8s.pod.uid":              oteltest.NewExpectedValue(oteltest.AttributeMatchTypeRegex, oteltest.UidRe),
				"k8s.job.name":             oteltest.NewExpectedValue(oteltest.AttributeMatchTypeEqual, "telemetrygen-"+testID+"-traces-job"),
				"k8s.namespace.name":       oteltest.NewExpectedValue(oteltest.AttributeMatchTypeEqual, testNs),
				"k8s.node.name":            oteltest.NewExpectedValue(exist, ""),
				"k8s.cluster.uid":          oteltest.NewExpectedValue(oteltest.AttributeMatchTypeRegex, oteltest.UidRe),
				"dt.kubernetes.cluster.id": oteltest.NewExpectedValue(oteltest.AttributeMatchTypeRegex, oteltest.UidRe),
			},
		},
		{
			name:    "traces-statefulset",
			service: "test-traces-statefulset",
			attrs: map[string]oteltest.ExpectedValue{
				"k8s.pod.name":                oteltest.NewExpectedValue(oteltest.AttributeMatchTypeEqual, "telemetrygen-"+testID+"-traces-statefulset-0"),
				"k8s.pod.uid":                 oteltest.NewExpectedValue(oteltest.AttributeMatchTypeRegex, oteltest.UidRe),
				"k8s.statefulset.name":        oteltest.NewExpectedValue(oteltest.AttributeMatchTypeEqual, "telemetrygen-"+testID+"-traces-statefulset"),
				"dt.kubernetes.workload.name": oteltest.NewExpectedValue(oteltest.AttributeMatchTypeEqual, "telemetrygen-"+testID+"-traces-statefulset"),
				"dt.kubernetes.workload.kind": oteltest.NewExpectedValue(oteltest.AttributeMatchTypeEqual, "statefulset"),
				"k8s.namespace.name":          oteltest.NewExpectedValue(oteltest.AttributeMatchTypeEqual, testNs),
				"k8s.node.name":               oteltest.NewExpectedValue(oteltest.AttributeMatchTypeExist, ""),
				"k8s.cluster.uid":             oteltest.NewExpectedValue(oteltest.AttributeMatchTypeRegex, oteltest.UidRe),
				"dt.kubernetes.cluster.id":    oteltest.NewExpectedValue(oteltest.AttributeMatchTypeRegex, oteltest.UidRe),
			},
		},
		{
			name:    "traces-deployment",
			service: "test-traces-deployment",
			attrs: map[string]oteltest.ExpectedValue{
				"k8s.pod.name":                oteltest.NewExpectedValue(oteltest.AttributeMatchTypeRegex, "telemetrygen-"+testID+"-traces-deployment-[a-z0-9]*-[a-z0-9]*"),
				"k8s.pod.uid":                 oteltest.NewExpectedValue(oteltest.AttributeMatchTypeRegex, oteltest.UidRe),
				"k8s.deployment.name":         oteltest.NewExpectedValue(oteltest.AttributeMatchTypeEqual, "telemetrygen-"+testID+"-traces-deployment"),
				"dt.kubernetes.workload.name": oteltest.NewExpectedValue(oteltest.AttributeMatchTypeEqual, "telemetrygen-"+testID+"-traces-deployment"),
				"dt.kubernetes.workload.kind": oteltest.NewExpectedValue(oteltest.AttributeMatchTypeEqual, "deployment"),
				"k8s.namespace.name":          oteltest.NewExpectedValue(oteltest.AttributeMatchTypeEqual, testNs),
				"k8s.node.name":               oteltest.NewExpectedValue(oteltest.AttributeMatchTypeExist, ""),
				"k8s.cluster.uid":             oteltest.NewExpectedValue(oteltest.AttributeMatchTypeRegex, oteltest.UidRe),
				"dt.kubernetes.cluster.id":    oteltest.NewExpectedValue(oteltest.AttributeMatchTypeRegex, oteltest.UidRe),
			},
		},
		{
			name:    "traces-daemonset",
			service: "test-traces-daemonset",
			attrs: map[string]oteltest.ExpectedValue{
				"k8s.pod.name":                oteltest.NewExpectedValue(oteltest.AttributeMatchTypeRegex, "telemetrygen-"+testID+"-traces-daemonset-[a-z0-9]*"),
				"k8s.pod.uid":                 oteltest.NewExpectedValue(oteltest.AttributeMatchTypeRegex, oteltest.UidRe),
				"k8s.daemonset.name":          oteltest.NewExpectedValue(oteltest.AttributeMatchTypeEqual, "telemetrygen-"+testID+"-traces-daemonset"),
				"dt.kubernetes.workload.name": oteltest.NewExpectedValue(oteltest.AttributeMatchTypeEqual, "telemetrygen-"+testID+"-traces-daemonset"),
				"dt.kubernetes.workload.kind": oteltest.NewExpectedValue(oteltest.AttributeMatchTypeEqual, "daemonset"),
				"k8s.namespace.name":          oteltest.NewExpectedValue(oteltest.AttributeMatchTypeEqual, testNs),
				"k8s.node.name":               oteltest.NewExpectedValue(oteltest.AttributeMatchTypeExist, ""),
				"k8s.cluster.uid":             oteltest.NewExpectedValue(oteltest.AttributeMatchTypeRegex, oteltest.UidRe),
				"dt.kubernetes.cluster.id":    oteltest.NewExpectedValue(oteltest.AttributeMatchTypeRegex, oteltest.UidRe),
			},
		},
	}

	for _, tc := range tcs {
		oteltest.ScanTracesForAttributes(t, tracesConsumer, tc.service, tc.attrs, nil)
	}
}
