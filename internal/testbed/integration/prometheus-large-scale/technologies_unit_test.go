// Copyright Dynatrace LLC
// SPDX-License-Identifier: Apache-2.0

package prometheus_large_scale

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func TestRenderManifest(t *testing.T) {
	in := "metadata:\n  namespace: ${NAMESPACE}\n---\nnamespace: ${NAMESPACE}\n"
	want := "metadata:\n  namespace: my-ns\n---\nnamespace: my-ns\n"
	assert.Equal(t, want, renderManifest(in, "my-ns"))
}

// newGaugeMetrics builds a single-batch pmetric.Metrics holding one gauge
// datapoint with the given resource and datapoint attributes.
func newGaugeMetrics(name string, resAttrs, dpAttrs map[string]string) pmetric.Metrics {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	for k, v := range resAttrs {
		rm.Resource().Attributes().PutStr(k, v)
	}
	m := rm.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	m.SetName(name)
	dp := m.SetEmptyGauge().DataPoints().AppendEmpty()
	for k, v := range dpAttrs {
		dp.Attributes().PutStr(k, v)
	}
	return md
}

func TestMetricAttrValues(t *testing.T) {
	batches := []pmetric.Metrics{
		newGaugeMetrics("coredns_build_info", map[string]string{"k8s.namespace.name": "kube-system"}, nil),
		newGaugeMetrics("coredns_build_info", nil, map[string]string{"k8s.namespace.name": "other"}),
		newGaugeMetrics("up", map[string]string{"k8s.namespace.name": "avalanche"}, nil),
	}

	assert.ElementsMatch(t, []string{"kube-system", "other"},
		metricAttrValues(batches, "coredns_build_info", "k8s.namespace.name"))
	assert.Empty(t, metricAttrValues(batches, "coredns_build_info", "missing"))
	assert.Empty(t, metricAttrValues(batches, "not_there", "k8s.namespace.name"))
}

func TestTechnologyNamespace(t *testing.T) {
	assert.Equal(t, "e2e-kafka", technology{name: "kafka"}.namespace())
}

func TestTechnologyOrigin(t *testing.T) {
	assert.Equal(t, "e2e-kafka", technology{name: "kafka"}.origin(),
		"workload installed by the test runs in the technology namespace")
	assert.Equal(t, "kube-system", technology{name: "coredns", originNamespace: "kube-system"}.origin(),
		"preinstalled workload runs where the cluster put it")
}

func TestNamespaceManifest(t *testing.T) {
	want := `apiVersion: v1
kind: Namespace
metadata:
  name: e2e-kafka
  labels:
    e2e.dynatrace.com/tech: kafka
`
	assert.Equal(t, want, namespaceManifest(technology{name: "kafka"}))
}
