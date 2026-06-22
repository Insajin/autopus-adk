package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
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
	// ownership maps relpath → first worktree that claimed it.
	ownership := make(map[string]string) // relpath → worktreePath
	conflicts := make(map[string]bool)
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

		for _, f := range files {
			if _, seen := ownership[f]; seen {
				conflicts[f] = true
			} else {
				ownership[f] = wt.path
			}
		}
	}

	// Step 4 & 5: copy non-conflicting files; collect merged worktree paths.
	for _, p := range plan {
		result.MergedWorktrees = append(result.MergedWorktrees, p.wt.path)
		for _, relpath := range p.files {
			if conflicts[relpath] {
				continue
			}
			// Only copy from the owning worktree.
			if ownership[relpath] != p.wt.path {
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

	return result, nil
}

// parseWorktreeList parses `git worktree list --porcelain` output into records.
// Format per worktree block:
//
//	worktree /abs/path
//	HEAD <sha>
//	branch refs/heads/<branchname>    (or "detached")
//	(blank line separator)
func parseWorktreeList(out string) []worktreeRecord {
	var records []worktreeRecord
	var cur worktreeRecord
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "worktree ") {
			cur = worktreeRecord{path: strings.TrimPrefix(line, "worktree ")}
		} else if strings.HasPrefix(line, "branch ") {
			ref := strings.TrimPrefix(line, "branch ")
			cur.branch = strings.TrimPrefix(ref, "refs/heads/")
		} else if line == "" && cur.path != "" {
			records = append(records, cur)
			cur = worktreeRecord{}
		}
	}
	if cur.path != "" {
		records = append(records, cur)
	}
	return records
}

// selectWorktrees returns worktrees that belong to runID.
// Match criteria (runtime convention, observed on v2.1.174):
//   - branch == "worktree-<runID>-<N>"
//   - OR base(path) starts with "<runID>-"
//
// In addition to the name match, the worktree MUST physically reside strictly
// under <workingDir>/.claude/worktrees/ (resolved via EvalSymlinks). This
// containment is a hard safety boundary: it excludes the main worktree (== the
// working dir, which would otherwise self-truncate on copy) and any unrelated
// worktree (e.g. one under .worktrees/), so a stray/short --run value can never
// cause a destructive `git worktree remove --force` of legitimate work.
func selectWorktrees(records []worktreeRecord, runID, workingDir string) []worktreeRecord {
	branchPrefix := "worktree-" + runID + "-"
	pathPrefix := runID + "-"

	root, err := filepath.Abs(filepath.Join(workingDir, ".claude", "worktrees"))
	if err != nil {
		return nil
	}
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}

	var out []worktreeRecord
	for _, r := range records {
		abs, err := filepath.Abs(r.path)
		if err != nil {
			continue
		}
		if resolved, err := filepath.EvalSymlinks(abs); err == nil {
			abs = resolved
		}
		if abs == root || !strings.HasPrefix(abs, root+string(os.PathSeparator)) {
			continue // outside the .claude/worktrees/ containment root
		}
		if strings.HasPrefix(r.branch, branchPrefix) || strings.HasPrefix(filepath.Base(r.path), pathPrefix) {
			out = append(out, r)
		}
	}
	return out
}

// changedFiles parses `git status --porcelain` in wtPath and returns the
// repo-relative paths of all changed/untracked files that are safe to copy.
// Handles: ?? (untracked), M (modified), A (added), R (renamed — uses post-rename).
// Skipped (silently): deleted entries (D — the file no longer exists), paths that
// escape after filepath.Clean, symlinks (never followed — exfil defence), and
// non-regular files. Skipping at collection time keeps the copy step total.
func changedFiles(ctx context.Context, r GitOutputRunner, wtPath string) ([]string, error) {
	// --untracked-files=all is REQUIRED: the default porcelain output collapses a
	// new untracked directory to a single "?? dir/" entry, which would be skipped
	// as a non-regular path and lose every file an executor created inside a new
	// package directory. -uall lists each nested file individually.
	stdout, exit, err := r.Run(ctx, wtPath, "status", "--porcelain", "--untracked-files=all")
	if err != nil || exit != 0 {
		return nil, fmt.Errorf("exit=%d: %w", exit, err)
	}
	var files []string
	for _, line := range strings.Split(stdout, "\n") {
		if len(line) < 4 {
			continue
		}
		// Porcelain format: XY SP <path>  OR  XY SP <oldpath> -> <newpath>
		xy := line[:2]
		if xy[0] == 'D' || xy[1] == 'D' {
			continue // deletion — nothing to copy; avoids a missing-source abort
		}
		rest := line[3:]

		// Handle rename: "old -> new" — take the post-rename (destination).
		var relpath string
		if idx := strings.Index(rest, " -> "); idx >= 0 {
			relpath = rest[idx+4:]
		} else {
			relpath = rest
		}
		relpath = strings.Trim(relpath, "\"")

		if !isSafePath(relpath) {
			continue
		}
		// Never copy or follow a symlink / non-regular source (exfil defence:
		// a symlink to ~/.ssh/id_ed25519 must not be read into the repo).
		info, err := os.Lstat(filepath.Join(wtPath, relpath))
		if err != nil {
			continue // missing/deleted between status and now
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			continue
		}
		// Skip hard-linked files (Nlink>1): a worktree file linked to an external
		// inode is an exfil signal (laundering an outside secret into the repo).
		if hardLinked(info) {
			continue
		}
		files = append(files, relpath)
	}
	return files, nil
}
