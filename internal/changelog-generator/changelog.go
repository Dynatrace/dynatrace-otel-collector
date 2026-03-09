package main

import (
	"fmt"
	"os"
	"strings"
)

const (
	nextVersionMarker    = "<!-- next version -->"
	previousVersionMarker = "<!-- previous-version -->"
)

// InsertChangelog reads CHANGELOG.md, finds the insertion point between the
// <!-- next version --> and <!-- previous-version --> markers, inserts the
// generated content, and writes the result back to disk.
//
// If a previously-generated upstream section already exists (detected by the
// presence of a "## v" header between the two markers), it is replaced.
func InsertChangelog(path, content string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading changelog: %w", err)
	}

	original := string(data)

	nextIdx := strings.Index(original, nextVersionMarker)
	if nextIdx == -1 {
		return fmt.Errorf("marker %q not found in %s", nextVersionMarker, path)
	}

	// Find the first <!-- previous-version --> after <!-- next version -->.
	prevIdx := strings.Index(original[nextIdx:], previousVersionMarker)
	if prevIdx == -1 {
		return fmt.Errorf("marker %q not found in %s", previousVersionMarker, path)
	}
	prevIdx += nextIdx // Absolute index.

	// Everything between the end of <!-- next version --> and the start of
	// <!-- previous-version --> is the "current block".
	afterNext := nextIdx + len(nextVersionMarker)
	currentBlock := original[afterNext:prevIdx]

	// If the current block already has an upstream-generated section (a "## v"
	// header) that was produced by a previous run, strip it so we can replace it.
	newBlock := buildNewBlock(currentBlock, content)

	result := original[:afterNext] + newBlock + original[prevIdx:]

	return os.WriteFile(path, []byte(result), 0o644)
}

// buildNewBlock merges any existing chloggen distro content with the new
// upstream-generated content.
//
// Strategy:
//   - If the current block already contains a "## v" header from a prior run
//     of this tool, drop that header and everything after it, keeping only the
//     distro-specific content before it.
//   - Append the new upstream content.
func buildNewBlock(currentBlock, newContent string) string {
	// Look for an existing upstream section marker ("## v").
	if idx := strings.Index(currentBlock, "\n## v"); idx != -1 {
		// Keep only the distro-specific part that precedes the old upstream block.
		currentBlock = currentBlock[:idx]
	}

	// Ensure there is exactly one blank line between the next-version marker
	// and the new content, and between any existing distro content and the new
	// upstream section.
	trimmed := strings.TrimRight(currentBlock, "\n")
	if trimmed == "" {
		return "\n\n" + newContent + "\n"
	}
	return "\n\n" + trimmed + "\n\n" + newContent + "\n"
}

