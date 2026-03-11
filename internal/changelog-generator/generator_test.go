package main

import (
	"strings"
	"testing"
)

func sampleFilteredChangelog() FilteredChangelog {
	return FilteredChangelog{
		DistVersion:      "v0.44.0",
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

func TestGenerateUpstreamContent_VersionIntro(t *testing.T) {
	uc := GenerateUpstreamContent(sampleFilteredChangelog())
	if uc.VersionIntro != "v0.145.0" {
		t.Errorf("expected VersionIntro %q, got %q", "v0.145.0", uc.VersionIntro)
	}
}

func TestGenerateUpstreamContent_NoVersionHeader(t *testing.T) {
	uc := GenerateUpstreamContent(sampleFilteredChangelog())
	// The section header (## vX.Y.Z) is now produced by the chloggen template,
	// not by the generator — verify it does not appear in any generated field.
	for _, field := range []string{uc.CollectorVersions, uc.BreakingChanges, uc.OtherChanges} {
		if strings.Contains(field, "## v") {
			t.Errorf("upstream content fields must not contain a '## v' section header: %s", field)
		}
	}
}

func TestGenerateUpstreamContent_CollectorVersions(t *testing.T) {
	uc := GenerateUpstreamContent(sampleFilteredChangelog())
	if !strings.Contains(uc.CollectorVersions, coreRepoURL+"/releases/tag/v0.145.0") {
		t.Errorf("CollectorVersions missing core release link:\n%s", uc.CollectorVersions)
	}
	if !strings.Contains(uc.CollectorVersions, contribRepoURL+"/releases/tag/v0.145.0") {
		t.Errorf("CollectorVersions missing contrib release link:\n%s", uc.CollectorVersions)
	}
}

func TestGenerateUpstreamContent_BreakingChangesHasHeader(t *testing.T) {
	uc := GenerateUpstreamContent(sampleFilteredChangelog())
	if !strings.Contains(uc.BreakingChanges, "Breaking changes") {
		t.Errorf("BreakingChanges missing header:\n%s", uc.BreakingChanges)
	}
	if !strings.Contains(uc.BreakingChanges, "processor/resourcedetection") {
		t.Errorf("BreakingChanges missing entry:\n%s", uc.BreakingChanges)
	}
}

func TestGenerateUpstreamContent_BreakingChangesDoesNotContainOther(t *testing.T) {
	uc := GenerateUpstreamContent(sampleFilteredChangelog())
	if strings.Contains(uc.BreakingChanges, "receiver/filelog") {
		t.Errorf("BreakingChanges should not contain enhancement entries")
	}
}

func TestGenerateUpstreamContent_OtherChangesContainsEnhancements(t *testing.T) {
	uc := GenerateUpstreamContent(sampleFilteredChangelog())
	if !strings.Contains(uc.OtherChanges, "receiver/filelog") {
		t.Errorf("OtherChanges missing enhancement entry:\n%s", uc.OtherChanges)
	}
}

func TestGenerateUpstreamContent_NoBreaking_EmptyBreakingField(t *testing.T) {
	fc := FilteredChangelog{
		UpstreamVersions: []string{"v0.145.0"},
		CoreRepoURL:      coreRepoURL,
		ContribRepoURL:   contribRepoURL,
		Enhancements: []ChangelogEntry{
			{Component: "receiver/filelog", ChangeType: Enhancement, Source: "contrib", UpstreamVersion: "v0.145.0", RepoURL: contribRepoURL},
		},
	}
	uc := GenerateUpstreamContent(fc)
	if uc.BreakingChanges != "" {
		t.Errorf("expected empty BreakingChanges, got %q", uc.BreakingChanges)
	}
}

func TestGenerateUpstreamContent_NoOther_DefaultMessage(t *testing.T) {
	fc := FilteredChangelog{
		UpstreamVersions: []string{"v0.145.0"},
		CoreRepoURL:      coreRepoURL,
		ContribRepoURL:   contribRepoURL,
		Breaking: []ChangelogEntry{
			{Component: "comp", ChangeType: Breaking, Source: "core", UpstreamVersion: "v0.145.0", RepoURL: coreRepoURL},
		},
	}
	uc := GenerateUpstreamContent(fc)
	if !strings.Contains(uc.OtherChanges, "No upstream highlights for this release.") {
		t.Errorf("expected 'No upstream highlights' message in OtherChanges, got %q", uc.OtherChanges)
	}
}

func TestGenerateUpstreamContent_EmptyVersions_ZeroValue(t *testing.T) {
	fc := FilteredChangelog{
		UpstreamVersions: []string{},
		CoreRepoURL:      coreRepoURL,
		ContribRepoURL:   contribRepoURL,
	}
	uc := GenerateUpstreamContent(fc)
	if uc.VersionIntro != "" || uc.CollectorVersions != "" || uc.BreakingChanges != "" || uc.OtherChanges != "" {
		t.Errorf("expected zero-value UpstreamContent when no upstream versions, got %+v", uc)
	}
}

func TestGenerateUpstreamContent_IssueLinks(t *testing.T) {
	uc := GenerateUpstreamContent(sampleFilteredChangelog())
	want := "[#45797](" + contribRepoURL + "/issues/45797)"
	if !strings.Contains(uc.BreakingChanges, want) {
		t.Errorf("BreakingChanges missing issue link:\n%s", uc.BreakingChanges)
	}
}

func TestGenerateUpstreamContent_CoreBeforeContrib(t *testing.T) {
	fc := FilteredChangelog{
		UpstreamVersions: []string{"v0.145.0"},
		CoreRepoURL:      coreRepoURL,
		ContribRepoURL:   contribRepoURL,
		BugFixes: []ChangelogEntry{
			{Component: "pkg/config/configoptional", ChangeType: BugFix, Source: "core", UpstreamVersion: "v0.145.0", RepoURL: coreRepoURL, Issues: []int{14500}},
			{Component: "receiver/filelog", ChangeType: BugFix, Source: "contrib", UpstreamVersion: "v0.145.0", RepoURL: contribRepoURL, Issues: []int{39011}},
		},
	}
	uc := GenerateUpstreamContent(fc)
	coreIdx := strings.Index(uc.OtherChanges, "pkg/config/configoptional")
	contribIdx := strings.Index(uc.OtherChanges, "receiver/filelog")
	if coreIdx == -1 || contribIdx == -1 {
		t.Fatal("missing expected entries in OtherChanges")
	}
	if coreIdx > contribIdx {
		t.Error("core entries should appear before contrib entries")
	}
}

func TestGenerateUpstreamContent_SubtextIndented(t *testing.T) {
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
	uc := GenerateUpstreamContent(fc)
	if !strings.Contains(uc.BreakingChanges, "  This is the subtext.") {
		t.Errorf("subtext should be indented by 2 spaces:\n%s", uc.BreakingChanges)
	}
}

func TestGenerateUpstreamContent_MultiVersion(t *testing.T) {
	fc := FilteredChangelog{
		DistVersion:      "v0.44.0",
		UpstreamVersions: []string{"v0.144.0", "v0.145.0"},
		CoreRepoURL:      coreRepoURL,
		ContribRepoURL:   contribRepoURL,
		Enhancements: []ChangelogEntry{
			{Component: "receiver/filelog", ChangeType: Enhancement, Source: "contrib", UpstreamVersion: "v0.144.0", RepoURL: contribRepoURL},
		},
	}
	uc := GenerateUpstreamContent(fc)
	if !strings.Contains(uc.VersionIntro, "v0.144.0") || !strings.Contains(uc.VersionIntro, "v0.145.0") {
		t.Errorf("VersionIntro should mention both upstream versions: %q", uc.VersionIntro)
	}
	if !strings.Contains(uc.CollectorVersions, "v0.144.0:") {
		t.Errorf("CollectorVersions missing v0.144.0 links:\n%s", uc.CollectorVersions)
	}
	if !strings.Contains(uc.CollectorVersions, "v0.145.0:") {
		t.Errorf("CollectorVersions missing v0.145.0 links:\n%s", uc.CollectorVersions)
	}
}

func TestGenerateUpstreamContent_UsesOnlyCollectorReleaseVersions(t *testing.T) {
	fc := FilteredChangelog{
		DistVersion:      "v0.44.0",
		UpstreamVersions: []string{"v0.145.0", "v1.51.0"},
		CoreRepoURL:      coreRepoURL,
		ContribRepoURL:   contribRepoURL,
	}
	uc := GenerateUpstreamContent(fc)
	if strings.Contains(uc.CollectorVersions, "v1.51.0:") {
		t.Errorf("should not render links for 1.x component versions:\n%s", uc.CollectorVersions)
	}
	if !strings.Contains(uc.CollectorVersions, "v0.145.0:") {
		t.Errorf("should render links for 0.x collector version:\n%s", uc.CollectorVersions)
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
