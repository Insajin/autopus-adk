// Package content_test verifies completion hook registration for the orchestra hook-IPC path.
// S1: claude-code platform emits a Stop event hook pointing to hook-claude-stop.sh.
// S2: antigravity-cli platform emits AfterAgent, codex platform emits Stop.
package content_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/insajin/autopus-adk/pkg/content"
)

// findHook returns the first HookConfig whose Event matches, or nil.
func findHook(hooks []adapter.HookConfig, event string) *adapter.HookConfig {
	for i := range hooks {
		if hooks[i].Event == event {
			return &hooks[i]
		}
	}
	return nil
}

// TestGenerateCLIHooks_StopEvent verifies S1 and S2 oracle values for completion
// hooks across claude-code, antigravity-cli, and codex platforms.
func TestGenerateCLIHooks_StopEvent(t *testing.T) {
	t.Parallel()

	// Empty HooksConf — completion hooks must be unconditional.
	cfg := config.DefaultFullConfig("demo")
	cfg.Hooks = config.HooksConf{}
	cfg.Features.CC21 = config.CC21FeaturesConf{}

	type wantHook struct {
		event   string
		command string
		typ     string
	}

	tests := []struct {
		platform string
		want     wantHook
	}{
		// S1: claude-code Stop hook
		{
			platform: "claude-code",
			want: wantHook{
				event:   "Stop",
				command: ".claude/hooks/autopus/hook-claude-stop.sh",
				typ:     "command",
			},
		},
		// S2a: antigravity-cli AfterAgent hook
		{
			platform: "antigravity-cli",
			want: wantHook{
				event:   "AfterAgent",
				command: ".claude/hooks/autopus/hook-gemini-afteragent.sh",
				typ:     "command",
			},
		},
		// S2b: codex Stop hook
		{
			platform: "codex",
			want: wantHook{
				event:   "Stop",
				command: ".claude/hooks/autopus/hook-codex-stop.sh",
				typ:     "command",
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.platform, func(t *testing.T) {
			t.Parallel()

			hooks, _, err := content.GenerateProjectHookConfigs(cfg, tc.platform, true)
			require.NoError(t, err)

			h := findHook(hooks, tc.want.event)
			require.NotNil(t, h,
				"platform %q: expected a HookConfig with Event=%q but none was found; all events: %v",
				tc.platform, tc.want.event, eventNames(hooks))

			assert.Equal(t, tc.want.command, h.Command,
				"platform %q: wrong Command for Event=%q", tc.platform, tc.want.event)
			assert.Equal(t, tc.want.typ, h.Type,
				"platform %q: wrong Type for Event=%q", tc.platform, tc.want.event)
			assert.Equal(t, "", h.Matcher,
				"platform %q: completion hook must have empty Matcher", tc.platform)
		})
	}
}

// TestGenerateCLIHooks_StopEvent_NoRegression verifies that platforms not listed
// above (opencode, antigravity-cli legacy "gemini" alias) do not unexpectedly
// acquire a Stop or AfterAgent hook.
func TestGenerateCLIHooks_CompletionHook_NotAddedForOtherPlatforms(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultFullConfig("demo")
	cfg.Hooks = config.HooksConf{}
	cfg.Features.CC21 = config.CC21FeaturesConf{}

	// opencode does not have a shell completion hook registered via generateCLIHooks.
	hooks, _, err := content.GenerateProjectHookConfigs(cfg, "opencode", true)
	require.NoError(t, err)

	assert.Nil(t, findHook(hooks, "Stop"),
		"opencode should not receive a Stop hook via generateCLIHooks")
	assert.Nil(t, findHook(hooks, "AfterAgent"),
		"opencode should not receive an AfterAgent hook via generateCLIHooks")
}

func eventNames(hooks []adapter.HookConfig) []string {
	names := make([]string, len(hooks))
	for i, h := range hooks {
		names[i] = h.Event
	}
	return names
}
