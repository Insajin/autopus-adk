package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestProviderEntryModelPolicyYAMLRoundTrip(t *testing.T) {
	t.Parallel()

	want := ProviderEntry{
		Binary:      "codex",
		Args:        []string{"exec", "--json"},
		PaneArgs:    []string{"--search"},
		ModelPolicy: ProviderModelPolicyQuality,
	}
	data, err := yaml.Marshal(want)
	require.NoError(t, err)
	assert.Contains(t, string(data), "model_policy: quality")

	var got ProviderEntry
	require.NoError(t, yaml.Unmarshal(data, &got))
	assert.Equal(t, want, got)
}

func TestHarnessConfigValidateRejectsUnknownProviderModelPolicy(t *testing.T) {
	t.Parallel()

	cfg := DefaultFullConfig("invalid-model-policy")
	entry := cfg.Orchestra.Providers["codex"]
	entry.ModelPolicy = "automatic"
	cfg.Orchestra.Providers["codex"] = entry

	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "model_policy")
	assert.Contains(t, err.Error(), "quality or pinned")
}

func TestApplyCodexProviderProfilePreservesNonModelArguments(t *testing.T) {
	t.Parallel()

	entry := ProviderEntry{
		Binary:      "codex",
		ModelPolicy: ProviderModelPolicyQuality,
		Args:        []string{"exec", "--json", "-c", `foo="bar"`, "-m", CodexLegacyModel, "-c", `model_reasoning_effort="xhigh"`, "--sandbox", "workspace-write"},
		PaneArgs:    []string{"--search", "--model=" + CodexLegacyModel, "-c", `model_reasoning_effort="medium"`},
	}

	got := ApplyCodexProviderProfile(entry, CodexProfile{Model: CodexSolModel, Effort: CodexEffortUltra})
	assert.Equal(t, []string{"exec", "--json", "-c", `foo="bar"`, "-m", CodexSolModel, "-c", `model_reasoning_effort="ultra"`, "--sandbox", "workspace-write"}, got.Args)
	assert.Equal(t, []string{"--search", "--model=" + CodexSolModel, "-c", `model_reasoning_effort="ultra"`}, got.PaneArgs)
}

func TestApplyCodexProviderProfileOmitsManagedFieldsForRuntimeDefault(t *testing.T) {
	t.Parallel()

	entry := ProviderEntry{
		Binary:      "codex",
		ModelPolicy: ProviderModelPolicyQuality,
		Args:        []string{"exec", "--sandbox", "workspace-write", "-m", CodexSolModel, "-c", `model_reasoning_effort="ultra"`, "-c", `foo="bar"`},
		PaneArgs:    []string{"--model=" + CodexSolModel, "-c", `model_reasoning_effort="ultra"`, "--search"},
	}

	got := ApplyCodexProviderProfile(entry, CodexProfile{})
	assert.Equal(t, []string{"exec", "--sandbox", "workspace-write", "-c", `foo="bar"`}, got.Args)
	assert.Equal(t, []string{"--search"}, got.PaneArgs)
}

func TestApplyCodexProviderProfilePreservesTerminatorSuffix(t *testing.T) {
	t.Parallel()

	entry := ProviderEntry{
		Binary:      "codex",
		ModelPolicy: ProviderModelPolicyQuality,
		Args: []string{
			"exec", "--model=" + CodexLegacyModel,
			"--", "child", "-m", "child-model", "--config=model_reasoning_effort=low",
		},
		PaneArgs: []string{
			"--", "child", "--model=child-model", "-c", `model_reasoning_effort="low"`,
		},
	}

	got := ApplyCodexProviderProfile(entry, CodexProfile{Model: CodexSolModel, Effort: CodexEffortUltra})
	assert.Equal(t, []string{
		"exec", "--model=" + CodexSolModel, "-c", `model_reasoning_effort="ultra"`,
		"--", "child", "-m", "child-model", "--config=model_reasoning_effort=low",
	}, got.Args)
	assert.Equal(t, []string{
		"-m", CodexSolModel, "-c", `model_reasoning_effort="ultra"`,
		"--", "child", "--model=child-model", "-c", `model_reasoning_effort="low"`,
	}, got.PaneArgs)

	runtimeDefault := ApplyCodexProviderProfile(entry, CodexProfile{})
	assert.Equal(t, []string{
		"exec", "--", "child", "-m", "child-model", "--config=model_reasoning_effort=low",
	}, runtimeDefault.Args)
	assert.Equal(t, []string{
		"--", "child", "--model=child-model", "-c", `model_reasoning_effort="low"`,
	}, runtimeDefault.PaneArgs)
}

func TestApplyCodexProviderProfileHandlesLongConfigOption(t *testing.T) {
	t.Parallel()

	entry := ProviderEntry{
		Binary:      "codex",
		ModelPolicy: ProviderModelPolicyQuality,
		Args: []string{
			"exec", "--model=" + CodexLegacyModel,
			`--config=model_reasoning_effort="xhigh"`, "--config=foo=bar",
		},
		PaneArgs: []string{"--config=model_reasoning_effort=xhigh"},
	}

	got := ApplyCodexProviderProfile(entry, CodexProfile{Model: CodexSolModel, Effort: CodexEffortUltra})
	assert.Equal(t, []string{
		"exec", "--model=" + CodexSolModel,
		`--config=model_reasoning_effort="ultra"`, "--config=foo=bar",
	}, got.Args)
	assert.Equal(t, []string{"--config=model_reasoning_effort=\"ultra\"", "-m", CodexSolModel}, got.PaneArgs)

	got = ApplyCodexProviderProfile(entry, CodexProfile{})
	assert.Equal(t, []string{"exec", "--config=foo=bar"}, got.Args)
	assert.Empty(t, got.PaneArgs)
}

func TestCodexProfileFromArgsIgnoresTerminatorSuffixAndReadsLongConfig(t *testing.T) {
	t.Parallel()

	got, ok := codexProfileFromArgs([]string{
		"exec", "--model=" + CodexSolModel, "--config=model_reasoning_effort=max",
		"--", "child", "--model=child-model", "--config=model_reasoning_effort=low",
	})
	require.True(t, ok)
	assert.Equal(t, CodexProfile{Model: CodexSolModel, Effort: CodexEffortMax}, got)
}

func TestResolveCodexProviderProfileUsesCatalogResolution(t *testing.T) {
	t.Parallel()

	entry := CodexProviderEntryForQuality(QualityConf{Default: "ultra"})
	catalog := []byte(`{"models":[{"slug":"gpt-5.6-sol","supported_reasoning_levels":[{"effort":"xhigh"},{"effort":"max"}]}]}`)

	got, resolution := ResolveCodexProviderProfile(entry, catalog)
	assert.Equal(t, CodexResolutionEffortUnavailable, resolution.Reason)
	assert.Contains(t, got.Args, CodexSolModel)
	assert.Contains(t, got.Args, `model_reasoning_effort="max"`)
	assert.Contains(t, got.PaneArgs, `model_reasoning_effort="max"`)
}
