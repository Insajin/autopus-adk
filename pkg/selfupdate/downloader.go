package selfupdate

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const (
	maxChecksumSize = 1 << 20       // 1 MB
	maxArchiveSize  = 100 << 20     // 100 MB
	maxExtractSize  = 100 << 20     // 100 MB per file
)

// Downloader downloads and verifies release archives.
type Downloader struct{}

// NewDownloader creates a new Downloader.
func NewDownloader() *Downloader {
	return &Downloader{}
}

// DownloadAndVerify downloads the archive and checksums, verifies integrity, and extracts the binary.
func (d *Downloader) DownloadAndVerify(archiveURL, checksumURL, archiveName, destDir string) (string, error) {
	checksumResp, err := http.Get(checksumURL)
	if err != nil {
		return "", err
	}
	defer checksumResp.Body.Close()

	checksumData, err := io.ReadAll(io.LimitReader(checksumResp.Body, maxChecksumSize))
	if err != nil {
		return "", err
	}

	checksums, err := ParseChecksums(checksumData)
	if err != nil {
		return "", err
	}

	expectedChecksum := checksums[archiveName]

	archiveResp, err := http.Get(archiveURL)
	if err != nil {
		return "", err
	}
	defer archiveResp.Body.Close()

	archiveData, err := io.ReadAll(io.LimitReader(archiveResp.Body, maxArchiveSize))
	if err != nil {
		return "", err
	}

	// @AX:NOTE: [AUTO] security-critical — SHA256 integrity verification guards against tampered binaries
	actualChecksum := fmt.Sprintf("%x", sha256.Sum256(archiveData))
	if actualChecksum != expectedChecksum {
		return "", fmt.Errorf("checksum mismatch: expected %s, got %s", expectedChecksum, actualChecksum)
	}

	return extractBinary(archiveData, destDir)
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

// extractBinary extracts the binary from tar.gz archive.
func extractBinary(data []byte, destDir string) (string, error) {
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

		if header.Typeflag == tar.TypeReg {
			baseName := filepath.Base(header.Name)
			if baseName == "." || baseName == ".." || baseName == "" {
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
	}

	return "", fmt.Errorf("no binary found in archive")
}
