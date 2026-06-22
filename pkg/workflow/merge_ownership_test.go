package workflow

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestParsePlanOwnership covers both the direct {tasks:[...]} shape and the
// {plan:{tasks:[...]}} wrapper the dispatcher persists from the segment-A return.
func TestParsePlanOwnership(t *testing.T) {
	direct := `{"tasks":[{"id":"T1","files":["pkg/a/a.go"]}]}`
	got, err := ParsePlanOwnership([]byte(direct))
	if err != nil || len(got) != 1 || got[0].ID != "T1" {
		t.Fatalf("direct parse failed: %v %+v", err, got)
	}
	wrapped := `{"plan":{"tasks":[{"id":"T2","files":["pkg/b/b.go","pkg/b/b_test.go"]}]}}`
	got, err = ParsePlanOwnership([]byte(wrapped))
	if err != nil || len(got) != 1 || got[0].ID != "T2" || len(got[0].Files) != 2 {
		t.Fatalf("wrapped parse failed: %v %+v", err, got)
	}
}

// TestOwnsFile verifies suffix-aligned matching against LLM-prefixed plan paths.
func TestOwnsFile(t *testing.T) {
	files := []string{"/Users/x/autopus-co/pkg/greeting/greeting.go"} // absolute, prefixed
	if !ownsFile(files, "pkg/greeting/greeting.go") {
		t.Error("prefixed absolute task file should own the repo-relative path")
	}
	if ownsFile(files, "pkg/greeting/greeting_test.go") {
		t.Error("must not own a different file")
	}
	if ownsFile([]string{"greeting.go"}, "pkg/other/greeting.go") {
		t.Error("basename-only must not over-match a different directory")
	}
}

// TestMerge_OwnershipEnforced is the hard-guarantee regression: two executors,
// where the second (test task) overreached and also created the impl file owned
// by the first task. With ownership enforcement, the overreaching impl copy is
// dropped as out-of-scope (no conflict), and each owned file is merged from its
// owner — reproducing the original chained-run overlap and proving it's closed.
func TestMerge_OwnershipEnforced(t *testing.T) {
	workingDir := t.TempDir()
	runID := "wf_own-001"
	wt1 := filepath.Join(worktreeRoot(workingDir), runID+"-1") // impl task T1
	wt2 := filepath.Join(worktreeRoot(workingDir), runID+"-2") // test task T2 (overreaches)
	must(t, os.MkdirAll(filepath.Join(wt1, "pkg/greeting"), 0o755))
	must(t, os.MkdirAll(filepath.Join(wt2, "pkg/greeting"), 0o755))
	must(t, os.WriteFile(filepath.Join(wt1, "pkg/greeting/greeting.go"), []byte("impl-correct"), 0o644))
	// wt2 owns greeting_test.go but also created greeting.go (overreach).
	must(t, os.WriteFile(filepath.Join(wt2, "pkg/greeting/greeting.go"), []byte("impl-stray"), 0o644))
	must(t, os.WriteFile(filepath.Join(wt2, "pkg/greeting/greeting_test.go"), []byte("test"), 0o644))

	ownership := []TaskOwnership{
		{ID: "T1", Files: []string{"pkg/greeting/greeting.go"}},
		{ID: "T2", Files: []string{"pkg/greeting/greeting_test.go"}},
	}
	runner := &fakeGitOutputRunner{
		worktreeListOut: buildWorktreeListOutput([]struct{ path, branch string }{
			{workingDir, "main"},
			{wt1, "worktree-" + runID + "-1"},
			{wt2, "worktree-" + runID + "-2"},
		}),
		statusOut: map[string]string{
			wt1: "?? pkg/greeting/greeting.go\n",
			wt2: "?? pkg/greeting/greeting.go\n?? pkg/greeting/greeting_test.go\n",
		},
	}

	result, err := MergeExecutorWorktreesWithOwnership(context.Background(), runner, workingDir, runID, ownership)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Conflicts) != 0 {
		t.Errorf("ownership enforcement must eliminate the conflict, got %v", result.Conflicts)
	}
	// Both owned files merged.
	for _, want := range []string{"pkg/greeting/greeting.go", "pkg/greeting/greeting_test.go"} {
		found := false
		for _, f := range result.MergedFiles {
			if f == want {
				found = true
			}
		}
		if !found {
			t.Errorf("owned file %q must be merged, got %v", want, result.MergedFiles)
		}
	}
	// wt2's stray greeting.go reported out-of-scope.
	if len(result.SkippedOutOfScope) != 1 || result.SkippedOutOfScope[0] != "pkg/greeting/greeting.go" {
		t.Errorf("stray greeting.go must be reported out-of-scope, got %v", result.SkippedOutOfScope)
	}
	// The correct (owner) impl content wins.
	assertFileContent(t, filepath.Join(workingDir, "pkg/greeting/greeting.go"), "impl-correct")
	assertFileContent(t, filepath.Join(workingDir, "pkg/greeting/greeting_test.go"), "test")
}
