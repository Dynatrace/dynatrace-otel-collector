package main

// FilteredChangelog holds the filtered and grouped entries ready for rendering.
type FilteredChangelog struct {
	UpstreamVersions []string // Distinct upstream versions covered (ordered).
	CoreRepoURL      string
	ContribRepoURL   string
	Breaking         []ChangelogEntry
	Deprecations     []ChangelogEntry
	NewComponents    []ChangelogEntry
	Enhancements     []ChangelogEntry
	BugFixes         []ChangelogEntry
}

// FilterEntries filters raw entries against the manifest component set and
// the allowlist/denylist from config.  Denylist takes precedence.
func FilterEntries(entries []ChangelogEntry, components map[string]bool, cfg Config) FilteredChangelog {
	fc := FilteredChangelog{}
	seenVersions := map[string]bool{}

	for _, e := range entries {
		// Track distinct upstream versions.
		if !seenVersions[e.UpstreamVersion] {
			seenVersions[e.UpstreamVersion] = true
			fc.UpstreamVersions = append(fc.UpstreamVersions, e.UpstreamVersion)
		}

		// Populate repo URLs from entries.
		if e.Source == "core" && fc.CoreRepoURL == "" {
			fc.CoreRepoURL = e.RepoURL
		}
		if e.Source == "contrib" && fc.ContribRepoURL == "" {
			fc.ContribRepoURL = e.RepoURL
		}

		if !shouldInclude(e.Component, components, cfg) {
			continue
		}

		switch e.ChangeType {
		case Breaking:
			fc.Breaking = append(fc.Breaking, e)
		case Deprecation:
			fc.Deprecations = append(fc.Deprecations, e)
		case NewComponent:
			fc.NewComponents = append(fc.NewComponents, e)
		case Enhancement:
			fc.Enhancements = append(fc.Enhancements, e)
		case BugFix:
			fc.BugFixes = append(fc.BugFixes, e)
		}
	}

	return fc
}

// knownComponentAliases maps canonical component IDs to known upstream aliases.
var knownComponentAliases = map[string][]string{
	"extension/healthcheck": {"extension/health_check"},
	"exporter/otlp":         {"exporter/otlp_grpc"},
	"exporter/otlphttp":     {"exporter/otlp_http"},
}

var aliasToCanonicalComponent = buildAliasToCanonical(knownComponentAliases)

func buildAliasToCanonical(in map[string][]string) map[string]string {
	out := make(map[string]string)
	for canonical, aliases := range in {
		for _, alias := range aliases {
			out[alias] = canonical
		}
	}
	return out
}

// shouldInclude decides whether a component should be included in the output.
//
//  1. If it matches the denylist → exclude.
//  2. If it matches a manifest component → include.
//  3. If it matches the allowlist → include.
//  4. Otherwise → exclude.
func shouldInclude(component string, manifestComponents map[string]bool, cfg Config) bool {
	candidates := componentMatchCandidates(component)

	for _, candidate := range candidates {
		if MatchesDenylist(candidate, cfg.Denylist) {
			return false
		}
	}
	for _, candidate := range candidates {
		if manifestComponents[candidate] {
			return true
		}
	}
	for _, candidate := range candidates {
		if MatchesAllowlist(candidate, cfg.Allowlist) {
			return true
		}
	}
	return false
}

func componentMatchCandidates(component string) []string {
	candidates := []string{component}
	if canonical, ok := aliasToCanonicalComponent[component]; ok {
		candidates = append(candidates, canonical)
	}
	if aliases, ok := knownComponentAliases[component]; ok {
		candidates = append(candidates, aliases...)
	}
	return candidates
}
