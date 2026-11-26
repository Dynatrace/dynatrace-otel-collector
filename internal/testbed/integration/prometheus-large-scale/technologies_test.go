// Copyright Dynatrace LLC
// SPDX-License-Identifier: Apache-2.0

package prometheus_large_scale

import (
	"testing"
	"time"

	"go.opentelemetry.io/collector/consumer/consumertest"
)

// technology describes a third-party technology scraped through the shared
// collector stack. Its monitor manifest lives in
// config_examples/prometheus-large-scale/monitor-examples/<kind>-<name>.yaml
// (e.g. podmonitor-coredns.yaml) and is applied by testdata/scripts/setup.sh
// together with the ScrapeConfig.
type technology struct {
	name string
	// metric proves that scraping works end to end.
	metric string
}

// technologies are run as subtests against the shared collector stack.
// Adding a technology: add its manifest to monitor-examples/ (setup.sh applies
// all of them) and add an entry here.
var technologies = []technology{
	{name: "coredns", metric: "coredns_build_info"},
}

// testTechnology verifies that the technology's metric arrives at the gateway sink.
func testTechnology(t *testing.T, tc technology, sink *consumertest.MetricsSink) {
	requireMetricsEventually(t, sink, []string{tc.metric}, 3*time.Minute)
}
