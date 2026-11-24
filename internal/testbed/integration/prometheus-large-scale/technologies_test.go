// Copyright Dynatrace LLC
// SPDX-License-Identifier: Apache-2.0

package prometheus_large_scale

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

// techLabel marks every technology namespace so teardown.sh can delete them
// all by label.
const techLabel = "e2e.dynatrace.com/tech"

// technology describes a third-party technology scraped through the shared
// collector stack. Its files are found by convention:
//   - monitors: config_examples/prometheus-large-scale/<name>/*.yaml (required)
//   - workload: testdata/technologies/<name>/*.yaml (optional, omit for
//     technologies that are preinstalled in the cluster)
//
// Both are applied into the namespace e2e-<name> with ${NAMESPACE} substituted.
type technology struct {
	name string
	// metric proves that scraping works end to end.
	metric string
	// originNamespace is where the scraped workload runs; empty means the
	// technology namespace, i.e. the workload is installed by the test.
	originNamespace string
}

// technologies are run as subtests against the shared collector stack.
// Adding a technology: add its files (see technology) and an entry here.
var technologies = []technology{
	{name: "coredns", metric: "coredns_build_info", originNamespace: "kube-system"},
}

func (tc technology) namespace() string {
	return "e2e-" + tc.name
}

func (tc technology) origin() string {
	if tc.originNamespace != "" {
		return tc.originNamespace
	}
	return tc.namespace()
}

func namespaceManifest(tc technology) string {
	return fmt.Sprintf(`apiVersion: v1
kind: Namespace
metadata:
  name: %s
  labels:
    %s: %s
`, tc.namespace(), techLabel, tc.name)
}

// testTechnology installs the technology's workload and monitors into its own
// namespace and verifies that its metric arrives at the gateway sink.
func testTechnology(t *testing.T, tc technology, configDir, workloadDir string, sink *consumertest.MetricsSink) {
	ns := tc.namespace()

	require.NoError(t, kubectlStdin(namespaceManifest(tc), "apply", "-f", "-"))
	// Deleting the namespace removes workload and monitors with it.
	t.Cleanup(func() {
		if err := exec.Command("kubectl", "delete", "namespace", ns, "--ignore-not-found", "--wait=false").Run(); err != nil {
			t.Logf("failed to delete namespace %s: %v", ns, err)
		}
	})

	workloads, err := filepath.Glob(filepath.Join(workloadDir, tc.name, "*.yaml"))
	require.NoError(t, err)
	if len(workloads) > 0 {
		applyManifests(t, workloads, ns)
		waitForPodsReady(t, ns, 5*time.Minute)
	}

	monitors, err := filepath.Glob(filepath.Join(configDir, tc.name, "*.yaml"))
	require.NoError(t, err)
	require.NotEmpty(t, monitors, "no monitor manifests in %s", filepath.Join(configDir, tc.name))
	applyManifests(t, monitors, ns)

	require.EventuallyWithTf(t, func(tt *assert.CollectT) {
		namespaces := metricAttrValues(sink.AllMetrics(), tc.metric, "k8s.namespace.name")
		assert.Contains(tt, namespaces, tc.origin())
	}, 3*time.Minute, 2*time.Second, "%s from namespace %s not found in sink", tc.metric, tc.origin())
}

// applyManifests renders each manifest for namespace and applies it.
func applyManifests(t *testing.T, paths []string, namespace string) {
	t.Helper()
	for _, path := range paths {
		content, err := os.ReadFile(path)
		require.NoError(t, err)
		require.NoError(t, kubectlStdin(renderManifest(string(content), namespace), "apply", "-f", "-"), "apply %s", path)
	}
}

// waitForPodsReady retries kubectl wait, because right after apply the
// controllers may not have created any pods yet.
func waitForPodsReady(t *testing.T, namespace string, timeout time.Duration) {
	t.Helper()
	require.Eventuallyf(t, func() bool {
		return exec.Command("kubectl", "wait", "pod", "--all", "--for=condition=Ready",
			"-n", namespace, "--timeout=10s").Run() == nil
	}, timeout, 2*time.Second, "pods in %s not ready within %v", namespace, timeout)
}

func kubectlStdin(stdin string, args ...string) error {
	cmd := exec.Command("kubectl", args...)
	cmd.Stdin = strings.NewReader(stdin)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// renderManifest substitutes ${NAMESPACE} the same way setup.sh does via sed.
func renderManifest(content, namespace string) string {
	return strings.ReplaceAll(content, "${NAMESPACE}", namespace)
}

// metricAttrValues collects the values of attribute key found on the resource
// or on any datapoint of every metric named metricName.
func metricAttrValues(batches []pmetric.Metrics, metricName, key string) []string {
	var values []string
	collect := func(attrs pcommon.Map) {
		if v, ok := attrs.Get(key); ok {
			values = append(values, v.AsString())
		}
	}
	for _, md := range batches {
		for i := 0; i < md.ResourceMetrics().Len(); i++ {
			rm := md.ResourceMetrics().At(i)
			for j := 0; j < rm.ScopeMetrics().Len(); j++ {
				sm := rm.ScopeMetrics().At(j)
				for k := 0; k < sm.Metrics().Len(); k++ {
					m := sm.Metrics().At(k)
					if m.Name() != metricName {
						continue
					}
					collect(rm.Resource().Attributes())
					for _, attrs := range dataPointAttrs(m) {
						collect(attrs)
					}
				}
			}
		}
	}
	return values
}

func dataPointAttrs(m pmetric.Metric) []pcommon.Map {
	var attrs []pcommon.Map
	switch m.Type() {
	case pmetric.MetricTypeGauge:
		for i := 0; i < m.Gauge().DataPoints().Len(); i++ {
			attrs = append(attrs, m.Gauge().DataPoints().At(i).Attributes())
		}
	case pmetric.MetricTypeSum:
		for i := 0; i < m.Sum().DataPoints().Len(); i++ {
			attrs = append(attrs, m.Sum().DataPoints().At(i).Attributes())
		}
	case pmetric.MetricTypeHistogram:
		for i := 0; i < m.Histogram().DataPoints().Len(); i++ {
			attrs = append(attrs, m.Histogram().DataPoints().At(i).Attributes())
		}
	case pmetric.MetricTypeSummary:
		for i := 0; i < m.Summary().DataPoints().Len(); i++ {
			attrs = append(attrs, m.Summary().DataPoints().At(i).Attributes())
		}
	}
	return attrs
}
