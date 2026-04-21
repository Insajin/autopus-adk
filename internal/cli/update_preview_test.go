package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/adapter"
)

func TestUpdateCmd_PlanShowsConfigAndCreatesWithoutWriting(t *testing.T) {
	dir := t.TempDir()
	binDir := t.TempDir()
	makeDummyBinary(t, binDir, "opencode")
	t.Setenv("PATH", binDir)

	initCmd := newTestRootCmd()
	initCmd.SetArgs([]string{"init", "--dir", dir, "--project", "preview-proj", "--platforms", "claude-code", "--yes"})
	require.NoError(t, initCmd.Execute())

	beforeConfig, err := os.ReadFile(filepath.Join(dir, "autopus.yaml"))
	require.NoError(t, err)

	var out bytes.Buffer
	updateCmd := newTestRootCmd()
	updateCmd.SetOut(&out)
	updateCmd.SetErr(&out)
	updateCmd.SetArgs([]string{"update", "--dir", dir, "--plan"})
	require.NoError(t, updateCmd.Execute())

	afterConfig, err := os.ReadFile(filepath.Join(dir, "autopus.yaml"))
	require.NoError(t, err)
	assert.Equal(t, string(beforeConfig), string(afterConfig), "preview must not rewrite config")
	assert.NoFileExists(t, filepath.Join(dir, "opencode.json"), "preview must not create new platform files")

	output := out.String()
	assert.Contains(t, output, "autopus.yaml")
	assert.Contains(t, output, "opencode.json")
	assert.Contains(t, output, "[config] update")
	assert.Contains(t, output, "[generated_surface] create")
}

func TestUpdateCmd_PlanShowsSkipAndPreserveWithoutWriting(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	initCmd := newTestRootCmd()
	initCmd.SetArgs([]string{"init", "--dir", dir, "--project", "preview-proj", "--platforms", "claude-code", "--yes"})
	require.NoError(t, initCmd.Execute())

	deletePath, preservePath := selectManagedPreviewTargets(t, dir, "claude-code")
	require.NoError(t, os.Remove(filepath.Join(dir, deletePath)))
	require.NoError(t, os.WriteFile(filepath.Join(dir, preservePath), []byte("user-modified"), 0o644))

	var out bytes.Buffer
	updateCmd := newTestRootCmd()
	updateCmd.SetOut(&out)
	updateCmd.SetErr(&out)
	updateCmd.SetArgs([]string{"update", "--dir", dir, "--plan"})
	require.NoError(t, updateCmd.Execute())

	assert.NoFileExists(t, filepath.Join(dir, deletePath), "preview must not recreate deleted file")
	data, err := os.ReadFile(filepath.Join(dir, preservePath))
	require.NoError(t, err)
	assert.Equal(t, "user-modified", string(data), "preview must not overwrite modified file")

	output := out.String()
	assert.Contains(t, output, "skip "+filepath.ToSlash(deletePath))
	assert.Contains(t, output, "preserve "+filepath.ToSlash(preservePath))
}

func TestUpdateCmd_PlanShowsLegacyConfigNormalizationWithoutWriting(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	initCmd := newTestRootCmd()
	initCmd.SetArgs([]string{"init", "--dir", dir, "--project", "preview-proj", "--platforms", "claude-code", "--yes"})
	require.NoError(t, initCmd.Execute())

	configPath := filepath.Join(dir, "autopus.yaml")
	before, err := os.ReadFile(configPath)
	require.NoError(t, err)

	legacy := strings.Replace(string(before), "claude-code", "claude", 1)
	require.NoError(t, os.WriteFile(configPath, []byte(legacy), 0o644))

	var out bytes.Buffer
	updateCmd := newTestRootCmd()
	updateCmd.SetOut(&out)
	updateCmd.SetErr(&out)
	updateCmd.SetArgs([]string{"update", "--dir", dir, "--plan", "--yes"})
	require.NoError(t, updateCmd.Execute())

	after, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assert.Equal(t, legacy, string(after), "preview must not normalize config in place")
	assert.Contains(t, out.String(), "autopus.yaml")
	assert.Contains(t, out.String(), "legacy platform names would be normalized")
}

func selectManagedPreviewTargets(t *testing.T, dir, platform string) (string, string) {
	t.Helper()

	manifest, err := adapter.LoadManifest(dir, platform)
	require.NoError(t, err)
	require.NotNil(t, manifest)

	paths := make([]string, 0, len(manifest.Files))
	for path, meta := range manifest.Files {
		if meta.Policy == adapter.OverwriteMarker {
			continue
		}
		paths = append(paths, path)
	}
	sort.Strings(paths)
	require.GreaterOrEqual(t, len(paths), 2, "need at least two non-marker managed files")

	return paths[0], paths[1]
}
