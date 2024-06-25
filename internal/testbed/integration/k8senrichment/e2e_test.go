//go:build e2e

package k8senrichment

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/receiver/otlpreceiver"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/multierr"

	"github.com/Dynatrace/dynatrace-otel-collector/internal/k8stest"
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

func newExpectedValue(mode int, value string) *expectedValue {
	return &expectedValue{
		mode:  mode,
		value: value,
	}
}

// TestE2E_ClusterRBAC tests the "Enrich from Kubernetes" use case
// See: https://docs.dynatrace.com/docs/shortlink/otel-collector-cases-k8s-enrich
func TestE2E_ClusterRBAC(t *testing.T) {
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
	shutdownSinks := startUpSinks(t, tracesConsumer)
	defer shutdownSinks()

	testID := uuid.NewString()[:8]
	collectorObjs := k8stest.CreateCollectorObjects(t, k8sClient, testID, filepath.Join(testDir, "collector"), os.Getenv("CONTAINER_REGISTRY"))
	createTeleOpts := &k8stest.TelemetrygenCreateOpts{
		ManifestsDir: filepath.Join(testDir, "telemetrygen"),
		TestID:       testID,
		OtlpEndpoint: fmt.Sprintf("otelcol-%s.%s:4317", testID, testNs),
		DataTypes:    []string{"metrics", "logs", "traces"},
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
	waitForData(t, wantEntries, tracesConsumer)

	tcs := []struct {
		name    string
		service string
		attrs   map[string]*expectedValue
	}{
		{
			name:    "traces-job",
			service: "test-traces-job",
			attrs: map[string]*expectedValue{
				"k8s.pod.name":             newExpectedValue(regex, "telemetrygen-"+testID+"-traces-job-[a-z0-9]*"),
				"k8s.pod.uid":              newExpectedValue(regex, uidRe),
				"k8s.job.name":             newExpectedValue(equal, "telemetrygen-"+testID+"-traces-job"),
				"k8s.namespace.name":       newExpectedValue(equal, testNs),
				"k8s.node.name":            newExpectedValue(exist, ""),
				"k8s.cluster.uid":          newExpectedValue(regex, uidRe),
				"dt.kubernetes.cluster.id": newExpectedValue(regex, uidRe),
			},
		},
		{
			name:    "traces-statefulset",
			service: "test-traces-statefulset",
			attrs: map[string]*expectedValue{
				"k8s.pod.name":                newExpectedValue(equal, "telemetrygen-"+testID+"-traces-statefulset-0"),
				"k8s.pod.uid":                 newExpectedValue(regex, uidRe),
				"k8s.statefulset.name":        newExpectedValue(equal, "telemetrygen-"+testID+"-traces-statefulset"),
				"dt.kubernetes.workload.name": newExpectedValue(equal, "telemetrygen-"+testID+"-traces-statefulset"),
				"dt.kubernetes.workload.kind": newExpectedValue(equal, "statefulset"),
				"k8s.namespace.name":          newExpectedValue(equal, testNs),
				"k8s.node.name":               newExpectedValue(exist, ""),
				"k8s.cluster.uid":             newExpectedValue(regex, uidRe),
				"dt.kubernetes.cluster.id":    newExpectedValue(regex, uidRe),
			},
		},
		{
			name:    "traces-deployment",
			service: "test-traces-deployment",
			attrs: map[string]*expectedValue{
				"k8s.pod.name":                newExpectedValue(regex, "telemetrygen-"+testID+"-traces-deployment-[a-z0-9]*-[a-z0-9]*"),
				"k8s.pod.uid":                 newExpectedValue(regex, uidRe),
				"k8s.deployment.name":         newExpectedValue(equal, "telemetrygen-"+testID+"-traces-deployment"),
				"dt.kubernetes.workload.name": newExpectedValue(equal, "telemetrygen-"+testID+"-traces-deployment"),
				"dt.kubernetes.workload.kind": newExpectedValue(equal, "deployment"),
				"k8s.namespace.name":          newExpectedValue(equal, testNs),
				"k8s.node.name":               newExpectedValue(exist, ""),
				"k8s.cluster.uid":             newExpectedValue(regex, uidRe),
				"dt.kubernetes.cluster.id":    newExpectedValue(regex, uidRe),
			},
		},
		{
			name:    "traces-daemonset",
			service: "test-traces-daemonset",
			attrs: map[string]*expectedValue{
				"k8s.pod.name":                newExpectedValue(regex, "telemetrygen-"+testID+"-traces-daemonset-[a-z0-9]*"),
				"k8s.pod.uid":                 newExpectedValue(regex, uidRe),
				"k8s.daemonset.name":          newExpectedValue(equal, "telemetrygen-"+testID+"-traces-daemonset"),
				"dt.kubernetes.workload.name": newExpectedValue(equal, "telemetrygen-"+testID+"-traces-daemonset"),
				"dt.kubernetes.workload.kind": newExpectedValue(equal, "daemonset"),
				"k8s.namespace.name":          newExpectedValue(equal, testNs),
				"k8s.node.name":               newExpectedValue(exist, ""),
				"k8s.cluster.uid":             newExpectedValue(regex, uidRe),
				"dt.kubernetes.cluster.id":    newExpectedValue(regex, uidRe),
			},
		},
	}

	for _, tc := range tcs {
		scanTracesForAttributes(t, tracesConsumer, tc.service, tc.attrs)
	}
}

func scanTracesForAttributes(t *testing.T, ts *consumertest.TracesSink, expectedService string,
	kvs map[string]*expectedValue) {
	for i := 0; i < len(ts.AllTraces()); i++ {
		traces := ts.AllTraces()[i]
		for i := 0; i < traces.ResourceSpans().Len(); i++ {
			resource := traces.ResourceSpans().At(i).Resource()
			service, exist := resource.Attributes().Get("service.name")
			assert.Equal(t, true, exist, "Resource does not have the 'service.name' attribute")
			if service.AsString() != expectedService {
				continue
			}
			assert.NoError(t, resourceHasAttributes(resource, kvs))
			return
		}
	}
	t.Fatalf("no spans found for service %s", expectedService)
}

func resourceHasAttributes(resource pcommon.Resource, kvs map[string]*expectedValue) error {
	foundAttrs := make(map[string]bool)
	for k := range kvs {
		foundAttrs[k] = false
	}

	resource.Attributes().Range(
		func(k string, v pcommon.Value) bool {
			if val, ok := kvs[k]; ok {
				switch val.mode {
				case equal:
					if val.value == v.AsString() {
						foundAttrs[k] = true
					}
				case regex:
					matched, _ := regexp.MatchString(val.value, v.AsString())
					if matched {
						foundAttrs[k] = true
					}
				case exist:
					foundAttrs[k] = true
				}

			}
			return true
		},
	)

	var err error
	for k, v := range foundAttrs {
		if !v {
			err = multierr.Append(err, fmt.Errorf("%v attribute not found", k))
		}
	}
	return err
}

func startUpSinks(t *testing.T, tc *consumertest.TracesSink) func() {
	f := otlpreceiver.NewFactory()
	cfg := f.CreateDefaultConfig().(*otlpreceiver.Config)

	rcvr, err := f.CreateTracesReceiver(context.Background(), receivertest.NewNopCreateSettings(), cfg, tc)
	require.NoError(t, err, "failed creating traces receiver")

	require.NoError(t, rcvr.Start(context.Background(), componenttest.NewNopHost()))
	return func() {
		assert.NoError(t, rcvr.Shutdown(context.Background()))
	}
}

func waitForData(t *testing.T, entriesNum int, tc *consumertest.TracesSink) {
	timeoutMinutes := 5
	require.Eventuallyf(t, func() bool {
		return len(tc.AllTraces()) > entriesNum
	}, time.Duration(timeoutMinutes)*time.Minute, 1*time.Second,
		"failed to receive %d entries,  received %d traces in %d minutes", entriesNum,
		len(tc.AllTraces()), timeoutMinutes)
}
