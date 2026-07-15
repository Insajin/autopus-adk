package promptlayer

import (
	"fmt"
	"path/filepath"
	"strings"
)

// VerifyContextSpecIdentity binds spec.md metadata to its containing SPEC
// directory so a valid file from another SPEC cannot be replayed.
func VerifyContextSpecIdentity(specDir string, raw []byte) error {
	expected := filepath.Base(filepath.Clean(filepath.FromSlash(specDir)))
	if expected == "." || expected == "" || !strings.HasPrefix(expected, "SPEC-") {
		return fmt.Errorf("invalid SPEC directory identity: %s", specDir)
	}
	identities := contextSpecIdentities(string(raw))
	if len(identities) == 0 {
		return fmt.Errorf("required context spec.md has no SPEC identity for directory %s", expected)
	}
	for _, observed := range identities {
		if observed != expected {
			return fmt.Errorf("wrong-SPEC context: directory %s contains identity %s", expected, observed)
		}
	}
	return nil
}

func contextSpecIdentities(content string) []string {
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	identities := make([]string, 0, 3)
	seen := make(map[string]bool, 3)
	frontmatterStarted, inFrontmatter, frontmatterDone := false, false, false
	headingFound := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "---" && !frontmatterDone {
			if !frontmatterStarted {
				frontmatterStarted, inFrontmatter = true, true
			} else if inFrontmatter {
				inFrontmatter, frontmatterDone = false, true
			}
			continue
		}
		if inFrontmatter && strings.HasPrefix(strings.ToLower(trimmed), "id:") {
			appendContextSpecIdentity(&identities, seen, strings.TrimSpace(trimmed[strings.IndexByte(trimmed, ':')+1:]))
		}
		if !headingFound && strings.HasPrefix(trimmed, "# ") {
			if identity := contextSpecHeadingIdentity(strings.TrimSpace(strings.TrimPrefix(trimmed, "# "))); identity != "" {
				appendContextSpecIdentity(&identities, seen, identity)
				headingFound = true
			}
		}
		lower := strings.ToLower(trimmed)
		for _, prefix := range []string{"**spec-id**:", "**spec id**:"} {
			if strings.HasPrefix(lower, prefix) {
				appendContextSpecIdentity(&identities, seen, strings.TrimSpace(trimmed[len(prefix):]))
			}
		}
	}
	return identities
}

func contextSpecHeadingIdentity(title string) string {
	if index := strings.IndexByte(title, ':'); index >= 0 {
		title = title[:index]
	}
	fields := strings.Fields(title)
	if len(fields) == 0 || !strings.HasPrefix(fields[0], "SPEC-") {
		return ""
	}
	return fields[0]
}

func appendContextSpecIdentity(identities *[]string, seen map[string]bool, value string) {
	fields := strings.Fields(value)
	if len(fields) == 0 {
		return
	}
	value = strings.Trim(fields[0], `"'`)
	if value != "" && !seen[value] {
		seen[value] = true
		*identities = append(*identities, value)
	}
}
