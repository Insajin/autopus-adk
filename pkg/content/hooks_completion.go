package content

import "github.com/insajin/autopus-adk/pkg/adapter"

const (
	// completionHookTimeoutSeconds bounds the Stop/AfterAgent completion hook.
	// The hook script (hook-*-stop.sh / hook-gemini-afteragent.sh) runs a
	// bidirectional-IPC wait loop of MAX_WAIT=600 iterations at ~200ms each
	// (~120s nominal, plus per-iteration python spawn overhead) waiting for the
	// next round's input (SPEC-ORCH-017). The timeout must comfortably exceed
	// 120s AND the Claude Code default (60s); otherwise Claude kills the hook
	// mid-round and the orchestrator's next-round input is lost. It must be a
	// positive value — a zero/omitted timeout serializes as "timeout": 0, which
	// Claude Code's settings schema rejects (surfaced by `/doctor`).
	completionHookTimeoutSeconds = 300
	// readyHookTimeoutSeconds bounds the SessionStart ready hook, which only
	// writes a single ready-signal file and exits. Generous bound that stays
	// above the Go-side fileIPCReadyTimeout (30s).
	readyHookTimeoutSeconds = 60
)

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
		Timeout: completionHookTimeoutSeconds,
	}}
	if ready.event != "" {
		hooks = append(hooks, adapter.HookConfig{
			Event:   ready.event,
			Matcher: "",
			Type:    "command",
			Command: ready.command,
			Timeout: readyHookTimeoutSeconds,
		})
	}
	return hooks
}
