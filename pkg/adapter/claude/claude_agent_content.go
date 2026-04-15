package claude

import pkgcontent "github.com/insajin/autopus-adk/pkg/content"

func normalizeClaudeContent(subDir string, data []byte) []byte {
	if subDir != "agents" {
		return data
	}
	return []byte(pkgcontent.NormalizeAgentReferences(string(data), "claude-code"))
}
