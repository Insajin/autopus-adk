package config

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMigrateOrchestraConfig_MarksExactHistoricalCodexDefaultsQualityManaged(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		quality string
		effort  string
	}{
		{name: "balanced", quality: "balanced", effort: CodexEffortXHigh},
		{name: "ultra", quality: "ultra", effort: CodexEffortUltra},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := &HarnessConfig{
				Platforms: []string{"codex"},
				Quality:   QualityConf{Default: tt.quality},
				Orchestra: OrchestraConf{
					Enabled: true,
					Providers: map[string]ProviderEntry{
						"codex": {
							Binary:   "codex",
							Args:     []string{"exec", "--sandbox", "workspace-write", "-m", CodexLegacyModel, "-c", `model_reasoning_effort="xhigh"`},
							PaneArgs: []string{"-m", CodexLegacyModel, "-c", `model_reasoning_effort="xhigh"`},
						},
					},
					Commands: map[string]CommandEntry{},
				},
			}

			changed, err := MigrateOrchestraConfig(cfg)
			require.NoError(t, err)
			assert.True(t, changed)

			got := cfg.Orchestra.Providers["codex"]
			assert.Equal(t, ProviderModelPolicyQuality, got.ModelPolicy)
			assert.Equal(t, []string{"exec", "--sandbox", "workspace-write", "-m", CodexSolModel, "-c", `model_reasoning_effort="` + tt.effort + `"`}, got.Args)
			assert.Equal(t, []string{"-m", CodexSolModel, "-c", `model_reasoning_effort="` + tt.effort + `"`}, got.PaneArgs)
		})
	}
}

func TestMigrateOrchestraConfig_UnmarkedCustomCodexBecomesPinnedWithoutArgvChanges(t *testing.T) {
	t.Parallel()

	args := []string{"exec", "--json", "-m", "user/codex", "-c", `model_reasoning_effort="ultra"`, "--sandbox", "danger-full-access"}
	paneArgs := []string{"--search", "-m", "user/codex-pane", "-c", `model_reasoning_effort="low"`}
	cfg := &HarnessConfig{
		Platforms: []string{"codex"},
		Quality:   QualityConf{Default: "ultra"},
		Orchestra: OrchestraConf{
			Enabled: true,
			Providers: map[string]ProviderEntry{
				"codex": {Binary: "codex-custom", Args: append([]string(nil), args...), PaneArgs: append([]string(nil), paneArgs...)},
			},
			Commands: map[string]CommandEntry{},
		},
	}

	_, err := MigrateOrchestraConfig(cfg)
	require.NoError(t, err)
	got := cfg.Orchestra.Providers["codex"]
	assert.Equal(t, ProviderModelPolicyPinned, got.ModelPolicy)
	assert.Equal(t, args, got.Args)
	assert.Equal(t, paneArgs, got.PaneArgs)
}

func TestMigrateOrchestraConfig_HistoricalCodexNearMatchesRemainPinned(t *testing.T) {
	t.Parallel()

	historicalArgs := []string{"exec", "--sandbox", "workspace-write", "-m", CodexLegacyModel, "-c", `model_reasoning_effort="xhigh"`}
	historicalPaneArgs := []string{"-m", CodexLegacyModel, "-c", `model_reasoning_effort="xhigh"`}
	tests := []struct {
		name     string
		args     []string
		paneArgs []string
	}{
		{name: "extra subprocess flag", args: append(append([]string(nil), historicalArgs...), "--json"), paneArgs: historicalPaneArgs},
		{name: "reordered subprocess flags", args: []string{"exec", "-m", CodexLegacyModel, "--sandbox", "workspace-write", "-c", `model_reasoning_effort="xhigh"`}, paneArgs: historicalPaneArgs},
		{name: "missing pane args", args: historicalArgs},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			wantArgs := append([]string(nil), tt.args...)
			wantPaneArgs := append([]string(nil), tt.paneArgs...)
			cfg := &HarnessConfig{
				Platforms: []string{"codex"},
				Quality:   QualityConf{Default: "ultra"},
				Orchestra: OrchestraConf{
					Enabled:   true,
					Providers: map[string]ProviderEntry{"codex": {Binary: "codex", Args: tt.args, PaneArgs: tt.paneArgs}},
					Commands:  map[string]CommandEntry{},
				},
			}

			_, err := MigrateOrchestraConfig(cfg)
			require.NoError(t, err)
			got := cfg.Orchestra.Providers["codex"]
			assert.Equal(t, ProviderModelPolicyPinned, got.ModelPolicy)
			assert.Equal(t, wantArgs, got.Args)
			assert.Equal(t, wantPaneArgs, got.PaneArgs)
		})
	}
}

func TestMigrateOrchestraConfig_ExplicitPinnedCodexRemainsByteForByte(t *testing.T) {
	t.Parallel()

	want := ProviderEntry{
		Binary:           "codex-wrapper",
		Args:             []string{"exec", "--full-auto", "-m", CodexLegacyModel},
		PaneArgs:         []string{"--search", "-m", "custom-pane"},
		ModelPolicy:      ProviderModelPolicyPinned,
		PromptViaArgs:    true,
		InteractiveInput: "args",
		WorkingPatterns:  []string{"custom-working"},
		Subprocess:       SubprocessProvConf{SchemaFlag: "--custom-schema", StdinMode: "file", OutputFormat: "text", Timeout: 999},
	}
	cfg := &HarnessConfig{
		Platforms: []string{"codex"},
		Quality:   QualityConf{Default: "ultra"},
		Orchestra: OrchestraConf{Enabled: true, Providers: map[string]ProviderEntry{"codex": want}, Commands: map[string]CommandEntry{}},
	}

	_, err := MigrateOrchestraConfig(cfg)
	require.NoError(t, err)
	got := cfg.Orchestra.Providers["codex"]
	if !reflect.DeepEqual(want, got) {
		t.Fatalf("pinned provider changed:\nwant: %#v\n got: %#v", want, got)
	}
}

func TestEnsureOrchestraProvider_PreservesPinnedCodexWithEmptyArgs(t *testing.T) {
	t.Parallel()

	want := ProviderEntry{
		Binary:           "codex-wrapper",
		PaneArgs:         []string{"--custom-pane"},
		ModelPolicy:      ProviderModelPolicyPinned,
		PromptViaArgs:    true,
		InteractiveInput: "args",
		WorkingPatterns:  []string{"custom-working"},
		Subprocess:       SubprocessProvConf{SchemaFlag: "--custom-schema", Timeout: 999},
	}
	cfg := &HarnessConfig{
		Quality: QualityConf{Default: "ultra"},
		Orchestra: OrchestraConf{
			Enabled:   true,
			Providers: map[string]ProviderEntry{"codex": want},
			Commands:  map[string]CommandEntry{"review": {Providers: []string{"claude"}}},
		},
	}

	require.NoError(t, EnsureOrchestraProvider(cfg, "codex"))
	got := cfg.Orchestra.Providers["codex"]
	if !reflect.DeepEqual(want, got) {
		t.Fatalf("pinned provider changed:\nwant: %#v\n got: %#v", want, got)
	}
	assert.Equal(t, []string{"claude", "codex"}, cfg.Orchestra.Commands["review"].Providers)
}

func TestEnsureOrchestraProvider_UsesQualityForCodexDefaults(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		providers map[string]ProviderEntry
	}{
		{name: "missing provider", providers: map[string]ProviderEntry{}},
		{name: "managed provider with empty args", providers: map[string]ProviderEntry{
			"codex": {Binary: "codex", ModelPolicy: ProviderModelPolicyQuality},
		}},
		{name: "zero-value unmarked provider", providers: map[string]ProviderEntry{
			"codex": {},
		}},
		{name: "canonical unmarked provider", providers: map[string]ProviderEntry{
			"codex": {Binary: "codex"},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := &HarnessConfig{
				Quality: QualityConf{Default: "ultra"},
				Orchestra: OrchestraConf{
					Enabled:   true,
					Providers: tt.providers,
				},
			}

			require.NoError(t, EnsureOrchestraProvider(cfg, "codex"))
			assert.Equal(t, CodexProviderEntryForQuality(cfg.Quality), cfg.Orchestra.Providers["codex"])
		})
	}
}

func TestEnsureOrchestraProvider_PinsAndPreservesUnmarkedCustomCodexWithEmptyArgs(t *testing.T) {
	t.Parallel()

	want := ProviderEntry{
		Binary:           "codex-wrapper",
		PaneArgs:         []string{"--custom-pane"},
		PromptViaArgs:    true,
		InteractiveInput: "stdin",
		WorkingPatterns:  []string{"custom-working"},
		Subprocess:       SubprocessProvConf{SchemaFlag: "--custom-schema", Timeout: 999},
	}
	cfg := &HarnessConfig{
		Quality: QualityConf{Default: "ultra"},
		Orchestra: OrchestraConf{
			Enabled:   true,
			Providers: map[string]ProviderEntry{"codex": want},
		},
	}

	require.NoError(t, EnsureOrchestraProvider(cfg, "codex"))
	want.ModelPolicy = ProviderModelPolicyPinned
	assert.Equal(t, want, cfg.Orchestra.Providers["codex"])
}
