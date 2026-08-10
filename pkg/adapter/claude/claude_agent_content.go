package claude

import (
	"strings"

	"github.com/insajin/autopus-adk/pkg/config"
	pkgcontent "github.com/insajin/autopus-adk/pkg/content"
)

// normalizeClaudeContent projects one embedded content file onto its installed
// Claude form. Only agents are rewritten: references are retargeted at
// claude-code, and the frontmatter `model:` tier is resolved from the active
// quality preset so Quality.Presets stays the single tier source.
func normalizeClaudeContent(cfg *config.HarnessConfig, subDir, filename string, data []byte) []byte {
	if subDir != "agents" {
		return data
	}
	normalized := pkgcontent.NormalizeAgentReferences(string(data), "claude-code")
	return []byte(applyClaudeAgentTier(cfg, filename, normalized))
}

// applyClaudeAgentTier rewrites the frontmatter `model:` value in place, so the
// surrounding keys keep their source order. The shipped value is the fallback
// tier, which leaves an agent that no preset mentions exactly as authored.
// Documents without frontmatter or without a `model:` key pass through
// untouched — the key is never inserted.
func applyClaudeAgentTier(cfg *config.HarnessConfig, filename, content string) string {
	if cfg == nil {
		return content
	}
	lines := strings.Split(content, "\n")
	if len(lines) == 0 || lines[0] != "---" {
		return content
	}
	for i := 1; i < len(lines); i++ {
		if lines[i] == "---" {
			return content
		}
		value, isModel := strings.CutPrefix(lines[i], "model:")
		if !isModel {
			continue
		}
		current := strings.TrimSpace(value)
		agent := config.NormalizeAgentName(strings.TrimSuffix(filename, ".md"))
		tier := cfg.Quality.AgentTier(config.QualityProviderClaude, agent, current)
		if tier == current {
			return content
		}
		lines[i] = "model: " + tier
		return strings.Join(lines, "\n")
	}
	return content
}
