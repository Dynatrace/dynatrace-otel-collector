// Copyright Dynatrace LLC
// SPDX-License-Identifier: Apache-2.0

package prometheus_large_scale

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

// technology describes a third-party technology scraped through the shared
// collector stack. Its monitor manifest lives in
// config_examples/prometheus-large-scale/<name>.yaml and is applied by
// testdata/scripts/setup.sh together with the ScrapeConfig.
type technology struct {
	name string
	// metric proves that scraping works end to end.
	metric string
	// namespace is where the scraped workload runs.
	namespace string
}

// technologies are run as subtests against the shared collector stack.
// Adding a technology: add its manifest, list it in setup.sh and add an entry here.
var technologies = []technology{
	{name: "coredns", metric: "coredns_build_info", namespace: "kube-system"},
}

// testTechnology verifies that the technology's metric arrives at the gateway
// sink, attributed to the namespace of the scraped workload.
func testTechnology(t *testing.T, tc technology, sink *consumertest.MetricsSink) {
	require.EventuallyWithTf(t, func(tt *assert.CollectT) {
		namespaces := metricAttrValues(sink.AllMetrics(), tc.metric, "k8s.namespace.name")
		assert.Contains(tt, namespaces, tc.namespace)
	}, 3*time.Minute, 2*time.Second, "%s from namespace %s not found in sink", tc.metric, tc.namespace)
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
