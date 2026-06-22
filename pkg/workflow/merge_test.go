package workflow

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// fakeGitOutputRunner returns canned stdout per git subcommand.
type fakeGitOutputRunner struct {
	// worktreeListOut is the canned output for `git worktree list --porcelain`.
	worktreeListOut string
	// statusOut maps worktree abs path → canned `git status --porcelain` output.
	statusOut map[string]string
}

func (f *fakeGitOutputRunner) Run(_ context.Context, dir string, args ...string) (string, int, error) {
	if len(args) > 0 && args[0] == "worktree" {
		return f.worktreeListOut, 0, nil
	}
	if len(args) > 0 && args[0] == "status" {
		return f.statusOut[dir], 0, nil
	}
	return "", 0, nil
}

// buildWorktreeListOutput builds a `git worktree list --porcelain` block.
func buildWorktreeListOutput(entries []struct{ path, branch string }) string {
	var sb string
	for _, e := range entries {
		sb += "worktree " + e.path + "\n"
		sb += "HEAD aabbccdd\n"
		sb += "branch refs/heads/" + e.branch + "\n"
		sb += "\n"
	}
	return sb
}

// worktreeRoot returns <workingDir>/.claude/worktrees, the containment root the
// merge step requires (mirrors the real Workflow-runtime layout).
func worktreeRoot(workingDir string) string {
	return filepath.Join(workingDir, ".claude", "worktrees")
}

// TestMergeExecutorWorktrees_DisjointFiles: two worktrees with non-overlapping
// files are both merged into workingDir. (case a)
func TestMergeExecutorWorktrees_DisjointFiles(t *testing.T) {
	workingDir := t.TempDir()
	runID := "wf_42474961-862"
	wt1 := filepath.Join(worktreeRoot(workingDir), runID+"-1")
	wt2 := filepath.Join(worktreeRoot(workingDir), runID+"-2")
	must(t, os.MkdirAll(filepath.Join(wt1, "pkg"), 0o755))
	must(t, os.MkdirAll(filepath.Join(wt2, "pkg"), 0o755))
	must(t, os.WriteFile(filepath.Join(wt1, "pkg/foo.go"), []byte("foo"), 0o644))
	must(t, os.WriteFile(filepath.Join(wt2, "pkg/bar.go"), []byte("bar"), 0o644))

	runner := &fakeGitOutputRunner{
		worktreeListOut: buildWorktreeListOutput([]struct{ path, branch string }{
			{workingDir, "main"},
			{wt1, "worktree-" + runID + "-1"},
			{wt2, "worktree-" + runID + "-2"},
		}),
		statusOut: map[string]string{
			wt1: "?? pkg/foo.go\n",
			wt2: "?? pkg/bar.go\n",
		},
	}

	result, err := MergeExecutorWorktrees(context.Background(), runner, workingDir, runID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Conflicts) != 0 {
		t.Errorf("expected no conflicts, got %v", result.Conflicts)
	}
	if len(result.MergedWorktrees) != 2 {
		t.Errorf("expected 2 merged worktrees, got %d", len(result.MergedWorktrees))
	}
	if len(result.MergedFiles) != 2 {
		t.Errorf("expected 2 merged files, got %v", result.MergedFiles)
	}
	assertFileContent(t, filepath.Join(workingDir, "pkg/foo.go"), "foo")
	assertFileContent(t, filepath.Join(workingDir, "pkg/bar.go"), "bar")
}

// TestMergeExecutorWorktrees_Conflict: a file claimed by two worktrees is
// reported in Conflicts and not copied. (case b)
func TestMergeExecutorWorktrees_Conflict(t *testing.T) {
	workingDir := t.TempDir()
	runID := "wf_99999999-001"
	wt1 := filepath.Join(worktreeRoot(workingDir), runID+"-1")
	wt2 := filepath.Join(worktreeRoot(workingDir), runID+"-2")
	must(t, os.MkdirAll(wt1, 0o755))
	must(t, os.MkdirAll(wt2, 0o755))
	must(t, os.WriteFile(filepath.Join(wt1, "shared.go"), []byte("v1"), 0o644))
	must(t, os.WriteFile(filepath.Join(wt2, "shared.go"), []byte("v2"), 0o644))

	runner := &fakeGitOutputRunner{
		worktreeListOut: buildWorktreeListOutput([]struct{ path, branch string }{
			{workingDir, "main"},
			{wt1, "worktree-" + runID + "-1"},
			{wt2, "worktree-" + runID + "-2"},
		}),
		statusOut: map[string]string{
			wt1: " M shared.go\n",
			wt2: " M shared.go\n",
		},
	}

	result, err := MergeExecutorWorktrees(context.Background(), runner, workingDir, runID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Conflicts) != 1 || result.Conflicts[0] != "shared.go" {
		t.Errorf("expected conflict on shared.go, got %v", result.Conflicts)
	}
	if len(result.MergedFiles) != 0 {
		t.Errorf("expected 0 merged files (conflict not copied), got %v", result.MergedFiles)
	}
	if _, err := os.Stat(filepath.Join(workingDir, "shared.go")); !os.IsNotExist(err) {
		t.Error("conflicting file must not be written to workingDir")
	}
}

// TestMergeExecutorWorktrees_PathTraversal: a path-traversal relpath is silently
// skipped and never written outside workingDir. (case c)
func TestMergeExecutorWorktrees_PathTraversal(t *testing.T) {
	workingDir := t.TempDir()
	runID := "wf_11111111-001"
	wt1 := filepath.Join(worktreeRoot(workingDir), runID+"-1")
	must(t, os.MkdirAll(filepath.Join(wt1, "pkg"), 0o755))
	must(t, os.WriteFile(filepath.Join(wt1, "pkg/safe.go"), []byte("safe"), 0o644))

	runner := &fakeGitOutputRunner{
		worktreeListOut: buildWorktreeListOutput([]struct{ path, branch string }{
			{workingDir, "main"},
			{wt1, "worktree-" + runID + "-1"},
		}),
		statusOut: map[string]string{
			wt1: "?? ../evil.go\n?? pkg/safe.go\n",
		},
	}

	result, err := MergeExecutorWorktrees(context.Background(), runner, workingDir, runID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, f := range result.MergedFiles {
		if f == "../evil.go" || f == "evil.go" {
			t.Errorf("traversal path must not be merged: %s", f)
		}
	}
	if _, err := os.Stat(filepath.Join(workingDir, "..", "evil.go")); !os.IsNotExist(err) {
		t.Error("path-traversal file must not be written outside workingDir")
	}
	assertFileContent(t, filepath.Join(workingDir, "pkg/safe.go"), "safe")
}

// TestMergeExecutorWorktrees_UnrelatedWorktreeIgnored: worktrees NOT matching
// runID are ignored. (case d)
func TestMergeExecutorWorktrees_UnrelatedWorktreeIgnored(t *testing.T) {
	workingDir := t.TempDir()
	runID := "wf_aaaaaaaa-001"
	otherRun := "wf_bbbbbbbb-002"
	otherWT := filepath.Join(worktreeRoot(workingDir), otherRun+"-1")
	must(t, os.MkdirAll(otherWT, 0o755))
	must(t, os.WriteFile(filepath.Join(otherWT, "foreign.go"), []byte("foreign"), 0o644))

	runner := &fakeGitOutputRunner{
		worktreeListOut: buildWorktreeListOutput([]struct{ path, branch string }{
			{workingDir, "main"},
			{otherWT, "worktree-" + otherRun + "-1"},
		}),
		statusOut: map[string]string{otherWT: "?? foreign.go\n"},
	}

	result, err := MergeExecutorWorktrees(context.Background(), runner, workingDir, runID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.MergedWorktrees) != 0 || len(result.MergedFiles) != 0 {
		t.Errorf("expected nothing merged for non-matching run, got %+v", result)
	}
	if _, err := os.Stat(filepath.Join(workingDir, "foreign.go")); !os.IsNotExist(err) {
		t.Error("unrelated worktree file must not be copied")
	}
}

// TestMergeExecutorWorktrees_NoMatchingWorktrees: empty result, no error. (case e)
func TestMergeExecutorWorktrees_NoMatchingWorktrees(t *testing.T) {
	workingDir := t.TempDir()
	runID := "wf_cccccccc-001"
	runner := &fakeGitOutputRunner{
		worktreeListOut: buildWorktreeListOutput([]struct{ path, branch string }{
			{workingDir, "main"},
		}),
		statusOut: map[string]string{},
	}
	result, err := MergeExecutorWorktrees(context.Background(), runner, workingDir, runID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.MergedWorktrees) != 0 || len(result.MergedFiles) != 0 || len(result.Conflicts) != 0 {
		t.Errorf("expected empty result, got %+v", result)
	}
	if result.RunID != runID {
		t.Errorf("RunID mismatch: got %q, want %q", result.RunID, runID)
	}
}

// --- helpers ---

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func assertFileContent(t *testing.T, path, want string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("expected file %s to exist: %v", path, err)
	}
	if string(data) != want {
		t.Errorf("file %s: got %q, want %q", path, string(data), want)
	}
}
