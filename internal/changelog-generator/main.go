package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	manifestPath := flag.String("manifest", "manifest.yaml", "Path to manifest.yaml")
	configPath := flag.String("config", "internal/changelog-generator/config.yaml", "Path to allow/denylist config")
	changelogPath := flag.String("changelog", "CHANGELOG.md", "Path to CHANGELOG.md")
	dryRun := flag.Bool("dry-run", false, "Print generated changelog to stdout without modifying files")
	flag.Parse()

	prURLs := flag.Args()
	if len(prURLs) == 0 {
		fmt.Fprintln(os.Stderr, "error: at least one PR URL is required")
		flag.Usage()
		os.Exit(1)
	}

	if err := run(*manifestPath, *configPath, *changelogPath, *dryRun, prURLs); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(manifestPath, configPath, changelogPath string, dryRun bool, prURLs []string) error {
	// 1. Parse manifest.yaml.
	components, distVersion, err := ParseManifest(manifestPath)
	if err != nil {
		return fmt.Errorf("parsing manifest: %w", err)
	}
	fmt.Fprintf(os.Stderr, "info: dist version: %s, %d components loaded from manifest\n",
		distVersion, len(components))

	// 2. Parse config.yaml.
	cfg, err := ParseConfig(configPath)
	if err != nil {
		return fmt.Errorf("parsing config: %w", err)
	}
	fmt.Fprintf(os.Stderr, "info: %d allowlist, %d denylist entries\n",
		len(cfg.Allowlist), len(cfg.Denylist))

	// 3. Fetch and parse chloggen entries from each PR.
	client := newGitHubClient()
	var allEntries []ChangelogEntry

	for _, prURL := range prURLs {
		fmt.Fprintf(os.Stderr, "info: fetching PR %s\n", prURL)

		info, err := client.FetchPRInfo(prURL)
		if err != nil {
			return fmt.Errorf("fetching PR info for %s: %w", prURL, err)
		}
		fmt.Fprintf(os.Stderr, "info: PR source=%s version=%s base=%s\n",
			info.Source, info.UpstreamVersion, info.BaseSHA[:min(8, len(info.BaseSHA))])

		entries, err := client.FetchChloggenEntries(info)
		if err != nil {
			return fmt.Errorf("fetching chloggen entries for %s: %w", prURL, err)
		}
		fmt.Fprintf(os.Stderr, "info: %d user-facing entries fetched from %s\n", len(entries), prURL)

		allEntries = append(allEntries, entries...)
	}

	// 4. Filter entries.
	fc := FilterEntries(allEntries, components, cfg)
	fmt.Fprintf(os.Stderr, "info: filtered to %d breaking, %d deprecations, %d new components, %d enhancements, %d bug fixes\n",
		len(fc.Breaking), len(fc.Deprecations), len(fc.NewComponents),
		len(fc.Enhancements), len(fc.BugFixes))

	// 5. Generate markdown.
	if highestVersion(fc.UpstreamVersions) == "" {
		return fmt.Errorf("no valid upstream versions found — check PR URLs and versions.yaml content")
	}
	markdown := GenerateChangelog(fc)

	// 6. Output.
	if dryRun {
		fmt.Print(markdown)
		return nil
	}

	if err := InsertChangelog(changelogPath, markdown); err != nil {
		return fmt.Errorf("inserting changelog: %w", err)
	}
	fmt.Fprintf(os.Stderr, "info: %s updated successfully\n", changelogPath)
	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
