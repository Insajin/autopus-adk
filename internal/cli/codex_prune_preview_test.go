package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCodexLegacyPrunePreviewMatchesApply(t *testing.T) {
	dir := t.TempDir()
	configurePreviewBinaries(t, "codex")
	initCmd := newTestRootCmd()
	initCmd.SetArgs([]string{"init", "--dir", dir, "--project", "codex-prune", "--platforms", "codex", "--yes"})
	require.NoError(t, initCmd.Execute())

	manifest, err := adapter.LoadManifest(dir, "codex")
	require.NoError(t, err)
	require.NotNil(t, manifest)
	legacy := []string{
		filepath.Join(".codex", "prompts", "auto.md"),
		filepath.Join(".codex", "rules", "autopus", "branding.md"),
		filepath.Join(".agents", "skills", "auto", "SKILL.md"),
	}
	for _, path := range legacy {
		require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(dir, path)), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, path), []byte("legacy\n"), 0o644))
		manifest.Files[path] = adapter.ManifestFile{
			Checksum: adapter.Checksum("legacy\n"), Policy: adapter.OverwriteAlways,
		}
	}
	require.NoError(t, manifest.Save(dir))

	var preview bytes.Buffer
	planCmd := newTestRootCmd()
	planCmd.SetOut(&preview)
	planCmd.SetErr(&preview)
	planCmd.SetArgs([]string{"update", "--dir", dir, "--plan"})
	require.NoError(t, planCmd.Execute())
	for _, path := range legacy {
		assert.Contains(t, preview.String(), "prune "+filepath.ToSlash(path))
	}

	applyCmd := newTestRootCmd()
	applyCmd.SetArgs([]string{"update", "--dir", dir, "--yes"})
	require.NoError(t, applyCmd.Execute())
	for _, path := range legacy {
		assert.NoFileExists(t, filepath.Join(dir, path))
	}
}
