package companionmanifest

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteAtomic_ExistingOutput_ReplacesWith0600CompleteFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.json")
	if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := WriteAtomic(path, []byte("complete-manifest")); err != nil {
		t.Fatalf("WriteAtomic() error = %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "complete-manifest" {
		t.Fatalf("content = %q", got)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if gotMode := info.Mode().Perm(); gotMode != 0o600 {
		t.Fatalf("mode = %04o, want 0600", gotMode)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "manifest.json" {
		t.Fatalf("temporary files retained: %#v", entries)
	}
}

func TestWriteAtomic_SymlinkOutput_IsRejected(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	link := filepath.Join(dir, "manifest.json")
	if err := os.WriteFile(target, []byte("safe"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	if err := WriteAtomic(link, []byte("unsafe")); err == nil {
		t.Fatal("WriteAtomic() error = nil, want symlink rejection")
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "safe" {
		t.Fatalf("symlink target changed to %q", got)
	}
}

func TestWriteSignedFiles_DistinctSameDirectory_CommitsBoth0600(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "manifest.json")
	signaturePath := filepath.Join(dir, "manifest.sig")
	if err := WriteSignedFiles(manifestPath, signaturePath, []byte("manifest"), []byte("signature")); err != nil {
		t.Fatalf("WriteSignedFiles() error = %v", err)
	}
	for path, want := range map[string]string{manifestPath: "manifest", signaturePath: "signature"} {
		got, err := os.ReadFile(path)
		if err != nil || string(got) != want {
			t.Fatalf("ReadFile(%s) = %q, %v", filepath.Base(path), got, err)
		}
		info, err := os.Stat(path)
		if err != nil || info.Mode().Perm() != 0o600 {
			t.Fatalf("Stat(%s) mode = %v, %v", filepath.Base(path), info.Mode().Perm(), err)
		}
	}
}

func TestAtomicOutputs_InvalidPaths_AreRejected(t *testing.T) {
	dir := t.TempDir()
	same := filepath.Join(dir, "same")
	otherDir := t.TempDir()
	cases := []struct {
		name string
		run  func() error
	}{
		{name: "invalid dot path", run: func() error { return WriteAtomic(".", nil) }},
		{name: "missing directory", run: func() error { return WriteAtomic(filepath.Join(dir, "missing", "file"), nil) }},
		{name: "directory as output", run: func() error { return WriteAtomic(dir, nil) }},
		{name: "same signed path", run: func() error { return WriteSignedFiles(same, same, nil, nil) }},
		{name: "different signed dirs", run: func() error {
			return WriteSignedFiles(filepath.Join(dir, "manifest"), filepath.Join(otherDir, "signature"), nil, nil)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.run(); err == nil {
				t.Fatal("error = nil, want rejection")
			}
		})
	}
}
