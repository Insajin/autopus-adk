package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
)

// GitOutputRunner runs a git subcommand in dir and captures stdout.
// The production implementation wraps os/exec; tests inject a fake.
// This interface is separate from CommandRunner because merge needs stdout
// text (parsed worktree list, changed-file list), not just an exit code.
type GitOutputRunner interface {
	// Run executes a git subcommand in dir and returns (stdout, exitCode, err).
	// err is non-nil when the process could not be started; exitCode is
	// authoritative for failure detection.
	Run(ctx context.Context, dir string, args ...string) (stdout string, exitCode int, err error)
}

// WorktreeMergeResult is the structured output emitted by MergeExecutorWorktrees
// and printed as JSON by `auto workflow merge`.
type WorktreeMergeResult struct {
	RunID           string   `json:"run_id"`
	MergedWorktrees []string `json:"merged_worktrees"`
	MergedFiles     []string `json:"merged_files"`
	Conflicts       []string `json:"conflicts"`
	// SkippedOutOfScope lists files an executor created outside its task's file
	// ownership (only populated when ownership enforcement is active). They are
	// reported but never copied.
	SkippedOutOfScope []string `json:"skipped_out_of_scope,omitempty"`
}

// EncodeJSON serializes the merge result for CLI stdout consumption.
func (r WorktreeMergeResult) EncodeJSON() ([]byte, error) {
	return json.Marshal(r)
}

// worktreeRecord holds parsed output of `git worktree list --porcelain`.
type worktreeRecord struct {
	path   string
	branch string
}

// MergeExecutorWorktrees consolidates uncommitted changes from all executor
// worktrees belonging to runID into workingDir.
//
// The Claude Code Workflow runtime creates one worktree per parallel executor.
// Worktree branch naming is an observed runtime convention (v2.1.174+):
//   - worktree path base: <runID>-<N>   (under .claude/worktrees/)
//   - branch:             worktree-<runID>-<N>
//
// Executor changes are left as uncommitted working-tree edits (git status shows
// ?? / M / A). This function copies those files into workingDir so that
// `auto workflow gate` sees them. It does NOT git-add, commit, or remove worktrees
// — the CLI layer handles those steps.
//
// Conflict policy: if more than one worktree touches the same repo-relative path,
// the path is added to Conflicts and skipped (not copied). Planner file-ownership
// makes this rare; the CLI/operator decides how to resolve.
func MergeExecutorWorktrees(ctx context.Context, r GitOutputRunner, workingDir, runID string) (WorktreeMergeResult, error) {
	return MergeExecutorWorktreesWithOwnership(ctx, r, workingDir, runID, nil)
}

// MergeExecutorWorktreesWithOwnership is MergeExecutorWorktrees with hard file
// ownership enforcement. When ownership is non-nil, each worktree is matched to
// the task it performed (best file overlap) and only files within that task's
// ownership are merged; files an executor created outside its ownership (overlap
// into another task's files) are reported in SkippedOutOfScope and never copied.
// This eliminates the executor-overlap conflict at its source. ownership=nil
// preserves the plain conflict-skip behavior.
func MergeExecutorWorktreesWithOwnership(ctx context.Context, r GitOutputRunner, workingDir, runID string, ownership []TaskOwnership) (WorktreeMergeResult, error) {
	result := WorktreeMergeResult{RunID: runID}

	// Step 1: list all worktrees.
	stdout, exit, err := r.Run(ctx, workingDir, "worktree", "list", "--porcelain")
	if err != nil || exit != 0 {
		return result, fmt.Errorf("git worktree list: exit=%d err=%w", exit, err)
	}

	records := parseWorktreeList(stdout)

	// Step 2: select worktrees belonging to this run, constrained to the
	// .claude/worktrees/ tree under workingDir (never the main worktree or an
	// unrelated worktree — prevents destructive over-selection).
	selected := selectWorktrees(records, runID, workingDir)
	if len(selected) == 0 {
		return result, nil
	}

	// Sort for deterministic output.
	sort.Slice(selected, func(i, j int) bool {
		return selected[i].path < selected[j].path
	})

	// Step 3: collect changed files per worktree.
	type wtFiles struct {
		wt    worktreeRecord
		files []string
	}
	var plan []wtFiles
	for _, wt := range selected {
		files, err := changedFiles(ctx, r, wt.path)
		if err != nil {
			return result, fmt.Errorf("git status in %s: %w", wt.path, err)
		}
		sort.Strings(files)
		plan = append(plan, wtFiles{wt: wt, files: files})
	}

	// Step 3b: hard ownership enforcement. Assign each worktree 1:1 to the task it
	// performed (global best-overlap, so two worktrees never claim one task) and
	// drop files outside that task's ownership as out-of-scope. This removes the
	// executor-overlap conflict at its source.
	if ownership != nil {
		sets := make([][]string, len(plan))
		for i := range plan {
			sets[i] = plan[i].files
		}
		assign := assignWorktreesToTasks(sets, ownership)
		for i := range plan {
			var kept []string
			for _, f := range plan[i].files {
				if assign[i] >= 0 && ownsFile(ownership[assign[i]].Files, f) {
					kept = append(kept, f)
				} else {
					result.SkippedOutOfScope = append(result.SkippedOutOfScope, f)
				}
			}
			plan[i].files = kept
		}
	}

	// Step 4: build file → owning-worktree map and detect cross-worktree conflicts.
	fileOwner := make(map[string]string) // relpath → worktreePath
	conflicts := make(map[string]bool)
	for _, p := range plan {
		for _, f := range p.files {
			if _, seen := fileOwner[f]; seen {
				conflicts[f] = true
			} else {
				fileOwner[f] = p.wt.path
			}
		}
	}

	// Step 5: copy non-conflicting files; collect merged worktree paths.
	for _, p := range plan {
		result.MergedWorktrees = append(result.MergedWorktrees, p.wt.path)
		for _, relpath := range p.files {
			if conflicts[relpath] {
				continue
			}
			if fileOwner[relpath] != p.wt.path { // copy only from the owning worktree
				continue
			}
			if err := copyFile(p.wt.path, relpath, workingDir); err != nil {
				return result, fmt.Errorf("copy %s from %s: %w", relpath, p.wt.path, err)
			}
			result.MergedFiles = append(result.MergedFiles, relpath)
		}
	}

	// Collect conflicts as a sorted slice.
	for f := range conflicts {
		result.Conflicts = append(result.Conflicts, f)
	}
	sort.Strings(result.Conflicts)
	sort.Strings(result.MergedFiles)
	result.SkippedOutOfScope = sortedUnique(result.SkippedOutOfScope)

	return result, nil
}

// sortedUnique returns the input sorted with duplicates removed (nil-safe).
func sortedUnique(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(in))
	var out []string
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}
