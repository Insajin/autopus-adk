// Package cli implements the initialPrompt guard check for subagent frontmatter.
package cli

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/insajin/autopus-adk/internal/cli/tui"
)

// subagentFrontmatter holds only the fields we need to detect from agent .md files.
// It is intentionally separate from agentFrontmatter in agent_create.go to avoid
// coupling the guard logic to the create-command struct.
type subagentFrontmatter struct {
	InitialPrompt string `yaml:"initialPrompt"`
}

// InitialPromptViolation describes a single subagent file that contains the
// forbidden initialPrompt field.
type InitialPromptViolation struct {
	// File is the relative path to the offending agent definition.
	File string
	// Line is the 1-based line number of the initialPrompt key. Zero means
	// we could not determine the line (still treated as a violation).
	Line int
}

// mainSessionAgentNames lists file names that represent the main-session auto
// agent definition. These files are EXEMPT from the initialPrompt guard.
var mainSessionAgentNames = map[string]bool{
	"auto.md": true,
}

// checkInitialPrompt scans subagent .md files under agentGlob patterns and
// reports an error if any file's YAML frontmatter contains initialPrompt.
// Main-session files listed in mainSessionAgentNames are skipped.
// Returns false if any violations are found.
func checkInitialPrompt(dir string, out io.Writer, quiet bool) bool {
	if !quiet {
		tui.SectionHeader(out, "initial-prompt-guard")
	}

	patterns := []string{
		filepath.Join(dir, ".claude", "agents", "**", "*.md"),
		filepath.Join(dir, ".claude", "agents", "*.md"),
	}

	violations, scanErr := scanForInitialPrompt(dir, patterns)
	if scanErr != nil {
		tui.Error(out, fmt.Sprintf("initial-prompt-guard scan error: %v", scanErr))
		return false
	}

	if len(violations) == 0 {
		if !quiet {
			tui.OK(out, "no subagent defines initialPrompt")
		}
		return true
	}

	for _, v := range violations {
		lineRef := ""
		if v.Line > 0 {
			lineRef = fmt.Sprintf(":%d", v.Line)
		}
		tui.FAIL(out, fmt.Sprintf(
			"[initial-prompt-guard] ERROR: %s%s\n  "+
				"`initialPrompt` is not supported on Agent()-spawned subagents (SPEC-CC21-001 R11b).\n  "+
				"Remove this field or move the content into the prompt passed to Agent().\n  "+
				"See: .claude/agents/autopus/AGENT-GUIDE.md",
			v.File, lineRef,
		))
	}
	return false
}

// scanForInitialPrompt walks the filesystem and returns violations for each
// subagent .md file that declares initialPrompt in its YAML frontmatter.
func scanForInitialPrompt(root string, patterns []string) ([]InitialPromptViolation, error) {
	visited := make(map[string]bool)
	var violations []InitialPromptViolation

	for _, pattern := range patterns {
		matches, err := globRecursive(root, pattern)
		if err != nil {
			return nil, err
		}
		for _, abs := range matches {
			if visited[abs] {
				continue
			}
			visited[abs] = true

			base := filepath.Base(abs)
			if mainSessionAgentNames[base] {
				continue
			}

			v, found, err := checkFileForInitialPrompt(root, abs)
			if err != nil {
				// Unreadable file — skip without failing the overall check.
				continue
			}
			if found {
				violations = append(violations, v)
			}
		}
	}
	return violations, nil
}

// globRecursive resolves a glob pattern that may contain "**" by walking the
// filesystem when the standard filepath.Glob cannot handle it.
func globRecursive(root, pattern string) ([]string, error) {
	if !strings.Contains(pattern, "**") {
		return filepath.Glob(pattern)
	}

	var result []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".md") {
			return nil
		}
		result = append(result, path)
		return nil
	})
	return result, err
}

// checkFileForInitialPrompt reads a single .md file and checks whether its YAML
// frontmatter contains the initialPrompt key.
func checkFileForInitialPrompt(root, abs string) (InitialPromptViolation, bool, error) {
	data, err := os.ReadFile(abs)
	if err != nil {
		return InitialPromptViolation{}, false, err
	}

	_, line, found := parseFrontmatterInitialPrompt(string(data))
	if !found {
		return InitialPromptViolation{}, false, nil
	}

	rel, _ := filepath.Rel(root, abs)
	return InitialPromptViolation{File: rel, Line: line}, true, nil
}

// parseFrontmatterInitialPrompt extracts the YAML frontmatter from a Markdown
// file and reports whether it contains an initialPrompt key. The second return
// value is the 1-based line number of that key (0 if not found).
func parseFrontmatterInitialPrompt(content string) (subagentFrontmatter, int, bool) {
	const fence = "---"

	lines := strings.Split(content, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != fence {
		return subagentFrontmatter{}, 0, false
	}

	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == fence {
			end = i
			break
		}
	}
	if end < 0 {
		return subagentFrontmatter{}, 0, false
	}

	yamlBlock := strings.Join(lines[1:end], "\n")

	var fm subagentFrontmatter
	if err := yaml.Unmarshal([]byte(yamlBlock), &fm); err != nil {
		return subagentFrontmatter{}, 0, false
	}

	if fm.InitialPrompt == "" {
		return subagentFrontmatter{}, 0, false
	}

	// Locate the line number of the initialPrompt key within the original file.
	keyLine := 0
	for i := 1; i <= end; i++ {
		trimmed := strings.TrimSpace(lines[i-1])
		if strings.HasPrefix(trimmed, "initialPrompt") {
			keyLine = i
			break
		}
	}

	return fm, keyLine, true
}
