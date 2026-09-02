package adapter

import (
	"os"
	"path/filepath"
	"testing"
)

// `auto update --dir .` passes the workspace root through as a relative path.
// EvalSymlinks keeps a relative path relative, so the containment check used to
// compare "." against "./" and refuse every prune -- the Claude permission
// ledger read that refusal as fatal and the whole platform update aborted.
// A refused prune is also a silent no-op for every other caller, which is why
// this is asserted at the guard rather than only at the update command.
func TestSafePruneFilePathAcceptsRelativeWorkspaceRoot(t *testing.T) {
	dir := t.TempDir()
	rel := filepath.Join(".autopus", "ledger.json")
	if err := os.MkdirAll(filepath.Join(dir, ".autopus"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, rel), []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)

	realDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(realDir, rel)

	for _, root := range []string{".", "./", "", dir} {
		target, err := safePruneFilePath(root, rel)
		if err != nil {
			t.Fatalf("root %q: %v", root, err)
		}
		if target != want {
			t.Fatalf("root %q: target = %q, want %q", root, target, want)
		}
	}
}

// The containment guard must still refuse a path that escapes the workspace
// when the root is relative -- making the root absolute is a normalization, not
// a relaxation. A symlink is used because a literal ".." is rejected earlier by
// the lexical check and would not exercise the resolved comparison.
func TestSafePruneFilePathRefusesEscapeUnderRelativeRoot(t *testing.T) {
	base := t.TempDir()
	workspace := filepath.Join(base, "workspace")
	outside := filepath.Join(base, "outside")
	for _, d := range []string{workspace, outside} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	secret := filepath.Join(outside, "secret.json")
	if err := os.WriteFile(secret, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(secret, filepath.Join(workspace, "escape.json")); err != nil {
		t.Fatal(err)
	}
	t.Chdir(workspace)

	for _, root := range []string{".", workspace} {
		if _, err := safePruneFilePath(root, "escape.json"); err == nil {
			t.Fatalf("root %q: escape was accepted", root)
		}
	}
}

// removeEmptyParents walks upward until it reaches the root. A relative root was
// never equal to the absolute directory being walked, so the walk relied on
// hitting a non-empty directory to stop -- and filepath.Dir("/") is "/".
func TestRemoveEmptyParentsStopsAtRelativeWorkspaceRoot(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "a", "b")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)

	if err := removeEmptyParents(".", nested); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "a")); !os.IsNotExist(err) {
		t.Fatalf("empty parents were not removed: %v", err)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("workspace root itself was removed: %v", err)
	}
}

// A workspace root that does not exist yet is an empty prune set, not an error.
// Generate reads the Claude permission preimage before it creates the workspace,
// and both guards used to reach that state through os.Lstat on the target -- a
// benign "nothing here" return. Resolving the root up front turned the same
// state into `resolve workspace root: lstat ...: no such file or directory`,
// which failed the whole generate.
func TestPruneGuardsTreatMissingWorkspaceRootAsEmpty(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "not-created-yet")

	target, err := safePruneFilePath(missing, filepath.Join(".autopus", "ledger.json"))
	if err != nil {
		t.Fatalf("safePruneFilePath: %v", err)
	}
	if target != "" {
		t.Fatalf("target = %q, want empty", target)
	}

	if err := removeEmptyParents(missing, filepath.Join(missing, "a", "b")); err != nil {
		t.Fatalf("removeEmptyParents: %v", err)
	}
}
