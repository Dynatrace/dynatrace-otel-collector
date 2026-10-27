// Copyright Dynatrace LLC
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"testing"
)

func FuzzParseChloggenEntry(f *testing.F) {
	seed, err := os.ReadFile("testdata/entries/contrib/filelog-enhancement.yaml")
	if err != nil {
		f.Fatal(err)
	}
	f.Add(seed)
	f.Add([]byte("change_type: bug_fix\ncomponent: x\nnote: y\nissues: [1]\nchange_logs: [api]\n"))
	f.Fuzz(func(_ *testing.T, data []byte) {
		_, _ = ParseChloggenEntry(data, "contrib", "v0.145.0", "https://github.com/open-telemetry/opentelemetry-collector-contrib")
	})
}

func FuzzExtractVersionFromVersionsYAML(f *testing.F) {
	f.Add([]byte("module-sets:\n  beta:\n    version: v0.145.0\n"))
	f.Add([]byte("module-sets: {}\n"))
	f.Fuzz(func(_ *testing.T, data []byte) {
		_, _ = extractVersionFromVersionsYAML(data)
	})
}

func FuzzParsePRURL(f *testing.F) {
	f.Add("https://github.com/open-telemetry/opentelemetry-collector/pull/123")
	f.Add("https://github.com/a/b/pull/99999999999999999999/")
	f.Fuzz(func(_ *testing.T, rawURL string) {
		_, _, _, _ = parsePRURL(rawURL)
	})
}
