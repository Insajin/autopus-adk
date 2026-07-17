package selfupdate

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	maxChecksumSize  = 1 << 20   // 1 MB
	maxSignatureSize = 4 << 10   // 4 KB; a detached ECDSA P-256 ASN.1/DER signature is ~70-72 bytes
	maxArchiveSize   = 100 << 20 // 100 MB
	maxExtractSize   = 100 << 20 // 100 MB per file
	downloadRetries  = 3         // retry count for transient HTTP errors
)

// Downloader downloads and verifies release archives.
type Downloader struct {
	trustedKeys []PinnedReleaseKey
	now         func() time.Time
}

// NewDownloader creates a new Downloader that verifies release signatures
// against the production EmbeddedReleaseKeys trust anchor.
func NewDownloader(opts ...DownloaderOption) *Downloader {
	d := &Downloader{
		trustedKeys: EmbeddedReleaseKeys,
		now:         time.Now,
	}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

// DownloaderOption is a functional option for Downloader.
type DownloaderOption func(*Downloader)

// WithPinnedKeys overrides the release-signature trust anchor. Production
// callers should never use this; it exists so tests verify against synthetic
// keys instead of the real embedded production key.
func WithPinnedKeys(keys []PinnedReleaseKey) DownloaderOption {
	return func(d *Downloader) {
		d.trustedKeys = keys
	}
}

// WithClock overrides the verification reference time, for expiry-gate tests.
func WithClock(now func() time.Time) DownloaderOption {
	return func(d *Downloader) {
		d.now = now
	}
}

// DownloadAndVerify downloads the archive, checksums, and publisher release
// signature, verifies the signature and checksum integrity, and extracts the
// binary.
//
// Signature verification runs before checksums.txt is trusted (REQ-002,
// REQ-006): a missing signatureURL or a signature that fails
// VerifyReleaseSignature aborts here, before ParseChecksums or the archive
// checksum comparison ever run — there is no checksum-only fallback path
// (REQ-003).
// Retries transient HTTP errors (non-200) up to downloadRetries times with exponential backoff.
func (d *Downloader) DownloadAndVerify(archiveURL, checksumURL, signatureURL, archiveName, destDir string) (string, error) {
	if signatureURL == "" {
		return "", errors.New("릴리스 서명을 찾을 수 없습니다")
	}

	checksumData, err := httpGetWithRetry(checksumURL, maxChecksumSize)
	if err != nil {
		return "", fmt.Errorf("checksums download: %w", err)
	}

	sigData, err := httpGetWithRetry(signatureURL, maxSignatureSize)
	if err != nil {
		return "", fmt.Errorf("signature download: %w", err)
	}

	if err := VerifyReleaseSignature(checksumData, sigData, d.trustedKeys, d.now()); err != nil {
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

// httpGetWithRetry downloads a URL with retry on non-200 responses.
// Handles CDN propagation delays after new releases.
func httpGetWithRetry(url string, maxSize int64) ([]byte, error) {
	var lastErr error
	for attempt := range downloadRetries {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt*2) * time.Second)
		}

		req, err := newSelfUpdateRequest(url, githubTokenForURL(url))
		if err != nil {
			lastErr = err
			continue
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		data, err := io.ReadAll(io.LimitReader(resp.Body, maxSize))
		resp.Body.Close()
		if err != nil {
			lastErr = err
			continue
		}

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
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
