package spec

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const defaultSpecStatus = "draft"

var (
	specTitleRe      = regexp.MustCompile(`^(SPEC-[\w-]+)(?::\s*(.+))?$`)
	legacyHeadingRe  = regexp.MustCompile(`(?i)^SPEC:\s*(.+)$`)
	legacyFieldLabel = regexp.MustCompile(`^\*\*([^*]+)\*\*:\s*(.+)$`)
)

// ParseSpecMetadata extracts top-level SPEC metadata from spec.md content.
func ParseSpecMetadata(content string) SpecDocument {
	doc := SpecDocument{
		Status:  defaultSpecStatus,
		Version: "0.1.0",
	}

	lines := strings.Split(content, "\n")
	parseHeadingMetadata(lines, &doc)

	frontmatter := parseSpecFrontmatter(lines)
	if v := frontmatter["id"]; v != "" {
		doc.ID = v
	}
	if v := frontmatter["title"]; v != "" {
		doc.Title = v
	}
	if v := frontmatter["version"]; v != "" {
		doc.Version = v
	}
	if doc.ID == "" {
		doc.ID = parseLegacyField(lines, "spec-id")
	}
	if doc.Title == "" {
		doc.Title = parseLegacyField(lines, "title")
	}
	if v := frontmatter["status"]; v != "" {
		doc.Status = strings.ToLower(v)
	} else if v := parseLegacyField(lines, "status"); v != "" {
		doc.Status = strings.ToLower(v)
	}

	return doc
}

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

func parseSpecFrontmatter(lines []string) map[string]string {
	start, end := frontmatterBounds(lines)
	if start < 0 || end <= start {
		return nil
	}

	fields := make(map[string]string)
	for _, line := range lines[start+1 : end] {
		parts := strings.SplitN(strings.TrimSpace(line), ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(parts[0]))
		value := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
		fields[key] = value
	}
	return fields
}

func frontmatterBounds(lines []string) (int, int) {
	first := firstNonEmptyLine(lines, 0)
	if first < 0 {
		return -1, -1
	}

	start := -1
	switch trimmed := strings.TrimSpace(lines[first]); {
	case trimmed == "---":
		start = first
	case strings.HasPrefix(trimmed, "# "):
		next := firstNonEmptyLine(lines, first+1)
		if next >= 0 && strings.TrimSpace(lines[next]) == "---" {
			start = next
		}
	}
	if start < 0 {
		return -1, -1
	}

	for i := start + 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return start, i
		}
	}
	return -1, -1
}

func parseHeadingMetadata(lines []string, doc *SpecDocument) {
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "# ") {
			continue
		}
		titleLine := strings.TrimSpace(strings.TrimPrefix(trimmed, "# "))
		if m := specTitleRe.FindStringSubmatch(titleLine); len(m) >= 2 {
			doc.ID = m[1]
			if len(m) >= 3 {
				doc.Title = strings.TrimSpace(m[2])
			}
			return
		}
		if m := legacyHeadingRe.FindStringSubmatch(titleLine); len(m) == 2 {
			doc.Title = strings.TrimSpace(m[1])
			return
		}
	}
}

func parseLegacyField(lines []string, key string) string {
	for _, line := range lines {
		label, value, ok := splitLegacyField(line)
		if !ok || label != strings.ToLower(key) {
			continue
		}
		if key == "status" {
			fields := strings.Fields(value)
			if len(fields) == 0 {
				return ""
			}
			return fields[0]
		}
		return value
	}
	return ""
}

func legacyFieldLineIndex(lines []string, key string) int {
	for i, line := range lines {
		label, _, ok := splitLegacyField(line)
		if ok && label == strings.ToLower(key) {
			return i
		}
	}
	return -1
}

func splitLegacyField(line string) (label, value string, ok bool) {
	trimmed := strings.TrimSpace(line)
	matches := legacyFieldLabel.FindStringSubmatch(trimmed)
	if len(matches) != 3 {
		return "", "", false
	}
	label = strings.ToLower(strings.TrimSpace(matches[1]))
	value = strings.TrimSpace(matches[2])
	return label, value, value != ""
}

func firstHeadingLineIndex(lines []string) int {
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "# ") {
			return i
		}
	}
	return -1
}

func firstNonEmptyLine(lines []string, start int) int {
	for i := start; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) != "" {
			return i
		}
	}
	return -1
}

func lineIndent(line string) string {
	return line[:len(line)-len(strings.TrimLeft(line, " \t"))]
}
