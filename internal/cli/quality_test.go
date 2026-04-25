package cli

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/config"
)

func TestQualityShowCmd(t *testing.T) {
	dir := writeQualityTestConfig(t, "balanced")
	configPath := filepath.Join(dir, "autopus.yaml")

	root := NewRootCmd()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"--config", configPath, "quality", "show"})

	require.NoError(t, root.Execute())
	out := buf.String()
	assert.Contains(t, out, "quality.default = balanced")
	assert.Contains(t, out, "available = ultra, balanced")
}

func TestQualitySetCmd_UpdatesDefault(t *testing.T) {
	dir := writeQualityTestConfig(t, "balanced")
	configPath := filepath.Join(dir, "autopus.yaml")

	root := NewRootCmd()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"--config", configPath, "quality", "set", "ultra"})

	require.NoError(t, root.Execute())
	assert.Contains(t, buf.String(), "quality.default = ultra")

	cfg, err := config.LoadPreview(dir)
	require.NoError(t, err)
	assert.Equal(t, "ultra", cfg.Quality.Default)
}

func TestQualityCmd_ArgIsSetShorthand(t *testing.T) {
	dir := writeQualityTestConfig(t, "ultra")
	configPath := filepath.Join(dir, "autopus.yaml")

	root := NewRootCmd()
	root.SetArgs([]string{"--config", configPath, "quality", "balanced"})

	require.NoError(t, root.Execute())
	cfg, err := config.LoadPreview(dir)
	require.NoError(t, err)
	assert.Equal(t, "balanced", cfg.Quality.Default)
}

func TestQualityCmd_InteractiveChoice(t *testing.T) {
	dir := writeQualityTestConfig(t, "balanced")
	configPath := filepath.Join(dir, "autopus.yaml")

	root := NewRootCmd()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetIn(strings.NewReader("1\n"))
	root.SetArgs([]string{"--config", configPath, "quality"})

	require.NoError(t, root.Execute())
	assert.Contains(t, buf.String(), "Select quality mode:")

	cfg, err := config.LoadPreview(dir)
	require.NoError(t, err)
	assert.Equal(t, "ultra", cfg.Quality.Default)
}

func TestQualityCmd_InvalidPresetFails(t *testing.T) {
	dir := writeQualityTestConfig(t, "balanced")
	configPath := filepath.Join(dir, "autopus.yaml")

	root := NewRootCmd()
	root.SetArgs([]string{"--config", configPath, "quality", "turbo"})

	err := root.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), `unknown quality preset "turbo"`)
}

func TestReplaceQualityDefaultLine_PreservesInlineComment(t *testing.T) {
	t.Parallel()
	raw := []byte("quality:\n  default: balanced # keep this\n")

	line, ok := findQualityDefaultLine(raw)
	require.True(t, ok)
	updated, err := replaceQualityDefaultLine(raw, line, "ultra")
	require.NoError(t, err)

	assert.Equal(t, "quality:\n  default: ultra # keep this\n", string(updated))
}

func writeQualityTestConfig(t *testing.T, defaultPreset string) string {
	t.Helper()
	dir := t.TempDir()
	cfg := config.DefaultFullConfig("test-project")
	cfg.Quality.Default = defaultPreset
	require.NoError(t, config.Save(dir, cfg))
	return dir
}
