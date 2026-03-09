package main

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var semverRegex = regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)$`)

type semVersion struct {
	major int
	minor int
	patch int
}

func parseSemVersion(v string) (semVersion, bool) {
	m := semverRegex.FindStringSubmatch(strings.TrimSpace(v))
	if m == nil {
		return semVersion{}, false
	}
	major, err := strconv.Atoi(m[1])
	if err != nil {
		return semVersion{}, false
	}
	minor, err := strconv.Atoi(m[2])
	if err != nil {
		return semVersion{}, false
	}
	patch, err := strconv.Atoi(m[3])
	if err != nil {
		return semVersion{}, false
	}
	return semVersion{major: major, minor: minor, patch: patch}, true
}

func canonicalVersion(v string) string {
	sv, ok := parseSemVersion(v)
	if !ok {
		return strings.TrimSpace(v)
	}
	return "v" + strconv.Itoa(sv.major) + "." + strconv.Itoa(sv.minor) + "." + strconv.Itoa(sv.patch)
}

func compareVersions(a, b string) int {
	sa, okA := parseSemVersion(a)
	sb, okB := parseSemVersion(b)
	if okA && okB {
		switch {
		case sa.major != sb.major:
			if sa.major < sb.major {
				return -1
			}
			return 1
		case sa.minor != sb.minor:
			if sa.minor < sb.minor {
				return -1
			}
			return 1
		case sa.patch != sb.patch:
			if sa.patch < sb.patch {
				return -1
			}
			return 1
		default:
			return 0
		}
	}
	if okA && !okB {
		return 1
	}
	if !okA && okB {
		return -1
	}
	ac := canonicalVersion(a)
	bc := canonicalVersion(b)
	switch {
	case ac < bc:
		return -1
	case ac > bc:
		return 1
	default:
		return 0
	}
}

func highestVersion(versions []string) string {
	best := ""
	for _, v := range versions {
		vc := canonicalVersion(v)
		if vc == "" {
			continue
		}
		if best == "" || compareVersions(vc, best) > 0 {
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
		if vc == "" || seen[vc] {
			continue
		}
		seen[vc] = true
		uniq = append(uniq, vc)
	}

	sort.SliceStable(uniq, func(i, j int) bool {
		return compareVersions(uniq[i], uniq[j]) < 0
	})

	return uniq
}
