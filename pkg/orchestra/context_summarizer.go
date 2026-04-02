package orchestra

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// contextSourceFiles lists project files to scan for context, in priority order.
var contextSourceFiles = []string{
	"ARCHITECTURE.md",
	".autopus/project/product.md",
	".autopus/project/structure.md",
	"go.mod",
}

// ContextSummarizerConfig holds settings for context collection.
type ContextSummarizerConfig struct {
	ProjectDir string // root directory of the project
	MaxTokens  int    // approximate token budget (1 token ~ 4 chars)
}

// ContextSummarizer scans project files and produces a compressed
// context preamble that fits within a token budget.
type ContextSummarizer struct {
	cfg ContextSummarizerConfig
}

// NewContextSummarizer creates a summarizer with the given config.
func NewContextSummarizer(cfg ContextSummarizerConfig) *ContextSummarizer {
	if cfg.MaxTokens <= 0 {
		cfg.MaxTokens = 2000
	}
	return &ContextSummarizer{cfg: cfg}
}

// Summarize scans project files and returns a populated PromptData
// with project context fields filled in.
func (cs *ContextSummarizer) Summarize() PromptData {
	data := PromptData{
		MaxTurns: 20,
	}

	data.ProjectName = cs.detectProjectName()
	data.MustReadFiles = cs.findExistingFiles()

	budget := cs.cfg.MaxTokens * 4 // approximate chars
	used := 0

	for _, file := range data.MustReadFiles {
		path := filepath.Join(cs.cfg.ProjectDir, file)
		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		text := string(content)
		remaining := budget - used
		if remaining <= 0 {
			break
		}
		if len(text) > remaining {
			text = text[:remaining]
		}
		used += len(text)
		cs.extractFromFile(file, text, &data)
	}

	if data.TargetModule == "" {
		data.TargetModule = data.ProjectName
	}

	return data
}

// detectProjectName extracts the project name from go.mod or directory name.
func (cs *ContextSummarizer) detectProjectName() string {
	gomod := filepath.Join(cs.cfg.ProjectDir, "go.mod")
	data, err := os.ReadFile(gomod)
	if err == nil {
		for _, line := range strings.SplitN(string(data), "\n", 5) {
			if strings.HasPrefix(line, "module ") {
				mod := strings.TrimPrefix(line, "module ")
				parts := strings.Split(strings.TrimSpace(mod), "/")
				return parts[len(parts)-1]
			}
		}
	}
	return filepath.Base(cs.cfg.ProjectDir)
}

// findExistingFiles returns context source files that exist on disk.
func (cs *ContextSummarizer) findExistingFiles() []string {
	var found []string
	for _, f := range contextSourceFiles {
		path := filepath.Join(cs.cfg.ProjectDir, f)
		if _, err := os.Stat(path); err == nil {
			found = append(found, f)
		}
	}
	return found
}

// extractFromFile extracts structured context from a file's content.
func (cs *ContextSummarizer) extractFromFile(name, content string, data *PromptData) {
	switch {
	case strings.HasSuffix(name, "product.md"):
		data.ProjectSummary = extractFirstParagraph(content)
	case name == "ARCHITECTURE.md":
		cs.extractArchitecture(content, data)
	case name == "go.mod":
		data.TechStack = "Go"
	}
}

// extractArchitecture parses ARCHITECTURE.md for components and tech stack.
func (cs *ContextSummarizer) extractArchitecture(content string, data *PromptData) {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		comp := extractListItem(trimmed)
		if comp != "" && len(data.Components) < 10 {
			data.Components = append(data.Components, comp)
		}
	}
	if data.ProjectSummary == "" {
		data.ProjectSummary = extractFirstParagraph(content)
	}
}

// extractListItem extracts a component name from a markdown list line.
// Handles "- `name`", "- **name**", "- name" formats.
func extractListItem(line string) string {
	if !strings.HasPrefix(line, "- ") {
		return ""
	}
	rest := strings.TrimPrefix(line, "- ")
	// Extract backtick-quoted name: `name`
	if strings.HasPrefix(rest, "`") {
		end := strings.Index(rest[1:], "`")
		if end > 0 {
			return rest[1 : end+1]
		}
	}
	// Extract bold name: **name**
	if strings.HasPrefix(rest, "**") {
		end := strings.Index(rest[2:], "**")
		if end > 0 {
			return rest[2 : end+2]
		}
	}
	return ""
}

// ScanRelevantPaths discovers source directories and adds them as RelevantPaths.
func (cs *ContextSummarizer) ScanRelevantPaths(topic string) []RelevantPath {
	dirs := []struct {
		path string
		desc string
	}{
		{"pkg", "library packages"},
		{"internal", "internal packages"},
		{"cmd", "CLI entry points"},
		{"src", "source code"},
		{"app", "application code"},
	}

	var paths []RelevantPath
	for _, d := range dirs {
		full := filepath.Join(cs.cfg.ProjectDir, d.path)
		if info, err := os.Stat(full); err == nil && info.IsDir() {
			paths = append(paths, RelevantPath{
				Path:        d.path,
				Description: d.desc,
			})
		}
	}
	return paths
}

// PopulatePromptData creates a fully populated PromptData for a given topic.
func (cs *ContextSummarizer) PopulatePromptData(topic string) PromptData {
	data := cs.Summarize()
	data.Topic = topic
	data.RelevantPaths = cs.ScanRelevantPaths(topic)
	return data
}

// extractFirstParagraph returns the first non-heading, non-empty paragraph.
func extractFirstParagraph(content string) string {
	lines := strings.Split(content, "\n")
	var para []string
	started := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !started {
			if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "---") {
				continue
			}
			started = true
		}
		if started {
			if trimmed == "" {
				break
			}
			para = append(para, trimmed)
		}
	}
	result := strings.Join(para, " ")
	if len(result) > 500 {
		result = result[:500]
	}
	return result
}

// EstimateTokens returns an approximate token count for a string.
// Uses the common heuristic of ~4 characters per token.
func EstimateTokens(s string) int {
	return (len(s) + 3) / 4
}

// TruncateToTokens truncates a string to approximately maxTokens.
func TruncateToTokens(s string, maxTokens int) string {
	maxChars := maxTokens * 4
	if len(s) <= maxChars {
		return s
	}
	truncated := s[:maxChars]
	if idx := strings.LastIndex(truncated, "\n"); idx > maxChars/2 {
		truncated = truncated[:idx]
	}
	return truncated + fmt.Sprintf("\n... [truncated to ~%d tokens]", maxTokens)
}
