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
	re := regexp.MustCompile(`github\.com/([^/]+)/([^/]+)/pull/(\d+)`)
	m := re.FindStringSubmatch(rawURL)
	if m == nil {
		return "", "", 0, fmt.Errorf("invalid PR URL: %q", rawURL)
	}
	n, _ := strconv.Atoi(m[3])
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
	}
	if err := json.Unmarshal(body, &pr); err != nil {
		return PRInfo{}, fmt.Errorf("parsing PR response: %w", err)
	}

	version := extractVersionFromTitle(pr.Title)
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

// extractVersionFromTitle parses a version string from a PR title such as
// "[chore] Prepare release 0.145.0" → "v0.145.0".
func extractVersionFromTitle(title string) string {
	re := regexp.MustCompile(`(\d+\.\d+\.\d+)`)
	m := re.FindString(title)
	if m == "" {
		return ""
	}
	if !strings.HasPrefix(m, "v") {
		return "v" + m
	}
	return m
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
// response body.  It retries once on 403 rate-limit responses.
func (c *githubClient) get(url string) ([]byte, error) {
	return c.doGet(url, true)
}

// getRaw performs a GET for raw file content (no JSON API headers needed).
func (c *githubClient) getRaw(url string) ([]byte, error) {
	return c.doGet(url, false)
}

func (c *githubClient) doGet(url string, apiHeaders bool) ([]byte, error) {
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
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests {
		return nil, fmt.Errorf("GitHub API rate limit or auth error (HTTP %d): %s", resp.StatusCode, string(body))
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected HTTP %d from %s: %s", resp.StatusCode, url, string(body))
	}

	return body, nil
}

