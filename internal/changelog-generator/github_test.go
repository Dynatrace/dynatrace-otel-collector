package main

import (
	"encoding/json"
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

func TestExtractVersionFromVersionsYAML_IgnoresHigherNonCoreVersions(t *testing.T) {
	versionsYAML := []byte(`module-sets:
  beta:
    version: v0.145.0
  stable:
    version: v1.51.0
`)

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
	body, err := c.do(http.MethodGet, ts.URL, nil, true)
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

func TestFetchChloggenFiles_GraphQL(t *testing.T) {
	// Simulate a GitHub GraphQL response with two .yaml entries and one to skip.
	graphQLResp := map[string]any{
		"data": map[string]any{
			"repository": map[string]any{
				"object": map[string]any{
					"entries": []any{
						map[string]any{"name": "fix-foo.yaml", "object": map[string]any{"text": "change_type: bug_fix\ncomponent: receiver/filelog\nnote: fix foo\nissues: [1]\n"}},
						map[string]any{"name": "TEMPLATE.yaml", "object": map[string]any{"text": "should be skipped"}},
						map[string]any{"name": "config.yaml", "object": map[string]any{"text": "should be skipped"}},
						map[string]any{"name": "not-yaml.txt", "object": map[string]any{"text": "should be skipped"}},
						map[string]any{"name": "enh-bar.yaml", "object": map[string]any{"text": "change_type: enhancement\ncomponent: processor/batch\nnote: add bar\nissues: [2]\n"}},
					},
				},
			},
		},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(graphQLResp)
	}))
	defer ts.Close()

	c := &githubClient{httpClient: ts.Client(), graphqlURL: ts.URL}
	files, err := c.fetchChloggenFiles("owner", "repo", "abc123sha")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d: %v", len(files), files)
	}
	if _, ok := files["fix-foo.yaml"]; !ok {
		t.Error("expected fix-foo.yaml in result")
	}
	if _, ok := files["enh-bar.yaml"]; !ok {
		t.Error("expected enh-bar.yaml in result")
	}
	if _, ok := files["TEMPLATE.yaml"]; ok {
		t.Error("TEMPLATE.yaml should be excluded")
	}
}

func TestFetchChloggenFiles_GraphQLError(t *testing.T) {
	resp := map[string]any{
		"errors": []any{map[string]any{"message": "something went wrong"}},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	c := &githubClient{httpClient: ts.Client(), graphqlURL: ts.URL}
	_, err := c.fetchChloggenFiles("owner", "repo", "abc123sha")
	if err == nil {
		t.Fatal("expected error from GraphQL errors field")
	}
}
