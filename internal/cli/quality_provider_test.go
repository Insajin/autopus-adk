package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/config"
)

func TestQualityProviderCmdPersistsIndependentModes(t *testing.T) {
	dir := writeQualityTestConfig(t, "balanced")
	path := filepath.Join(dir, "autopus.yaml")

	claudeRoot := NewRootCmd()
	claudeRoot.SetOut(&bytes.Buffer{})
	claudeRoot.SetArgs([]string{"--config", path, "quality", "provider", "claude-code", "ultra"})
	require.NoError(t, claudeRoot.Execute())

	codexRoot := NewRootCmd()
	codexRoot.SetOut(&bytes.Buffer{})
	codexRoot.SetArgs([]string{"--config", path, "quality", "provider", "codex", "balanced"})
	require.NoError(t, codexRoot.Execute())

	cfg, err := config.LoadPreview(dir)
	require.NoError(t, err)
	assert.Equal(t, "balanced", cfg.Quality.Default)
	assert.Equal(t, "ultra", cfg.Quality.Providers[config.QualityProviderClaude])
	assert.Equal(t, "balanced", cfg.Quality.Providers[config.QualityProviderCodex])
	assert.Equal(t, "ultra", cfg.Quality.EffectiveMode(config.QualityProviderClaude))
	assert.Equal(t, "balanced", cfg.Quality.EffectiveMode(config.QualityProviderCodex))
}

func TestQualityProviderCmdInheritRemovesOverride(t *testing.T) {
	dir := writeQualityTestConfig(t, "balanced")
	path := filepath.Join(dir, "autopus.yaml")
	cfg, err := config.LoadPreview(dir)
	require.NoError(t, err)
	cfg.Quality.Providers = map[string]string{config.QualityProviderClaude: "ultra"}
	require.NoError(t, config.Save(dir, cfg))

	root := NewRootCmd()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetArgs([]string{"--config", path, "quality", "provider", "claude", "inherit"})
	require.NoError(t, root.Execute())

	updated, err := config.LoadPreview(dir)
	require.NoError(t, err)
	assert.NotContains(t, updated.Quality.Providers, config.QualityProviderClaude)
	assert.Equal(t, "balanced", updated.Quality.EffectiveMode(config.QualityProviderClaude))
	assert.Contains(t, buf.String(), "quality.providers.claude = inherit")
	assert.Contains(t, buf.String(), "quality.effective.claude = balanced")
}

func TestQualityProviderCmdRejectsUnknownProviderAndPreset(t *testing.T) {
	dir := writeQualityTestConfig(t, "balanced")
	path := filepath.Join(dir, "autopus.yaml")
	before, err := os.ReadFile(path)
	require.NoError(t, err)

	original := qualityPlatformUpdater
	t.Cleanup(func() { qualityPlatformUpdater = original })
	updateCalls := 0
	qualityPlatformUpdater = func(context.Context, string, string, *config.HarnessConfig) (bool, error) {
		updateCalls++
		return true, nil
	}

	unknownProvider := NewRootCmd()
	unknownProvider.SetArgs([]string{"--config", path, "quality", "provider", "opencode", "ultra"})
	err = unknownProvider.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), `unknown quality provider "opencode"`)

	unknownPreset := NewRootCmd()
	unknownPreset.SetArgs([]string{"--config", path, "quality", "provider", "codex", "turbo"})
	err = unknownPreset.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), `unknown quality preset "turbo"`)

	after, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, before, after)
	assert.Zero(t, updateCalls)
}

func TestQualityProviderApplyUpdatesClaudeOnly(t *testing.T) {
	dir := writeQualityTestConfig(t, "balanced")
	cfg, err := config.LoadPreview(dir)
	require.NoError(t, err)
	cfg.Platforms = []string{"claude-code", "codex", "opencode"}
	require.NoError(t, config.Save(dir, cfg))

	original := qualityPlatformUpdater
	t.Cleanup(func() { qualityPlatformUpdater = original })
	var applied []string
	qualityPlatformUpdater = func(_ context.Context, _ string, platform string, _ *config.HarnessConfig) (bool, error) {
		applied = append(applied, platform)
		return true, nil
	}

	root := NewRootCmd()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetArgs([]string{
		"--config", filepath.Join(dir, "autopus.yaml"),
		"quality", "provider", "claude-code", "ultra", "--apply",
	})
	require.NoError(t, root.Execute())

	assert.Equal(t, []string{"claude-code"}, applied)
	assert.Contains(t, buf.String(), "quality.applied_platforms = 1")
}

func TestQualityShowIncludesProviderEffectiveModes(t *testing.T) {
	dir := writeQualityTestConfig(t, "balanced")
	cfg, err := config.LoadPreview(dir)
	require.NoError(t, err)
	cfg.Quality.Providers = map[string]string{config.QualityProviderClaude: "ultra"}
	require.NoError(t, config.Save(dir, cfg))

	root := NewRootCmd()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetArgs([]string{"--config", filepath.Join(dir, "autopus.yaml"), "quality", "show"})
	require.NoError(t, root.Execute())

	out := buf.String()
	assert.Contains(t, out, "quality.providers.claude = ultra")
	assert.Contains(t, out, "quality.effective.claude = ultra")
	assert.Contains(t, out, "quality.providers.codex = inherit")
	assert.Contains(t, out, "quality.effective.codex = balanced")
}

func TestQualityProviderApplyUpdatesOnlyTargetPlatform(t *testing.T) {
	dir := writeQualityTestConfig(t, "balanced")
	cfg, err := config.LoadPreview(dir)
	require.NoError(t, err)
	cfg.Platforms = []string{"claude-code", "codex"}
	require.NoError(t, config.Save(dir, cfg))

	original := qualityPlatformUpdater
	t.Cleanup(func() { qualityPlatformUpdater = original })
	var applied []string
	qualityPlatformUpdater = func(_ context.Context, gotDir, platform string, gotCfg *config.HarnessConfig) (bool, error) {
		assert.Equal(t, dir, gotDir)
		assert.Equal(t, "ultra", gotCfg.Quality.EffectiveMode(config.QualityProviderCodex))
		applied = append(applied, platform)
		return true, nil
	}

	root := NewRootCmd()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetArgs([]string{
		"--config", filepath.Join(dir, "autopus.yaml"),
		"quality", "provider", "codex", "ultra", "--apply",
	})
	require.NoError(t, root.Execute())

	assert.Equal(t, []string{"codex"}, applied)
	assert.Contains(t, buf.String(), "quality.applied_platforms = 1")
	assert.Contains(t, buf.String(), "Start a new Codex session")
}

func TestQualityProviderApplySkipsUnconfiguredPlatform(t *testing.T) {
	dir := writeQualityTestConfig(t, "balanced")
	cfg, err := config.LoadPreview(dir)
	require.NoError(t, err)
	cfg.Platforms = []string{"claude-code"}
	require.NoError(t, config.Save(dir, cfg))

	original := qualityPlatformUpdater
	t.Cleanup(func() { qualityPlatformUpdater = original })
	called := false
	qualityPlatformUpdater = func(context.Context, string, string, *config.HarnessConfig) (bool, error) {
		called = true
		return true, nil
	}

	root := NewRootCmd()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetArgs([]string{
		"--config", filepath.Join(dir, "autopus.yaml"),
		"quality", "provider", "codex", "ultra", "--apply",
	})
	require.NoError(t, root.Execute())

	assert.False(t, called)
	assert.Contains(t, buf.String(), "skipped unconfigured platform: codex")
	assert.Contains(t, buf.String(), "quality.applied_platforms = 0")
	updated, err := config.LoadPreview(dir)
	require.NoError(t, err)
	assert.Equal(t, "ultra", updated.Quality.Providers[config.QualityProviderCodex])
}
