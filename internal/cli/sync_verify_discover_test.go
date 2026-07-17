package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- shared fixture helpers for sync verify oracle tests ---

func syncGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v in %s failed: %v\n%s", args, dir, err, out)
	}
}

func syncGitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v in %s failed: %v\n%s", args, dir, err, out)
	}
	return string(out)
}

func syncWrite(t *testing.T, dir, rel, content string) {
	t.Helper()
	path := filepath.Join(dir, filepath.FromSlash(rel))
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

// initSyncRepo initializes a git repo with a deterministic identity and an
// initial seed commit so HEAD exists.
func initSyncRepo(t *testing.T, dir string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(dir, 0o755))
	syncGit(t, dir, "init")
	syncGit(t, dir, "config", "user.email", "test@test.com")
	syncGit(t, dir, "config", "user.name", "Test")
	syncGit(t, dir, "config", "commit.gpgsign", "false")
	syncWrite(t, dir, ".seed", "seed\n")
	syncGit(t, dir, "add", ".seed")
	syncGit(t, dir, "commit", "-m", "seed")
}

// nestedRepo creates and seeds a nested repo under root, returning its path.
func nestedRepo(t *testing.T, root, name string) string {
	t.Helper()
	dir := filepath.Join(root, name)
	initSyncRepo(t, dir)
	return dir
}

func findRepo(repos []repoDirty, path string) *repoDirty {
	for i := range repos {
		if repos[i].Path == path {
			return &repos[i]
		}
	}
	return nil
}

func relSet(files []dirtyFile) map[string]bool {
	s := map[string]bool{}
	for _, f := range files {
		s[f.Rel] = true
	}
	return s
}

// --- S1: repo attribution ---

func TestSyncVerifyS1RepoAttribution(t *testing.T) {
	root := t.TempDir()
	initSyncRepo(t, root)
	modA := nestedRepo(t, root, "mod-a")
	syncWrite(t, modA, "pkg/x.go", "package pkg\n")
	syncWrite(t, root, "ARCHITECTURE.md", "# arch\n")

	repos, err := collectDirty(root)
	require.NoError(t, err)

	rootRepo := findRepo(repos, ".")
	modARepo := findRepo(repos, "mod-a")
	require.NotNil(t, rootRepo)
	require.NotNil(t, modARepo)

	// Each dirty file appears in exactly one repo, count 1 each.
	require.Len(t, rootRepo.Files, 1, "root dirty count")
	require.Len(t, modARepo.Files, 1, "mod-a dirty count")
	assert.True(t, relSet(rootRepo.Files)["ARCHITECTURE.md"])
	assert.True(t, relSet(modARepo.Files)["pkg/x.go"])
	// The nested repo directory itself must not leak into the root inventory.
	assert.False(t, relSet(rootRepo.Files)["mod-a"])
	assert.False(t, relSet(rootRepo.Files)["pkg/x.go"])
	assert.False(t, relSet(modARepo.Files)["ARCHITECTURE.md"])

	// Phase attribution: root ARCHITECTURE.md -> Phase B, mod-a pkg/x.go -> Phase A.
	phaseA, phaseB := classifyPhases(repos)
	require.Len(t, phaseA, 1)
	assert.Equal(t, "mod-a", phaseA[0].RepoPath)
	assert.Equal(t, []string{"pkg/x.go"}, phaseA[0].Files)
	assert.Equal(t, []string{"ARCHITECTURE.md"}, phaseB.Files)
}

// --- S8: read-only invariant (zero git mutation) ---

func TestSyncVerifyS8ReadOnlyInvariant(t *testing.T) {
	root := t.TempDir()
	initSyncRepo(t, root)
	modA := nestedRepo(t, root, "mod-a")
	// Rich dirty state: staged + unstaged in root, untracked in module.
	syncWrite(t, root, "ARCHITECTURE.md", "# arch\n")
	syncGit(t, root, "add", "ARCHITECTURE.md")
	syncWrite(t, root, ".autopus/project/tech.md", "tech\n")
	syncWrite(t, modA, "src/app.ts", "export const x = 1\n")

	repoDirs := map[string]string{".": root, "mod-a": modA}
	type snap struct{ status, head string }
	before := map[string]snap{}
	for label, dir := range repoDirs {
		before[label] = snap{
			status: syncGitOut(t, dir, "status", "--porcelain=v1", "--untracked-files=all"),
			head:   syncGitOut(t, dir, "rev-parse", "HEAD"),
		}
	}

	// Exercise the default, --spec, and --strict variants.
	for _, variant := range []struct {
		spec   string
		strict bool
	}{{"", false}, {"", true}} {
		var buf bytes.Buffer
		_, _ = executeSyncVerify(&buf, root, variant.spec, variant.strict)
	}

	for label, dir := range repoDirs {
		after := snap{
			status: syncGitOut(t, dir, "status", "--porcelain=v1", "--untracked-files=all"),
			head:   syncGitOut(t, dir, "rev-parse", "HEAD"),
		}
		assert.Equal(t, before[label].status, after.status, "porcelain status unchanged for %s", label)
		assert.Equal(t, before[label].head, after.head, "HEAD unchanged for %s", label)
	}
}
