package main

import (
	"fmt"
	"os"
	"strings"
)

const (
	upstreamStartMarker    = "<!-- upstream-start -->"
	upstreamEndMarker      = "<!-- upstream-end -->"
	upstreamVersionMarker  = "<!-- upstream-version -->"
	upstreamVersionsMarker = "<!-- upstream-collector-versions -->"
	upstreamBreakingMarker = "<!-- upstream-breaking-changes -->"
	upstreamOtherMarker    = "<!-- upstream-other-changes -->"
)

// FillUpstreamPlaceholders reads the CHANGELOG.md at path and either fills the
// upstream placeholder comments with content from UpstreamContent, or removes
// the entire upstream section when UpstreamContent is zero-valued (no upstream
// release).
//
// The expected scaffold (produced by `make chlog-update`) contains:
//
//	<!-- upstream-start -->
//	This release includes version <!-- upstream-version --> ...
//	<!-- upstream-collector-versions -->
//	<!-- upstream-breaking-changes -->
//	<details>
//	<!-- upstream-other-changes -->
//	</details>
//	<!-- upstream-end -->
//
// After this function runs, the boundary markers are removed and only the
// rendered content (or nothing) remains.
func FillUpstreamPlaceholders(path string, content UpstreamContent) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading changelog: %w", err)
	}

	s := string(data)

	if content.VersionIntro == "" {
		s = removeUpstreamSection(s)
	} else {
		s = strings.ReplaceAll(s, upstreamVersionMarker, content.VersionIntro)
		s = fillOrRemovePlaceholder(s, upstreamVersionsMarker, content.CollectorVersions)
		s = fillOrRemovePlaceholder(s, upstreamBreakingMarker, content.BreakingChanges)
		s = fillOrRemovePlaceholder(s, upstreamOtherMarker, content.OtherChanges)
		// Remove the boundary markers now that the content is in place.
		s = strings.ReplaceAll(s, upstreamStartMarker+"\n", "")
		s = strings.ReplaceAll(s, "\n\n"+upstreamEndMarker, "\n")
	}

	return os.WriteFile(path, []byte(s), 0o644)
}

// removeUpstreamSection removes everything between <!-- upstream-start --> and
// <!-- upstream-end --> (both inclusive) and normalizes the surrounding
// whitespace to a single blank line.
func removeUpstreamSection(s string) string {
	startIdx := strings.Index(s, upstreamStartMarker)
	endIdx := strings.Index(s, upstreamEndMarker)
	if startIdx == -1 || endIdx == -1 {
		return s
	}
	endIdx += len(upstreamEndMarker)

	before := strings.TrimRight(s[:startIdx], " \t\n")
	after := strings.TrimLeft(s[endIdx:], " \t\n")
	return before + "\n\n" + after
}

// fillOrRemovePlaceholder replaces a placeholder comment that lives on its own
// line (surrounded by blank lines) with content. When content is empty the
// placeholder line is removed entirely to avoid stray blank lines.
func fillOrRemovePlaceholder(s, placeholder, content string) string {
	if content == "" {
		return strings.ReplaceAll(s, "\n\n"+placeholder+"\n", "\n")
	}
	return strings.ReplaceAll(s, placeholder, content)
}
