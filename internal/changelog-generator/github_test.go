package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParsePRURL_Strict(t *testing.T) {
	owner, repo, number, err := parsePRURL("https://github.com/open-telemetry/opentelemetry-collector/pull/14515")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if owner != "open-telemetry" || repo != "opentelemetry-collector" || number != 14515 {
		t.Fatalf("unexpected parse result: owner=%q repo=%q number=%d", owner, repo, number)
	}

	if _, _, _, err := parsePRURL("foo https://github.com/open-telemetry/opentelemetry-collector/pull/14515 bar"); err == nil {
		t.Fatal("expected malformed URL to be rejected")
	}
}

func TestExtractVersionFromVersionsYAML(t *testing.T) {
	versionsYAML := []byte(`module-sets:
  core:
    version: v0.145.0
  contrib-base:
    version: 0.145.0
`) // mixed with and without leading v

	got, err := extractVersionFromVersionsYAML(versionsYAML)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "v0.145.0" {
		t.Fatalf("got %q, want %q", got, "v0.145.0")
	}
}

func TestDoGet_RetriesOnRateLimit(t *testing.T) {
	attempts := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte("rate limited"))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer ts.Close()

	c := &githubClient{httpClient: ts.Client()}
	body, err := c.doGet(ts.URL, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(body) != "ok" {
		t.Fatalf("unexpected body %q", string(body))
	}
	if attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", attempts)
	}
}
