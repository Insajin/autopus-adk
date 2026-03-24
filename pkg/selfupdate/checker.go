package selfupdate

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// Checker fetches and checks for new releases.
type Checker struct {
	apiBaseURL string
}

// CheckLatest checks GitHub API for the latest release.
func (c *Checker) CheckLatest(currentVersion string) (*ReleaseInfo, error) {
	resp, err := http.Get(c.apiBaseURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}

	tagName, ok := release["tag_name"].(string)
	if !ok {
		return nil, fmt.Errorf("unexpected API response: missing or invalid tag_name")
	}
	latestVersion := strings.TrimPrefix(tagName, "v")

	if !IsNewerVersion(latestVersion, currentVersion) {
		return nil, nil
	}

	assets, ok := release["assets"].([]any)
	if !ok {
		return nil, fmt.Errorf("unexpected API response: missing or invalid assets")
	}
	var archiveURL, checksumURL, archiveName string

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

		if strings.HasSuffix(name, ".tar.gz") {
			archiveName = name
			archiveURL = url
		} else if name == "checksums.txt" {
			checksumURL = url
		}
	}

	return &ReleaseInfo{
		TagName:     tagName,
		ArchiveURL:  archiveURL,
		ChecksumURL: checksumURL,
		ArchiveName: archiveName,
	}, nil
}

// NewChecker creates a new Checker with default settings.
func NewChecker(opts ...CheckerOption) *Checker {
	// @AX:NOTE: [AUTO] magic constant — GitHub releases API URL, repo path must match goreleaser config
	c := &Checker{
		apiBaseURL: "https://api.github.com/repos/insajin/autopus-adk/releases/latest",
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

// IsNewerVersion returns true if latest > current using semantic versioning.
func IsNewerVersion(latest, current string) bool {
	latestParts := strings.Split(strings.TrimPrefix(latest, "v"), ".")
	currentParts := strings.Split(strings.TrimPrefix(current, "v"), ".")

	for i := 0; i < len(latestParts) && i < len(currentParts); i++ {
		var lv, cv int
		fmt.Sscanf(latestParts[i], "%d", &lv)
		fmt.Sscanf(currentParts[i], "%d", &cv)

		if lv > cv {
			return true
		}
		if lv < cv {
			return false
		}
	}

	return len(latestParts) > len(currentParts)
}
