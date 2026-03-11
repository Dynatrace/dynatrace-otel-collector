package main

import (
	"fmt"
	"strings"

	"golang.org/x/mod/semver"
)

const (
	coreRepoURL    = "https://github.com/open-telemetry/opentelemetry-collector"
	contribRepoURL = "https://github.com/open-telemetry/opentelemetry-collector-contrib"
)

// UpstreamContent holds the generated upstream changelog pieces, each
// corresponding to one placeholder in the summary.tmpl scaffold.
type UpstreamContent struct {
	// VersionIntro is the human-readable upstream version string, e.g.
	// "v0.145.0" or "v0.144.0 and v0.145.0". Replaces <!-- upstream-version -->.
	VersionIntro string
	// CollectorVersions is the release-link block for each upstream version.
	// Replaces <!-- upstream-collector-versions -->.
	CollectorVersions string
	// BreakingChanges is the formatted breaking-changes section (including its
	// header), or empty if there are none. Replaces <!-- upstream-breaking-changes -->.
	BreakingChanges string
	// OtherChanges contains the remaining upstream sections (deprecations, new
	// components, enhancements, bug fixes) with their headers, or a "no
	// highlights" note when all are empty. Replaces <!-- upstream-other-changes -->.
	OtherChanges string
}

// GenerateUpstreamContent builds the UpstreamContent for a FilteredChangelog.
// When the FilteredChangelog contains no upstream versions the returned struct
// is zero-valued, signaling to the caller that the upstream section should be
// removed from CHANGELOG.md entirely.
func GenerateUpstreamContent(fc FilteredChangelog) UpstreamContent {
	orderedVersions := filterCollectorReleaseVersions(sortedUniqueVersions(fc.UpstreamVersions))
	if len(orderedVersions) == 0 {
		return UpstreamContent{}
	}

	// Use collected repo URLs, falling back to well-known defaults.
	coreURL := fc.CoreRepoURL
	if coreURL == "" {
		coreURL = coreRepoURL
	}
	contribURL := fc.ContribRepoURL
	if contribURL == "" {
		contribURL = contribRepoURL
	}

	// --- Version intro ---
	versionIntro := joinVersions(orderedVersions)

	// --- Collector version links ---
	var upstreamVersionsBuilder strings.Builder
	for _, v := range orderedVersions {
		fmt.Fprintf(&upstreamVersionsBuilder, "%s:\n\n", v)
		fmt.Fprintf(&upstreamVersionsBuilder, "- <%s/releases/tag/%s>\n", coreURL, v)
		fmt.Fprintf(&upstreamVersionsBuilder, "- <%s/releases/tag/%s>\n\n", contribURL, v)
	}

	// --- Breaking changes (top-level, outside <details>) ---
	var breakingChangesBuilder strings.Builder
	if len(fc.Breaking) > 0 {
		breakingChangesBuilder.WriteString("### 🛑 Breaking changes 🛑\n\n")
		writeEntries(&breakingChangesBuilder, fc.Breaking)
	}

	// --- Other changes (inside <details>) ---
	var otherChangesBuilder strings.Builder
	hasOther := false
	if len(fc.Deprecations) > 0 {
		otherChangesBuilder.WriteString("### ⚠️ Deprecations ⚠️\n\n")
		writeEntries(&otherChangesBuilder, fc.Deprecations)
		hasOther = true
	}
	if len(fc.NewComponents) > 0 {
		if hasOther {
			otherChangesBuilder.WriteString("\n")
		}
		otherChangesBuilder.WriteString("### 🚀 New components 🚀\n\n")
		writeEntries(&otherChangesBuilder, fc.NewComponents)
		hasOther = true
	}
	if len(fc.Enhancements) > 0 {
		if hasOther {
			otherChangesBuilder.WriteString("\n")
		}
		otherChangesBuilder.WriteString("### 💡 Enhancements 💡\n\n")
		writeEntries(&otherChangesBuilder, fc.Enhancements)
		hasOther = true
	}
	if len(fc.BugFixes) > 0 {
		if hasOther {
			otherChangesBuilder.WriteString("\n")
		}
		otherChangesBuilder.WriteString("### 🧰 Bug fixes 🧰\n\n")
		writeEntries(&otherChangesBuilder, fc.BugFixes)
		hasOther = true
	}

	otherStr := strings.TrimRight(otherChangesBuilder.String(), "\n")
	if !hasOther {
		otherStr = "No upstream highlights for this release."
	}

	return UpstreamContent{
		VersionIntro:      versionIntro,
		CollectorVersions: strings.TrimRight(upstreamVersionsBuilder.String(), "\n"),
		BreakingChanges:   strings.TrimRight(breakingChangesBuilder.String(), "\n"),
		OtherChanges:      otherStr,
	}
}

// writeEntries renders a slice of ChangelogEntry items as Markdown list items.
// Core entries come before contrib entries.
func writeEntries(sb *strings.Builder, entries []ChangelogEntry) {
	// Write core entries first, then contrib.
	for _, src := range []string{"core", "contrib"} {
		for _, e := range entries {
			if e.Source != src {
				continue
			}
			writeEntry(sb, e)
		}
	}
}

func writeEntry(sb *strings.Builder, e ChangelogEntry) {
	issueLinks := buildIssueLinks(e.Issues, e.RepoURL)
	if issueLinks != "" {
		fmt.Fprintf(sb, "- `%s`: %s (%s)\n", e.Component, e.Note, issueLinks)
	} else {
		fmt.Fprintf(sb, "- `%s`: %s\n", e.Component, e.Note)
	}
	if e.Subtext != "" {
		// Indent each line of subtext by two spaces.
		lines := strings.Split(e.Subtext, "\n")
		for _, line := range lines {
			if line == "" {
				sb.WriteString("\n")
			} else {
				fmt.Fprintf(sb, "  %s\n", line)
			}
		}
	}
}

// buildIssueLinks converts issue numbers into GitHub issue hyperlinks.
func buildIssueLinks(issues []int, repoURL string) string {
	if len(issues) == 0 {
		return ""
	}
	parts := make([]string, len(issues))
	for i, num := range issues {
		parts[i] = fmt.Sprintf("[#%d](%s/issues/%d)", num, repoURL, num)
	}
	return strings.Join(parts, ", ")
}

// joinVersions formats a list of version strings for the intro sentence.
func joinVersions(versions []string) string {
	switch len(versions) {
	case 0:
		return ""
	case 1:
		return versions[0]
	case 2:
		return versions[0] + " and " + versions[1]
	default:
		return strings.Join(versions[:len(versions)-1], ", ") + ", and " + versions[len(versions)-1]
	}
}

func filterCollectorReleaseVersions(versions []string) []string {
	collector := make([]string, 0, len(versions))
	for _, v := range versions {
		vc := canonicalVersion(v)
		if semver.IsValid(vc) && semver.Major(vc) == "v0" {
			collector = append(collector, vc)
		}
	}
	if len(collector) > 0 {
		return sortedUniqueVersions(collector)
	}
	return versions
}
