package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigOperations_RejectSymlinkedConfig(t *testing.T) {
	t.Parallel()
	testConfigSymlinkRejection(t, func(t *testing.T, root, outside string) string {
		t.Helper()
		if err := os.Symlink(filepath.Join(outside, configFileName), filepath.Join(root, configFileName)); err != nil {
			t.Skipf("symlink unavailable: %v", err)
		}
		return root
	})
}

func TestConfigOperations_RejectSymlinkedIntermediateDirectory(t *testing.T) {
	t.Parallel()
	testConfigSymlinkRejection(t, func(t *testing.T, root, outside string) string {
		t.Helper()
		linkedDir := filepath.Join(root, "linked-project")
		if err := os.Symlink(outside, linkedDir); err != nil {
			t.Skipf("symlink unavailable: %v", err)
		}
		return linkedDir
	})
}

func testConfigSymlinkRejection(t *testing.T, configDir func(*testing.T, string, string) string) {
	t.Helper()
	operations := []struct {
		name string
		run  func(string) error
	}{
		{name: "Load", run: func(dir string) error { _, err := Load(dir); return err }},
		{name: "LoadPreview", run: func(dir string) error { _, err := LoadPreview(dir); return err }},
		{name: "LoadPreviewWithMetadata", run: func(dir string) error {
			_, _, err := LoadPreviewWithMetadata(dir)
			return err
		}},
		{name: "MissingTopLevelKey", run: func(dir string) error {
			_, err := MissingTopLevelKey(dir, "language")
			return err
		}},
		{name: "Save", run: func(dir string) error { return Save(dir, DefaultFullConfig("replacement")) }},
	}

	for _, operation := range operations {
		operation := operation
		t.Run(operation.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			outside := t.TempDir()
			outsidePath := filepath.Join(outside, configFileName)
			before := []byte("mode: full\nproject_name: outside\nplatforms: [codex]\n")
			require.NoError(t, os.WriteFile(outsidePath, before, 0o600))
			dir := configDir(t, root, outside)

			require.Error(t, operation.run(dir))
			after, err := os.ReadFile(outsidePath)
			require.NoError(t, err)
			assert.Equal(t, before, after)
		})
	}

	t.Run("Exists", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		outside := t.TempDir()
		outsidePath := filepath.Join(outside, configFileName)
		before := []byte("mode: full\nproject_name: outside\nplatforms: [codex]\n")
		require.NoError(t, os.WriteFile(outsidePath, before, 0o600))
		dir := configDir(t, root, outside)

		assert.False(t, Exists(dir))
		after, err := os.ReadFile(outsidePath)
		require.NoError(t, err)
		assert.Equal(t, before, after)
	})
}

func TestLoad_NormalizationPreservesEnvironmentPlaceholder(t *testing.T) {
	dir := t.TempDir()
	secret := "expanded-secret-value"
	t.Setenv("SECRET", secret)
	raw := []byte("mode: full\nproject_name: ${SECRET}\nplatforms: [claude]\n")
	require.NoError(t, os.WriteFile(filepath.Join(dir, configFileName), raw, 0o600))

	cfg, err := Load(dir)
	require.NoError(t, err)
	assert.Equal(t, secret, cfg.ProjectName)
	assert.Equal(t, []string{"claude-code"}, cfg.Platforms)
	assertConfigKeepsPlaceholder(t, dir, secret)
}

func TestSave_PreservesEnvironmentPlaceholderFromExistingConfig(t *testing.T) {
	dir := t.TempDir()
	secret := "expanded-secret-value"
	t.Setenv("SECRET", secret)
	raw := []byte("mode: full\nproject_name: ${SECRET}\nplatforms: [codex]\n")
	require.NoError(t, os.WriteFile(filepath.Join(dir, configFileName), raw, 0o600))
	cfg, err := LoadPreview(dir)
	require.NoError(t, err)

	require.NoError(t, Save(dir, cfg))
	assertConfigKeepsPlaceholder(t, dir, secret)
}

func TestSave_PreservesEnvironmentPlaceholderInMapKey(t *testing.T) {
	dir := t.TempDir()
	secret := "expanded-provider-name"
	t.Setenv("SECRET", secret)
	raw := []byte("mode: full\nproject_name: map-key-test\nplatforms: [codex]\norchestra:\n  providers:\n    ${SECRET}:\n      binary: custom-provider\n")
	require.NoError(t, os.WriteFile(filepath.Join(dir, configFileName), raw, 0o600))
	cfg, err := LoadPreview(dir)
	require.NoError(t, err)
	_, ok := cfg.Orchestra.Providers[secret]
	require.True(t, ok)

	require.NoError(t, Save(dir, cfg))
	assertConfigKeepsPlaceholder(t, dir, secret)
}

func assertConfigKeepsPlaceholder(t *testing.T, dir, secret string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, configFileName))
	require.NoError(t, err)
	assert.Contains(t, string(data), "${SECRET}")
	assert.NotContains(t, string(data), secret)
}
