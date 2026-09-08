package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// singleAutopusRepo creates a lone Autopus project: one Git repository whose
// root carries autopus.yaml and holds no nested repositories.
func singleAutopusRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	initSyncRepo(t, dir)
	syncWrite(t, dir, "autopus.yaml", "project: single\n")
	syncGit(t, dir, "add", "autopus.yaml")
	syncGit(t, dir, "commit", "-m", "workspace config")
	return dir
}

// A single repository has no module boundary, so product code is a commit
// candidate rather than a path "outside the canonical root keep set" — that
// keep set exists only to push code out of a multi-repo meta root. The
// generated/runtime exclusion and the unclassified fail-closed guard stay.
func TestSyncVerifySingleRepoPartitionsEveryDirtyPath(t *testing.T) {
	root := singleAutopusRepo(t)
	syncWrite(t, root, "apps/web/src/app.ts", "export const x = 1\n")
	syncWrite(t, root, "pkg/service/handler.go", "package service\n")
	syncWrite(t, root, ".autopus/project/product.md", "product\n")
	syncWrite(t, root, ".autopus/runtime/state.json", "{}\n")
	syncWrite(t, root, ".claude/settings.json", "{}\n")
	unsafe := "pkg/$(touch PWNED).go"
	syncWrite(t, root, unsafe, "package pkg\n")

	var out bytes.Buffer
	warnings, err := executeSyncVerify(&out, root, "", true)
	require.ErrorIs(t, err, errSyncVerifyStrict)
	assert.Greater(t, warnings, 0)

	text := out.String()
	assert.Contains(t, text, "topology: single-repo")
	plan := strings.Split(text, "\nWarnings")[0]

	assert.Contains(t, plan, "apps/web/src/app.ts", "product code is a commit candidate")
	assert.Contains(t, plan, "pkg/service/handler.go")
	assert.Contains(t, plan, ".autopus/project/product.md", "root-scoped meta docs live here too")
	assert.NotContains(t, text, "outside canonical root keep set")
	assert.NotContains(t, text, "root-scoped meta path in module")

	assert.NotContains(t, plan, ".autopus/runtime/state.json")
	assert.NotContains(t, plan, ".claude/settings.json")
	assert.NotContains(t, plan, unsafe)
	assert.Contains(t, text, "blocked-path")
	assert.Contains(t, text, "generated/runtime")
	assert.Contains(t, text, "unclassified-path")
	assert.NoFileExists(t, filepath.Join(root, "PWNED"))
}

// A linked worktree has a .git file rather than a directory; only Git can name
// its worktree root, and that root — not the main checkout — owns the dirty
// paths being planned.
func TestSyncVerifySingleRepoLinkedWorktree(t *testing.T) {
	root := singleAutopusRepo(t)
	worktree := filepath.Join(t.TempDir(), "wt")
	syncGit(t, root, "worktree", "add", "-b", "feature", worktree)

	syncWrite(t, worktree, "src/feature.ts", "export const f = 1\n")
	syncWrite(t, worktree, ".autopus/runtime/state.json", "{}\n")

	var out bytes.Buffer
	warnings, err := executeSyncVerify(&out, worktree, "", true)
	require.ErrorIs(t, err, errSyncVerifyStrict)
	assert.Greater(t, warnings, 0)

	text := out.String()
	assert.Contains(t, text, "topology: single-repo")
	plan := strings.Split(text, "\nWarnings")[0]
	assert.Contains(t, plan, "git -C . add -- src/feature.ts")
	assert.NotContains(t, plan, ".autopus/runtime/state.json")
	assert.NotContains(t, text, "Phase A — module commits:")
}

// A meta root still wins over the single-repo fallback, keeps the two-phase
// plan, and needs no autopus.yaml of its own to be detected.
func TestSyncVerifyMultiRepoKeepsTwoPhasePlanAndNamesTopology(t *testing.T) {
	root := t.TempDir()
	initSyncRepo(t, root)
	modA := nestedRepo(t, root, "mod-a")
	syncWrite(t, root, "ARCHITECTURE.md", "# arch\n")
	syncWrite(t, modA, "pkg/x.go", "package pkg\n")

	var out bytes.Buffer
	_, err := executeSyncVerify(&out, root, "", false)
	require.NoError(t, err)

	text := out.String()
	assert.Contains(t, text, "topology: multi-repo (meta root .)")
	assert.NotContains(t, text, "topology: single-repo")
	assert.Contains(t, text, "Phase A — module commits:")
	assert.Contains(t, text, "git -C mod-a add -- pkg/x.go")
	assert.Contains(t, text, "Phase B — meta commit:")
	assert.Contains(t, text, "git -C . add -- ARCHITECTURE.md")
	assert.NotContains(t, text, "Commit candidates:")
}

// A meta root is detected from a bare ".git" stat, so a stale or half-created
// entry anchors a workspace Git cannot touch. That must surface as a topology
// diagnostic, not as an opaque failure from the first git status.
func TestSyncVerifyMetaRootThatIsNotAGitRepository(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".git"), 0o755))
	modA := nestedRepo(t, root, "mod-a")
	syncWrite(t, modA, "pkg/x.go", "package pkg\n")

	var out bytes.Buffer
	_, err := executeSyncVerify(&out, root, "", false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported topology:")
	assert.Contains(t, err.Error(), "meta root")
	assert.Contains(t, err.Error(), "is not a Git repository")
	assert.NotContains(t, err.Error(), "git status")
	assert.NotContains(t, err.Error(), root, "no absolute path in the diagnostic")
	assert.Equal(t, exitCodeUnsupportedTopology, exitCodeForError(err))
	assert.Empty(t, out.String())

	// The diagnostic tells the caller to run from a module, so that must work.
	syncWrite(t, modA, "autopus.yaml", "project: module\n")
	var fromModule bytes.Buffer
	_, err = executeSyncVerify(&fromModule, modA, "", false)
	require.NoError(t, err)
	assert.Contains(t, fromModule.String(), "topology: single-repo")
	assert.Contains(t, fromModule.String(), "git -C . add -- autopus.yaml pkg/x.go")
}

// An inapplicable directory must not look like a dirty working tree. The
// diagnostic names the topology problem and claims its own exit code, so an
// agent can tell "this command does not apply here" from "your changes are
// blocked" without parsing prose.
func TestSyncVerifyUnsupportedTopologyIsDistinctFromClassificationFailure(t *testing.T) {
	t.Run("no git repository", func(t *testing.T) {
		var out bytes.Buffer
		_, err := executeSyncVerify(&out, t.TempDir(), "", false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported topology:")
		assert.NotErrorIs(t, err, errSyncVerifyStrict)
		assert.Equal(t, exitCodeUnsupportedTopology, exitCodeForError(err))
		assert.Empty(t, out.String())
	})

	t.Run("git repository without autopus.yaml", func(t *testing.T) {
		dir := t.TempDir()
		initSyncRepo(t, dir)
		syncWrite(t, dir, "src/app.ts", "export const x = 1\n")

		var out bytes.Buffer
		_, err := executeSyncVerify(&out, dir, "", false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported topology:")
		assert.Contains(t, err.Error(), "autopus.yaml")
		assert.Equal(t, exitCodeUnsupportedTopology, exitCodeForError(err))
		assert.Empty(t, out.String())
	})

	t.Run("blocked path keeps the ordinary failure code", func(t *testing.T) {
		root := singleAutopusRepo(t)
		syncWrite(t, root, ".autopus/runtime/state.json", "{}\n")

		var out bytes.Buffer
		_, err := executeSyncVerify(&out, root, "", true)
		require.ErrorIs(t, err, errSyncVerifyStrict)
		assert.NotContains(t, err.Error(), "unsupported topology")
		assert.Equal(t, 1, exitCodeForError(err))
	})
}
