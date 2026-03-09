package main

import (
	"testing"
)

func TestMatchPattern(t *testing.T) {
	cases := []struct {
		component string
		pattern   string
		want      bool
	}{
		// Exact match.
		{"pkg/ottl", "pkg/ottl", true},
		{"pkg/ottl/other", "pkg/ottl", true}, // prefix match
		{"pkg/ottl2", "pkg/ottl", false},     // not a prefix match
		// Glob suffix.
		{"internal/metadataproviders", "internal/*", true},
		{"cmd/builder", "cmd/*", true},
		{"receiver/filelog", "internal/*", false},
		// All.
		{"all", "all", true},
	}
	for _, tc := range cases {
		got := matchPattern(tc.component, tc.pattern)
		if got != tc.want {
			t.Errorf("matchPattern(%q, %q) = %v, want %v", tc.component, tc.pattern, got, tc.want)
		}
	}
}

func TestFilterEntries(t *testing.T) {
	components := map[string]bool{
		"receiver/filelog":            true,
		"processor/resourcedetection": true,
	}
	cfg := Config{
		Allowlist: []string{"pkg/ottl", "all"},
		Denylist:  []string{"internal/*"},
	}

	entries := []ChangelogEntry{
		{Component: "receiver/filelog", ChangeType: Enhancement, Source: "contrib", UpstreamVersion: "v0.145.0", RepoURL: contribRepoURL},
		{Component: "processor/resourcedetection", ChangeType: Breaking, Source: "contrib", UpstreamVersion: "v0.145.0", RepoURL: contribRepoURL},
		{Component: "pkg/ottl", ChangeType: Enhancement, Source: "contrib", UpstreamVersion: "v0.145.0", RepoURL: contribRepoURL},
		{Component: "unrelated/component", ChangeType: Enhancement, Source: "contrib", UpstreamVersion: "v0.145.0", RepoURL: contribRepoURL},
		{Component: "internal/metadataproviders", ChangeType: BugFix, Source: "contrib", UpstreamVersion: "v0.145.0", RepoURL: contribRepoURL},
		{Component: "all", ChangeType: Enhancement, Source: "core", UpstreamVersion: "v0.145.0", RepoURL: coreRepoURL},
	}

	fc := FilterEntries(entries, components, cfg)

	if len(fc.Breaking) != 1 || fc.Breaking[0].Component != "processor/resourcedetection" {
		t.Errorf("breaking: got %v", fc.Breaking)
	}
	if len(fc.Enhancements) != 3 {
		t.Errorf("enhancements: got %d entries, want 3", len(fc.Enhancements))
	}
	if len(fc.BugFixes) != 0 {
		t.Errorf("bug fixes: expected internal/* to be excluded, got %v", fc.BugFixes)
	}
	// unrelated/component should be excluded.
	for _, e := range fc.Enhancements {
		if e.Component == "unrelated/component" {
			t.Error("unrelated/component should have been filtered out")
		}
	}
}

func TestShouldInclude_DenylistTakesPrecedence(t *testing.T) {
	components := map[string]bool{"internal/something": true}
	cfg := Config{
		Allowlist: []string{"internal/*"},
		Denylist:  []string{"internal/*"},
	}
	// Even though it's in manifest AND allowlist, denylist wins.
	if shouldInclude("internal/something", components, cfg) {
		t.Error("denylist should take precedence over manifest and allowlist")
	}
}
