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

// TestDownloadAndVerify_InvalidGzip verifies that a corrupt gzip archive
// returns an error after checksum verification passes.
func TestDownloadAndVerify_InvalidGzip(t *testing.T) {
	t.Parallel()

	archiveContent := []byte("this is not a valid gzip file at all")
	checksum := fmt.Sprintf("%x", sha256.Sum256(archiveContent))
	archiveName := "autopus-adk_0.7.0_darwin_arm64.tar.gz"
	checksumLine := checksum + "  " + archiveName + "\n"

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
}

// TestDownloadAndVerify_EmptyArchive verifies that a valid gzip/tar with no
// regular files returns an error.
func TestDownloadAndVerify_EmptyArchive(t *testing.T) {
	t.Parallel()

	archiveContent := buildEmptyTarGz(t, "emptydir/")
	checksum := fmt.Sprintf("%x", sha256.Sum256(archiveContent))
	archiveName := "autopus-adk_0.7.0_darwin_arm64.tar.gz"
	checksumLine := checksum + "  " + archiveName + "\n"

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
	assert.Contains(t, err.Error(), "not found in archive")
}
