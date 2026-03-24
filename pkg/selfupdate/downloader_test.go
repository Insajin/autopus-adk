package selfupdate

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// buildTarGz creates a minimal tar.gz archive containing a single file with
// the given name and content. Returns the archive bytes.
func buildTarGz(t *testing.T, filename, content string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)

	body := []byte(content)
	hdr := &tar.Header{
		Name: filename,
		Mode: 0755,
		Size: int64(len(body)),
	}
	require.NoError(t, tw.WriteHeader(hdr))
	_, err := tw.Write(body)
	require.NoError(t, err)
	require.NoError(t, tw.Close())
	require.NoError(t, gz.Close())
	return buf.Bytes()
}

// TestDownloadAndVerify_Success verifies that a valid tar.gz archive with a
// matching checksum is downloaded and extracted successfully.
// R2: download release archive. R3: extract binary. R6: verify SHA256 checksum.
func TestDownloadAndVerify_Success(t *testing.T) {
	t.Parallel()

	// Given: a tar.gz archive containing a fake binary
	archiveContent := buildTarGz(t, "autopus-adk", "#!/bin/sh\necho hello")
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

	// When: DownloadAndVerify is called
	destDir := t.TempDir()
	dl := NewDownloader()
	binaryPath, err := dl.DownloadAndVerify(
		srv.URL+"/"+archiveName,
		srv.URL+"/checksums.txt",
		archiveName,
		destDir,
	)

	// Then: binary is extracted without error
	require.NoError(t, err)
	assert.NotEmpty(t, binaryPath)
}

// TestDownloadAndVerify_ChecksumMismatch verifies that a checksum mismatch
// results in an error and the downloaded file is not used.
// R6: SHA256 checksum verification must fail if checksum does not match.
func TestDownloadAndVerify_ChecksumMismatch(t *testing.T) {
	t.Parallel()

	// Given: a tar.gz archive with a wrong checksum in checksums.txt
	archiveContent := buildTarGz(t, "autopus-adk", "#!/bin/sh\necho hello")
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

	// When: DownloadAndVerify is called
	destDir := t.TempDir()
	dl := NewDownloader()
	_, err := dl.DownloadAndVerify(
		srv.URL+"/"+archiveName,
		srv.URL+"/checksums.txt",
		archiveName,
		destDir,
	)

	// Then: error is returned indicating checksum mismatch
	require.Error(t, err)
	assert.Contains(t, err.Error(), "checksum")
}

// TestDownloadAndVerify_InvalidGzip verifies that a corrupt gzip archive
// returns an error after checksum verification passes.
func TestDownloadAndVerify_InvalidGzip(t *testing.T) {
	t.Parallel()

	// Given: random bytes that pass checksum but fail gzip decoding
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

	// When: DownloadAndVerify is called with corrupt gzip data
	destDir := t.TempDir()
	dl := NewDownloader()
	_, err := dl.DownloadAndVerify(
		srv.URL+"/"+archiveName,
		srv.URL+"/checksums.txt",
		archiveName,
		destDir,
	)

	// Then: error is returned from gzip reader
	require.Error(t, err)
}

// TestDownloadAndVerify_EmptyArchive verifies that a valid gzip/tar with no
// regular files returns an error.
func TestDownloadAndVerify_EmptyArchive(t *testing.T) {
	t.Parallel()

	// Given: a tar.gz that contains only a directory entry (no regular files)
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	hdr := &tar.Header{
		Typeflag: tar.TypeDir,
		Name:     "emptydir/",
		Mode:     0755,
	}
	require.NoError(t, tw.WriteHeader(hdr))
	require.NoError(t, tw.Close())
	require.NoError(t, gz.Close())
	archiveContent := buf.Bytes()

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

	// When: DownloadAndVerify is called
	destDir := t.TempDir()
	dl := NewDownloader()
	_, err := dl.DownloadAndVerify(
		srv.URL+"/"+archiveName,
		srv.URL+"/checksums.txt",
		archiveName,
		destDir,
	)

	// Then: error is returned indicating no binary found
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no binary found")
}

// TestParseChecksums verifies that the checksums.txt file format is parsed
// correctly into a map of filename to SHA256 hash.
func TestParseChecksums(t *testing.T) {
	t.Parallel()

	// Given: a checksums.txt content in standard format
	input := "abc123  file_darwin_arm64.tar.gz\n" +
		"def456  file_linux_amd64.tar.gz\n"

	// When: ParseChecksums is called
	got, err := ParseChecksums([]byte(input))

	// Then: map contains correct entries
	require.NoError(t, err)
	assert.Equal(t, "abc123", got["file_darwin_arm64.tar.gz"])
	assert.Equal(t, "def456", got["file_linux_amd64.tar.gz"])
}
