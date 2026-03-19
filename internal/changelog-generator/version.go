package main

import (
	"strings"

	"golang.org/x/mod/semver"
)

// canonicalVersion normalizes a version string to the "vX.Y.Z" form expected
// by golang.org/x/mod/semver. Bare versions without the "v" prefix (e.g.
// "0.145.0") are accepted and normalized automatically.
func canonicalVersion(v string) string {
	v = strings.TrimSpace(v)
	if !strings.HasPrefix(v, "v") {
		v = "v" + v
	}
	c := semver.Canonical(v)
	if c == "" {
		return strings.TrimSpace(v) // not a valid semver; return as-is
	}
	return c
}

func highestVersion(versions []string) string {
	best := ""
	for _, v := range versions {
		vc := canonicalVersion(v)
		if !semver.IsValid(vc) {
			continue
		}
		if best == "" || semver.Compare(vc, best) > 0 {
			best = vc
		}
	}
	return best
}

func sortedUniqueVersions(versions []string) []string {
	seen := make(map[string]bool)
	uniq := make([]string, 0, len(versions))
	for _, v := range versions {
		vc := canonicalVersion(v)
		if !semver.IsValid(vc) || seen[vc] {
			continue
		}
		seen[vc] = true
		uniq = append(uniq, vc)
	}
	semver.Sort(uniq)
	return uniq
}
