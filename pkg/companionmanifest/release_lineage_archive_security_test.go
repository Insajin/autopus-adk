package companionmanifest

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestLineageArchiveSourcesRejectSymlinksAndOversizedFiles(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	source := filepath.Join(root, "source.tar.gz")
	writeLineageMaterializationArchive(t, source,
		map[string][]byte{"auto": []byte("binary")})
	symlink := filepath.Join(root, "source-link.tar.gz")
	if err := os.Symlink(source, symlink); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	checks := map[string]func() error{
		"decode": func() error {
			_, err := decodeLineageArchiveFile(symlink)
			return err
		},
		"digest": func() error {
			_, err := lineageArchiveFileDigest(symlink)
			return err
		},
		"materialize": func() error {
			_, err := materializeLineageArchive(symlink,
				filepath.Join(root, "linked.tar.gz"), os.Link)
			return err
		},
		"rewrite": func() error {
			return rewriteLineageArchiveTarget(symlink,
				filepath.Join(root, "rewritten.tar.gz"), "auto",
				func(data []byte) ([]byte, bool) { return data, true })
		},
	}
	for name, check := range checks {
		if err := check(); err == nil {
			t.Fatalf("%s accepted a symlink archive source", name)
		}
	}

	oversized := filepath.Join(root, "oversized.tar.gz")
	file, err := os.OpenFile(oversized, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if err := errors.Join(
		file.Truncate(maxLineageArchiveCompressedBytes+1), file.Close(),
	); err != nil {
		t.Fatal(err)
	}
	if _, err := lineageArchiveFileDigest(oversized); err == nil {
		t.Fatal("digest accepted an oversized compressed archive")
	}
}

func TestMaterializeLineageArchiveRejectsWrongLinkedIdentity(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	source := filepath.Join(root, "source.tar.gz")
	other := filepath.Join(root, "other.tar.gz")
	target := filepath.Join(root, "target.tar.gz")
	if err := os.WriteFile(source, []byte("source"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(other, []byte("other"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := materializeLineageArchive(source, target,
		func(string, string) error { return os.Link(other, target) })
	if err == nil {
		t.Fatal("materialization accepted a hard link to the wrong source")
	}
	if _, statErr := os.Lstat(target); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("identity mismatch left a target behind: %v", statErr)
	}
}

func TestLineageArchiveBudgetsRejectOversizedEntryAndEntryCount(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	oversized := filepath.Join(root, "oversized-entry.tar.gz")
	writeLineageArchiveHeaders(t, oversized, []tar.Header{{
		Name: "manifest.json", Mode: 0o600,
		Size: maxLineageArchiveEntryBytes + 1,
	}}, false)
	target := filepath.Join(root, "oversized-target.tar.gz")
	err := rewriteLineageArchiveTarget(oversized, target, "manifest.json",
		func(data []byte) ([]byte, bool) { return data, true })
	if err == nil {
		t.Fatal("rewrite accepted an oversized archive entry")
	}
	if _, statErr := os.Lstat(target); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("oversized entry left a partial target: %v", statErr)
	}

	headers := make([]tar.Header, maxLineageArchiveEntries+1)
	for index := range headers {
		headers[index] = tar.Header{Name: fmt.Sprintf("entries/%03d", index),
			Mode: 0o600}
	}
	tooMany := filepath.Join(root, "too-many-entries.tar.gz")
	writeLineageArchiveHeaders(t, tooMany, headers, true)
	if _, err := decodeLineageArchiveFile(tooMany); err == nil {
		t.Fatal("decode accepted too many archive entries")
	}

	budget := lineageArchiveBudget{}
	for _, header := range []tar.Header{
		{Name: "first", Size: maxLineageArchiveEntryBytes},
		{Name: "second", Size: maxLineageArchiveEntryBytes},
	} {
		if err := budget.track(&header); err != nil {
			t.Fatalf("valid bounded archive entry rejected: %v", err)
		}
	}
	if err := budget.track(&tar.Header{Name: "overflow", Size: 1}); err == nil {
		t.Fatal("archive expansion budget accepted an extra byte")
	}
	if err := budget.replaceEntrySize("second", maxLineageArchiveEntryBytes,
		maxLineageArchiveEntryBytes+1); err == nil {
		t.Fatal("archive mutation accepted an oversized replacement entry")
	}
	if err := budget.replaceEntrySize("second", maxLineageArchiveEntryBytes,
		maxLineageArchiveEntryBytes); err != nil {
		t.Fatalf("same-size archive mutation was rejected: %v", err)
	}
	if err := budget.replaceEntrySize("second", maxLineageArchiveEntryBytes,
		maxLineageArchiveEntryBytes-1); err != nil {
		t.Fatalf("shrinking archive mutation was rejected: %v", err)
	}
	overflowBudget := lineageArchiveBudget{expanded: maxLineageArchiveExpandedBytes}
	if err := overflowBudget.replaceEntrySize("second", maxLineageArchiveEntryBytes-1,
		maxLineageArchiveEntryBytes); err == nil {
		t.Fatal("archive mutation exceeded the total expansion budget")
	}
}

func writeLineageArchiveHeaders(
	t *testing.T, path string, headers []tar.Header, finalize bool,
) {
	t.Helper()
	output, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	gzipWriter := gzip.NewWriter(output)
	tarWriter := tar.NewWriter(gzipWriter)
	for index := range headers {
		if err := tarWriter.WriteHeader(&headers[index]); err != nil {
			t.Fatal(err)
		}
	}
	if finalize {
		err = tarWriter.Close()
	}
	if err := errors.Join(err, gzipWriter.Close(), output.Close()); err != nil {
		t.Fatal(err)
	}
}
