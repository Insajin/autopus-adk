package content

import "github.com/insajin/autopus-adk/pkg/adapter"

// generateCompletionHooks returns the platform-specific completion hook that
// signals the orchestra hook-IPC collector when the agent session ends.
// The scripts are installed to .claude/hooks/autopus/ by the claude adapter;
// that Command path is shared across all platforms that reference them.
// Platforms without a known completion hook (e.g. opencode) return nil.
func generateCompletionHooks(platform string) []adapter.HookConfig {
	type entry struct {
		event   string
		command string
	}

	var e entry
	switch platform {
	case "claude", "claude-code":
		e = entry{"Stop", ".claude/hooks/autopus/hook-claude-stop.sh"}
	case "antigravity-cli", "gemini", "gemini-cli":
		// AfterAgent is the Antigravity CLI event fired when the agent session ends.
		e = entry{"AfterAgent", ".claude/hooks/autopus/hook-gemini-afteragent.sh"}
	case "codex":
		e = entry{"Stop", ".claude/hooks/autopus/hook-codex-stop.sh"}
	default:
		return nil
	}

	return []adapter.HookConfig{{
		Event:   e.event,
		Matcher: "",
		Type:    "command",
		Command: e.command,
	}}
}
