package workflow

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// realGitRunner is a GitOutputRunner backed by the real git binary, used to
// catch issues that a faked runner cannot (e.g. a missing CLI flag).
type realGitRunner struct{}

func (realGitRunner) Run(ctx context.Context, dir string, args ...string) (string, int, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err == nil {
		return string(out), 0, nil
	}
	if ee, ok := err.(*exec.ExitError); ok {
		return string(out), ee.ExitCode(), err
	}
	return "", 1, err
}

// TestMerge_RealGit_NewDirectoryEnumerated is a real-git regression for the
// untracked-directory collapse bug: an executor that creates files inside a NEW
// directory must have every nested file merged. Default `git status --porcelain`
// collapses such a dir to a single "?? dir/" entry; the merge must pass
// --untracked-files=all so the nested files are enumerated and copied.
func TestMerge_RealGit_NewDirectoryEnumerated(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	ctx := context.Background()
	runID := "wf_realgit-001"

	workingDir := t.TempDir()
	initGitRepo(t, ctx, workingDir)

	// Create an executor worktree under .claude/worktrees/ with a brand-new
	// package directory containing two nested files (the masked case).
	wt := filepath.Join(worktreeRoot(workingDir), runID+"-1")
	must(t, os.MkdirAll(filepath.Join(wt, "pkg", "greeting"), 0o755))
	initGitRepo(t, ctx, wt) // make the worktree dir its own repo so status works
	must(t, os.WriteFile(filepath.Join(wt, "pkg/greeting/greeting.go"), []byte("package greeting\n"), 0o644))
	must(t, os.WriteFile(filepath.Join(wt, "pkg/greeting/greeting_test.go"), []byte("package greeting\n"), 0o644))

	files, err := changedFiles(ctx, realGitRunner{}, wt)
	if err != nil {
		t.Fatalf("changedFiles: %v", err)
	}

	got := map[string]bool{}
	for _, f := range files {
		got[f] = true
	}
	for _, want := range []string{"pkg/greeting/greeting.go", "pkg/greeting/greeting_test.go"} {
		if !got[want] {
			t.Errorf("nested new-directory file %q not enumerated (got %v) — untracked-dir collapse not handled", want, files)
		}
	}
}

func initGitRepo(t *testing.T, ctx context.Context, dir string) {
	t.Helper()
	for _, args := range [][]string{
		{"init", "-q"},
		{"config", "user.email", "test@example.com"},
		{"config", "user.name", "test"},
	} {
		cmd := exec.CommandContext(ctx, "git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}
