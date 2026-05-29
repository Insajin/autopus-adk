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

var archSourceExtensions = map[string]bool{
	".c":     true,
	".cc":    true,
	".cjs":   true,
	".cpp":   true,
	".cs":    true,
	".css":   true,
	".cxx":   true,
	".go":    true,
	".h":     true,
	".hpp":   true,
	".java":  true,
	".js":    true,
	".jsx":   true,
	".kt":    true,
	".kts":   true,
	".less":  true,
	".mjs":   true,
	".php":   true,
	".py":    true,
	".rb":    true,
	".rs":    true,
	".sass":  true,
	".scss":  true,
	".sh":    true,
	".swift": true,
	".ts":    true,
	".tsx":   true,
	".vue":   true,
}

var archSkipDirs = map[string]bool{
	".agents":           true,
	".autopus":          true,
	".claude":           true,
	".codex":            true,
	".gemini":           true,
	".git":              true,
	".next":             true,
	".nuxt":             true,
	".opencode":         true,
	".output":           true,
	".svelte-kit":       true,
	".worktrees":        true,
	"build":             true,
	"coverage":          true,
	"dist":              true,
	"node_modules":      true,
	"playwright-report": true,
	"target":            true,
	"test-results":      true,
	"vendor":            true,
}

// checkArchStaged checks only git-staged source files for size limits.
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
		if shouldSkipArchRel(rel) {
			continue
		}
		if !isArchSourceFile(rel) {
			continue
		}
		if isGeneratedSourceFile(filepath.Base(rel)) {
			continue
		}

		lines, err := countStagedLines(dir, rel)
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

// checkArchWalk walks the directory tree checking all source files.
// Skips submodule directories (detected by a .git file inside) and worktree dirs.
func checkArchWalk(dir string, out io.Writer, quiet bool) bool {
	passed := true
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if archSkipDirs[d.Name()] {
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
		if !isArchSourceFile(path) {
			return nil
		}
		if isGeneratedSourceFile(d.Name()) {
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

func isArchSourceFile(path string) bool {
	return archSourceExtensions[strings.ToLower(filepath.Ext(path))]
}

func isGeneratedSourceFile(name string) bool {
	lower := strings.ToLower(name)
	if isGeneratedGoFile(lower) {
		return true
	}
	if strings.HasSuffix(lower, ".d.ts") ||
		strings.HasSuffix(lower, ".min.css") ||
		strings.HasSuffix(lower, ".min.js") ||
		lower == "build.rs" ||
		strings.HasPrefix(lower, "mock_") && strings.HasSuffix(lower, ".go") {
		return true
	}
	for _, marker := range []string{"_generated.", ".generated.", "_gen.", ".gen.", ".pb."} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func shouldSkipArchRel(rel string) bool {
	for _, part := range strings.Split(filepath.ToSlash(rel), "/") {
		if archSkipDirs[part] {
			return true
		}
	}
	return false
}

func countStagedLines(dir, rel string) (int, error) {
	cmd := exec.Command("git", "show", ":"+rel)
	cmd.Dir = dir
	data, err := cmd.Output()
	if err != nil {
		return 0, err
	}
	return countLinesInBytes(data), nil
}

func countLinesInBytes(data []byte) int {
	if len(data) == 0 {
		return 0
	}
	count := bytes.Count(data, []byte{'\n'})
	if data[len(data)-1] != '\n' {
		count++
	}
	return count
}
