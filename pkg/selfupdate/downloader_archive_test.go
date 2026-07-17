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
	priv, pinned := generateReleaseTestKey(t, "2099-12-31")
	sig := signReleaseChecksums(t, priv, []byte(checksumLine))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/" + archiveName:
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
	_, err := dl.DownloadAndVerify(
		srv.URL+"/"+archiveName,
		srv.URL+"/checksums.txt",
		srv.URL+"/checksums.txt.sig",
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
	priv, pinned := generateReleaseTestKey(t, "2099-12-31")
	sig := signReleaseChecksums(t, priv, []byte(checksumLine))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/" + archiveName:
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
	_, err := dl.DownloadAndVerify(
		srv.URL+"/"+archiveName,
		srv.URL+"/checksums.txt",
		srv.URL+"/checksums.txt.sig",
		archiveName,
		destDir,
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found in archive")
}
