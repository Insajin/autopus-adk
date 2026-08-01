// Package content_test는 조건부 룰 디스패처 등록 파생의 테스트이다.
package content_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/insajin/autopus-adk/pkg/content"
	"github.com/insajin/autopus-adk/pkg/rulecond"
)

// parseSyntheticRule builds a rule from inline frontmatter, so a scope the
// shipped rule set does not yet use can still be exercised.
func parseSyntheticRule(t *testing.T, filename, frontmatter string) *rulecond.Rule {
	t.Helper()
	raw := "---\n" + frontmatter + "---\n\n# Body\n\nRule body text.\n"
	rule, err := rulecond.ParseRule(filename, []byte(raw))
	require.NoError(t, err)
	require.NoError(t, rulecond.Validate(rule))
	return rule
}

func dispatcherEntries(hooks []adapter.HookConfig) []adapter.HookConfig {
	out := make([]adapter.HookConfig, 0, len(hooks))
	for _, hook := range hooks {
		if hook.Command == "auto rules fire --event PreToolUse" {
			out = append(out, hook)
		}
	}
	return out
}

// TestConditionalDispatcherHooks_EditScopeRegistersEditMatcher is the wiring
// regression. Registration used to be a hardcoded PreToolUse/Bash entry, so a
// rule scoped to a file-editing tool had its body relocated out of baseline
// context and then never fired: no Edit matcher was ever registered.
func TestConditionalDispatcherHooks_EditScopeRegistersEditMatcher(t *testing.T) {
	t.Parallel()

	rules := []*rulecond.Rule{
		parseSyntheticRule(t, "edit-scoped.md",
			"name: edit-scoped\ncondition: 'TODO'\nscope: tool:edit(*.go)\n"),
	}

	hooks, err := content.ConditionalDispatcherHooks(rules)
	require.NoError(t, err)
	require.Len(t, hooks, 1, "one entry per distinct event and matcher pair")

	assert.Equal(t, rulecond.EventPreToolUse, hooks[0].Event)
	assert.Equal(t, rulecond.MatcherEdit, hooks[0].Matcher,
		"an edit-scoped rule must register the file-editing matcher, not Bash")
	assert.Equal(t, "auto rules fire --event PreToolUse", hooks[0].Command)
	assert.Positive(t, hooks[0].Timeout, "Claude Code's settings schema rejects a zero timeout")
}

// TestConditionalDispatcherHooks_DistinctPairsEachRegister covers a mixed rule
// set: Bash and Edit scopes are separate pairs, and rules sharing a pair
// collapse to one entry.
func TestConditionalDispatcherHooks_DistinctPairsEachRegister(t *testing.T) {
	t.Parallel()

	rules := []*rulecond.Rule{
		parseSyntheticRule(t, "bash-one.md", "name: bash-one\ncondition: 'git commit'\nscope: tool:bash\n"),
		parseSyntheticRule(t, "bash-two.md", "name: bash-two\ncondition: 'git gc'\nscope: tool:bash\n"),
		parseSyntheticRule(t, "edit-one.md", "name: edit-one\ncondition: 'TODO'\nscope: tool:edit(*.go)\n"),
		parseSyntheticRule(t, "always.md", "name: always\ndescription: no trigger\n"),
	}

	hooks, err := content.ConditionalDispatcherHooks(rules)
	require.NoError(t, err)

	matchers := make([]string, 0, len(hooks))
	for _, hook := range hooks {
		matchers = append(matchers, hook.Matcher)
	}
	assert.ElementsMatch(t, []string{rulecond.MatcherBash, rulecond.MatcherEdit}, matchers,
		"two Bash rules collapse to one entry; the edit-scoped rule adds its own")
}

// TestConditionalDispatcherHooks_NoTriggeredRulesRegistersNothing keeps the
// derivation honest: an untriggered rule set must not register a dispatcher.
func TestConditionalDispatcherHooks_NoTriggeredRulesRegistersNothing(t *testing.T) {
	t.Parallel()

	rules := []*rulecond.Rule{
		parseSyntheticRule(t, "always.md", "name: always\ndescription: no trigger\n"),
	}

	hooks, err := content.ConditionalDispatcherHooks(rules)
	require.NoError(t, err)
	assert.Empty(t, hooks)
}

// TestGenerateProjectHookConfigs_ShippedRulesRegisterSingleBashDispatcher pins
// the generated surface for today's rule set. The derivation must reproduce the
// exact entry the previous hardcoded registration produced, so settings.json
// stays byte-identical for the current three hook-fired rules.
func TestGenerateProjectHookConfigs_ShippedRulesRegisterSingleBashDispatcher(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultFullConfig("dispatcher-wiring")
	hooks, _, err := content.GenerateProjectHookConfigs(cfg, "claude-code", true)
	require.NoError(t, err)

	entries := dispatcherEntries(hooks)
	require.Len(t, entries, 1, "the three shipped tool:bash rules collapse to one entry")
	assert.Equal(t, adapter.HookConfig{
		Event:   "PreToolUse",
		Matcher: "Bash",
		Type:    "command",
		Command: "auto rules fire --event PreToolUse",
		Timeout: 10,
	}, entries[0])
}

// TestGenerateProjectHookConfigs_NonClaudePlatformsSkipDispatcher keeps the
// dispatcher off platforms that cannot consume additionalContext.
func TestGenerateProjectHookConfigs_NonClaudePlatformsSkipDispatcher(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultFullConfig("dispatcher-wiring")
	for _, platform := range []string{"codex", "opencode", "antigravity-cli", "omp"} {
		hooks, _, err := content.GenerateProjectHookConfigs(cfg, platform, true)
		require.NoError(t, err)
		assert.Empty(t, dispatcherEntries(hooks), "%s must not register the dispatcher", platform)
	}
}
