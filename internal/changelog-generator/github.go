package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/mod/semver"
	"gopkg.in/yaml.v3"
)

// githubClient wraps the GitHub REST API.
type githubClient struct {
	token      string
	httpClient *http.Client
}

func newGitHubClient() *githubClient {
	return &githubClient{
		token: os.Getenv("GITHUB_TOKEN"),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// PRInfo holds the information extracted from a GitHub pull request.
type PRInfo struct {
	BaseSHA         string
	UpstreamVersion string // e.g. "v0.145.0"
	Source          string // "core" or "contrib"
	RepoURL         string // base HTML URL without trailing slash
	Owner           string
	Repo            string
}

// parsePRURL extracts owner, repo, and PR number from a GitHub PR URL.
// Accepted format: https://github.com/{owner}/{repo}/pull/{number}
func parsePRURL(rawURL string) (owner, repo string, number int, err error) {
	re := regexp.MustCompile(`^https://github\.com/([^/]+)/([^/]+)/pull/(\d+)/?$`)
	m := re.FindStringSubmatch(strings.TrimSpace(rawURL))
	if m == nil {
		return "", "", 0, fmt.Errorf("invalid PR URL: %q", rawURL)
	}
	n, err := strconv.Atoi(m[3])
	if err != nil {
		return "", "", 0, fmt.Errorf("invalid PR number %q: %w", m[3], err)
	}
	return m[1], m[2], n, nil
}

// FetchPRInfo retrieves metadata about a "prepare release" PR.
func (c *githubClient) FetchPRInfo(prURL string) (PRInfo, error) {
	owner, repo, number, err := parsePRURL(prURL)
	if err != nil {
		return PRInfo{}, err
	}

	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls/%d", owner, repo, number)
	body, err := c.get(apiURL)
	if err != nil {
		return PRInfo{}, fmt.Errorf("fetching PR %s: %w", apiURL, err)
	}

	var pr struct {
		Title string `json:"title"`
		Base  struct {
			SHA string `json:"sha"`
		} `json:"base"`
		Head struct {
			SHA string `json:"sha"`
		} `json:"head"`
	}
	if err := json.Unmarshal(body, &pr); err != nil {
		return PRInfo{}, fmt.Errorf("parsing PR response: %w", err)
	}
	if pr.Base.SHA == "" {
		return PRInfo{}, fmt.Errorf("PR base SHA is empty")
	}
	if pr.Head.SHA == "" {
		return PRInfo{}, fmt.Errorf("PR head SHA is empty")
	}

	version, err := c.fetchVersionFromVersionsFile(owner, repo, pr.Head.SHA)
	if err != nil {
		return PRInfo{}, fmt.Errorf("reading versions.yaml for %s/%s at %s: %w", owner, repo, pr.Head.SHA[:min(8, len(pr.Head.SHA))], err)
	}

	source := "core"
	if strings.Contains(repo, "contrib") {
		source = "contrib"
	}
	repoURL := "https://github.com/" + owner + "/" + repo

	return PRInfo{
		BaseSHA:         pr.Base.SHA,
		UpstreamVersion: version,
		Source:          source,
		RepoURL:         repoURL,
		Owner:           owner,
		Repo:            repo,
	}, nil
}

func (c *githubClient) fetchVersionFromVersionsFile(owner, repo, ref string) (string, error) {
	// Fetch versions.yaml directly from raw.githubusercontent.com — no metadata round-trip needed.
	rawURL := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/versions.yaml", owner, repo, ref)
	data, err := c.getRaw(rawURL)
	if err != nil {
		return "", fmt.Errorf("fetching versions.yaml: %w", err)
	}

	version, err := extractVersionFromVersionsYAML(data)
	if err != nil {
		return "", err
	}
	return version, nil
}

// knownModuleSetKeys are the module-set names tried in order when looking up
// the release version in versions.yaml. The list covers:
//   - "beta"         — opentelemetry-collector core beta modules
//   - "stable"         — opentelemetry-collector core stable modules
//   - "contrib-base" — opentelemetry-collector-contrib
var knownModuleSetKeys = []string{"beta", "contrib-base", "stable"}

// versionsYAML mirrors the relevant subset of the versions.yaml schema used by
// the opentelemetry-collector and opentelemetry-collector-contrib release tooling.
type versionsYAML struct {
	ModuleSets map[string]struct {
		Version string `yaml:"version"`
	} `yaml:"module-sets"`
}

func extractVersionFromVersionsYAML(data []byte) (string, error) {
	var v versionsYAML
	if err := yaml.Unmarshal(data, &v); err != nil {
		return "", fmt.Errorf("parsing versions.yaml: %w", err)
	}

	for _, key := range knownModuleSetKeys {
		version := strings.TrimSpace(v.ModuleSets[key].Version)
		if version == "" {
			continue
		}
		vc := canonicalVersion(version)
		if !semver.IsValid(vc) {
			return "", fmt.Errorf("invalid version %q in versions.yaml module-sets.%s", version, key)
		}
		return vc, nil
	}

	return "", fmt.Errorf("versions.yaml: could not find version under any of module-sets.{%s}", strings.Join(knownModuleSetKeys, ", "))
}

// githubContent is a file entry returned by the GitHub contents API.
type githubContent struct {
	Name        string `json:"name"`
	DownloadURL string `json:"download_url"`
	Type        string `json:"type"`
}

// FetchChloggenEntries fetches and parses all .chloggen/*.yaml files from the
// base commit of the given PR.
func (c *githubClient) FetchChloggenEntries(info PRInfo) ([]ChangelogEntry, error) {
	// List .chloggen/ directory at base SHA.
	listURL := fmt.Sprintf(
		"https://api.github.com/repos/%s/%s/contents/.chloggen?ref=%s",
		info.Owner, info.Repo, info.BaseSHA,
	)
	body, err := c.get(listURL)
	if err != nil {
		return nil, fmt.Errorf("listing .chloggen at %s: %w", listURL, err)
	}

	var contents []githubContent
	if err := json.Unmarshal(body, &contents); err != nil {
		return nil, fmt.Errorf("parsing .chloggen listing: %w", err)
	}

	var entries []ChangelogEntry
	for _, f := range contents {
		if f.Type != "file" {
			continue
		}
		if !strings.HasSuffix(f.Name, ".yaml") {
			continue
		}
		// Skip template and config files.
		if f.Name == "TEMPLATE.yaml" || f.Name == "config.yaml" {
			continue
		}

		fileBody, err := c.getRaw(f.DownloadURL)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: skipping %s: %v\n", f.Name, err)
			continue
		}

		entry, err := ParseChloggenEntry(fileBody, info.Source, info.UpstreamVersion, info.RepoURL)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: parsing %s: %v\n", f.Name, err)
			continue
		}
		if entry == nil {
			// Skipped (e.g. api-only).
			continue
		}
		entries = append(entries, *entry)
	}

	return entries, nil
}

// get performs an authenticated GET against the GitHub API and returns the
// response body. It retries once on rate-limit responses.
func (c *githubClient) get(url string) ([]byte, error) {
	return c.doGet(url, true)
}

// getRaw performs a GET for raw file content (no JSON API headers needed).
func (c *githubClient) getRaw(url string) ([]byte, error) {
	return c.doGet(url, false)
}

func (c *githubClient) doGet(url string, apiHeaders bool) ([]byte, error) {
	const maxAttempts = 2

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		if c.token != "" {
			req.Header.Set("Authorization", "Bearer "+c.token)
		}
		if apiHeaders {
			req.Header.Set("Accept", "application/vnd.github+json")
			req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, err
		}

		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("reading response body: %w", readErr)
		}

		if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests {
			if attempt < maxAttempts {
				time.Sleep(retryDelay(resp.Header))
				continue
			}
			return nil, fmt.Errorf("GitHub API rate limit or auth error (HTTP %d): %s", resp.StatusCode, string(body))
		}
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("unexpected HTTP %d from %s: %s", resp.StatusCode, url, string(body))
		}

		return body, nil
	}

	return nil, fmt.Errorf("exhausted retries for %s", url)
}

func retryDelay(headers http.Header) time.Duration {
	if raw := strings.TrimSpace(headers.Get("Retry-After")); raw != "" {
		if seconds, err := strconv.Atoi(raw); err == nil {
			d := time.Duration(seconds) * time.Second
			if d < 0 {
				return 2 * time.Second
			}
			if d > 30*time.Second {
				return 30 * time.Second
			}
			return d
		}
	}

	if raw := strings.TrimSpace(headers.Get("X-RateLimit-Reset")); raw != "" {
		if ts, err := strconv.ParseInt(raw, 10, 64); err == nil {
			wait := time.Until(time.Unix(ts, 0))
			if wait <= 0 {
				return 2 * time.Second
			}
			if wait > 30*time.Second {
				return 30 * time.Second
			}
			return wait
		}
	}

	return 2 * time.Second
}
