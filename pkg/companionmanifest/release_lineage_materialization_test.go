package companionmanifest

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestLineageCachePathSurvivesProducerCleanup(t *testing.T) {
	t.Parallel()

	producerRoot := filepath.Join(t.TempDir(), "producer")
	if err := os.Mkdir(producerRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(producerRoot, "baseline.tar.gz")
	want := bytes.Repeat([]byte("cached-archive"), 128)
	if err := os.WriteFile(source, want, 0o600); err != nil {
		t.Fatal(err)
	}
	cached := filepath.Join(t.TempDir(), "baseline.tar.gz")
	if _, err := materializeLineageArchive(source, cached, os.Link); err != nil {
		t.Fatal(err)
	}
	evidence := goReleaserA0Evidence{archives: map[string]string{"amd64": cached}}
	clone := cloneGoReleaserEvidence(&evidence)
	if err := os.RemoveAll(producerRoot); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(clone.archives["amd64"])
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("cached archive did not survive producer cleanup: %v", err)
	}
	if clone.archives["amd64"] != cached {
		t.Fatal("lineage evidence clone did not reuse the immutable cache path")
	}
}

func TestMaterializeLineageArchiveHardlinksOrStreamsExactBytes(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	source := filepath.Join(root, "source.tar.gz")
	linked := filepath.Join(root, "linked.tar.gz")
	copied := filepath.Join(root, "copied.tar.gz")
	portableCopy := filepath.Join(root, "portable-copy.tar.gz")
	want := bytes.Repeat([]byte("archive"), 1024)
	if err := os.WriteFile(source, want, 0o600); err != nil {
		t.Fatal(err)
	}

	usedLink, err := materializeLineageArchive(source, linked, os.Link)
	if err != nil || !usedLink {
		t.Fatalf("hard-link materialization = %t, %v", usedLink, err)
	}
	assertSameLineageFile(t, source, linked, true, want)

	linkAcrossDevices := func(oldPath, newPath string) error {
		return &os.LinkError{Op: "link", Old: oldPath, New: newPath, Err: syscall.EXDEV}
	}
	usedLink, err = materializeLineageArchive(source, copied, linkAcrossDevices)
	if err != nil || usedLink {
		t.Fatalf("copy fallback materialization = %t, %v", usedLink, err)
	}
	assertSameLineageFile(t, source, copied, false, want)

	linkUnavailable := errors.New("hard links disabled")
	usedLink, err = materializeLineageArchive(source, portableCopy,
		func(string, string) error { return linkUnavailable })
	if err != nil || usedLink {
		t.Fatalf("portable copy fallback = %t, %v", usedLink, err)
	}
	assertSameLineageFile(t, source, portableCopy, false, want)

	occupied := filepath.Join(root, "occupied.tar.gz")
	if err := os.WriteFile(occupied, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = materializeLineageArchive(source, occupied,
		func(string, string) error { return linkUnavailable })
	if !errors.Is(err, linkUnavailable) || !errors.Is(err, os.ErrExist) {
		t.Fatalf("joined link/copy error = %v", err)
	}
	if got := string(readLineageFile(t, occupied)); got != "keep" {
		t.Fatalf("failed copy changed existing target to %q", got)
	}
}

func TestRewriteLineageArchiveTargetStreamsOnlySelectedEntry(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	source := filepath.Join(root, "source.tar.gz")
	target := filepath.Join(root, "target.tar.gz")
	large := bytes.Repeat([]byte("untouched-architecture-entry"), 1<<16)
	writeLineageMaterializationArchive(t, source, map[string][]byte{
		"auto": large, "manifest.json": []byte("before"), "signature": []byte("sig"),
	})
	calls := 0
	err := rewriteLineageArchiveTarget(
		source, target, "manifest.json",
		func(data []byte) ([]byte, bool) {
			calls++
			if !bytes.Equal(data, []byte("before")) {
				t.Fatalf("target mutation received %d unrelated bytes", len(data))
			}
			return []byte("after"), true
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("target mutation calls = %d, want 1", calls)
	}
	entries, err := decodeLineageArchive(readLineageFile(t, target))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(entries["auto"].data, large) ||
		!bytes.Equal(entries["manifest.json"].data, []byte("after")) ||
		!bytes.Equal(entries["signature"].data, []byte("sig")) {
		t.Fatal("target-only rewrite changed an untouched archive entry")
	}
}

func TestLineageFixtureReusesUntouchedArchitectureAndRemovesTargetEntry(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	amd64Source := filepath.Join(root, "amd64-source.tar.gz")
	arm64Source := filepath.Join(root, "arm64-source.tar.gz")
	amd64Target := filepath.Join(root, "amd64-target.tar.gz")
	arm64Target := filepath.Join(root, "arm64-target.tar.gz")
	entries := map[string][]byte{"auto": []byte("binary"), "signature": []byte("sig")}
	writeLineageMaterializationArchive(t, amd64Source, entries)
	writeLineageMaterializationArchive(t, arm64Source, entries)
	if err := rewriteLineageArchiveTarget(
		amd64Source, amd64Target, "signature",
		func([]byte) ([]byte, bool) { return nil, false },
	); err != nil {
		t.Fatal(err)
	}
	if usedLink, err := materializeLineageArchive(arm64Source, arm64Target, os.Link); err != nil || !usedLink {
		t.Fatalf("untouched architecture reuse = %t, %v", usedLink, err)
	}
	assertSameLineageFile(t, arm64Source, arm64Target, true,
		readLineageFile(t, arm64Source))
	decoded, err := decodeLineageArchive(readLineageFile(t, amd64Target))
	if err != nil {
		t.Fatal(err)
	}
	if _, present := decoded["signature"]; present {
		t.Fatal("selected archive entry was not removed")
	}
	if string(decoded["auto"].data) != "binary" {
		t.Fatal("entry removal changed an untouched archive entry")
	}
}

func TestRewriteLineageArchiveTargetRequiresExactlyOneEntry(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	t.Run("missing", func(t *testing.T) {
		source := filepath.Join(root, "missing-source.tar.gz")
		target := filepath.Join(root, "missing-target.tar.gz")
		writeLineageMaterializationArchive(t, source,
			map[string][]byte{"auto": []byte("binary")})
		calls := 0
		err := rewriteLineageArchiveTarget(source, target, "manifest.json",
			func(data []byte) ([]byte, bool) {
				calls++
				return data, true
			})
		if err == nil || calls != 0 {
			t.Fatalf("missing target result = %v, mutation calls = %d", err, calls)
		}
		if _, statErr := os.Lstat(target); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("missing target left partial output: %v", statErr)
		}
	})
	t.Run("duplicate", func(t *testing.T) {
		source := filepath.Join(root, "duplicate-source.tar.gz")
		target := filepath.Join(root, "duplicate-target.tar.gz")
		writeLineageMaterializationEntries(t, source, []lineageMaterializationEntry{
			{name: "manifest.json", data: []byte("first")},
			{name: "manifest.json", data: []byte("second")},
		})
		calls := 0
		err := rewriteLineageArchiveTarget(source, target, "manifest.json",
			func(data []byte) ([]byte, bool) {
				calls++
				return data, true
			})
		if err == nil || calls != 1 {
			t.Fatalf("duplicate target result = %v, mutation calls = %d", err, calls)
		}
		if _, statErr := os.Lstat(target); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("duplicate target left partial output: %v", statErr)
		}
	})
}

func assertSameLineageFile(
	t *testing.T, source, target string, sameIdentity bool, want []byte,
) {
	t.Helper()
	sourceInfo, err := os.Lstat(source)
	if err != nil {
		t.Fatal(err)
	}
	targetInfo, err := os.Lstat(target)
	if err != nil {
		t.Fatal(err)
	}
	if os.SameFile(sourceInfo, targetInfo) != sameIdentity {
		t.Fatalf("materialized identity same = %t, want %t",
			os.SameFile(sourceInfo, targetInfo), sameIdentity)
	}
	got, err := os.ReadFile(target)
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("materialized bytes differ: %v", err)
	}
}

func writeLineageMaterializationArchive(
	t *testing.T, path string, entries map[string][]byte,
) {
	t.Helper()
	ordered := make([]lineageMaterializationEntry, 0, len(entries))
	for _, name := range []string{"auto", "manifest.json", "signature"} {
		if data, ok := entries[name]; ok {
			ordered = append(ordered, lineageMaterializationEntry{name: name, data: data})
		}
	}
	writeLineageMaterializationEntries(t, path, ordered)
}

type lineageMaterializationEntry struct {
	name string
	data []byte
}

func writeLineageMaterializationEntries(
	t *testing.T, path string, entries []lineageMaterializationEntry,
) {
	t.Helper()
	output, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	gzipWriter := gzip.NewWriter(output)
	tarWriter := tar.NewWriter(gzipWriter)
	for _, entry := range entries {
		header := &tar.Header{
			Name: entry.name, Mode: 0o600, Size: int64(len(entry.data)),
		}
		if err := tarWriter.WriteHeader(header); err != nil {
			t.Fatal(err)
		}
		if _, err := io.Copy(tarWriter, bytes.NewReader(entry.data)); err != nil {
			t.Fatal(err)
		}
	}
	if err := errors.Join(tarWriter.Close(), gzipWriter.Close(), output.Close()); err != nil {
		t.Fatal(err)
	}
}
