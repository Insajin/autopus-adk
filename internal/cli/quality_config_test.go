package cli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/config"
)

func TestQualitySupervisorCmdPreservesRawConfigAndEnvPlaceholder(t *testing.T) {
	dir := writeQualityTestConfig(t, "balanced")
	path := filepath.Join(dir, "autopus.yaml")
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	raw = []byte(strings.Replace(string(raw),
		"project_name: test-project",
		"project_name: \"${AUTOPUS_TEST_SECRET}\" # keep project comment",
		1,
	))
	raw = []byte(strings.Replace(string(raw), "    supervisor_model_policy: inherit\n", "", 1))
	raw = append(raw, []byte("future_extension: keep-me # keep future field\n")...)
	require.NoError(t, os.WriteFile(path, raw, 0o640))
	require.NoError(t, os.Chmod(path, 0o640))
	t.Setenv("AUTOPUS_TEST_SECRET", "materialized-secret")

	root := NewRootCmd()
	root.SetOut(&bytes.Buffer{})
	root.SetArgs([]string{"--config", path, "quality", "supervisor", "quality"})
	require.NoError(t, root.Execute())

	updated, err := os.ReadFile(path)
	require.NoError(t, err)
	content := string(updated)
	assert.Contains(t, content, "project_name: \"${AUTOPUS_TEST_SECRET}\" # keep project comment")
	assert.NotContains(t, content, "materialized-secret")
	assert.Contains(t, content, "future_extension: keep-me # keep future field")
	assert.Contains(t, content, "supervisor_model_policy: quality")
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o640), info.Mode().Perm())
}

func TestQualitySupervisorCmdWriteFailurePreservesOriginal(t *testing.T) {
	dir := writeQualityTestConfig(t, "balanced")
	path := filepath.Join(dir, "autopus.yaml")
	original, err := os.ReadFile(path)
	require.NoError(t, err)

	originalRename := renameQualityConfig
	renameQualityConfig = func(string, string) error { return errors.New("rename failed") }
	t.Cleanup(func() { renameQualityConfig = originalRename })

	root := NewRootCmd()
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"--config", path, "quality", "supervisor", "quality"})
	err = root.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rename failed")
	after, readErr := os.ReadFile(path)
	require.NoError(t, readErr)
	assert.Equal(t, original, after)
}

func TestQualityCmdRejectsCustomConfigFilename(t *testing.T) {
	dir := writeQualityTestConfig(t, "balanced")
	customPath := filepath.Join(dir, "custom.yaml")
	require.NoError(t, os.WriteFile(customPath, []byte("project_name: custom\n"), 0o644))

	originalUpdater := qualityPlatformUpdater
	called := false
	qualityPlatformUpdater = func(context.Context, string, string, *config.HarnessConfig) (bool, error) {
		called = true
		return true, nil
	}
	t.Cleanup(func() { qualityPlatformUpdater = originalUpdater })

	root := NewRootCmd()
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"--config", customPath, "quality", "ultra", "--apply"})
	err := root.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "file named autopus.yaml")
	assert.False(t, called)
	cfg, loadErr := config.LoadPreview(dir)
	require.NoError(t, loadErr)
	assert.Equal(t, "balanced", cfg.Quality.Default)
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
