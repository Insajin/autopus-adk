package orchestra

import "strings"

// providerArtifactIdentity returns the stable provider name used by generated
// hook scripts. Custom provider identities remain unchanged.
func providerArtifactIdentity(provider string) string {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "claude", "claude-code":
		return "claude"
	case "gemini", "antigravity", "antigravity-cli", "gemini-cli", "agy":
		return "gemini"
	case "codex":
		return "codex"
	case "opencode":
		return "opencode"
	default:
		return provider
	}
}
