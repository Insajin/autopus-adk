package companionmanifest

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
)

type lineageArchiveEntry struct {
	header tar.Header
	data   []byte
}

func decodeLineageArchive(data []byte) (map[string]lineageArchiveEntry, error) {
	if len(data) > maxLineageArchiveCompressedBytes {
		return nil, fmt.Errorf("lineage archive source exceeds %d bytes",
			maxLineageArchiveCompressedBytes)
	}
	return decodeLineageArchiveReader(bytes.NewReader(data))
}

// @AX:ANCHOR [AUTO]: Preserve validated file-backed decoding for lineage archive consumers.
// @AX:REASON [AUTO]: Thirteen fixture call sites depend on bounded streaming and duplicate-entry rejection.
func decodeLineageArchiveFile(path string) (map[string]lineageArchiveEntry, error) {
	file, _, err := openLineageArchiveSource(path)
	if err != nil {
		return nil, err
	}
	entries, decodeErr := decodeLineageArchiveReader(file)
	if err := errors.Join(decodeErr, file.Close()); err != nil {
		return nil, err
	}
	return entries, nil
}

func decodeLineageArchiveReader(
	reader io.Reader,
) (map[string]lineageArchiveEntry, error) {
	gzipReader, err := gzip.NewReader(reader)
	if err != nil {
		return nil, err
	}
	entries := make(map[string]lineageArchiveEntry)
	budget := lineageArchiveBudget{}
	tarReader := tar.NewReader(gzipReader)
	for {
		header, nextErr := tarReader.Next()
		if nextErr == io.EOF {
			break
		}
		if nextErr != nil {
			_ = gzipReader.Close()
			return nil, nextErr
		}
		if err := budget.track(header); err != nil {
			_ = gzipReader.Close()
			return nil, err
		}
		if _, exists := entries[header.Name]; exists {
			_ = gzipReader.Close()
			return nil, fmt.Errorf("lineage archive entry %q is duplicated", header.Name)
		}
		entry, readErr := readLineageArchiveEntry(tarReader, header)
		if readErr != nil {
			_ = gzipReader.Close()
			return nil, readErr
		}
		entries[header.Name] = lineageArchiveEntry{header: *header, data: entry}
	}
	drainErr := drainLineageArchive(gzipReader)
	if err := errors.Join(drainErr, gzipReader.Close()); err != nil {
		return nil, err
	}
	return entries, nil
}

// @AX:NOTE [AUTO]: [downgraded from ANCHOR — lowest fan_in under file cap] Stream the validated source and reject size drift.
func lineageArchiveFileDigest(path string) (string, error) {
	file, info, err := openLineageArchiveSource(path)
	if err != nil {
		return "", err
	}
	digest := sha256.New()
	written, copyErr := io.Copy(digest,
		io.LimitReader(file, maxLineageArchiveCompressedBytes+1))
	if copyErr == nil && written != info.Size() {
		copyErr = errors.New("lineage archive source size changed during digest")
	}
	if err := errors.Join(copyErr, file.Close()); err != nil {
		return "", err
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}

// @AX:ANCHOR [AUTO]: Preserve exact hard-link-or-exclusive-copy materialization semantics.
// @AX:REASON [AUTO]: Ten fixture call sites rely on immutable identity checks and non-overwriting fallback behavior.
func materializeLineageArchive(
	source, target string,
	link func(string, string) error,
) (bool, error) {
	input, sourceInfo, err := openLineageArchiveSource(source)
	if err != nil {
		return false, err
	}
	defer func() { _ = input.Close() }()
	linkErr := link(source, target)
	if linkErr == nil {
		targetInfo, statErr := os.Lstat(target)
		if statErr != nil || !targetInfo.Mode().IsRegular() ||
			!os.SameFile(sourceInfo, targetInfo) {
			_ = os.Remove(target)
			return false, errors.Join(
				errors.New("hard-linked lineage archive identity mismatch"), statErr,
			)
		}
		return true, nil
	}
	if err := copyLineageArchive(input, target, sourceInfo.Size()); err != nil {
		return false, errors.Join(
			fmt.Errorf("hard-link lineage archive: %w", linkErr),
			fmt.Errorf("copy lineage archive: %w", err),
		)
	}
	return false, nil
}

func copyLineageArchive(input *os.File, target string, expectedSize int64) (err error) {
	output, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	complete := false
	defer func() {
		if !complete {
			_ = os.Remove(target)
		}
	}()
	written, copyErr := io.Copy(output,
		io.LimitReader(input, maxLineageArchiveCompressedBytes+1))
	if copyErr == nil && written != expectedSize {
		copyErr = errors.New("lineage archive source size changed during copy")
	}
	err = errors.Join(copyErr, output.Sync(), output.Close())
	if err != nil {
		return err
	}
	complete = true
	return nil
}

// @AX:ANCHOR [AUTO]: Preserve the exact-one-entry streaming rewrite contract.
// @AX:REASON [AUTO]: Seven tamper fixtures rely on untouched entries streaming byte-for-byte and partial targets being removed.
// @AX:WARN [AUTO]: This function coordinates more than eight fail-closed archive branches.
// @AX:REASON [AUTO]: Open, read, budget, duplicate, missing, write, close, and cleanup ordering must remain atomic as a contract.
func rewriteLineageArchiveTarget(
	source, target, entryName string,
	mutate func([]byte) ([]byte, bool),
) (err error) {
	input, _, err := openLineageArchiveSource(source)
	if err != nil {
		return err
	}
	gzipReader, err := gzip.NewReader(input)
	if err != nil {
		return errors.Join(err, input.Close())
	}
	output, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return errors.Join(err, gzipReader.Close(), input.Close())
	}
	gzipWriter, err := gzip.NewWriterLevel(output, gzip.BestSpeed)
	if err != nil {
		return errors.Join(err, output.Close(), gzipReader.Close(), input.Close(),
			os.Remove(target))
	}
	tarReader := tar.NewReader(gzipReader)
	tarWriter := tar.NewWriter(gzipWriter)
	complete := false
	defer func() {
		if !complete {
			_ = tarWriter.Close()
			_ = gzipWriter.Close()
			_ = output.Close()
			_ = gzipReader.Close()
			_ = input.Close()
			_ = os.Remove(target)
		}
	}()
	matches := 0
	budget := lineageArchiveBudget{}
	for {
		header, nextErr := tarReader.Next()
		if nextErr == io.EOF {
			break
		}
		if nextErr != nil {
			return nextErr
		}
		if err := budget.track(header); err != nil {
			return err
		}
		headerCopy := *header
		if header.Name == entryName {
			matches++
			if matches > 1 {
				return fmt.Errorf("lineage archive entry %q is duplicated", entryName)
			}
			entry, readErr := readLineageArchiveEntry(tarReader, header)
			if readErr != nil {
				return readErr
			}
			entry, keep := mutate(entry)
			if !keep {
				continue
			}
			if err := budget.replaceEntrySize(
				entryName, header.Size, int64(len(entry)),
			); err != nil {
				return err
			}
			headerCopy.Size = int64(len(entry))
			if err := tarWriter.WriteHeader(&headerCopy); err != nil {
				return err
			}
			if _, err := io.Copy(tarWriter, bytes.NewReader(entry)); err != nil {
				return err
			}
			continue
		}
		if err := tarWriter.WriteHeader(&headerCopy); err != nil {
			return err
		}
		written, copyErr := io.Copy(tarWriter, tarReader)
		if copyErr != nil {
			return copyErr
		}
		if written != header.Size {
			return io.ErrUnexpectedEOF
		}
	}
	if matches != 1 {
		return fmt.Errorf("lineage archive entry %q is missing", entryName)
	}
	drainErr := drainLineageArchive(gzipReader)
	err = errors.Join(drainErr, tarWriter.Close(), gzipWriter.Close(),
		output.Sync(), output.Close(), gzipReader.Close(), input.Close())
	if err != nil {
		return err
	}
	complete = true
	return nil
}
