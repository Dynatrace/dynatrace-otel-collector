package main

import (
	"fmt"
	"strings"
)

const (
	coreRepoURL    = "https://github.com/open-telemetry/opentelemetry-collector"
	contribRepoURL = "https://github.com/open-telemetry/opentelemetry-collector-contrib"
)

// GenerateChangelog renders a FilteredChangelog into the markdown section that
// will be inserted into CHANGELOG.md.
func GenerateChangelog(fc FilteredChangelog) string {
	var sb strings.Builder

	// Determine the version label for the ## header (highest/last version).
	headerVersion := ""
	if len(fc.UpstreamVersions) > 0 {
		headerVersion = fc.UpstreamVersions[len(fc.UpstreamVersions)-1]
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

	// --- Version header ---
	fmt.Fprintf(&sb, "## %s\n\n", headerVersion)

	// --- Intro line ---
	if len(fc.UpstreamVersions) == 1 {
		fmt.Fprintf(&sb, "This release includes version %s of the upstream Collector components.\n\n", headerVersion)
	} else {
		versions := make([]string, len(fc.UpstreamVersions))
		for i, v := range fc.UpstreamVersions {
			versions[i] = v
		}
		fmt.Fprintf(&sb, "This release includes versions %s of the upstream Collector components.\n\n",
			joinVersions(versions))
	}

	// --- Upstream release links ---
	sb.WriteString("The individual upstream Collector changelogs can be found here:\n\n")
	for _, v := range fc.UpstreamVersions {
		fmt.Fprintf(&sb, "%s:\n\n", v)
		fmt.Fprintf(&sb, "- <%s/releases/tag/%s>\n", coreURL, v)
		fmt.Fprintf(&sb, "- <%s/releases/tag/%s>\n\n", contribURL, v)
	}

	// --- Top-level breaking changes (outside <details>) ---
	if len(fc.Breaking) > 0 {
		sb.WriteString("### 🛑 Breaking changes 🛑\n\n")
		writeEntries(&sb, fc.Breaking)
		sb.WriteString("\n")
	}

	// --- Dynatrace distribution changelog separator ---
	sb.WriteString("#### Dynatrace distribution changelog:\n\n")

	// --- <details> block for the rest ---
	sb.WriteString("<details>\n")
	sb.WriteString("<summary>Highlights from the upstream Collector changelog</summary>\n\n")
	sb.WriteString("---\n\n")

	detailsHasContent := false

	if len(fc.Deprecations) > 0 {
		sb.WriteString("### ⚠️ Deprecations ⚠️\n\n")
		writeEntries(&sb, fc.Deprecations)
		sb.WriteString("\n")
		detailsHasContent = true
	}
	if len(fc.NewComponents) > 0 {
		sb.WriteString("### 🚀 New components 🚀\n\n")
		writeEntries(&sb, fc.NewComponents)
		sb.WriteString("\n")
		detailsHasContent = true
	}
	if len(fc.Enhancements) > 0 {
		sb.WriteString("### 💡 Enhancements 💡\n\n")
		writeEntries(&sb, fc.Enhancements)
		sb.WriteString("\n")
		detailsHasContent = true
	}
	if len(fc.BugFixes) > 0 {
		sb.WriteString("### 🧰 Bug fixes 🧰\n\n")
		writeEntries(&sb, fc.BugFixes)
		sb.WriteString("\n")
		detailsHasContent = true
	}

	if !detailsHasContent {
		sb.WriteString("No upstream highlights for this release.\n\n")
	}

	sb.WriteString("</details>\n")

	return sb.String()
}

// writeEntries renders a slice of ChangelogEntry items as markdown list items.
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

