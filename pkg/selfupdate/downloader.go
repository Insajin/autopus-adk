package selfupdate

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	maxChecksumSize = 1 << 20   // 1 MB
	maxArchiveSize  = 100 << 20 // 100 MB
	maxExtractSize  = 100 << 20 // 100 MB per file
	downloadRetries = 3         // retry count for transient HTTP errors
)

// Downloader downloads and verifies release archives.
type Downloader struct {
	trustedKeys []pinnedReleaseKey
	now         func() time.Time
}

// NewDownloader creates a downloader using the compiled release trust anchor.
func NewDownloader() *Downloader {
	return &Downloader{
		trustedKeys: embeddedReleaseKeys[:],
		now:         time.Now,
	}
}

func newDownloaderForTest(keys []pinnedReleaseKey, now time.Time) *Downloader {
	return &Downloader{
		trustedKeys: keys,
		now:         func() time.Time { return now },
	}
}

// DownloadAndVerify preserves the original four-argument API. It derives the
// sibling envelope URL from an exact checksums.txt asset URL and never falls
// back to checksum-only verification.
func (d *Downloader) DownloadAndVerify(archiveURL, checksumURL, archiveName, destDir string) (string, error) {
	signatureURL, err := deriveReleaseSignaturesURL(checksumURL)
	if err != nil {
		return "", err
	}
	return d.DownloadAndVerifyWithSignature(archiveURL, checksumURL, signatureURL, archiveName, destDir)
}

// DownloadAndVerifyWithSignature verifies the publisher envelope before it
// trusts checksums.txt, then verifies and extracts the requested archive.
func (d *Downloader) DownloadAndVerifyWithSignature(
	archiveURL, checksumURL, signatureURL, archiveName, destDir string,
) (string, error) {
	if signatureURL == "" {
		return "", errors.New("release signatures URL is required")
	}

	checksumData, err := httpGetWithRetry(checksumURL, maxChecksumSize)
	if err != nil {
		return "", fmt.Errorf("checksums download: %w", err)
	}
	envelope, err := httpGetWithRetry(signatureURL, MaxReleaseSignatureEnvelopeSize)
	if err != nil {
		return "", fmt.Errorf("signature envelope download: %w", err)
	}
	if err := verifyReleaseSignatures(checksumData, envelope, d.trustedKeys, d.now()); err != nil {
		return "", fmt.Errorf("release signature verification failed: %w", err)
	}

	checksums, err := ParseChecksums(checksumData)
	if err != nil {
		return "", err
	}
	expectedChecksum := checksums[archiveName]
	if expectedChecksum == "" {
		return "", fmt.Errorf("checksum not found for %s in checksums.txt", archiveName)
	}

	archiveData, err := httpGetWithRetry(archiveURL, maxArchiveSize)
	if err != nil {
		return "", fmt.Errorf("archive download: %w", err)
	}
	// @AX:NOTE: [AUTO] security-critical — SHA256 integrity verification guards against tampered binaries
	actualChecksum := fmt.Sprintf("%x", sha256.Sum256(archiveData))
	if actualChecksum != expectedChecksum {
		return "", fmt.Errorf("checksum mismatch: expected %s, got %s", expectedChecksum, actualChecksum)
	}

	if strings.HasSuffix(archiveName, ".zip") {
		return extractBinaryZip(archiveData, destDir)
	}
	return extractBinaryTarGz(archiveData, destDir)
}

func deriveReleaseSignaturesURL(checksumURL string) (string, error) {
	u, err := url.Parse(checksumURL)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" ||
		u.User != nil || u.Fragment != "" || u.RawPath != "" ||
		!strings.HasSuffix(u.Path, "/checksums.txt") {
		return "", fmt.Errorf("cannot derive release signatures URL from exact checksums.txt asset URL")
	}
	u.Path = strings.TrimSuffix(u.Path, "checksums.txt") + "checksums.txt.signatures"
	return u.String(), nil
}

// httpGetWithRetry downloads a URL with retry on non-200 responses.
// Handles CDN propagation delays after new releases.
func httpGetWithRetry(rawURL string, maxSize int64) ([]byte, error) {
	var lastErr error
	for attempt := range downloadRetries {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt*2) * time.Second)
		}
		req, err := newSelfUpdateRequest(rawURL, githubTokenForURL(rawURL))
		if err != nil {
			lastErr = err
			continue
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		data, readErr := io.ReadAll(io.LimitReader(resp.Body, maxSize+1))
		closeErr := resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			continue
		}
		if closeErr != nil {
			lastErr = closeErr
			continue
		}
		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
			continue
		}
		if int64(len(data)) > maxSize {
			lastErr = fmt.Errorf("HTTP response exceeds %d-byte limit", maxSize)
			continue
		}
		return data, nil
	}
	return nil, fmt.Errorf("failed after %d attempts: %w", downloadRetries, lastErr)
}

func githubTokenForURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	host := strings.ToLower(u.Hostname())
	if host == "github.com" || host == "api.github.com" || strings.HasSuffix(host, ".github.com") {
		return githubTokenFromEnv()
	}
	return ""
}

// ParseChecksums parses checksums.txt format into a map.
func ParseChecksums(data []byte) (map[string]string, error) {
	result := make(map[string]string)
	for line := range strings.SplitSeq(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			result[parts[1]] = parts[0]
		}
	}
	return result, nil
}

// binaryName is the expected binary filename inside the archive.
const binaryName = "auto"

// extractBinaryTarGz extracts the "auto" binary from a tar.gz archive.
func extractBinaryTarGz(data []byte, destDir string) (string, error) {
	gzr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	defer func() {
		_ = gzr.Close()
	}()

	tr := tar.NewReader(gzr)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}

		baseName := filepath.Base(header.Name)
		if header.Typeflag != tar.TypeReg || (baseName != binaryName && baseName != binaryName+".exe") {
			continue
		}

		path := filepath.Join(destDir, baseName)
		f, err := os.Create(path)
		if err != nil {
			return "", err
		}
		if _, err := io.Copy(f, io.LimitReader(tr, maxExtractSize)); err != nil {
			f.Close()
			return "", err
		}
		f.Close()

		if err := os.Chmod(path, os.FileMode(header.Mode)); err != nil {
			return "", err
		}

		return path, nil
	}

	return "", fmt.Errorf("binary %q not found in archive", binaryName)
}

// extractBinaryZip extracts the "auto" (or "auto.exe") binary from a zip archive.
func extractBinaryZip(data []byte, destDir string) (string, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("open zip: %w", err)
	}

	// Accept both "auto" and "auto.exe" for Windows.
	match := func(name string) bool {
		base := filepath.Base(name)
		return base == binaryName || base == binaryName+".exe"
	}

	for _, f := range zr.File {
		if f.FileInfo().IsDir() || !match(f.Name) {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return "", fmt.Errorf("open zip entry %s: %w", f.Name, err)
		}

		outName := filepath.Base(f.Name)
		path := filepath.Join(destDir, outName)
		out, err := os.Create(path)
		if err != nil {
			rc.Close()
			return "", err
		}
		if _, err := io.Copy(out, io.LimitReader(rc, maxExtractSize)); err != nil {
			out.Close()
			rc.Close()
			return "", err
		}
		out.Close()
		rc.Close()

		if err := os.Chmod(path, f.Mode()); err != nil {
			return "", err
		}
		return path, nil
	}

	return "", fmt.Errorf("binary %q not found in zip archive", binaryName)
}
