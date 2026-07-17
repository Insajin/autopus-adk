package selfupdate

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// Checker fetches and checks for new releases.
type Checker struct {
	apiBaseURL string
	authToken  string
	client     *http.Client
}

// CheckLatest checks GitHub API for the latest release.
// Returns nil if currentVersion is already up to date.
func (c *Checker) CheckLatest(currentVersion, goos, goarch string) (*ReleaseInfo, error) {
	info, err := c.FetchLatest(goos, goarch)
	if err != nil {
		return nil, err
	}

	latestVersion := strings.TrimPrefix(info.TagName, "v")
	if !IsNewerVersion(latestVersion, currentVersion) {
		return nil, nil
	}
	return info, nil
}

// FetchLatest fetches the latest release info regardless of version comparison.
// Used when --force reinstalls the current version.
func (c *Checker) FetchLatest(goos, goarch string) (*ReleaseInfo, error) {
	req, err := newSelfUpdateRequest(c.apiBaseURL, c.authToken)
	if err != nil {
		return nil, err
	}

	client := c.client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, githubAPIStatusError(resp)
	}

	var release map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}

	tagName, ok := release["tag_name"].(string)
	if !ok {
		return nil, fmt.Errorf("unexpected API response: missing or invalid tag_name")
	}
	version := strings.TrimPrefix(tagName, "v")

	assets, ok := release["assets"].([]any)
	if !ok {
		return nil, fmt.Errorf("unexpected API response: missing or invalid assets")
	}

	var archiveURL, checksumURL, signatureURL, archiveName string
	expectedArchive := ArchiveName(goos, goarch, version)

	for _, asset := range assets {
		a, ok := asset.(map[string]any)
		if !ok {
			continue
		}
		name, ok := a["name"].(string)
		if !ok {
			continue
		}
		url, ok := a["browser_download_url"].(string)
		if !ok {
			continue
		}

		switch name {
		case expectedArchive:
			archiveName = name
			archiveURL = url
		case "checksums.txt":
			checksumURL = url
		case "checksums.txt.sig":
			signatureURL = url
		}
	}

	return &ReleaseInfo{
		TagName:      tagName,
		ArchiveURL:   archiveURL,
		ChecksumURL:  checksumURL,
		SignatureURL: signatureURL,
		ArchiveName:  archiveName,
	}, nil
}

// NewChecker creates a new Checker with default settings.
func NewChecker(opts ...CheckerOption) *Checker {
	// @AX:NOTE: [AUTO] magic constant — GitHub releases API URL, repo path must match goreleaser config
	c := &Checker{
		apiBaseURL: "https://api.github.com/repos/insajin/autopus-adk/releases/latest",
		authToken:  githubTokenFromEnv(),
		client:     http.DefaultClient,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// CheckerOption is a functional option for Checker.
type CheckerOption func(*Checker)

// WithAPIBaseURL sets a custom API base URL for testing.
func WithAPIBaseURL(url string) CheckerOption {
	return func(c *Checker) {
		c.apiBaseURL = url
	}
}

// WithGitHubToken sets an explicit token for GitHub API requests.
func WithGitHubToken(token string) CheckerOption {
	return func(c *Checker) {
		c.authToken = strings.TrimSpace(token)
	}
}

// WithHTTPClient sets a custom HTTP client for testing.
func WithHTTPClient(client *http.Client) CheckerOption {
	return func(c *Checker) {
		if client != nil {
			c.client = client
		}
	}
}

func githubTokenFromEnv() string {
	for _, name := range []string{"AUTOPUS_GITHUB_TOKEN", "GITHUB_TOKEN", "GH_TOKEN"} {
		if token := strings.TrimSpace(os.Getenv(name)); token != "" {
			return token
		}
	}
	return ""
}

func newSelfUpdateRequest(rawURL, token string) (*http.Request, error) {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "autopus-adk-selfupdate")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if token = strings.TrimSpace(token); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return req, nil
}

func githubAPIStatusError(resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	message := strings.TrimSpace(githubAPIMessage(body))
	details := []string{fmt.Sprintf("GitHub API returned status %d", resp.StatusCode)}
	if message != "" {
		details = append(details, message)
	}
	if resp.StatusCode == http.StatusForbidden && resp.Header.Get("X-RateLimit-Remaining") == "0" {
		rateHint := "GitHub API rate limit exceeded; set GH_TOKEN, GITHUB_TOKEN, or AUTOPUS_GITHUB_TOKEN and retry"
		if reset := githubRateLimitReset(resp.Header.Get("X-RateLimit-Reset")); reset != "" {
			rateHint += "; reset at " + reset
		}
		details = append(details, rateHint)
	}
	return fmt.Errorf("%s", strings.Join(details, ": "))
}

func githubAPIMessage(body []byte) string {
	var payload struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &payload); err == nil && payload.Message != "" {
		return payload.Message
	}
	return string(body)
}

func githubRateLimitReset(raw string) string {
	seconds, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil || seconds <= 0 {
		return ""
	}
	return time.Unix(seconds, 0).UTC().Format(time.RFC3339)
}

// stripPreRelease removes pre-release suffixes ("-0.2026..." or "+dirty") from a version string,
// returning only the major.minor.patch portion for clean semver comparison.
func stripPreRelease(v string) string {
	v = strings.TrimPrefix(v, "v")
	if idx := strings.IndexByte(v, '-'); idx != -1 {
		v = v[:idx]
	}
	if idx := strings.IndexByte(v, '+'); idx != -1 {
		v = v[:idx]
	}
	return v
}

// IsNewerVersion returns true if latest > current using semantic versioning.
// Pre-release suffixes (e.g., "-0.20260328...+dirty") are stripped before comparison.
func IsNewerVersion(latest, current string) bool {
	latestParts := strings.Split(stripPreRelease(latest), ".")
	currentParts := strings.Split(stripPreRelease(current), ".")

	for i := 0; i < len(latestParts) && i < len(currentParts); i++ {
		lv, latestErr := strconv.Atoi(latestParts[i])
		cv, currentErr := strconv.Atoi(currentParts[i])
		if latestErr != nil || currentErr != nil {
			return latest > current
		}

		if lv > cv {
			return true
		}
		if lv < cv {
			return false
		}
	}

	return len(latestParts) > len(currentParts)
}
