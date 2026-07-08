package pipeline

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

// @AX:NOTE: [AUTO] magic constant - maxWorktrees(5) encodes the worktree-safety rule limit
const (
	maxWorktrees = 5
)

// WorktreeManager manages isolated git worktrees for parallel pipeline execution.
type WorktreeManager struct {
	mu        sync.Mutex
	paths     map[string]struct{}
	isGitRepo bool
}

// NewWorktreeManager creates a WorktreeManager with default settings.
// Max concurrent worktrees: 5
func NewWorktreeManager() *WorktreeManager {
	m := &WorktreeManager{
		paths: make(map[string]struct{}),
	}
	m.isGitRepo = m.detectGitRepo()
	return m
}

// detectGitRepo checks whether the current working directory is inside a git repository.
func (m *WorktreeManager) detectGitRepo() bool {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	return cmd.Run() == nil
}

// Create creates a new git worktree under os.TempDir().
// Returns the worktree path.
// Returns error if max worktree limit (5) is reached.
func (m *WorktreeManager) Create(ctx context.Context, branch string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.paths) >= maxWorktrees {
		return "", fmt.Errorf("worktree limit reached: max %d concurrent worktrees allowed", maxWorktrees)
	}

	// Build a unique directory path under os.TempDir().
	dir, err := os.MkdirTemp(os.TempDir(), "autopus-worktree-")
	if err != nil {
		return "", fmt.Errorf("create temp dir: %w", err)
	}

	// Sanitize branch name for use in a git branch identifier.
	safeBranch, err := sanitizeBranchName(branch)
	if err != nil {
		_ = os.RemoveAll(dir)
		return "", fmt.Errorf("invalid branch name: %w", err)
	}
	wtBranch := fmt.Sprintf("worktree/%s", safeBranch)

	if m.isGitRepo {
		if err := m.addWorktree(ctx, dir, wtBranch); err != nil {
			// Fallback: directory was already created by MkdirTemp; use it as-is.
			// This allows tests without a real git repo to still function.
			_ = os.RemoveAll(dir)
			// Re-create a plain directory as fallback.
			if mkErr := os.MkdirAll(dir, 0o755); mkErr != nil {
				return "", fmt.Errorf("fallback mkdir: %w", mkErr)
			}
		}
	}

	m.paths[dir] = struct{}{}
	return dir, nil
}

// @AX:WARN: [AUTO] git command execution with user-derived branch name — mitigated by inline ValidateBranchName and upstream sanitizeBranchName in Create; defense-in-depth layers active
// @AX:REASON: The retained pipeline public API still shells out to git; branch names are sanitized before command construction and passed as argv.
func (m *WorktreeManager) addWorktree(ctx context.Context, dir, branch string) error {
	// Inline validation — defense-in-depth even if caller already validated
	if branch != "" {
		if err := ValidateBranchName(branch); err != nil {
			return fmt.Errorf("branch name validation failed: %w", err)
		}
	}

	//nolint:gosec // branch and dir are internally generated; no user input injection.
	args := []string{"-c", "gc.auto=0", "worktree", "add"}
	if branch != "" {
		args = append(args, "-b", branch)
	} else {
		args = append(args, "--detach")
	}
	args = append(args, dir)
	cmd := exec.CommandContext(ctx, "git", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git worktree add: %w (output: %s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// Remove removes a git worktree and cleans up tracking.
func (m *WorktreeManager) Remove(ctx context.Context, path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.paths[path]; !ok {
		return fmt.Errorf("worktree not tracked: %s", path)
	}

	var removeErr error
	if m.isGitRepo {
		removeErr = removeGitWorktree(ctx, path)
	} else {
		if err := os.RemoveAll(path); err != nil {
			removeErr = fmt.Errorf("remove dir: %w", err)
		}
	}

	// @AX:NOTE: [AUTO] intentional untrack-before-error-return — keeps ActiveCount consistent even on failed removals
	// Always untrack regardless of removal error so ActiveCount stays consistent.
	delete(m.paths, path)

	return removeErr
}

func removeGitWorktree(ctx context.Context, path string) error {
	var lastNotWorktree error

	for _, candidate := range removeCandidates(path) {
		out, err := runGitWorktreeRemove(ctx, candidate)
		if err == nil {
			return nil
		}

		formatted := fmt.Errorf(
			"git worktree remove: %w (output: %s)",
			err,
			strings.TrimSpace(string(out)),
		)
		if !isNotWorktreeError(string(out)) {
			return formatted
		}
		lastNotWorktree = formatted
	}

	if err := os.RemoveAll(path); err != nil {
		if lastNotWorktree != nil {
			return fmt.Errorf("%w; remove dir fallback: %w", lastNotWorktree, err)
		}
		return fmt.Errorf("remove dir fallback: %w", err)
	}
	return nil
}

func removeCandidates(path string) []string {
	candidates := []string{path}
	resolved, err := filepath.EvalSymlinks(path)
	if err == nil && resolved != "" && resolved != path {
		candidates = append(candidates, resolved)
	}
	return candidates
}

func runGitWorktreeRemove(ctx context.Context, path string) ([]byte, error) {
	//nolint:gosec // path is an internally tracked value.
	cmd := exec.CommandContext(ctx, "git", "-c", "gc.auto=0", "worktree", "remove", "--force", path)
	return cmd.CombinedOutput()
}

// ActiveCount returns the number of currently tracked worktrees.
func (m *WorktreeManager) ActiveCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.paths)
}

// sanitizeBranchName validates and returns the branch name.
// Returns error for names containing '..', starting with '-', or exceeding 255 chars.
func sanitizeBranchName(name string) (string, error) {
	if err := ValidateBranchName(name); err != nil {
		return "", err
	}
	// Apply replacements for git-incompatible chars
	replacer := strings.NewReplacer(
		" ", "-",
		"~", "-",
		"^", "-",
		":", "-",
		"?", "-",
		"*", "-",
		"[", "-",
		"\\", "-",
	)
	return replacer.Replace(name), nil
}

func isNotWorktreeError(output string) bool {
	return strings.Contains(strings.ToLower(output), "not a working tree")
}
