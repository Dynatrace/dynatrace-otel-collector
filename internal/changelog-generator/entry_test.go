package main

import (
	"os"
	"testing"
)

func TestParseChloggenEntry_Enhancement(t *testing.T) {
	data, err := os.ReadFile("testdata/entries/contrib/filelog-enhancement.yaml")
	if err != nil {
		t.Fatal(err)
	}
	entry, err := ParseChloggenEntry(data, "contrib", "v0.145.0", "https://github.com/open-telemetry/opentelemetry-collector-contrib")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entry == nil {
		t.Fatal("expected entry, got nil")
	}
	if entry.Component != "receiver/filelog" {
		t.Errorf("component: got %q, want %q", entry.Component, "receiver/filelog")
	}
	if entry.ChangeType != Enhancement {
		t.Errorf("change_type: got %q, want %q", entry.ChangeType, Enhancement)
	}
	if len(entry.Issues) != 1 || entry.Issues[0] != 39491 {
		t.Errorf("issues: got %v, want [39491]", entry.Issues)
	}
	if entry.Subtext == "" {
		t.Error("expected non-empty subtext")
	}
}

func TestParseChloggenEntry_Breaking(t *testing.T) {
	data, err := os.ReadFile("testdata/entries/contrib/resourcedetection-breaking.yaml")
	if err != nil {
		t.Fatal(err)
	}
	entry, err := ParseChloggenEntry(data, "contrib", "v0.145.0", "https://github.com/open-telemetry/opentelemetry-collector-contrib")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entry == nil {
		t.Fatal("expected entry, got nil")
	}
	if entry.ChangeType != Breaking {
		t.Errorf("change_type: got %q, want %q", entry.ChangeType, Breaking)
	}
}

func TestParseChloggenEntry_APIOnly(t *testing.T) {
	data, err := os.ReadFile("testdata/entries/contrib/api-only.yaml")
	if err != nil {
		t.Fatal(err)
	}
	entry, err := ParseChloggenEntry(data, "contrib", "v0.145.0", "https://github.com/open-telemetry/opentelemetry-collector-contrib")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entry != nil {
		t.Errorf("expected nil entry for api-only change_logs, got %+v", entry)
	}
}

func TestParseChloggenEntry_InvalidChangeType(t *testing.T) {
	yaml := []byte("change_type: invalid\ncomponent: foo\nnote: bar\n")
	_, err := ParseChloggenEntry(yaml, "contrib", "v0.145.0", "https://example.com")
	if err == nil {
		t.Error("expected error for invalid change_type")
	}
}

func TestIsUserFacing(t *testing.T) {
	cases := []struct {
		changeLogs []string
		want       bool
	}{
		{nil, true},
		{[]string{}, true},
		{[]string{"user"}, true},
		{[]string{"user", "api"}, true},
		{[]string{"api"}, false},
	}
	for _, tc := range cases {
		got := isUserFacing(tc.changeLogs)
		if got != tc.want {
			t.Errorf("isUserFacing(%v) = %v, want %v", tc.changeLogs, got, tc.want)
		}
	}
}
