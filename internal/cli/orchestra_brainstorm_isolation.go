package cli

import (
	"context"
	"os"
	"os/exec"

	"github.com/insajin/autopus-adk/pkg/orchestra"
)

// orchestraExecution is the per-run provider execution environment: whether
// providers run read-only, the isolated provider cwd, and whether the caller's
// worktree is guarded against provider mutation (issue #108).
type orchestraExecution struct {
	readOnly bool
	workDir  string // isolated provider cwd; empty keeps the repo cwd
	guarded  bool
	cleanup  func()
}

// outsideRepo reports whether providers start from a directory that is not
// the caller's repository.
func (e *orchestraExecution) outsideRepo() bool {
	return e.workDir != ""
}

// prepareOrchestraExecution builds the execution environment for a command.
// Brainstorm always runs read-only and guarded; without --context its
// providers start in a fresh temp directory outside the repo. The guard needs
// to observe completion, so a guarded run never detaches. Other commands keep
// their existing behavior.
func prepareOrchestraExecution(commandName string, contextAware bool) (*orchestraExecution, error) {
	execution := &orchestraExecution{cleanup: func() {}}
	if commandName != "brainstorm" {
		return execution, nil
	}
	execution.readOnly = true
	execution.guarded = true
	if contextAware {
		return execution, nil
	}
	dir, err := os.MkdirTemp("", "autopus-brainstorm-*")
	if err != nil {
		return nil, err
	}
	initIsolatedProviderRepo(dir)
	execution.workDir = dir
	execution.cleanup = func() { _ = os.RemoveAll(dir) }
	return execution, nil
}

// initIsolatedProviderRepo turns the temp provider directory into an empty git
// repository so interactive Codex sessions do not stall on their untrusted
// directory prompt. Best effort: the subprocess path also passes
// --skip-git-repo-check, and the directory is deleted after the run.
func initIsolatedProviderRepo(dir string) {
	cmd := exec.Command("git", "init", "-q")
	cmd.Dir = dir
	_ = cmd.Run()
}

// runGuardedOrchestra executes the run and, when guarded, compares the repo
// worktree before and after. A detected mutation outranks the run's own
// failure in the surfaced error while that failure stays attached as the
// cause and inside the diagnostics artifact. Nothing is rolled back.
func runGuardedOrchestra(ctx context.Context, cfg orchestra.OrchestraConfig, execution *orchestraExecution) (*orchestra.OrchestraResult, error) {
	var guard workspaceGuard
	if execution.guarded {
		guard = newWorkspaceGuard(cfg.WorkingDir)
	}
	result, err := runOrchestraExecute(ctx, cfg)
	if err == nil && shouldTreatOrchestraResultAsFailure(result) {
		err = synthesizeOrchestraFailureError(result)
	}
	if !execution.guarded {
		return result, err
	}
	evidence := guard.compare()
	orchestra.ApplyWorkspaceEvidence(result, evidence)
	if evidence.MutationDetected {
		err = &orchestra.WorkspaceMutationError{ChangedFiles: evidence.ChangedFiles, Cause: err}
	}
	return result, err
}

// applyJudgeReadOnlyPolicy projects a judge that was resolved outside the
// participant list so the judge process obeys the same read-only argv.
func applyJudgeReadOnlyPolicy(commandName string, judge *orchestra.ProviderConfig, opts readOnlyPolicyOptions) (*orchestra.ProviderConfig, error) {
	if judge == nil {
		return nil, nil
	}
	projected, err := applyCommandReadOnlyPolicy(commandName, []orchestra.ProviderConfig{*judge}, opts)
	if err != nil {
		return nil, err
	}
	return &projected[0], nil
}
