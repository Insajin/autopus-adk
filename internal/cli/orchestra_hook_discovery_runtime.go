package cli

import (
	"path/filepath"
	"strings"
)

// hookRuntimeFamily resolves the hook configuration surface that the selected
// executable actually consumes. Binary wins over a generic provider name so a
// provider named "gemini" can correctly select either agy or the legacy CLI.
func hookRuntimeFamily(name, binary string) string {
	if runtime := knownHookRuntime(binaryIdentity(binary)); runtime != "" {
		return runtime
	}
	if runtime := knownHookRuntime(binaryIdentity(name)); runtime != "" {
		return runtime
	}
	return strings.ToLower(strings.TrimSpace(name))
}

func binaryIdentity(value string) string {
	value = strings.ReplaceAll(strings.TrimSpace(value), `\`, "/")
	value = strings.TrimSuffix(strings.ToLower(filepath.Base(value)), ".exe")
	return value
}

func knownHookRuntime(identity string) string {
	switch identity {
	case "claude", "claude-code":
		return "claude"
	case "codex":
		return "codex"
	case "agy", "antigravity", "antigravity-cli":
		return "antigravity"
	case "gemini", "gemini-cli":
		return "gemini"
	case "opencode":
		return "opencode"
	default:
		return ""
	}
}
