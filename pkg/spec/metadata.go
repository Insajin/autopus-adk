package spec

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const defaultSpecStatus = "draft"

var specTitleRe = regexp.MustCompile(`^(SPEC-[\w-]+)(?::\s*(.+))?$`)

// ParseSpecMetadata extracts top-level SPEC metadata from spec.md content.
func ParseSpecMetadata(content string) SpecDocument {
	doc := SpecDocument{
		Status:  defaultSpecStatus,
		Version: "0.1.0",
	}

	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "# ") {
			continue
		}
		titleLine := strings.TrimPrefix(trimmed, "# ")
		if m := specTitleRe.FindStringSubmatch(titleLine); len(m) >= 2 {
			doc.ID = m[1]
			if len(m) >= 3 {
				doc.Title = strings.TrimSpace(m[2])
			}
			break
		}
	}

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
	if v := frontmatter["status"]; v != "" {
		doc.Status = strings.ToLower(v)
	} else if v := parseLegacyStatus(lines); v != "" {
		doc.Status = strings.ToLower(v)
	}

	return doc
}

// UpdateStatus rewrites the spec.md frontmatter status field.
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
	if start < 0 || end <= start {
		return "", fmt.Errorf("spec.md frontmatter not found")
	}

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

	return strings.Join(lines, "\n"), nil
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
	start := -1
	for i, line := range lines {
		if strings.TrimSpace(line) != "---" {
			continue
		}
		if start == -1 {
			start = i
			continue
		}
		return start, i
	}
	return -1, -1
}

func parseLegacyStatus(lines []string) string {
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		lower := strings.ToLower(trimmed)
		if !strings.HasPrefix(lower, "**status**") {
			continue
		}
		parts := strings.SplitN(trimmed, ":", 2)
		if len(parts) != 2 {
			continue
		}
		fields := strings.Fields(strings.TrimSpace(parts[1]))
		if len(fields) == 0 {
			continue
		}
		return fields[0]
	}
	return ""
}
