package selfupdate

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDownloadAndVerify_ChecksumMismatch verifies that a checksum mismatch
// results in an error and the downloaded file is not used.
// R6: SHA256 checksum verification must fail if checksum does not match.
func TestDownloadAndVerify_ChecksumMismatch(t *testing.T) {
	t.Parallel()

	archiveContent := buildTarGz(t, "auto", "#!/bin/sh\necho hello")
	archiveName := "autopus-adk_0.7.0_darwin_arm64.tar.gz"
	wrongChecksum := "0000000000000000000000000000000000000000000000000000000000000000"
	checksumLine := wrongChecksum + "  " + archiveName + "\n"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/" + archiveName:
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
	_, err := dl.DownloadAndVerify(
		srv.URL+"/"+archiveName,
		srv.URL+"/checksums.txt",
		archiveName,
		destDir,
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "checksum")
}

// TestDownloadAndVerify_ChecksumNotFound verifies that a missing archive entry
// in checksums.txt returns a clear error.
func TestDownloadAndVerify_ChecksumNotFound(t *testing.T) {
	t.Parallel()

	archiveName := "autopus-adk_0.7.0_darwin_arm64.tar.gz"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/checksums.txt":
			_, _ = w.Write([]byte("abc123  other_file.tar.gz\n"))
		default:
			http.NotFound(w, r)
		}
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
	assert.Contains(t, err.Error(), "checksum not found")
}

// TestParseChecksums verifies that the checksums.txt file format is parsed
// correctly into a map of filename to SHA256 hash.
func TestParseChecksums(t *testing.T) {
	t.Parallel()

	input := "abc123  file_darwin_arm64.tar.gz\n" +
		"def456  file_linux_amd64.tar.gz\n"

	got, err := ParseChecksums([]byte(input))

	require.NoError(t, err)
	assert.Equal(t, "abc123", got["file_darwin_arm64.tar.gz"])
	assert.Equal(t, "def456", got["file_linux_amd64.tar.gz"])
}
