package main

import "testing"

func TestHighestVersion(t *testing.T) {
	got := highestVersion([]string{"v0.144.0", "0.145.0", "v0.143.9"})
	if got != "v0.145.0" {
		t.Fatalf("got %q, want %q", got, "v0.145.0")
	}
}

func TestSortedUniqueVersions(t *testing.T) {
	got := sortedUniqueVersions([]string{"v0.145.0", "0.144.0", "v0.145.0"})
	if len(got) != 2 || got[0] != "v0.144.0" || got[1] != "v0.145.0" {
		t.Fatalf("unexpected versions: %v", got)
	}
}
