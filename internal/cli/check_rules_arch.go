package cli

import (
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

// checkArchStaged checks only git-staged .go files for size limits.
func checkArchStaged(dir string, out io.Writer, quiet bool) bool {
	cmd := exec.Command("git", "diff", "--cached", "--name-only", "--diff-filter=ACM")
	cmd.Dir = dir
	var buf bytes.Buffer
	cmd.Stdout = &buf
	if err := cmd.Run(); err != nil {
		// No git or no staged files — pass silently.
		return true
	}

	passed := true
	for _, rel := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		if rel == "" {
			continue
		}
		if !strings.HasSuffix(rel, ".go") {
			continue
		}
		if isGeneratedGoFile(filepath.Base(rel)) {
			continue
		}

		abs := filepath.Join(dir, rel)
		lines, err := countLines(abs)
		if err != nil {
			continue
		}

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
	}
	return passed
}

// checkArchWalk walks the directory tree checking all .go files.
// Skips submodule directories (detected by a .git file inside) and worktree dirs.
func checkArchWalk(dir string, out io.Writer, quiet bool) bool {
	passed := true
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			// Skip .claude/worktrees/ directories (agent worktree leftovers).
			if d.Name() == "worktrees" && strings.HasSuffix(filepath.Dir(path), ".claude") {
				return filepath.SkipDir
			}
			// Skip submodule directories: they contain a .git file (not directory).
			if d.Name() != "." {
				gitPath := filepath.Join(path, ".git")
				if info, statErr := os.Lstat(gitPath); statErr == nil && !info.IsDir() {
					return filepath.SkipDir
				}
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
