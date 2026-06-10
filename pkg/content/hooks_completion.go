package content

import "github.com/insajin/autopus-adk/pkg/adapter"

// generateCompletionHooks returns the platform-specific orchestra hook-IPC hooks:
// a completion hook that signals when the agent session ends, and (for claude) a
// SessionStart hook that signals when the session is ready to receive a prompt
// (SPEC-ORCH-022). The scripts are installed to .claude/hooks/autopus/ by the
// claude adapter; that Command path is shared across platforms that reference them.
// Platforms without a known completion hook (e.g. opencode) return nil.
func generateCompletionHooks(platform string) []adapter.HookConfig {
	type entry struct {
		event   string
		command string
	}

	var completion entry
	// readyEvent is the SessionStart-equivalent event whose hook writes the
	// ready signal. Empty when the platform's session-start event is not known;
	// those providers fall back to screen-scrape readiness (waitForPaneReady).
	var ready entry
	switch platform {
	case "claude", "claude-code":
		completion = entry{"Stop", ".claude/hooks/autopus/hook-claude-stop.sh"}
		ready = entry{"SessionStart", ".claude/hooks/autopus/hook-claude-sessionstart.sh"}
	case "antigravity-cli", "gemini", "gemini-cli":
		// AfterAgent is the Antigravity CLI event fired when the agent session ends.
		completion = entry{"AfterAgent", ".claude/hooks/autopus/hook-gemini-afteragent.sh"}
	case "codex":
		completion = entry{"Stop", ".claude/hooks/autopus/hook-codex-stop.sh"}
	default:
		return nil
	}

	hooks := []adapter.HookConfig{{
		Event:   completion.event,
		Matcher: "",
		Type:    "command",
		Command: completion.command,
	}}
	if ready.event != "" {
		hooks = append(hooks, adapter.HookConfig{
			Event:   ready.event,
			Matcher: "",
			Type:    "command",
			Command: ready.command,
		})
	}
	return hooks
}
