package selfupdate

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	maxChecksumSize = 1 << 20       // 1 MB
	maxArchiveSize  = 100 << 20     // 100 MB
	maxExtractSize  = 100 << 20     // 100 MB per file
	downloadRetries = 3             // retry count for transient HTTP errors
)

// Downloader downloads and verifies release archives.
type Downloader struct{}

// NewDownloader creates a new Downloader.
func NewDownloader() *Downloader {
	return &Downloader{}
}

// DownloadAndVerify downloads the archive and checksums, verifies integrity, and extracts the binary.
// Retries transient HTTP errors (non-200) up to downloadRetries times with exponential backoff.
func (d *Downloader) DownloadAndVerify(archiveURL, checksumURL, archiveName, destDir string) (string, error) {
	checksumData, err := httpGetWithRetry(checksumURL, maxChecksumSize)
	if err != nil {
		return "", fmt.Errorf("checksums download: %w", err)
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

		resp, err := http.Get(url)
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
	defer gzr.Close()

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
