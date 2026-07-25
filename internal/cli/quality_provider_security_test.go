package cli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/insajin/autopus-adk/pkg/config"
)

func TestQualityProviderRejectsUnsafeConfiguredPresetNamesWithoutMutation(t *testing.T) {
	for _, preset := range []string{
		"custom # comment",
		"custom:value",
		"custom;command",
		"custom'quote",
		"custom\ninjected: value",
	} {
		t.Run(preset, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "autopus.yaml")
			cfg := config.DefaultFullConfig("unsafe-preset")
			cfg.Platforms = []string{"claude-code", "codex"}
			cfg.Quality.Presets[preset] = cfg.Quality.Presets["balanced"]
			raw, err := yaml.Marshal(cfg)
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(path, raw, 0o640))

			originalUpdater := qualityPlatformUpdater
			t.Cleanup(func() { qualityPlatformUpdater = originalUpdater })
			updateCalls := 0
			qualityPlatformUpdater = func(context.Context, string, string, *config.HarnessConfig) (bool, error) {
				updateCalls++
				return true, nil
			}

			root := NewRootCmd()
			root.SetOut(&bytes.Buffer{})
			root.SetErr(&bytes.Buffer{})
			root.SetArgs([]string{
				"--config", path,
				"quality", "provider", "claude", preset, "--apply",
			})
			err = root.Execute()
			require.Error(t, err)
			assert.Contains(t, err.Error(), "quality.presets name")

			after, readErr := os.ReadFile(path)
			require.NoError(t, readErr)
			assert.Equal(t, raw, after)
			assert.Zero(t, updateCalls)
		})
	}
}

func TestQualityProviderSafeCustomPresetRoundTripsAsString(t *testing.T) {
	dir := writeQualityTestConfig(t, "balanced")
	cfg, err := config.LoadPreview(dir)
	require.NoError(t, err)
	cfg.Quality.Presets["true"] = cfg.Quality.Presets["balanced"]
	require.NoError(t, config.Save(dir, cfg))

	root := NewRootCmd()
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{
		"--config", filepath.Join(dir, "autopus.yaml"),
		"quality", "provider", "codex", "true",
	})
	require.NoError(t, root.Execute())

	updated, err := config.LoadPreview(dir)
	require.NoError(t, err)
	assert.Equal(t, "true", updated.Quality.Providers[config.QualityProviderCodex])
	raw, err := os.ReadFile(filepath.Join(dir, "autopus.yaml"))
	require.NoError(t, err)
	assert.Contains(t, string(raw), `codex: "true"`)
}

func TestQualityProviderApplyRetryQuotesProviderAndPreset(t *testing.T) {
	dir := writeQualityTestConfig(t, "balanced")
	cfg, err := config.LoadPreview(dir)
	require.NoError(t, err)
	cfg.Platforms = []string{"claude-code"}
	cfg.Quality.Presets["custom-safe"] = cfg.Quality.Presets["balanced"]
	require.NoError(t, config.Save(dir, cfg))

	originalUpdater := qualityPlatformUpdater
	t.Cleanup(func() { qualityPlatformUpdater = originalUpdater })
	qualityPlatformUpdater = func(context.Context, string, string, *config.HarnessConfig) (bool, error) {
		return true, errors.New("apply failed")
	}

	root := NewRootCmd()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{
		"--config", filepath.Join(dir, "autopus.yaml"),
		"quality", "provider", "claude", "custom-safe", "--apply",
	})
	err = root.Execute()
	require.Error(t, err)

	retry := "auto quality provider 'claude' 'custom-safe' --apply"
	assert.Contains(t, buf.String(), "Retry: "+qualityRetryCommand(dir, retry))
	assert.NotContains(t, buf.String(), "quality provider claude custom-safe --apply")
}
