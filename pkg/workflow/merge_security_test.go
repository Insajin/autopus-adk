package workflow

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestMerge_SymlinkSourceSkipped (H-1): a symlink in a worktree pointing at a
// host file outside the repo must NOT be copied/followed, so its target content
// never lands in workingDir (secret-exfil defence).
func TestMerge_SymlinkSourceSkipped(t *testing.T) {
	workingDir := t.TempDir()
	runID := "wf_seclink-001"
	wt := filepath.Join(worktreeRoot(workingDir), runID+"-1")
	must(t, os.MkdirAll(wt, 0o755))

	// A "secret" host file outside the repo.
	secretDir := t.TempDir()
	secret := filepath.Join(secretDir, "credentials")
	must(t, os.WriteFile(secret, []byte("SECRET-TOKEN"), 0o600))

	// Executor leaves a symlink to the secret plus one legit regular file.
	must(t, os.Symlink(secret, filepath.Join(wt, "stolen.txt")))
	must(t, os.WriteFile(filepath.Join(wt, "real.go"), []byte("real"), 0o644))

	runner := &fakeGitOutputRunner{
		worktreeListOut: buildWorktreeListOutput([]struct{ path, branch string }{
			{workingDir, "main"},
			{wt, "worktree-" + runID + "-1"},
		}),
		statusOut: map[string]string{wt: "?? stolen.txt\n?? real.go\n"},
	}

	result, err := MergeExecutorWorktrees(context.Background(), runner, workingDir, runID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, f := range result.MergedFiles {
		if f == "stolen.txt" {
			t.Error("symlink source must not be merged")
		}
	}
	if _, err := os.Stat(filepath.Join(workingDir, "stolen.txt")); !os.IsNotExist(err) {
		t.Error("symlink/target must not be copied into workingDir")
	}
	// The legit regular file should still merge.
	assertFileContent(t, filepath.Join(workingDir, "real.go"), "real")
}

// TestMerge_HardlinkSourceSkipped (H-1-RESIDUAL): a hard link in a worktree
// pointing at a host file (Nlink>1, but a regular file — passes symlink checks)
// must NOT be copied, so its target content never lands in workingDir.
func TestMerge_HardlinkSourceSkipped(t *testing.T) {
	workingDir := t.TempDir()
	runID := "wf_hardlink-001"
	wt := filepath.Join(worktreeRoot(workingDir), runID+"-1")
	must(t, os.MkdirAll(wt, 0o755))

	// A "secret" host file; hard-link it into the worktree (same FS via TempDir
	// is not guaranteed, so fall back to skipping if Link fails cross-device).
	secret := filepath.Join(t.TempDir(), "credentials")
	must(t, os.WriteFile(secret, []byte("PRIVATE-KEY-MATERIAL"), 0o600))
	stolen := filepath.Join(wt, "stolen.txt")
	if err := os.Link(secret, stolen); err != nil {
		t.Skipf("hard link not supported here: %v", err)
	}
	must(t, os.WriteFile(filepath.Join(wt, "real.go"), []byte("real"), 0o644))

	runner := &fakeGitOutputRunner{
		worktreeListOut: buildWorktreeListOutput([]struct{ path, branch string }{
			{workingDir, "main"},
			{wt, "worktree-" + runID + "-1"},
		}),
		statusOut: map[string]string{wt: "?? stolen.txt\n?? real.go\n"},
	}

	result, err := MergeExecutorWorktrees(context.Background(), runner, workingDir, runID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, f := range result.MergedFiles {
		if f == "stolen.txt" {
			t.Error("hard-linked source must not be merged")
		}
	}
	if _, err := os.Stat(filepath.Join(workingDir, "stolen.txt")); !os.IsNotExist(err) {
		t.Error("hard-linked target must not be copied into workingDir")
	}
	assertFileContent(t, filepath.Join(workingDir, "real.go"), "real")
}

// TestMerge_ContainmentExcludesOutsideTree (H-2): a worktree whose name matches
// the runID prefix but which lives OUTSIDE <workingDir>/.claude/worktrees/ must
// be excluded — so a stray --run can never select (and later --force remove) the
// main worktree or an unrelated worktree, and its files are never copied.
func TestMerge_ContainmentExcludesOutsideTree(t *testing.T) {
	workingDir := t.TempDir()
	runID := "wf_contain-001"

	// (a) name-matching worktree placed directly under workingDir (NOT under
	// .claude/worktrees/) — must be excluded.
	stray := filepath.Join(workingDir, runID+"-1")
	must(t, os.MkdirAll(stray, 0o755))
	must(t, os.WriteFile(filepath.Join(stray, "stray.go"), []byte("stray"), 0o644))

	// (b) name-matching worktree in a sibling tree entirely outside workingDir.
	sibling := filepath.Join(t.TempDir(), runID+"-2")
	must(t, os.MkdirAll(sibling, 0o755))
	must(t, os.WriteFile(filepath.Join(sibling, "outside.go"), []byte("outside"), 0o644))

	runner := &fakeGitOutputRunner{
		worktreeListOut: buildWorktreeListOutput([]struct{ path, branch string }{
			{workingDir, "main"},
			{stray, "worktree-" + runID + "-1"},
			{sibling, "worktree-" + runID + "-2"},
		}),
		statusOut: map[string]string{
			stray:   "?? stray.go\n",
			sibling: "?? outside.go\n",
		},
	}

	result, err := MergeExecutorWorktrees(context.Background(), runner, workingDir, runID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.MergedWorktrees) != 0 || len(result.MergedFiles) != 0 {
		t.Errorf("worktrees outside .claude/worktrees/ must be excluded, got %+v", result)
	}
	if _, err := os.Stat(filepath.Join(workingDir, "stray.go")); !os.IsNotExist(err) {
		t.Error("stray worktree file must not be copied")
	}
	if _, err := os.Stat(filepath.Join(workingDir, "outside.go")); !os.IsNotExist(err) {
		t.Error("outside worktree file must not be copied")
	}
}

// TestMerge_DeletedFileSkipped (reviewer MEDIUM): a deleted entry in
// `git status --porcelain` must be skipped, not treated as a missing-source
// copy that aborts the whole merge.
func TestMerge_DeletedFileSkipped(t *testing.T) {
	workingDir := t.TempDir()
	runID := "wf_del-001"
	wt := filepath.Join(worktreeRoot(workingDir), runID+"-1")
	must(t, os.MkdirAll(wt, 0o755))
	must(t, os.WriteFile(filepath.Join(wt, "kept.go"), []byte("kept"), 0o644))

	runner := &fakeGitOutputRunner{
		worktreeListOut: buildWorktreeListOutput([]struct{ path, branch string }{
			{workingDir, "main"},
			{wt, "worktree-" + runID + "-1"},
		}),
		// " D gone.go" = deletion (file absent in worktree); "?? kept.go" = new.
		statusOut: map[string]string{wt: " D gone.go\n?? kept.go\n"},
	}

	result, err := MergeExecutorWorktrees(context.Background(), runner, workingDir, runID)
	if err != nil {
		t.Fatalf("deleted entry must not abort the merge: %v", err)
	}
	for _, f := range result.MergedFiles {
		if f == "gone.go" {
			t.Error("deleted file must not be merged")
		}
	}
	assertFileContent(t, filepath.Join(workingDir, "kept.go"), "kept")
}
