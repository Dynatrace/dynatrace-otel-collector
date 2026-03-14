package main

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// ChangeType represents the type of a changelog entry.
type ChangeType string

const (
	Breaking     ChangeType = "breaking"
	Deprecation  ChangeType = "deprecation"
	NewComponent ChangeType = "new_component"
	Enhancement  ChangeType = "enhancement"
	BugFix       ChangeType = "bug_fix"
)

// validChangeTypes is the set of accepted change_type values.
var validChangeTypes = map[ChangeType]bool{
	Breaking:     true,
	Deprecation:  true,
	NewComponent: true,
	Enhancement:  true,
	BugFix:       true,
}

// ChloggenEntry is the raw upstream .chloggen/*.yaml entry.
type ChloggenEntry struct {
	ChangeType string   `yaml:"change_type"`
	Component  string   `yaml:"component"`
	Note       string   `yaml:"note"`
	Issues     []int    `yaml:"issues"`
	Subtext    string   `yaml:"subtext"`
	ChangeLogs []string `yaml:"change_logs"`
}

// ChangelogEntry is a processed entry ready for filtering and rendering.
type ChangelogEntry struct {
	Component       string
	Note            string
	Issues          []int
	Subtext         string
	ChangeType      ChangeType
	Source          string // "core" or "contrib"
	UpstreamVersion string // e.g., "v0.145.0"
	RepoURL         string // e.g., "https://github.com/open-telemetry/opentelemetry-collector-contrib"
}

// ParseChloggenEntry parses a single .chloggen YAML file's content.
// It returns nil if the entry should be skipped (e.g. api-only change_logs).
func ParseChloggenEntry(data []byte, source, upstreamVersion, repoURL string) (*ChangelogEntry, error) {
	var raw ChloggenEntry
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("yaml unmarshal: %w", err)
	}

	ct := ChangeType(strings.TrimSpace(raw.ChangeType))
	if !validChangeTypes[ct] {
		return nil, fmt.Errorf("unknown change_type %q", raw.ChangeType)
	}

	// Skip entries that are API-only (not user-facing).
	if !isUserFacing(raw.ChangeLogs) {
		return nil, nil
	}

	return &ChangelogEntry{
		Component:       strings.TrimSpace(raw.Component),
		Note:            strings.TrimSpace(raw.Note),
		Issues:          raw.Issues,
		Subtext:         strings.TrimSpace(raw.Subtext),
		ChangeType:      ct,
		Source:          source,
		UpstreamVersion: upstreamVersion,
		RepoURL:         repoURL,
	}, nil
}

// isUserFacing returns true when the entry targets the user-facing changelog.
// An empty/nil change_logs list is treated as [user] (the upstream default).
func isUserFacing(changeLogs []string) bool {
	if len(changeLogs) == 0 {
		return true
	}
	for _, cl := range changeLogs {
		if strings.TrimSpace(cl) == "user" {
			return true
		}
	}
	return false
}
