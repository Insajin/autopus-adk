package parallel

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// WorktreeManager creates and removes git worktrees for task isolation.
// All git commands use gc.auto=0 to suppress garbage collection during
// parallel execution, preventing pack file contention.
type WorktreeManager struct {
	baseDir string
}

// NewWorktreeManager creates a manager rooted at the given repository directory.
func NewWorktreeManager(baseDir string) *WorktreeManager {
	return &WorktreeManager{baseDir: baseDir}
}

// Create creates a new worktree for the given task on a fresh branch.
// Returns the worktree path.
func (m *WorktreeManager) Create(taskID string) (string, error) {
	wtPath := m.worktreePath(taskID)
	branch := fmt.Sprintf("worker-%s", taskID)

	cmd := exec.Command(
		"git", "-c", "gc.auto=0",
		"worktree", "add", wtPath, "-b", branch,
	)
	cmd.Dir = m.baseDir

	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("worktree create %s: %s: %w", taskID, strings.TrimSpace(string(out)), err)
	}
	if err := m.ensureRuntimeExclude(wtPath); err != nil {
		return "", err
	}
	return wtPath, nil
}

// Remove removes a worktree. Use force=true for failed/aborted tasks.
func (m *WorktreeManager) Remove(worktreePath string, force bool) error {
	args := []string{"-c", "gc.auto=0", "worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, worktreePath)

	cmd := exec.Command("git", args...)
	cmd.Dir = m.baseDir

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("worktree remove %s: %s: %w", worktreePath, strings.TrimSpace(string(out)), err)
	}
	return nil
}

// IsClean reports whether the worktree has no modified or untracked files.
func (m *WorktreeManager) IsClean(worktreePath string) (bool, error) {
	cmd := exec.Command("git", "status", "--porcelain", "--untracked-files=normal")
	cmd.Dir = worktreePath

	out, err := cmd.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("worktree status %s: %s: %w", worktreePath, strings.TrimSpace(string(out)), err)
	}
	return strings.TrimSpace(string(out)) == "", nil
}

// RemoveIfClean removes the worktree only when it has no local changes.
func (m *WorktreeManager) RemoveIfClean(worktreePath string) (bool, error) {
	clean, err := m.IsClean(worktreePath)
	if err != nil {
		return false, err
	}
	if !clean {
		return false, nil
	}
	return true, m.Remove(worktreePath, false)
}

// List returns all active worktree paths (excluding the main worktree).
func (m *WorktreeManager) List() ([]string, error) {
	cmd := exec.Command("git", "-c", "gc.auto=0", "worktree", "list", "--porcelain")
	cmd.Dir = m.baseDir

	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("worktree list: %s: %w", strings.TrimSpace(string(out)), err)
	}

	var paths []string
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "worktree ") {
			path := strings.TrimPrefix(line, "worktree ")
			// Skip the main worktree (the base directory itself).
			if path != m.baseDir {
				paths = append(paths, path)
			}
		}
	}
	return paths, nil
}

// worktreePath returns the filesystem path for a task's worktree.
func (m *WorktreeManager) worktreePath(taskID string) string {
	return filepath.Join(m.baseDir, ".worktrees", fmt.Sprintf("worker-%s", taskID))
}

func (m *WorktreeManager) ensureRuntimeExclude(worktreePath string) error {
	cmd := exec.Command("git", "rev-parse", "--git-common-dir")
	cmd.Dir = worktreePath
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("resolve common git dir for %s: %s: %w", worktreePath, strings.TrimSpace(string(out)), err)
	}

	commonGitDir := strings.TrimSpace(string(out))
	if !filepath.IsAbs(commonGitDir) {
		commonGitDir = filepath.Join(worktreePath, commonGitDir)
	}
	excludePath := filepath.Join(commonGitDir, "info", "exclude")
	if err := os.MkdirAll(filepath.Dir(excludePath), 0o755); err != nil {
		return fmt.Errorf("prepare git exclude dir: %w", err)
	}

	current, err := os.ReadFile(excludePath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read git exclude: %w", err)
	}
	if strings.Contains(string(current), ".symphony/") {
		return nil
	}

	var next strings.Builder
	next.Write(current)
	if len(current) > 0 && !strings.HasSuffix(string(current), "\n") {
		next.WriteByte('\n')
	}
	next.WriteString(".symphony/\n")

	if err := os.WriteFile(excludePath, []byte(next.String()), 0o644); err != nil {
		return fmt.Errorf("write git exclude: %w", err)
	}
	return nil
}
