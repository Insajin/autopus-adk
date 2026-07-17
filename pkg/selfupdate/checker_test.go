package selfupdate

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCheckLatest_NewVersionAvailable verifies that when GitHub API returns a
// newer version than the current one, ReleaseInfo is returned with the new version.
// R1: auto update --self fetches latest release from GitHub API.
func TestCheckLatest_NewVersionAvailable(t *testing.T) {
	t.Parallel()

	// Given: a mock GitHub API server that returns v0.7.0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"tag_name": "v0.7.0",
			"assets": []map[string]any{
				{"name": "autopus-adk_0.7.0_darwin_arm64.tar.gz", "browser_download_url": "https://example.com/autopus-adk_0.7.0_darwin_arm64.tar.gz"},
				{"name": "checksums.txt", "browser_download_url": "https://example.com/checksums.txt"},
				{"name": "checksums.txt.signatures", "browser_download_url": "https://example.com/checksums.txt.signatures"},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	// When: CheckLatest is called with current version 0.6.0
	checker := NewChecker(WithAPIBaseURL(srv.URL))
	info, err := checker.CheckLatest("0.6.0", "darwin", "arm64")

	// Then: ReleaseInfo is returned with the newer version
	require.NoError(t, err)
	require.NotNil(t, info)
	assert.Equal(t, "v0.7.0", info.TagName)
	assert.Equal(t, "https://example.com/checksums.txt.signatures", info.SignatureURL)
}

// TestCheckLatest_AlreadyUpToDate verifies that when current version matches
// the latest, nil is returned indicating no update is needed.
// R7: if already up-to-date, print message and exit 0.
func TestCheckLatest_AlreadyUpToDate(t *testing.T) {
	t.Parallel()

	// Given: a mock GitHub API server that returns v0.6.0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"tag_name": "v0.6.0",
			"assets":   []map[string]any{},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	// When: CheckLatest is called with current version 0.6.0
	checker := NewChecker(WithAPIBaseURL(srv.URL))
	info, err := checker.CheckLatest("0.6.0", "darwin", "arm64")

	// Then: nil ReleaseInfo is returned (no update needed)
	require.NoError(t, err)
	assert.Nil(t, info)
}

// TestCheckLatest_APIError verifies that HTTP errors from GitHub API are
// propagated as errors.
func TestCheckLatest_APIError(t *testing.T) {
	t.Parallel()

	// Given: a mock GitHub API server that returns HTTP 500
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	// When: CheckLatest is called
	checker := NewChecker(WithAPIBaseURL(srv.URL))
	info, err := checker.CheckLatest("0.6.0", "darwin", "arm64")

	// Then: an error is returned
	require.Error(t, err)
	assert.Nil(t, info)
}

// TestCheckLatest_SendsGitHubHeaders verifies that release checks identify the
// client with GitHub's expected REST API headers.
func TestCheckLatest_SendsGitHubHeaders(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "autopus-adk-selfupdate", r.Header.Get("User-Agent"))
		assert.Equal(t, "application/vnd.github+json", r.Header.Get("Accept"))
		assert.Equal(t, "2022-11-28", r.Header.Get("X-GitHub-Api-Version"))

		resp := map[string]any{
			"tag_name": "v0.7.0",
			"assets":   []map[string]any{},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	checker := NewChecker(WithAPIBaseURL(srv.URL))
	info, err := checker.CheckLatest("0.6.0", "darwin", "arm64")

	require.NoError(t, err)
	require.NotNil(t, info)
	assert.Equal(t, "v0.7.0", info.TagName)
}

// TestCheckLatest_UsesConfiguredToken verifies that callers can raise GitHub
// API rate limits by supplying an auth token.
func TestCheckLatest_UsesConfiguredToken(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		resp := map[string]any{
			"tag_name": "v0.7.0",
			"assets":   []map[string]any{},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	checker := NewChecker(WithAPIBaseURL(srv.URL), WithGitHubToken(" test-token "))
	info, err := checker.CheckLatest("0.6.0", "darwin", "arm64")

	require.NoError(t, err)
	require.NotNil(t, info)
}

// TestCheckLatest_RateLimit403Actionable verifies that GitHub API rate-limit
// failures tell the user how to retry with an authenticated request.
func TestCheckLatest_RateLimit403Actionable(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.Header().Set("X-RateLimit-Reset", "1767225600")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message":"API rate limit exceeded"}`))
	}))
	defer srv.Close()

	checker := NewChecker(WithAPIBaseURL(srv.URL))
	info, err := checker.CheckLatest("0.6.0", "darwin", "arm64")

	require.Error(t, err)
	assert.Nil(t, info)
	assert.Contains(t, err.Error(), "GitHub API returned status 403")
	assert.Contains(t, err.Error(), "API rate limit exceeded")
	assert.Contains(t, err.Error(), "GH_TOKEN")
	assert.Contains(t, err.Error(), "2026-01-01T00:00:00Z")
}

// TestCheckLatest_InvalidJSON verifies that a malformed JSON response from
// GitHub API returns an error rather than panicking on type assertion.
func TestCheckLatest_InvalidJSON(t *testing.T) {
	t.Parallel()

	// Given: a mock server that returns malformed JSON
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{not valid json`))
	}))
	defer srv.Close()

	// When: CheckLatest is called
	checker := NewChecker(WithAPIBaseURL(srv.URL))
	info, err := checker.CheckLatest("0.6.0", "darwin", "arm64")

	// Then: an error is returned
	require.Error(t, err)
	assert.Nil(t, info)
}

// TestCheckLatest_MissingTagName verifies that a response without tag_name
// returns an error instead of panicking.
func TestCheckLatest_MissingTagName(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"other_field": 123})
	}))
	defer srv.Close()

	checker := NewChecker(WithAPIBaseURL(srv.URL))
	info, err := checker.CheckLatest("0.6.0", "darwin", "arm64")
	require.Error(t, err)
	assert.Nil(t, info)
	assert.Contains(t, err.Error(), "tag_name")
}

// TestCheckLatest_MissingAssets verifies that a response without assets
// returns an error instead of panicking.
func TestCheckLatest_MissingAssets(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"tag_name": "v0.7.0"})
	}))
	defer srv.Close()

	checker := NewChecker(WithAPIBaseURL(srv.URL))
	info, err := checker.CheckLatest("0.6.0", "darwin", "arm64")
	require.Error(t, err)
	assert.Nil(t, info)
	assert.Contains(t, err.Error(), "assets")
}

// TestCompareSemver verifies semantic version comparison logic.
func TestCompareSemver(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		current   string
		latest    string
		wantNewer bool
	}{
		{"newer minor version", "0.6.0", "0.7.0", true},
		{"older vs newer major", "0.9.9", "1.0.0", true},
		{"same version", "0.6.0", "0.6.0", false},
		{"current newer than latest", "0.7.0", "0.6.0", false},
		{"patch version newer", "0.6.0", "0.6.1", true},
		{"major bump", "1.0.0", "2.0.0", true},
		{"pseudo-version current same as latest", "0.21.2-0.20260328130835-dd328b13c758+dirty", "0.21.2", false},
		{"pseudo-version current older than latest", "0.21.2-0.20260328130835-dd328b13c758+dirty", "0.21.3", true},
		{"dirty suffix stripped", "0.21.2+dirty", "0.21.2", false},
		{"v prefix with pseudo", "v0.21.2-0.2026+dirty", "v0.21.3", true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// When: CompareSemver is called
			got := IsNewerVersion(tt.latest, tt.current)

			// Then: result matches expected
			assert.Equal(t, tt.wantNewer, got)
		})
	}
}
