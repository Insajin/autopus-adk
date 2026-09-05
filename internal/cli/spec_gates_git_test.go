package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveGatesChangeSet_GitBaseAndUntracked(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	root := t.TempDir()
	runGit := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com", "GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	require.NoError(t, os.MkdirAll(filepath.Join(root, "pkg", "a"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "pkg", "a", "x.go"), []byte("package a\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "README.md"), []byte("# a\n"), 0o644))
	runGit("init", "-q")
	runGit("add", ".")
	runGit("commit", "-qm", "base")
	runGit("tag", "base")
	require.NoError(t, os.WriteFile(filepath.Join(root, "pkg", "a", "x.go"), []byte("package a\n// edit\n"), 0o644))
	runGit("commit", "-qam", "edit")
	require.NoError(t, os.MkdirAll(filepath.Join(root, "pkg", "b"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "pkg", "b", "y.go"), []byte("package b\n"), 0o644))

	sinceBase, err := resolveGatesChangeSet(root, "", "base")
	require.NoError(t, err)
	assert.Equal(t, []string{"pkg/a/x.go", "pkg/b/y.go"}, sinceBase, "--base reports committed edits plus untracked files")

	sinceHead, err := resolveGatesChangeSet(root, "", "")
	require.NoError(t, err)
	assert.Equal(t, []string{"pkg/b/y.go"}, sinceHead, "default compares the working tree with HEAD")

	explicit, err := resolveGatesChangeSet(root, "docs/a.md, "+filepath.Join(root, "pkg", "a", "x.go"), "base")
	require.NoError(t, err)
	assert.Equal(t, []string{"docs/a.md", "pkg/a/x.go"}, explicit, "--changed wins and absolute paths become root-relative")
}

func TestResolveGatesChangeSet_WithoutGitRequiresChanged(t *testing.T) {
	root := t.TempDir()
	_, err := resolveGatesChangeSet(root, "", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pass --changed")
}

func TestResolveGatesTarget_DerivesRootFromSpecDir(t *testing.T) {
	root := t.TempDir()
	specDir := filepath.Join(root, ".autopus", "specs", "SPEC-T-001")
	require.NoError(t, os.MkdirAll(specDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(specDir, "spec.md"), []byte("# spec\n"), 0o644))

	target, err := resolveGatesTarget(specDir)
	require.NoError(t, err)
	assert.Equal(t, "SPEC-T-001", target.SpecID)
	assert.Equal(t, specDir, target.SpecDir)
	assert.Equal(t, root, target.Root)

	loose := filepath.Join(root, "loose")
	require.NoError(t, os.MkdirAll(loose, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(loose, "spec.md"), []byte("# spec\n"), 0o644))
	target, err = resolveGatesTarget(loose)
	require.NoError(t, err)
	assert.Equal(t, ".", target.Root, "a SPEC outside .autopus/specs falls back to the working directory")

	_, err = resolveGatesTarget("")
	require.Error(t, err)
}
