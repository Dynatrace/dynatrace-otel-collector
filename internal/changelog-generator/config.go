package main

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config holds the allowlist and denylist for upstream entry filtering.
type Config struct {
	Allowlist []string `yaml:"allowlist"`
	Denylist  []string `yaml:"denylist"`
}

// ParseConfig reads and parses the YAML config file.
func ParseConfig(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("reading config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parsing config: %w", err)
	}
	return cfg, nil
}

// MatchesAllowlist returns true if component matches any allowlist entry.
// Matching rules:
//   - Exact match: "pkg/ottl" matches "pkg/ottl"
//   - Prefix match: "pkg/stanza" matches "pkg/stanza" and "pkg/stanza/something"
//   - Glob suffix: "pkg/*" matches "pkg/anything"
func MatchesAllowlist(component string, allowlist []string) bool {
	for _, pattern := range allowlist {
		if matchPattern(component, pattern) {
			return true
		}
	}
	return false
}

// MatchesDenylist returns true if component matches any denylist entry.
func MatchesDenylist(component string, denylist []string) bool {
	for _, pattern := range denylist {
		if matchPattern(component, pattern) {
			return true
		}
	}
	return false
}

// matchPattern checks whether component matches a single pattern.
// Patterns ending with "*" are prefix-matched (glob-style).
// All other patterns match exactly or as a path prefix.
func matchPattern(component, pattern string) bool {
	if strings.HasSuffix(pattern, "*") {
		prefix := pattern[:len(pattern)-1]
		return strings.HasPrefix(component, prefix)
	}
	// Exact match.
	if component == pattern {
		return true
	}
	// Prefix match: "pkg/stanza" should also match "pkg/stanza/something".
	if strings.HasPrefix(component, pattern+"/") {
		return true
	}
	return false
}
