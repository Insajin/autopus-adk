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

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/" + archiveName:
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = w.Write(archiveContent)
		case "/checksums.txt":
			_, _ = w.Write([]byte(checksumLine))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	destDir := t.TempDir()
	dl := NewDownloader()
	binaryPath, err := dl.DownloadAndVerify(
		srv.URL+"/"+archiveName,
		srv.URL+"/checksums.txt",
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
		archiveName,
		destDir,
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "HTTP 403")
}

// TestDownloadAndVerify_RetrySuccess verifies that transient HTTP errors
// are retried and succeed on subsequent attempts.
func TestDownloadAndVerify_RetrySuccess(t *testing.T) {
	t.Parallel()

	archiveContent := buildTarGz(t, "auto", "#!/bin/sh\necho hello")
	checksum := fmt.Sprintf("%x", sha256.Sum256(archiveContent))
	archiveName := "autopus-adk_0.7.0_darwin_arm64.tar.gz"
	checksumLine := checksum + "  " + archiveName + "\n"

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
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	destDir := t.TempDir()
	dl := NewDownloader()
	binaryPath, err := dl.DownloadAndVerify(
		srv.URL+"/"+archiveName,
		srv.URL+"/checksums.txt",
		archiveName,
		destDir,
	)

	require.NoError(t, err)
	assert.NotEmpty(t, binaryPath)
}
