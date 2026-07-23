package content

import (
	"path"
	"strings"
)

// IsGeneratedTemplatePath reports whether GenerateAllTemplates owns relPath.
// Static command, prompt, rule, shared, auto-* route, and platform-native team
// templates are excluded so reverse drift checks can flag obsolete generated
// residue without treating hand-authored templates as deleted-source artifacts.
func IsGeneratedTemplatePath(relPath string) bool {
	clean := path.Clean(strings.ReplaceAll(relPath, "\\", "/"))
	if clean == "." || path.IsAbs(clean) || strings.HasPrefix(clean, "../") {
		return false
	}
	parts := strings.Split(clean, "/")
	if len(parts) == 3 && parts[0] == "codex" && parts[1] == "agents" {
		return hasNamedSuffix(parts[2], ".toml.tmpl")
	}
	if len(parts) == 3 && parts[0] == "gemini" && parts[1] == "agents" {
		return hasNamedSuffix(parts[2], ".md.tmpl")
	}
	if len(parts) == 3 && parts[0] == "codex" && parts[1] == "skills" {
		return parts[2] != "agent-teams.md.tmpl" &&
			!strings.HasPrefix(parts[2], "auto-") && hasNamedSuffix(parts[2], ".md.tmpl")
	}
	if len(parts) == 4 && parts[0] == "gemini" && parts[1] == "skills" {
		return parts[2] != "" && parts[2] != "agent-teams" &&
			!strings.HasPrefix(parts[2], "auto-") && parts[3] == "SKILL.md.tmpl"
	}
	if len(parts) == 3 && parts[0] == "claude" && parts[1] == "workflows" {
		return hasNamedSuffix(parts[2], ".workflow.js.tmpl")
	}
	return false
}

func hasNamedSuffix(name, suffix string) bool {
	return len(name) > len(suffix) && strings.HasSuffix(name, suffix)
}
