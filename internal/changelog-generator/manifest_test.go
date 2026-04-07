package main

import (
	"testing"
)

func TestParseManifest(t *testing.T) {
	// Build a fake chloggen index from the expected IDs so the test
	// doesn't make real HTTP calls.
	expected := []string{
		"receiver/otlp",
		"receiver/file_log", // canonical chloggen form
		"receiver/hostmetrics",
		"receiver/prometheus",
		"exporter/otlp",
		"exporter/otlp_http", // canonical chloggen form
		"exporter/loadbalancing",
		"extension/file_storage", // canonical chloggen form
		"extension/health_check", // canonical chloggen form
		"processor/batch",
		"processor/resourcedetection",
		"processor/k8s_attributes", // canonical chloggen form
		"processor/tail_sampling",  // canonical chloggen form
		"connector/spanmetrics",
	}
	fakeChloggen := make(map[string]bool, len(expected))
	for _, id := range expected {
		fakeChloggen[id] = true
	}
	index := BuildChloggenIndex(fakeChloggen)

	components, distVersion, err := ParseManifest("testdata/manifest.yaml", index)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if distVersion != "0.44.0" {
		t.Errorf("distVersion: got %q, want %q", distVersion, "0.44.0")
	}

	// Check using canonical chloggen IDs (with underscores where applicable).
	for _, c := range expected {
		if !components[c] {
			t.Errorf("expected component %q to be in set", c)
		}
	}

	// Providers should NOT be in the component set.
	if components["provider/envprovider"] {
		t.Error("providers should not be included in component set")
	}
}

func TestGomodToComponentID(t *testing.T) {
	cases := []struct {
		gomod    string
		compType string
		want     string
	}{
		{
			"go.opentelemetry.io/collector/receiver/otlpreceiver v0.145.0",
			"receiver", "receiver/otlp",
		},
		{
			"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/filelogreceiver v0.145.0",
			"receiver", "receiver/filelog", // intermediate form — "file_log" is resolved via index later
		},
		{
			"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor v0.145.0",
			"processor", "processor/resourcedetection",
		},
		{
			"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter v0.145.0",
			"exporter", "exporter/loadbalancing",
		},
		{
			// filestorage has no "extension" suffix — should keep name as-is.
			"github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage/filestorage v0.145.0",
			"extension", "extension/filestorage",
		},
		{
			"github.com/open-telemetry/opentelemetry-collector-contrib/connector/spanmetricsconnector v0.145.0",
			"connector", "connector/spanmetrics",
		},
	}
	for _, tc := range cases {
		got := gomodToComponentID(tc.gomod, tc.compType)
		if got != tc.want {
			t.Errorf("gomodToComponentID(%q, %q) = %q, want %q", tc.gomod, tc.compType, got, tc.want)
		}
	}
}
