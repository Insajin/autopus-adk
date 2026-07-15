package companionmanifest

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func copyLineageRepository(t *testing.T, destination string) {
	t.Helper()
	root := lineageRepositoryRoot(t)
	if err := os.Mkdir(destination, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"cmd", "content", "internal", "pkg", "scripts", "templates"} {
		copyLineagePath(t, filepath.Join(root, name), filepath.Join(destination, name))
	}
	for _, name := range []string{
		".goreleaser.yaml", "go.mod", "go.sum", "LICENSE", "README.md", "CHANGELOG.md",
	} {
		copyLineagePath(t, filepath.Join(root, name), filepath.Join(destination, name))
	}
}

func copyLineagePath(t *testing.T, source, destination string) {
	t.Helper()
	info, err := os.Lstat(source)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("release fixture refuses symlink %s", source)
	}
	if info.IsDir() {
		if err := os.Mkdir(destination, info.Mode().Perm()); err != nil {
			t.Fatal(err)
		}
		entries, err := os.ReadDir(source)
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range entries {
			copyLineagePath(t, filepath.Join(source, entry.Name()), filepath.Join(destination, entry.Name()))
		}
		return
	}
	copyLineageFile(t, source, destination, info.Mode().Perm())
}

func copyLineageFile(t *testing.T, source, destination string, mode os.FileMode) {
	t.Helper()
	input, err := os.Open(source)
	if err != nil {
		t.Fatal(err)
	}
	output, createErr := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if createErr != nil {
		_ = input.Close()
		t.Fatal(createErr)
	}
	_, copyErr := io.Copy(output, input)
	closeErr := errors.Join(input.Close(), output.Close())
	if copyErr != nil || closeErr != nil {
		t.Fatalf("copy %s: %v / %v", source, copyErr, closeErr)
	}
}

func runLineageCommand(t *testing.T, dir, name string, arguments ...string) string {
	t.Helper()
	command := exec.Command(name, arguments...)
	command.Dir = dir
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v: %v\n%s", name, arguments, err, output)
	}
	return string(output)
}

func lineageRepositoryRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}
