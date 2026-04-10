package cli

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/insajin/autopus-adk/internal/cli/tui"
)

const (
	// warnLineLimit is the line count that triggers a warning.
	warnLineLimit = 200
	// hardLineLimit is the line count that triggers an error.
	hardLineLimit = 300
)

// loreValidTypes defines allowed Lore commit type prefixes.
var loreValidTypes = []string{
	"feat(", "fix(", "refactor(", "test(", "docs(", "chore(", "perf(",
}

// loreSignOff is the required Lore sign-off line.
const loreSignOff = "🐙 Autopus <noreply@autopus.co>"

// skipDirs contains directory names that should be skipped during arch walk.
// These include version control, tooling worktrees, and dependency directories.
var skipDirs = map[string]bool{
	"vendor":       true,
	".git":         true,
	"node_modules": true,
}

// checkArch verifies file size limits (300-line hard limit for .go source files).
// Skips vendor/, .git/, node_modules/, submodule directories (containing a .git file),
// .claude/worktrees/, *_generated.go, *_gen.go, and *.pb.go files.
// When stagedOnly is true, only git-staged .go files are checked.
// Returns false if any file exceeds the hard limit.
func checkArch(dir string, out io.Writer, quiet, stagedOnly bool) bool {
	if !quiet {
		tui.SectionHeader(out, "arch: file size")
	}

	if stagedOnly {
		return checkArchStaged(dir, out, quiet)
	}
	return checkArchWalk(dir, out, quiet)
}


// isGeneratedGoFile reports whether a file name matches generated file patterns.
func isGeneratedGoFile(name string) bool {
	return strings.HasSuffix(name, "_generated.go") ||
		strings.HasSuffix(name, "_gen.go") ||
		strings.HasSuffix(name, ".pb.go")
}

// countLines counts the number of lines in a file.
func countLines(path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close() //nolint:errcheck

	count := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		count++
	}
	return count, scanner.Err()
}

// checkLore verifies that the most recent commit uses Lore format.
// It checks for a valid type prefix and the required sign-off line.
// Experiment branches (experiment/*) are exempt from Lore format.
// Returns false if the format is invalid or git is unavailable.
func checkLore(dir string, out io.Writer, quiet bool) bool {
	if !quiet {
		tui.SectionHeader(out, "lore: commit format")
	}

	// Experiment branches use a simplified commit format, not Lore.
	if isExperimentBranch(dir) {
		if !quiet {
			tui.SKIP(out, "experiment branch — Lore format not required")
		}
		return true
	}

	msg, err := lastCommitMessage(dir)
	if err != nil {
		// No commits yet or git unavailable — skip silently.
		if !quiet {
			tui.Info(out, "no commits found, skipping lore check")
		}
		return true
	}

	validType := hasValidLoreType(msg)
	hasSignOff := strings.Contains(msg, loreSignOff)

	if validType && hasSignOff {
		if !quiet {
			tui.OK(out, "last commit follows Lore format")
		}
		return true
	}

	if !validType {
		tui.FAIL(out, "last commit missing valid Lore type prefix (e.g. feat(...): ...)")
	}
	if !hasSignOff {
		tui.FAIL(out, fmt.Sprintf("last commit missing sign-off: %s", loreSignOff))
	}
	return false
}

// checkLoreFromFile validates a commit message read from the given file path.
// This is used by the commit-msg git hook where the message file is passed as $1.
func checkLoreFromFile(msgFile string, out io.Writer, quiet bool) bool {
	if !quiet {
		tui.SectionHeader(out, "lore: commit format (message file)")
	}

	data, err := os.ReadFile(msgFile)
	if err != nil {
		tui.Error(out, fmt.Sprintf("cannot read message file: %v", err))
		return false
	}

	msg := strings.TrimSpace(string(data))
	if msg == "" {
		tui.FAIL(out, "empty commit message")
		return false
	}

	validType := hasValidLoreType(msg)
	hasSignOff := strings.Contains(msg, loreSignOff)

	if validType && hasSignOff {
		if !quiet {
			tui.OK(out, "commit message follows Lore format")
		}
		return true
	}

	if !validType {
		tui.FAIL(out, "commit message missing valid Lore type prefix (e.g. feat(...): ...)")
	}
	if !hasSignOff {
		tui.FAIL(out, fmt.Sprintf("commit message missing sign-off: %s", loreSignOff))
	}
	return false
}

// lastCommitMessage returns the full body of the most recent commit.
func lastCommitMessage(dir string) (string, error) {
	cmd := exec.Command("git", "log", "-1", "--format=%B")
	cmd.Dir = dir
	var buf bytes.Buffer
	cmd.Stdout = &buf
	if err := cmd.Run(); err != nil {
		return "", err
	}
	msg := strings.TrimSpace(buf.String())
	if msg == "" {
		return "", fmt.Errorf("empty commit message")
	}
	return msg, nil
}

// hasValidLoreType reports whether msg starts with a recognised Lore type prefix.
func hasValidLoreType(msg string) bool {
	for _, t := range loreValidTypes {
		if strings.HasPrefix(msg, t) {
			return true
		}
	}
	return false
}

// isExperimentBranch reports whether the current branch has the experiment/ prefix.
func isExperimentBranch(dir string) bool {
	cmd := exec.Command("git", "branch", "--show-current")
	cmd.Dir = dir
	var buf bytes.Buffer
	cmd.Stdout = &buf
	if err := cmd.Run(); err != nil {
		return false
	}
	return strings.HasPrefix(strings.TrimSpace(buf.String()), "experiment/")
}
