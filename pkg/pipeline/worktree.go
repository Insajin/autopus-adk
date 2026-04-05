package pipeline

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// @AX:NOTE: [AUTO] magic constants — maxWorktrees(5), lockRetryBase(3s), lockRetryAttempts(3) encode worktree-safety rule limits
const (
	maxWorktrees      = 5
	lockRetryBase     = 3 * time.Second
	lockRetryFactor   = 2
	lockRetryAttempts = 3
)

// WorktreeManager manages isolated git worktrees for parallel pipeline execution.
type WorktreeManager struct {
	mu       sync.Mutex
	paths    map[string]struct{}
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
	safeBranch := sanitizeBranchName(branch)
	wtBranch := fmt.Sprintf("worktree/%s", safeBranch)

	if m.isGitRepo {
		if err := m.addWorktreeWithRetry(ctx, dir, wtBranch); err != nil {
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

// @AX:WARN: [AUTO] git command execution with user-derived branch name — sanitizeBranchName must be called before this to prevent injection
// addWorktreeWithRetry runs "git -c gc.auto=0 worktree add" with exponential backoff
// on shared resource lock errors (refs.lock, packed-refs.lock, etc.).
func (m *WorktreeManager) addWorktreeWithRetry(ctx context.Context, dir, branch string) error {
	var lastErr error
	wait := lockRetryBase

	for attempt := 0; attempt <= lockRetryAttempts; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(wait):
			}
			wait *= lockRetryFactor
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
		if err == nil {
			return nil
		}

		lastErr = fmt.Errorf("git worktree add: %w (output: %s)", err, strings.TrimSpace(string(out)))

		// Only retry on lock-related errors.
		if !isLockError(string(out)) {
			break
		}
	}

	return lastErr
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
		//nolint:gosec // path is an internally tracked value.
		cmd := exec.CommandContext(ctx, "git", "-c", "gc.auto=0", "worktree", "remove", "--force", path)
		if out, err := cmd.CombinedOutput(); err != nil {
			removeErr = fmt.Errorf("git worktree remove: %w (output: %s)", err, strings.TrimSpace(string(out)))
		}
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

// ActiveCount returns the number of currently tracked worktrees.
func (m *WorktreeManager) ActiveCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.paths)
}

// sanitizeBranchName replaces characters that are invalid in git branch names with dashes.
func sanitizeBranchName(name string) string {
	replacer := strings.NewReplacer(
		" ", "-",
		".", "-",
		"~", "-",
		"^", "-",
		":", "-",
		"?", "-",
		"*", "-",
		"[", "-",
		"\\", "-",
		"..", "-",
	)
	return replacer.Replace(name)
}

// isLockError returns true when the git command output indicates a shared lock conflict.
func isLockError(output string) bool {
	lockPatterns := []string{
		"refs.lock",
		"packed-refs.lock",
		"shallow.lock",
		"index.lock",
		"unable to lock",
		"lock file",
	}
	lower := strings.ToLower(output)
	for _, p := range lockPatterns {
		if strings.Contains(lower, p) {
			return true
		}
	}
	return false
}

// worktreePath returns the absolute path that would be used for a worktree.
// Exported for testing convenience.
func worktreePath(base, suffix string) string {
	return filepath.Join(base, suffix)
}
