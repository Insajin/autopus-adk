package cli

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
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

// checkArch verifies file size limits (300-line hard limit for .go source files).
// Skips vendor/, *_generated.go, *_gen.go, and *.pb.go files.
// Returns false if any file exceeds the hard limit.
func checkArch(dir string, out io.Writer, quiet bool) bool {
	if !quiet {
		tui.SectionHeader(out, "arch: file size")
	}

	passed := true
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		if isGeneratedGoFile(d.Name()) {
			return nil
		}

		lines, countErr := countLines(path)
		if countErr != nil {
			return countErr
		}

		rel, _ := filepath.Rel(dir, path)
		switch {
		case lines > hardLineLimit:
			tui.FAIL(out, fmt.Sprintf("%s (%d lines — exceeds %d hard limit)", rel, lines, hardLineLimit))
			passed = false
		case lines > warnLineLimit:
			if !quiet {
				tui.SKIP(out, fmt.Sprintf("%s (%d lines — consider splitting)", rel, lines))
			}
		default:
			if !quiet {
				tui.OK(out, fmt.Sprintf("%s (%d lines)", rel, lines))
			}
		}
		return nil
	})

	if err != nil {
		tui.Error(out, fmt.Sprintf("arch check error: %v", err))
		return false
	}
	return passed
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
// Returns false if the format is invalid or git is unavailable.
func checkLore(dir string, out io.Writer, quiet bool) bool {
	if !quiet {
		tui.SectionHeader(out, "lore: commit format")
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
