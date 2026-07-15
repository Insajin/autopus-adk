package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCodexProviderEntryForQuality_StructuredUsageOnlyOnSubprocess(t *testing.T) {
	t.Parallel()

	for _, mode := range []string{"balanced", "ultra"} {
		entry := CodexProviderEntryForQuality(QualityConf{Default: mode})
		assert.Equal(t, 1, countString(entry.Args, "--json"), "%s subprocess args", mode)
		assert.NotContains(t, entry.PaneArgs, "--json", "%s pane args", mode)
	}
}

func TestApplyCodexProviderProfile_PreservesStructuredAndCustomArgs(t *testing.T) {
	t.Parallel()
	entry := ProviderEntry{
		Args:     []string{"exec", "--json", "--custom-flag", "custom-value", "-m", "old-model"},
		PaneArgs: []string{"--custom-pane", "pane-value", "-m", "old-model"},
	}

	got := ApplyCodexProviderProfile(entry, CodexProfile{Model: "new-model", Effort: "high"})

	assert.Equal(t, 1, countString(got.Args, "--json"))
	assert.Contains(t, got.Args, "--custom-flag")
	assert.Contains(t, got.Args, "custom-value")
	assert.NotContains(t, got.PaneArgs, "--json")
	assert.Contains(t, got.PaneArgs, "--custom-pane")
	assert.Contains(t, got.PaneArgs, "pane-value")
}

func TestApplyCodexProviderProfile_NormalizesStructuredUsageExactlyOnce(t *testing.T) {
	t.Parallel()
	entry := ProviderEntry{
		ModelPolicy: ProviderModelPolicyQuality,
		Args:        []string{"exec", "--json", "--custom", "--json"},
		PaneArgs:    []string{"--json", "--search", "--json"},
	}

	got := ApplyCodexProviderProfile(entry, CodexProfile{Model: "gpt-5.4", Effort: "high"})

	assert.Equal(t, 1, countString(got.Args, "--json"))
	assert.Equal(t, []string{"exec", "--json", "--custom", "-m", "gpt-5.4", "-c", `model_reasoning_effort="high"`}, got.Args)
	assert.Equal(t, []string{"--search", "-m", "gpt-5.4", "-c", `model_reasoning_effort="high"`}, got.PaneArgs)
}

func TestApplyCodexProviderProfile_AddsStructuredUsageBeforeTerminator(t *testing.T) {
	t.Parallel()
	entry := ProviderEntry{Args: []string{"exec", "--sandbox", "workspace-write", "--", "child", "--json"}}

	got := ApplyCodexProviderProfile(entry, CodexProfile{})

	assert.Equal(t, []string{"exec", "--json", "--sandbox", "workspace-write", "--", "child", "--json"}, got.Args)
}

func countString(values []string, target string) int {
	count := 0
	for _, value := range values {
		if value == target {
			count++
		}
	}
	return count
}
