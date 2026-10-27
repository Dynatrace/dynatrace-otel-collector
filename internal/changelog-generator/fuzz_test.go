// Copyright Dynatrace LLC
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"golang.org/x/mod/semver"
)

func FuzzParseChloggenEntry(f *testing.F) {
	seed, err := os.ReadFile("testdata/entries/contrib/filelog-enhancement.yaml")
	if err != nil {
		f.Fatal(err)
	}
	f.Add(seed)
	f.Add([]byte("change_type: bug_fix\ncomponent: x\nnote: y\nissues: [1]\nchange_logs: [api]\n"))
	f.Fuzz(func(t *testing.T, data []byte) {
		entry, err := ParseChloggenEntry(data, "contrib", "v0.145.0", "https://github.com/open-telemetry/opentelemetry-collector-contrib")
		if err != nil {
			if entry != nil {
				t.Fatalf("got entry %+v alongside error %v", entry, err)
			}
			return
		}
		if entry == nil {
			return // skipped: not user-facing
		}
		if !validChangeTypes[entry.ChangeType] {
			t.Fatalf("invalid change type %q accepted", entry.ChangeType)
		}
		for _, s := range []string{entry.Component, entry.Note, entry.Subtext} {
			if s != strings.TrimSpace(s) {
				t.Fatalf("field not trimmed: %q", s)
			}
		}
	})
}

func FuzzExtractVersionFromVersionsYAML(f *testing.F) {
	f.Add([]byte("module-sets:\n  beta:\n    version: v0.145.0\n"))
	f.Add([]byte("module-sets: {}\n"))
	f.Fuzz(func(t *testing.T, data []byte) {
		v, err := extractVersionFromVersionsYAML(data)
		if err != nil {
			if v != "" {
				t.Fatalf("got version %q alongside error %v", v, err)
			}
			return
		}
		if !semver.IsValid(v) {
			t.Fatalf("returned invalid semver %q", v)
		}
	})
}

func FuzzParsePRURL(f *testing.F) {
	f.Add("https://github.com/open-telemetry/opentelemetry-collector/pull/123")
	f.Add("https://github.com/a/b/pull/99999999999999999999/")
	f.Fuzz(func(t *testing.T, rawURL string) {
		owner, repo, number, err := parsePRURL(rawURL)
		if err != nil {
			return
		}
		if owner == "" || repo == "" || strings.Contains(owner+repo, "/") || number < 0 {
			t.Fatalf("bad result for %q: %q %q %d", rawURL, owner, repo, number)
		}
		// Round trip: the canonical URL must parse back to the same values.
		o2, r2, n2, err := parsePRURL(fmt.Sprintf("https://github.com/%s/%s/pull/%d", owner, repo, number))
		if err != nil || o2 != owner || r2 != repo || n2 != number {
			t.Fatalf("round trip failed for %q: %q %q %d %v", rawURL, o2, r2, n2, err)
		}
	})
}
