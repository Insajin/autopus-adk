package spec

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// UpdateStatus rewrites the spec.md status field in frontmatter or legacy metadata.
func UpdateStatus(specDir, status string) error {
	status = strings.ToLower(strings.TrimSpace(status))
	if status == "" {
		return fmt.Errorf("spec status must not be empty")
	}

	path := filepath.Join(specDir, "spec.md")
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read spec.md: %w", err)
	}

	updated, err := rewriteSpecStatus(string(content), status)
	if err != nil {
		return err
	}

	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		return fmt.Errorf("write spec.md: %w", err)
	}
	return nil
}

func rewriteSpecStatus(content, status string) (string, error) {
	lines := strings.Split(content, "\n")
	start, end := frontmatterBounds(lines)
	if start >= 0 && end > start {
		return rewriteFrontmatterStatus(lines, start, end, status), nil
	}

	if idx := legacyFieldLineIndex(lines, "status"); idx >= 0 {
		lines[idx] = lineIndent(lines[idx]) + "**Status**: " + status
		return strings.Join(lines, "\n"), nil
	}

	if idx := firstHeadingLineIndex(lines); idx >= 0 {
		return strings.Join(insertLegacyStatusLine(lines, idx, status), "\n"), nil
	}

	return "", fmt.Errorf("spec.md status field not found")
}

func rewriteFrontmatterStatus(lines []string, start, end int, status string) string {
	replaced := false
	for i := start + 1; i < end; i++ {
		trimmed := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(strings.ToLower(trimmed), "status:") {
			continue
		}
		indent := lines[i][:len(lines[i])-len(strings.TrimLeft(lines[i], " \t"))]
		lines[i] = indent + "status: " + status
		replaced = true
		break
	}
	if !replaced {
		injected := append([]string{}, lines[:end]...)
		injected = append(injected, "status: "+status)
		lines = append(injected, lines[end:]...)
	}

	return strings.Join(lines, "\n")
}

func insertLegacyStatusLine(lines []string, headingIdx int, status string) []string {
	insertAt := headingIdx + 1
	for insertAt < len(lines) && strings.TrimSpace(lines[insertAt]) == "" {
		insertAt++
	}

	before := append([]string{}, lines[:insertAt]...)
	after := append([]string{}, lines[insertAt:]...)
	if len(before) > 0 && strings.TrimSpace(before[len(before)-1]) != "" {
		before = append(before, "")
	}
	before = append(before, "**Status**: "+status)
	if len(after) > 0 && strings.TrimSpace(after[0]) != "" {
		before = append(before, "")
	}
	return append(before, after...)
}

func firstHeadingLineIndex(lines []string) int {
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "# ") {
			return i
		}
	}
	return -1
}
