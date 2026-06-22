package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/pkg/workflow"
)

// newWorkflowMergeCmd builds the `auto workflow merge` subcommand.
// It consolidates uncommitted executor worktree changes into the working dir,
// then stages merged files via `git add`, and removes the worktrees.
func newWorkflowMergeCmd() *cobra.Command {
	var runID string
	var workingDir string

	cmd := &cobra.Command{
		Use:           "merge",
		Short:         "Consolidate executor worktree changes into the working directory (JS->Go bridge)",
		Long:          "Copies uncommitted changes from all executor worktrees matching --run into workingDir, stages them with git add, and removes the worktrees. Emits a WorktreeMergeResult JSON to stdout.",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if runID == "" {
				return fmt.Errorf("--run is required")
			}

			dir := workingDir
			if dir == "" {
				cwd, err := os.Getwd()
				if err != nil {
					return fmt.Errorf("getwd: %w", err)
				}
				dir = cwd
			}

			runner := liveGitOutputRunner{}
			result, err := workflow.MergeExecutorWorktrees(cmd.Context(), runner, dir, runID)
			if err != nil {
				return fmt.Errorf("merge executor worktrees: %w", err)
			}

			// Stage merged files so that `auto workflow gate` sees them.
			if len(result.MergedFiles) > 0 {
				if err := gitAdd(cmd.Context(), dir, result.MergedFiles); err != nil {
					return fmt.Errorf("git add: %w", err)
				}
			}

			// Remove each merged worktree and delete its branch.
			// Use --force per worktree-safety.md (these are throwaway worktrees).
			for _, wtPath := range result.MergedWorktrees {
				if err := removeWorktree(cmd.Context(), dir, wtPath); err != nil {
					// Log but do not abort — a cleanup failure must not block the gate.
					fmt.Fprintf(cmd.ErrOrStderr(), "[workflow merge] cleanup warning: %v\n", err)
				}
			}

			data, err := result.EncodeJSON()
			if err != nil {
				return fmt.Errorf("encode merge result: %w", err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), string(data))
			return nil
		},
	}

	cmd.Flags().StringVar(&runID, "run", "", "Run ID whose executor worktrees to merge (required)")
	cmd.Flags().StringVar(&workingDir, "working-dir", "", "Working directory (defaults to cwd)")
	return cmd
}

// liveGitOutputRunner is the production GitOutputRunner using os/exec.
// It runs `git -c gc.auto=0 <args...>` in dir and captures stdout.
// The gc.auto=0 flag suppresses automatic GC per worktree-safety.md.
type liveGitOutputRunner struct{}

func (liveGitOutputRunner) Run(ctx context.Context, dir string, args ...string) (string, int, error) {
	// Prepend -c gc.auto=0 to suppress GC during parallel-worktree operations.
	fullArgs := append([]string{"-c", "gc.auto=0"}, args...)
	cmd := exec.CommandContext(ctx, "git", fullArgs...) //nolint:gosec // args come from operator flags
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

// gitAdd stages the given repo-relative file paths with `git add`.
// Uses gc.auto=0 per worktree-safety.md.
func gitAdd(ctx context.Context, dir string, files []string) error {
	// Sort for determinism.
	sorted := make([]string, len(files))
	copy(sorted, files)
	sort.Strings(sorted)

	args := append([]string{"-c", "gc.auto=0", "add", "--"}, sorted...)
	cmd := exec.CommandContext(ctx, "git", args...) //nolint:gosec // paths already validated in merge
	cmd.Dir = dir
	return cmd.Run()
}

// removeWorktree removes a worktree and deletes its branch.
// branch name is inferred from the worktree path base name: "worktree-<base>".
// Uses --force per worktree-safety.md for throwaway executor worktrees.
//
// Defence in depth: even though MergeExecutorWorktrees only returns worktrees
// contained under .claude/worktrees/, this re-checks the path before a
// destructive --force removal so a caller bug can never destroy an arbitrary
// worktree.
func removeWorktree(ctx context.Context, dir, wtPath string) error {
	if !strings.Contains(filepath.ToSlash(wtPath), "/.claude/worktrees/") {
		return fmt.Errorf("refusing to remove worktree outside .claude/worktrees/: %s", wtPath)
	}

	// Remove worktree.
	removeArgs := []string{"-c", "gc.auto=0", "worktree", "remove", "--force", wtPath}
	rmCmd := exec.CommandContext(ctx, "git", removeArgs...) //nolint:gosec
	rmCmd.Dir = dir
	if err := rmCmd.Run(); err != nil {
		return fmt.Errorf("worktree remove %s: %w", wtPath, err)
	}

	// Infer branch name from worktree path base: the runtime names branches
	// "worktree-<base>" where base = filepath.Base(wtPath).
	// This is an observed convention (Claude Code Workflow runtime v2.1.174+).
	branch := "worktree-" + filepath.Base(wtPath)

	// Delete branch (ignore error if branch doesn't exist or was already removed).
	delArgs := []string{"-c", "gc.auto=0", "branch", "-D", branch}
	delCmd := exec.CommandContext(ctx, "git", delArgs...) //nolint:gosec
	delCmd.Dir = dir
	_ = delCmd.Run() // best-effort; branch may not exist

	return nil
}
