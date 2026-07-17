package selfupdate

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

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
