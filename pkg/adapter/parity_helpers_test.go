package adapter_test

import (
	"path/filepath"
	"strings"

	pkgcontent "github.com/insajin/autopus-adk/pkg/content"
)

// platformRuleExclusions documents intended per-platform rule gaps.
// Empty after PARITY-002 closes the Gemini rule gap.
var platformRuleExclusions = map[string]map[string]bool{
	"claude":   {},
	"codex":    {},
	"gemini":   {},
	"opencode": {},
}

// platformSkillExclusions documents intended per-platform skill gaps.
var platformSkillExclusions = map[string]map[string]bool{
	"claude":   {},
	"codex":    {},
	"gemini":   {},
	"opencode": {},
}

// expectedPlatformValues maps an adapter to the platform frontmatter values it
// may legitimately emit.
var expectedPlatformValues = map[string][]string{
	"claude":   {"claude", "claude-code"},
	"codex":    {"codex"},
	"gemini":   {"antigravity-cli"},
	"opencode": {"opencode"},
}

type CoverageFinding struct {
	Platform string
	Item     string
	Type     string // "rule", "skill", "platform-value"
	Message  string
}

func isSkillCompatible(skill pkgcontent.CatalogSkill, platform string) bool {
	if len(skill.CompileTargets) == 0 {
		return true
	}
	for _, p := range skill.CompileTargets {
		if p == platform || (p == "claude" && platform == "claude-code") {
			return true
		}
	}
	return false
}

func extractRuleName(targetPath string) string {
	base := filepath.Base(targetPath)
	if strings.HasPrefix(base, "rules-autopus-") {
		return strings.TrimPrefix(base, "rules-autopus-")
	}
	return base
}

func parsePlatformFromFrontmatter(content string) (string, bool) {
	if !strings.HasPrefix(content, "---\n") {
		return "", false
	}
	rest := strings.TrimPrefix(content, "---\n")
	idx := strings.Index(rest, "\n---\n")
	if idx < 0 {
		return "", false
	}
	frontmatter := rest[:idx]
	lines := strings.Split(frontmatter, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "platform:") {
			val := strings.TrimSpace(strings.TrimPrefix(line, "platform:"))
			return val, true
		}
	}
	return "", false
}

func parseSkillNameFromContent(content string) string {
	if !strings.HasPrefix(content, "---\n") {
		return ""
	}
	rest := strings.TrimPrefix(content, "---\n")
	idx := strings.Index(rest, "\n---\n")
	if idx < 0 {
		return ""
	}
	frontmatter := rest[:idx]
	lines := strings.Split(frontmatter, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "name:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "name:"))
		}
	}
	return ""
}

func extractSkillNameFromPath(targetPath string) string {
	targetPath = filepath.ToSlash(targetPath)
	parts := strings.Split(targetPath, "/")
	for i, part := range parts {
		if part == "skills" && i+1 < len(parts) {
			next := parts[i+1]
			if next == "autopus" && i+2 < len(parts) {
				return parts[i+2]
			}
			return strings.TrimSuffix(next, ".md")
		}
	}
	return ""
}
