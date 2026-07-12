package cli

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/insajin/autopus-adk/pkg/orchestra"
)

func TestLoadHarnessConfigForDir_CodexRuntimeOverridesAreEphemeral(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfg := config.DefaultFullConfig("runtime-quality")
	cfg.Platforms = []string{"codex"}
	cfg.Quality.Default = "balanced"
	cfg.Orchestra.Providers["codex"] = managedCodexProviderForTest(cfg.Quality)
	require.NoError(t, config.Save(dir, cfg))
	rootSentinel := []byte("root-model-sentinel\n")
	agentSentinel := []byte("agent-model-sentinel\n")
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".codex", "agents"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".codex", "config.toml"), rootSentinel, 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".codex", "agents", "executor.toml"), agentSentinel, 0644))

	effective, err := loadHarnessConfigForDir(dir, globalFlags{Quality: "ultra", Effort: config.CodexEffortMax})
	require.NoError(t, err)
	assert.Equal(t, "ultra", effective.Quality.Default)
	provider := effective.Orchestra.Providers["codex"]
	assert.Equal(t, config.ProviderModelPolicyQuality, provider.ModelPolicy)
	assert.Contains(t, provider.Args, config.CodexSolModel)
	assert.Contains(t, provider.Args, `model_reasoning_effort="max"`)
	assert.Contains(t, provider.PaneArgs, `model_reasoning_effort="max"`)

	disk, err := os.ReadFile(filepath.Join(dir, "autopus.yaml"))
	require.NoError(t, err)
	assert.Contains(t, string(disk), "default: balanced")
	assert.Contains(t, string(disk), `model_reasoning_effort="xhigh"`)
	assert.NotContains(t, string(disk), `model_reasoning_effort="max"`)
	rootAfter, err := os.ReadFile(filepath.Join(dir, ".codex", "config.toml"))
	require.NoError(t, err)
	agentAfter, err := os.ReadFile(filepath.Join(dir, ".codex", "agents", "executor.toml"))
	require.NoError(t, err)
	assert.Equal(t, rootSentinel, rootAfter)
	assert.Equal(t, agentSentinel, agentAfter)
}

func TestLoadHarnessConfigForDir_RuntimeBalancedOverridesPersistentUltra(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfg := config.DefaultFullConfig("runtime-balanced")
	cfg.Platforms = []string{"codex"}
	cfg.Quality.Default = "ultra"
	cfg.Orchestra.Providers["codex"] = managedCodexProviderForTest(cfg.Quality)
	require.NoError(t, config.Save(dir, cfg))

	effective, err := loadHarnessConfigForDir(dir, globalFlags{Quality: "balanced"})
	require.NoError(t, err)
	assert.Equal(t, "balanced", effective.Quality.Default)
	provider := effective.Orchestra.Providers["codex"]
	assertCodexProfileInArgs(t, provider.Args, config.CodexSolModel, config.CodexEffortXHigh)
	assertCodexProfileInArgs(t, provider.PaneArgs, config.CodexSolModel, config.CodexEffortXHigh)

	disk, err := os.ReadFile(filepath.Join(dir, "autopus.yaml"))
	require.NoError(t, err)
	assert.Contains(t, string(disk), "default: ultra")
	assert.Contains(t, string(disk), `model_reasoning_effort="max"`)
}

func TestRunOrchestraCommand_AppliesRuntimeCodexQualityAndEffort(t *testing.T) {
	installRuntimeCodexCatalogFixture(t)
	dir := t.TempDir()
	cfg := config.DefaultFullConfig("general-orchestra-quality")
	cfg.Platforms = []string{"codex"}
	cfg.Quality.Default = "balanced"
	cfg.Orchestra.Providers = map[string]config.ProviderEntry{"codex": managedCodexProviderForTest(cfg.Quality)}
	cfg.Orchestra.Commands["plan"] = config.CommandEntry{Strategy: "consensus", Providers: []string{"codex"}}
	require.NoError(t, config.Save(dir, cfg))

	originalWD, err := os.Getwd()
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Chdir(originalWD) })
	require.NoError(t, os.Chdir(dir))

	originalRun := runOrchestraExecute
	t.Cleanup(func() { runOrchestraExecute = originalRun })
	var captured orchestra.OrchestraConfig
	runOrchestraExecute = func(_ context.Context, cfg orchestra.OrchestraConfig) (*orchestra.OrchestraResult, error) {
		captured = cfg
		return &orchestra.OrchestraResult{Merged: "ok", Summary: "done"}, nil
	}

	ctx := withGlobalFlags(context.Background(), globalFlags{Quality: "ultra", Effort: config.CodexEffortMax})
	err = runOrchestraCommand(ctx, "plan", "", []string{"codex"}, 30, "", "topic", 0, 0, OrchestraFlags{NoDetach: true})
	require.NoError(t, err)
	require.Len(t, captured.Providers, 1)
	assertCodexProfileInArgs(t, captured.Providers[0].Args, config.CodexSolModel, config.CodexEffortMax)
	assertCodexProfileInArgs(t, captured.Providers[0].PaneArgs, config.CodexSolModel, config.CodexEffortMax)
}

func installRuntimeCodexCatalogFixture(t *testing.T) {
	t.Helper()
	originalProbe := runtimeCodexCatalogProbe
	originalWriter := runtimeCodexFallbackWriter
	t.Cleanup(func() {
		runtimeCodexCatalogProbe = originalProbe
		runtimeCodexFallbackWriter = originalWriter
	})
	runtimeCodexCatalogProbe = func(context.Context, string) ([]byte, error) {
		return []byte(`{"models":[{"slug":"gpt-5.6-sol","supported_reasoning_levels":[{"effort":"xhigh"},{"effort":"max"},{"effort":"ultra"}]},{"slug":"gpt-5.5","supported_reasoning_levels":[{"effort":"xhigh"}]}]}`), nil
	}
	runtimeCodexFallbackWriter = io.Discard
}

func TestLoadHarnessConfigForDir_PinnedCodexIgnoresRuntimeOverrides(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfg := config.DefaultFullConfig("runtime-pinned")
	cfg.Platforms = []string{"codex"}
	cfg.Quality.Default = "balanced"
	wantArgs := []string{"exec", "--json", "-m", "user/model", "-c", `model_reasoning_effort="low"`}
	wantPaneArgs := []string{"--search", "-m", "user/pane"}
	cfg.Orchestra.Providers["codex"] = config.ProviderEntry{
		Binary:      "codex-wrapper",
		Args:        append([]string(nil), wantArgs...),
		PaneArgs:    append([]string(nil), wantPaneArgs...),
		ModelPolicy: config.ProviderModelPolicyPinned,
	}
	require.NoError(t, config.Save(dir, cfg))

	effective, err := loadHarnessConfigForDir(dir, globalFlags{Quality: "ultra", Effort: config.CodexEffortUltra})
	require.NoError(t, err)
	provider := effective.Orchestra.Providers["codex"]
	assert.Equal(t, wantArgs, provider.Args)
	assert.Equal(t, wantPaneArgs, provider.PaneArgs)
}

func TestBuildProviderConfigsForRuntime_CodexQualityMatrix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		quality string
		effort  string
		want    string
	}{
		{name: "default balanced", want: config.CodexEffortXHigh},
		{name: "balanced", quality: "balanced", want: config.CodexEffortXHigh},
		{name: "ultra", quality: "ultra", want: config.CodexEffortMax},
		{name: "explicit effort", quality: "ultra", effort: config.CodexEffortMax, want: config.CodexEffortMax},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			providers := buildProviderConfigsForRuntime([]string{"codex"}, tt.quality, tt.effort)
			require.Len(t, providers, 1)
			assert.Contains(t, providers[0].Args, config.CodexSolModel)
			assert.Contains(t, providers[0].Args, `model_reasoning_effort="`+tt.want+`"`)
			assert.Contains(t, providers[0].PaneArgs, `model_reasoning_effort="`+tt.want+`"`)
			assert.NotEqual(t, "exec", providers[0].PaneArgs[0])
		})
	}
}

func managedCodexProviderForTest(quality config.QualityConf) config.ProviderEntry {
	profile := quality.CodexOrchestraProfile()
	return config.ProviderEntry{
		Binary:      "codex",
		Args:        []string{"exec", "--sandbox", "workspace-write", "-m", profile.Model, "-c", `model_reasoning_effort="` + profile.Effort + `"`},
		PaneArgs:    []string{"-m", profile.Model, "-c", `model_reasoning_effort="` + profile.Effort + `"`},
		ModelPolicy: config.ProviderModelPolicyQuality,
		Subprocess:  config.SubprocessProvConf{SchemaFlag: "--output-schema", Timeout: config.CodexOrchestraTimeoutSeconds},
	}
}

func assertCodexProfileInArgs(t *testing.T, args []string, model, effort string) {
	t.Helper()
	joined := strings.Join(args, "\x00")
	assert.Contains(t, joined, model)
	assert.Contains(t, joined, `model_reasoning_effort="`+effort+`"`)
}
