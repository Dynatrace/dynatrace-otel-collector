package main

import (
	"strings"
	"testing"
)

func sampleFilteredChangelog() FilteredChangelog {
	return FilteredChangelog{
		UpstreamVersions: []string{"v0.145.0"},
		CoreRepoURL:      coreRepoURL,
		ContribRepoURL:   contribRepoURL,
		Breaking: []ChangelogEntry{
			{
				Component:       "processor/resourcedetection",
				Note:            "Promote feature gate to Stable",
				Issues:          []int{45797},
				Subtext:         "The faas.id attribute is replaced by the faas.instance attribute.",
				ChangeType:      Breaking,
				Source:          "contrib",
				UpstreamVersion: "v0.145.0",
				RepoURL:         contribRepoURL,
			},
		},
		Enhancements: []ChangelogEntry{
			{
				Component: "receiver/filelog", Note: "Suppress repeated permission-denied errors",
				Issues: []int{39491}, ChangeType: Enhancement, Source: "contrib",
				UpstreamVersion: "v0.145.0", RepoURL: contribRepoURL,
			},
			{
				Component: "pkg/ottl", Note: "Added generic path to get/set span flags",
				Issues: []int{34739}, ChangeType: Enhancement, Source: "contrib",
				UpstreamVersion: "v0.145.0", RepoURL: contribRepoURL,
			},
		},
		BugFixes: []ChangelogEntry{
			{
				Component: "pkg/config/configoptional", Note: "Fix Unmarshal methods not being called",
				Issues: []int{14500}, Subtext: "This bug notably manifested in the sending_queue config.",
				ChangeType: BugFix, Source: "core", UpstreamVersion: "v0.145.0", RepoURL: coreRepoURL,
			},
		},
	}
}

func TestGenerateChangelog_ContainsVersionHeader(t *testing.T) {
	out := GenerateChangelog(sampleFilteredChangelog())
	if !strings.Contains(out, "## v0.145.0") {
		t.Errorf("output missing version header:\n%s", out)
	}
}

func TestGenerateChangelog_ContainsReleaseLinks(t *testing.T) {
	out := GenerateChangelog(sampleFilteredChangelog())
	if !strings.Contains(out, coreRepoURL+"/releases/tag/v0.145.0") {
		t.Errorf("output missing core release link:\n%s", out)
	}
	if !strings.Contains(out, contribRepoURL+"/releases/tag/v0.145.0") {
		t.Errorf("output missing contrib release link:\n%s", out)
	}
}

func TestGenerateChangelog_BreakingOutsideDetails(t *testing.T) {
	out := GenerateChangelog(sampleFilteredChangelog())
	breakingIdx := strings.Index(out, "Breaking changes")
	detailsIdx := strings.Index(out, "<details>")
	if breakingIdx == -1 {
		t.Fatal("output missing breaking changes section")
	}
	if detailsIdx == -1 {
		t.Fatal("output missing <details> block")
	}
	if breakingIdx > detailsIdx {
		t.Error("breaking changes section should appear BEFORE <details> block")
	}
	entryIdx := strings.Index(out, "processor/resourcedetection")
	if entryIdx > detailsIdx {
		t.Error("breaking change entry should be outside (before) the <details> block")
	}
}

func TestGenerateChangelog_EnhancementsInsideDetails(t *testing.T) {
	out := GenerateChangelog(sampleFilteredChangelog())
	detailsIdx := strings.Index(out, "<details>")
	filelogIdx := strings.Index(out, "receiver/filelog")
	if filelogIdx == -1 {
		t.Fatal("output missing receiver/filelog entry")
	}
	if filelogIdx < detailsIdx {
		t.Error("enhancement entry should be INSIDE the <details> block")
	}
}

func TestGenerateChangelog_IssueLinks(t *testing.T) {
	out := GenerateChangelog(sampleFilteredChangelog())
	want := "[#45797](" + contribRepoURL + "/issues/45797)"
	if !strings.Contains(out, want) {
		t.Errorf("output missing issue link:\n%s", out)
	}
}

func TestGenerateChangelog_CoreBeforeContrib(t *testing.T) {
	fc := FilteredChangelog{
		UpstreamVersions: []string{"v0.145.0"},
		CoreRepoURL:      coreRepoURL,
		ContribRepoURL:   contribRepoURL,
		BugFixes: []ChangelogEntry{
			{Component: "pkg/config/configoptional", ChangeType: BugFix, Source: "core", UpstreamVersion: "v0.145.0", RepoURL: coreRepoURL, Issues: []int{14500}},
			{Component: "receiver/filelog", ChangeType: BugFix, Source: "contrib", UpstreamVersion: "v0.145.0", RepoURL: contribRepoURL, Issues: []int{39011}},
		},
	}
	out := GenerateChangelog(fc)
	coreIdx := strings.Index(out, "pkg/config/configoptional")
	contribIdx := strings.Index(out, "receiver/filelog")
	if coreIdx == -1 || contribIdx == -1 {
		t.Fatal("missing expected entries")
	}
	if coreIdx > contribIdx {
		t.Error("core entries should appear before contrib entries")
	}
}

func TestGenerateChangelog_NoBreaking_OmitsHeader(t *testing.T) {
	fc := FilteredChangelog{
		UpstreamVersions: []string{"v0.145.0"},
		CoreRepoURL:      coreRepoURL,
		ContribRepoURL:   contribRepoURL,
		Enhancements: []ChangelogEntry{
			{Component: "receiver/filelog", ChangeType: Enhancement, Source: "contrib", UpstreamVersion: "v0.145.0", RepoURL: contribRepoURL},
		},
	}
	out := GenerateChangelog(fc)
	if strings.Contains(out, "Breaking changes") {
		t.Error("should not emit breaking changes header when there are no breaking changes")
	}
}

func TestGenerateChangelog_MultiVersion(t *testing.T) {
	fc := FilteredChangelog{
		UpstreamVersions: []string{"v0.144.0", "v0.145.0"},
		CoreRepoURL:      coreRepoURL,
		ContribRepoURL:   contribRepoURL,
		Enhancements: []ChangelogEntry{
			{Component: "receiver/filelog", ChangeType: Enhancement, Source: "contrib", UpstreamVersion: "v0.144.0", RepoURL: contribRepoURL},
		},
	}
	out := GenerateChangelog(fc)
	if !strings.Contains(out, "## v0.145.0") {
		t.Errorf("expected ## v0.145.0 header for multi-version:\n%s", out)
	}
	if !strings.Contains(out, "v0.144.0:") {
		t.Errorf("expected v0.144.0 links:\n%s", out)
	}
	if !strings.Contains(out, "v0.145.0:") {
		t.Errorf("expected v0.145.0 links:\n%s", out)
	}
	if !strings.Contains(out, "v0.144.0 and v0.145.0") {
		t.Errorf("expected both versions in intro:\n%s", out)
	}
}

func TestGenerateChangelog_MultiVersion_HeaderUsesHighestSemver(t *testing.T) {
	fc := FilteredChangelog{
		UpstreamVersions: []string{"v0.145.0", "v0.144.0"},
		CoreRepoURL:      coreRepoURL,
		ContribRepoURL:   contribRepoURL,
	}

	out := GenerateChangelog(fc)
	if !strings.Contains(out, "## v0.145.0") {
		t.Errorf("expected highest semver in header:\n%s", out)
	}
}

func TestBuildIssueLinks(t *testing.T) {
	got := buildIssueLinks([]int{42650}, contribRepoURL)
	want := "[#42650](" + contribRepoURL + "/issues/42650)"
	if got != want {
		t.Errorf("buildIssueLinks: got %q, want %q", got, want)
	}
}

func TestBuildIssueLinks_Multiple(t *testing.T) {
	got := buildIssueLinks([]int{1, 2}, "https://example.com")
	if !strings.Contains(got, "[#1](https://example.com/issues/1)") {
		t.Errorf("missing first link in: %q", got)
	}
	if !strings.Contains(got, "[#2](https://example.com/issues/2)") {
		t.Errorf("missing second link in: %q", got)
	}
}

func TestBuildIssueLinks_Empty(t *testing.T) {
	got := buildIssueLinks(nil, "https://example.com")
	if got != "" {
		t.Errorf("expected empty string for no issues, got %q", got)
	}
}

func TestGenerateChangelog_SubtextIndented(t *testing.T) {
	fc := FilteredChangelog{
		UpstreamVersions: []string{"v0.145.0"},
		CoreRepoURL:      coreRepoURL,
		ContribRepoURL:   contribRepoURL,
		Breaking: []ChangelogEntry{
			{Component: "processor/resourcedetection", Note: "Some breaking change",
				Issues: []int{1}, Subtext: "This is the subtext.", ChangeType: Breaking,
				Source: "contrib", RepoURL: contribRepoURL},
		},
	}
	out := GenerateChangelog(fc)
	if !strings.Contains(out, "  This is the subtext.") {
		t.Errorf("subtext should be indented by 2 spaces:\n%s", out)
	}
}
