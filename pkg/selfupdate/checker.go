package selfupdate

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

// Checker fetches and checks for new releases.
type Checker struct {
	apiBaseURL string
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
	version := strings.TrimPrefix(tagName, "v")

	assets, ok := release["assets"].([]any)
	if !ok {
		return nil, fmt.Errorf("unexpected API response: missing or invalid assets")
	}

	var archiveURL, checksumURL, archiveName string
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
