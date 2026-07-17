package selfupdate

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDownloadAndVerify_Success verifies that a valid tar.gz archive with a
// matching checksum is downloaded and extracted successfully.
// R2: download release archive. R3: extract binary. R6: verify SHA256 checksum.
func TestDownloadAndVerify_Success(t *testing.T) {
	t.Parallel()

	archiveContent := buildTarGz(t, "auto", "#!/bin/sh\necho hello")
	checksum := fmt.Sprintf("%x", sha256.Sum256(archiveContent))
	archiveName := "autopus-adk_0.7.0_darwin_arm64.tar.gz"
	checksumLine := checksum + "  " + archiveName + "\n"
	priv, pinned := generateReleaseTestKey(t, "2099-12-31")
	sig := signReleaseChecksums(t, priv, []byte(checksumLine))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "autopus-adk-selfupdate", r.Header.Get("User-Agent"))

		switch r.URL.Path {
		case "/" + archiveName:
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = w.Write(archiveContent)
		case "/checksums.txt":
			_, _ = w.Write([]byte(checksumLine))
		case "/checksums.txt.sig":
			_, _ = w.Write(sig)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	destDir := t.TempDir()
	dl := NewDownloader(WithPinnedKeys([]PinnedReleaseKey{pinned}))
	binaryPath, err := dl.DownloadAndVerify(
		srv.URL+"/"+archiveName,
		srv.URL+"/checksums.txt",
		srv.URL+"/checksums.txt.sig",
		archiveName,
		destDir,
	)

	require.NoError(t, err)
	assert.NotEmpty(t, binaryPath)
}

// TestDownloadAndVerify_HTTPError verifies that non-200 HTTP responses
// are detected and retried instead of silently hashing HTML error pages.
func TestDownloadAndVerify_HTTPError(t *testing.T) {
	t.Parallel()

	archiveName := "autopus-adk_0.7.0_darwin_arm64.tar.gz"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("<html>rate limited</html>"))
	}))
	defer srv.Close()

	destDir := t.TempDir()
	dl := NewDownloader()
	_, err := dl.DownloadAndVerify(
		srv.URL+"/"+archiveName,
		srv.URL+"/checksums.txt",
		srv.URL+"/checksums.txt.sig",
		archiveName,
		destDir,
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "HTTP 403")
}

func TestGitHubTokenForURL(t *testing.T) {
	t.Setenv("AUTOPUS_GITHUB_TOKEN", "env-token")

	tests := []struct {
		name string
		url  string
		want string
	}{
		{"github", "https://github.com/insajin/autopus-adk/releases/download/v0.7.0/archive.zip", "env-token"},
		{"api github", "https://api.github.com/repos/insajin/autopus-adk/releases/latest", "env-token"},
		{"github subdomain", "https://uploads.github.com/repos/insajin/autopus-adk/releases/assets/1", "env-token"},
		{"githubusercontent cdn", "https://objects.githubusercontent.com/github-production-release-asset/example", ""},
		{"non github", "https://example.com/archive.zip", ""},
		{"invalid", "://bad-url", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, githubTokenForURL(tt.url))
		})
	}
}

// TestDownloadAndVerify_RetrySuccess verifies that transient HTTP errors
// are retried and succeed on subsequent attempts.
func TestDownloadAndVerify_RetrySuccess(t *testing.T) {
	t.Parallel()

	archiveContent := buildTarGz(t, "auto", "#!/bin/sh\necho hello")
	checksum := fmt.Sprintf("%x", sha256.Sum256(archiveContent))
	archiveName := "autopus-adk_0.7.0_darwin_arm64.tar.gz"
	checksumLine := checksum + "  " + archiveName + "\n"
	priv, pinned := generateReleaseTestKey(t, "2099-12-31")
	sig := signReleaseChecksums(t, priv, []byte(checksumLine))

	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if r.URL.Path == "/"+archiveName && callCount <= 2 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}

		switch r.URL.Path {
		case "/" + archiveName:
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = w.Write(archiveContent)
		case "/checksums.txt":
			_, _ = w.Write([]byte(checksumLine))
		case "/checksums.txt.sig":
			_, _ = w.Write(sig)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	destDir := t.TempDir()
	dl := NewDownloader(WithPinnedKeys([]PinnedReleaseKey{pinned}))
	binaryPath, err := dl.DownloadAndVerify(
		srv.URL+"/"+archiveName,
		srv.URL+"/checksums.txt",
		srv.URL+"/checksums.txt.sig",
		archiveName,
		destDir,
	)

	require.NoError(t, err)
	assert.NotEmpty(t, binaryPath)
}
