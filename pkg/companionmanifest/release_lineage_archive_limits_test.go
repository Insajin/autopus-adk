package companionmanifest

import (
	"archive/tar"
	"errors"
	"fmt"
	"io"
	"os"
)

// @AX:NOTE [AUTO]: These limits admit release fixtures while bounding decompression bombs, entry fan-out, and trailing data.
const (
	maxLineageArchiveCompressedBytes = 64 << 20
	maxLineageArchiveEntryBytes      = 64 << 20
	maxLineageArchiveExpandedBytes   = 128 << 20
	maxLineageArchiveEntries         = 256
	maxLineageArchiveTrailingBytes   = 1 << 20
)

type lineageArchiveBudget struct {
	entries  int
	expanded int64
}

// @AX:ANCHOR [AUTO]: Keep entry count and expanded-size accounting in this common archive gate.
// @AX:REASON [AUTO]: Decode, rewrite, and adversarial fixture tests rely on the same fail-closed budget semantics.
func (budget *lineageArchiveBudget) track(header *tar.Header) error {
	budget.entries++
	if budget.entries > maxLineageArchiveEntries {
		return fmt.Errorf("lineage archive has more than %d entries",
			maxLineageArchiveEntries)
	}
	if header.Size < 0 || header.Size > maxLineageArchiveEntryBytes {
		return fmt.Errorf("lineage archive entry %q has invalid size %d",
			header.Name, header.Size)
	}
	if budget.expanded > maxLineageArchiveExpandedBytes-header.Size {
		return fmt.Errorf("lineage archive expands beyond %d bytes",
			maxLineageArchiveExpandedBytes)
	}
	budget.expanded += header.Size
	return nil
}

func (budget *lineageArchiveBudget) replaceEntrySize(
	name string, original, replacement int64,
) error {
	if replacement < 0 || replacement > maxLineageArchiveEntryBytes {
		return fmt.Errorf("mutated lineage archive entry %q has invalid size %d",
			name, replacement)
	}
	remaining := budget.expanded - original
	if remaining < 0 || remaining > maxLineageArchiveExpandedBytes-replacement {
		return fmt.Errorf("mutated lineage archive expands beyond %d bytes",
			maxLineageArchiveExpandedBytes)
	}
	budget.expanded = remaining + replacement
	return nil
}

// @AX:ANCHOR [AUTO]: Keep archive source validation centralized before any content is consumed.
// @AX:REASON [AUTO]: Digest, decode, materialize, and rewrite callers depend on the same regular-file, size, and inode checks.
func openLineageArchiveSource(path string) (*os.File, os.FileInfo, error) {
	expected, err := os.Lstat(path)
	if err != nil {
		return nil, nil, err
	}
	if !expected.Mode().IsRegular() {
		return nil, nil, errors.New("lineage archive source is not a regular file")
	}
	if expected.Size() > maxLineageArchiveCompressedBytes {
		return nil, nil, fmt.Errorf("lineage archive source exceeds %d bytes",
			maxLineageArchiveCompressedBytes)
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	current, statErr := file.Stat()
	if statErr != nil || !current.Mode().IsRegular() ||
		!os.SameFile(expected, current) || current.Size() != expected.Size() {
		return nil, nil, errors.Join(
			errors.New("lineage archive source identity changed during open"),
			statErr, file.Close(),
		)
	}
	return file, current, nil
}

func readLineageArchiveEntry(reader io.Reader, header *tar.Header) ([]byte, error) {
	entry, err := io.ReadAll(io.LimitReader(reader, maxLineageArchiveEntryBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(entry)) != header.Size {
		return nil, io.ErrUnexpectedEOF
	}
	return entry, nil
}

func drainLineageArchive(reader io.Reader) error {
	read, err := io.Copy(io.Discard,
		io.LimitReader(reader, maxLineageArchiveTrailingBytes+1))
	if err != nil {
		return err
	}
	if read > maxLineageArchiveTrailingBytes {
		return fmt.Errorf("lineage archive trailing data exceeds %d bytes",
			maxLineageArchiveTrailingBytes)
	}
	return nil
}
