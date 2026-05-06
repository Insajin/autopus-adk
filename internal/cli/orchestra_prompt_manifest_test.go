package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/orchestra"
)

func TestExecuteDryRunWritesPromptLayerManifest(t *testing.T) {
	dir := t.TempDir()
	oldWd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() {
		_ = os.Chdir(oldWd)
	})

	data := orchestra.PromptData{
		ProjectName:    "autopus-adk",
		ProjectSummary: "CLI",
		TechStack:      "Go",
		Topic:          "prompt layer test",
		TargetModule:   ".",
		MaxTurns:       4,
	}

	err = executeDryRun("prompt layer test", data, []orchestra.ProviderConfig{{Name: "codex"}}, 1)
	require.NoError(t, err)

	promptPath := filepath.Join(dir, "orchestra-r1-prompt-layer-test.md")
	promptInfo, err := os.Stat(promptPath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), promptInfo.Mode().Perm())

	manifestPath := filepath.Join(dir, "orchestra-r1-prompt-layer-test.manifest.json")
	assert.FileExists(t, manifestPath)
	manifestInfo, err := os.Stat(manifestPath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), manifestInfo.Mode().Perm())
	body, err := os.ReadFile(manifestPath)
	require.NoError(t, err)
	assert.Contains(t, string(body), `"entries"`)
	assert.Contains(t, string(body), `"orchestra:debater_r1:task"`)
}

func TestExecuteDryRunRejectsUnsafeOutputTargets(t *testing.T) {
	dir := t.TempDir()
	oldWd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() {
		_ = os.Chdir(oldWd)
	})

	data := orchestra.PromptData{
		ProjectName: "autopus-adk",
		Topic:       "prompt layer test",
	}
	if err := os.Symlink("target", "orchestra-r1-prompt-layer-test.manifest.json"); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	err = executeDryRun("prompt layer test", data, []orchestra.ProviderConfig{{Name: "codex"}}, 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "refuse to overwrite symlink")
}

func TestExecuteDryRunRejectsEmptySanitizedTopic(t *testing.T) {
	err := executeDryRun("!!!", orchestra.PromptData{}, []orchestra.ProviderConfig{{Name: "codex"}}, 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "safe filename")
}
