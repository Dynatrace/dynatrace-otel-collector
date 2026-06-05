package main

import (
	"testing"
)

func TestParseManifest(t *testing.T) {
	index := map[string]string{
		"receiver/otlp":                "receiver/otlp",
		"receiver/filelog":             "receiver/file_log",
		"receiver/hostmetrics":         "receiver/hostmetrics",
		"receiver/prometheus":          "receiver/prometheus",
		"exporter/otlp":                "exporter/otlp",
		"exporter/otlphttp":            "exporter/otlp_http",
		"exporter/load_balancing":      "exporter/load_balancing",
		"extension/filestorage":        "extension/file_storage",
		"extension/healthcheck":        "extension/health_check",
		"processor/batch":              "processor/batch",
		"processor/resource_detection": "processor/resource_detection",
		"processor/k8sattributes":      "processor/k8s_attributes",
		"processor/tailsampling":       "processor/tail_sampling",
		"connector/spanmetrics":        "connector/spanmetrics",
	}

	components, distVersion, err := ParseManifest("testdata/manifest.yaml", index)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if distVersion != "0.44.0" {
		t.Errorf("distVersion: got %q, want %q", distVersion, "0.44.0")
	}

	expected := []string{
		"receiver/otlp",
		"receiver/file_log",
		"receiver/hostmetrics",
		"receiver/prometheus",
		"exporter/otlp",
		"exporter/otlp_http",
		"exporter/load_balancing",
		"extension/file_storage",
		"extension/health_check",
		"processor/batch",
		"processor/resource_detection",
		"processor/k8s_attributes",
		"processor/tail_sampling",
		"connector/spanmetrics",
	}
	for _, c := range expected {
		if !components[c] {
			t.Errorf("expected component %q to be in set", c)
		}
	}

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
			"processor", "processor/resource_detection",
		},
		{
			"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter v0.145.0",
			"exporter", "exporter/load_balancing",
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
