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

// conditionalBodySegment is the claude-code conditional rule body root.
// SPEC-CONDRULE-001 relocates hook-fired rule bodies out of baseline context
// into this directory; REQ-CONDRULE-VERIFY-02 keeps them counted as rules.
const conditionalBodySegment = "hooks/autopus/conditional/"

// isRuleTargetPath reports whether a generated target path holds a managed
// rule. classifyFile, runCoverageGate, and generatePlatformRules all route
// through it so the heuristic cannot drift into three divergent copies.
//
// A conditional body under conditionalBodySegment counts; the compiled
// manifest .claude/hooks/autopus/conditional-rules.json does not, because that
// filename has no conditional/ segment.
func isRuleTargetPath(targetPath string) bool {
	p := strings.ToLower(targetPath)
	return strings.Contains(p, "rules/") ||
		strings.Contains(p, `rules\`) ||
		strings.Contains(p, "rules-autopus") ||
		strings.Contains(p, conditionalBodySegment) ||
		strings.Contains(p, `hooks\autopus\conditional\`)
}

// extractRuleName resolves the source rule filename a generated path maps to.
// Conditional bodies keep their source basename, so relocation leaves the name
// unchanged and platformRuleExclusions["claude"] can stay empty.
func extractRuleName(targetPath string) string {
	base := filepath.Base(targetPath)
	if strings.HasPrefix(base, "rules-autopus-") {
		return strings.TrimPrefix(base, "rules-autopus-")
	}
	return base
}

// frontmatterKeySet returns the top-level frontmatter keys of a rule file.
// A document without frontmatter yields an empty set rather than failing, since
// several managed rules ship no frontmatter at all.
func frontmatterKeySet(content string) map[string]bool {
	keys := map[string]bool{}
	if !strings.HasPrefix(content, "---\n") {
		return keys
	}
	rest := strings.TrimPrefix(content, "---\n")
	idx := strings.Index(rest, "\n---\n")
	if idx < 0 {
		return keys
	}
	for _, line := range strings.Split(rest[:idx], "\n") {
		if line == "" || strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			continue
		}
		if key, _, ok := strings.Cut(line, ":"); ok {
			keys[key] = true
		}
	}
	return keys
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
