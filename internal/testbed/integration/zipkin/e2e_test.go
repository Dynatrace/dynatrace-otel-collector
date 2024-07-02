//go:build e2e

package zipkin

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
	testKubeConfig   = "/tmp/kube-config-collector-e2e-testing"
	kubeConfigEnvVar = "KUBECONFIG"
	uidRe            = "[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}"
)

type expectedValue struct {
	mode  int
	value string
}

func newExpectedValue(mode int, value string) expectedValue {
	return expectedValue{
		mode:  mode,
		value: value,
	}
}

// TestE2E_ZipkinReceiver tests the "Ingest data from Zipkin" use case
// See: https://docs.dynatrace.com/docs/extend-dynatrace/opentelemetry/collector/use-cases/zipkin
func TestE2E_ZipkinReceiver(t *testing.T) {
	testDir := filepath.Join("testdata")

	kubeconfigPath := testKubeConfig

	if kubeConfigFromEnv := os.Getenv(kubeConfigEnvVar); kubeConfigFromEnv != "" {
		kubeconfigPath = kubeConfigFromEnv
	}

	k8sClient, err := k8stest.NewK8sClient(kubeconfigPath)
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
	collectorObjs := k8stest.CreateCollectorObjects(t, k8sClient, testID, filepath.Join(testDir, "collector"))
	createZipkinOpts := &k8stest.ZipkinAppCreateOpts{
		ManifestsDir: filepath.Join(testDir, "zipkin"),
		TestID:       testID,
		OtlpEndpoint: fmt.Sprintf("otelcol-%s.%s:4317", testID, testNs),
	}

	zipkinObjs, zipkinObjInfos := k8stest.CreateZipkinAppObjects(t, k8sClient, createZipkinOpts)
	defer func() {
		for _, obj := range append(collectorObjs, zipkinObjs...) {
			require.NoErrorf(t, k8stest.DeleteObject(k8sClient, obj), "failed to delete object %s", obj.GetName())
		}
	}()

	for _, info := range zipkinObjInfos {
		k8stest.WaitForZipkinAppToStart(t, k8sClient, info.Namespace, info.PodLabelSelectors)
	}

	wantEntries := 30 // Minimal number of traces to wait for.
	waitForData(t, wantEntries, tracesConsumer)

	tcs := []struct {
		name           string
		service        string
		attrs          map[string]expectedValue
		scopeSpanAttrs []map[string]expectedValue
	}{
		{
			name:    "frontend-traces",
			service: "frontend",
			scopeSpanAttrs: []map[string]expectedValue{
				{
					"http.method": newExpectedValue(equal, "GET"),
					"http.path":   newExpectedValue(equal, "/"),
				},
				{
					"http.method":  newExpectedValue(equal, "GET"),
					"http.path":    newExpectedValue(equal, "/api"),
					"peer.service": newExpectedValue(equal, "backend"),
				},
			},
		},
		{
			name:    "backend-traces",
			service: "backend",
			scopeSpanAttrs: []map[string]expectedValue{
				{
					"http.method": newExpectedValue(equal, "GET"),
					"http.path":   newExpectedValue(equal, "/api"),
				},
			},
		},
	}

	for _, tc := range tcs {
		scanTracesForAttributes(t, tracesConsumer, tc.service, tc.attrs, tc.scopeSpanAttrs)
	}
}

func scanTracesForAttributes(t *testing.T, ts *consumertest.TracesSink, expectedService string, kvs map[string]expectedValue, scopeSpanAttrs []map[string]expectedValue) {
	for i := 0; i < len(ts.AllTraces()); i++ {
		traces := ts.AllTraces()[i]
		for i := 0; i < traces.ResourceSpans().Len(); i++ {
			resource := traces.ResourceSpans().At(i).Resource()
			service, exist := resource.Attributes().Get("service.name")
			assert.Equal(t, true, exist, "Resource does not have the 'service.name' attribute")
			if service.AsString() != expectedService {
				continue
			}
			assert.NoError(t, attributesContainValues(resource.Attributes(), kvs))
			assert.NotZero(t, traces.ResourceSpans().At(i).ScopeSpans().Len())
			assert.NotZero(t, traces.ResourceSpans().At(i).ScopeSpans().At(0).Spans().Len())

			scopeSpan := traces.ResourceSpans().At(i).ScopeSpans().At(0)

			// look for matching spans containing the desired attributes
			for _, spanAttrs := range scopeSpanAttrs {
				var err error
				for j := 0; j < scopeSpan.Spans().Len(); j++ {
					err = attributesContainValues(scopeSpan.Spans().At(j).Attributes(), spanAttrs)
					if err == nil {
						break
					}
				}
				assert.NoError(t, err)
			}

			return
		}
	}
	t.Fatalf("no spans found for service %s", expectedService)
}

func attributesContainValues(attrs pcommon.Map, kvs map[string]expectedValue) error {
	foundAttrs := make(map[string]bool)
	for k := range kvs {
		foundAttrs[k] = false
	}

	attrs.Range(
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
			err = multierr.Append(err, fmt.Errorf("%v attribute not found. expected attributes: %#v. actual attributes: %#v", k, kvs, attrs.AsRaw()))
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
