package companionmanifest

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"testing"
)

type lineageArchiveEntry struct {
	header tar.Header
	data   []byte
}

func decodeLineageArchive(data []byte) (map[string]lineageArchiveEntry, error) {
	gzipReader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	entries := make(map[string]lineageArchiveEntry)
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
		entry, readErr := io.ReadAll(tarReader)
		if readErr != nil {
			_ = gzipReader.Close()
			return nil, readErr
		}
		entries[header.Name] = lineageArchiveEntry{header: *header, data: entry}
	}
	if _, err := io.Copy(io.Discard, gzipReader); err != nil {
		_ = gzipReader.Close()
		return nil, err
	}
	if err := gzipReader.Close(); err != nil {
		return nil, err
	}
	return entries, nil
}

func rewriteLineageArchive(
	t *testing.T,
	data []byte,
	mutate func(string, []byte) ([]byte, bool),
) []byte {
	t.Helper()
	entries, err := orderedLineageArchiveEntries(data)
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	gzipWriter, err := gzip.NewWriterLevel(&output, gzip.BestSpeed)
	if err != nil {
		t.Fatal(err)
	}
	tarWriter := tar.NewWriter(gzipWriter)
	for _, entry := range entries {
		entryData, keep := mutate(entry.header.Name, append([]byte(nil), entry.data...))
		if !keep {
			continue
		}
		header := entry.header
		header.Size = int64(len(entryData))
		if err := tarWriter.WriteHeader(&header); err != nil {
			t.Fatal(err)
		}
		if _, err := tarWriter.Write(entryData); err != nil {
			t.Fatal(err)
		}
	}
	if err := errors.Join(tarWriter.Close(), gzipWriter.Close()); err != nil {
		t.Fatalf("finalize rewritten lineage archive: %v", err)
	}
	return output.Bytes()
}

func orderedLineageArchiveEntries(data []byte) ([]lineageArchiveEntry, error) {
	gzipReader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	var entries []lineageArchiveEntry
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
		entry, readErr := io.ReadAll(tarReader)
		if readErr != nil {
			_ = gzipReader.Close()
			return nil, readErr
		}
		entries = append(entries, lineageArchiveEntry{header: *header, data: entry})
	}
	if _, err := io.Copy(io.Discard, gzipReader); err != nil {
		_ = gzipReader.Close()
		return nil, err
	}
	return entries, gzipReader.Close()
}
