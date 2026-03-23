package experiment

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// Git provides experiment-specific git operations for a repository at dir.
type Git struct {
	dir string
}

// @AX:ANCHOR [AUTO]: Primary constructor for git operations — 3+ consumers depend on this signature.
// @AX:REASON: internal/cli/experiment.go, internal/cli/experiment_helpers.go, git_test.go all call NewGit.
// Changing the signature requires updating all call sites.
// NewGit creates a Git handle rooted at the given directory.
func NewGit(dir string) *Git {
	return &Git{dir: dir}
}

// run executes a git command with gc.auto=0 suppression.
func (g *Git) run(args ...string) (string, error) {
	fullArgs := append([]string{"-c", "gc.auto=0"}, args...)
	cmd := exec.Command("git", fullArgs...)
	cmd.Dir = g.dir

	var out bytes.Buffer
	var errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %w\n%s", strings.Join(args, " "), err, errBuf.String())
	}

	return strings.TrimSpace(out.String()), nil
}

// CreateExperimentBranch creates and checks out branch experiment/XLOOP-{sessionID}.
func (g *Git) CreateExperimentBranch(sessionID string) error {
	branch := "experiment/XLOOP-" + sessionID
	_, err := g.run("checkout", "-b", branch)
	return err
}

// CommitExperiment stages all changes and commits with the standard format.
// Returns the full 40-character commit hash.
func (g *Git) CommitExperiment(iteration int, description string) (string, error) {
	if _, err := g.run("add", "-A"); err != nil {
		return "", err
	}

	// Derive session from branch name; fall back to "unknown"
	session := g.sessionFromBranch()
	msg := fmt.Sprintf("experiment(XLOOP-%s): iteration %d - %s", session, iteration, description)

	if _, err := g.run("commit", "-m", msg); err != nil {
		return "", err
	}

	hash, err := g.run("rev-parse", "HEAD")
	if err != nil {
		return "", err
	}

	return hash, nil
}

// @AX:WARN [AUTO]: Destructive operation — discards all uncommitted changes and untracked files.
// @AX:REASON: git reset --hard + clean -fd is irreversible without a prior commit. Callers must
// ensure the target commitHash is valid and the worktree state is intentionally discardable.
// ResetToCommit hard-resets to commitHash and removes untracked files.
func (g *Git) ResetToCommit(commitHash string) error {
	if _, err := g.run("reset", "--hard", commitHash); err != nil {
		return err
	}

	_, err := g.run("clean", "-fd")
	return err
}

// CheckCleanWorktree returns an error if the worktree has any uncommitted changes.
func (g *Git) CheckCleanWorktree() error {
	out, err := g.run("status", "--porcelain")
	if err != nil {
		return err
	}

	if out != "" {
		return fmt.Errorf("worktree is dirty:\n%s", out)
	}

	return nil
}

// GetDiffStats returns lines added, lines removed, and affected file paths
// relative to baseCommit.
func (g *Git) GetDiffStats(baseCommit string) (added int, removed int, files []string, err error) {
	// --numstat outputs: added<TAB>removed<TAB>filename
	out, err := g.run("diff", "--numstat", baseCommit, "HEAD")
	if err != nil {
		return 0, 0, nil, err
	}

	if out == "" {
		return 0, 0, nil, nil
	}

	for _, line := range strings.Split(out, "\n") {
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 3 {
			continue
		}

		var a, r int
		fmt.Sscanf(parts[0], "%d", &a)
		fmt.Sscanf(parts[1], "%d", &r)

		added += a
		removed += r
		files = append(files, filepath.Base(parts[2]))
	}

	return added, removed, files, nil
}

// IsExperimentBranch reports whether the current branch starts with "experiment/".
func (g *Git) IsExperimentBranch() (bool, error) {
	branch, err := g.run("branch", "--show-current")
	if err != nil {
		return false, err
	}

	return strings.HasPrefix(branch, "experiment/"), nil
}

// CheckScope returns whether all changed files since baseCommit are within allowedPaths.
// It also returns the list of out-of-scope files.
func (g *Git) CheckScope(baseCommit string, allowedPaths []string) (bool, []string, error) {
	_, _, changedFiles, err := g.GetDiffStats(baseCommit)
	if err != nil {
		return false, nil, err
	}

	allowed := make(map[string]bool, len(allowedPaths))
	for _, p := range allowedPaths {
		allowed[filepath.Base(p)] = true
	}

	var violations []string
	for _, f := range changedFiles {
		if !allowed[f] {
			violations = append(violations, f)
		}
	}

	return len(violations) == 0, violations, nil
}

// sessionFromBranch extracts the session ID from experiment/XLOOP-{sessionID}.
func (g *Git) sessionFromBranch() string {
	branch, err := g.run("branch", "--show-current")
	if err != nil {
		return "unknown"
	}

	prefix := "experiment/XLOOP-"
	if strings.HasPrefix(branch, prefix) {
		return branch[len(prefix):]
	}

	return "unknown"
}
