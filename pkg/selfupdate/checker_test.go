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
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	// When: CheckLatest is called with current version 0.6.0
	checker := NewChecker(WithAPIBaseURL(srv.URL))
	info, err := checker.CheckLatest("0.6.0")

	// Then: ReleaseInfo is returned with the newer version
	require.NoError(t, err)
	require.NotNil(t, info)
	assert.Equal(t, "v0.7.0", info.TagName)
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
	info, err := checker.CheckLatest("0.6.0")

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
	info, err := checker.CheckLatest("0.6.0")

	// Then: an error is returned
	require.Error(t, err)
	assert.Nil(t, info)
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
	info, err := checker.CheckLatest("0.6.0")

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
	info, err := checker.CheckLatest("0.6.0")
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
	info, err := checker.CheckLatest("0.6.0")
	require.Error(t, err)
	assert.Nil(t, info)
	assert.Contains(t, err.Error(), "assets")
}

// TestCompareSemver verifies semantic version comparison logic.
func TestCompareSemver(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		current  string
		latest   string
		wantNewer bool
	}{
		{"newer minor version", "0.6.0", "0.7.0", true},
		{"older vs newer major", "0.9.9", "1.0.0", true},
		{"same version", "0.6.0", "0.6.0", false},
		{"current newer than latest", "0.7.0", "0.6.0", false},
		{"patch version newer", "0.6.0", "0.6.1", true},
		{"major bump", "1.0.0", "2.0.0", true},
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
