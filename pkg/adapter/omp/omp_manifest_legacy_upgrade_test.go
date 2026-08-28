package omp

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOMPUpdate_MigratesLegacyReadOnlyStateToOwnerOnly(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	adapterUnderTest := NewWithRoot(root)
	cfg := configForOMP()
	_, err := adapterUnderTest.Generate(context.Background(), cfg)
	require.NoError(t, err)

	manifestPath := filepath.Join(root, ".autopus", "omp-manifest.json")
	require.NoError(t, os.Chmod(manifestPath, 0o644))
	configPath := filepath.Join(root, configFile)
	require.NoError(t, os.WriteFile(configPath, []byte("theme:\n  dark: legacy-user\n"), 0o644))

	_, err = adapterUnderTest.Update(context.Background(), cfg)
	require.NoError(t, err)

	info, err := os.Stat(manifestPath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	info, err = os.Stat(configPath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func TestOMPUpdate_RejectsWritableLegacyManifest(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	adapterUnderTest := NewWithRoot(root)
	cfg := configForOMP()
	_, err := adapterUnderTest.Generate(context.Background(), cfg)
	require.NoError(t, err)

	manifestPath := filepath.Join(root, ".autopus", "omp-manifest.json")
	require.NoError(t, os.Chmod(manifestPath, 0o664))

	_, err = adapterUnderTest.Update(context.Background(), cfg)
	require.ErrorContains(t, err, "must have mode 0600")

	info, statErr := os.Stat(manifestPath)
	require.NoError(t, statErr)
	assert.Equal(t, os.FileMode(0o664), info.Mode().Perm())
}

func TestOMPUpdate_RejectsWritableLegacyConfig(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	adapterUnderTest := NewWithRoot(root)
	cfg := configForOMP()
	_, err := adapterUnderTest.Generate(context.Background(), cfg)
	require.NoError(t, err)

	configPath := filepath.Join(root, configFile)
	require.NoError(t, os.WriteFile(configPath, []byte("theme:\n  dark: unsafe\n"), 0o664))
	require.NoError(t, os.Chmod(configPath, 0o664))

	_, err = adapterUnderTest.Update(context.Background(), cfg)
	require.ErrorContains(t, err, "must have mode 0600")

	info, statErr := os.Stat(configPath)
	require.NoError(t, statErr)
	assert.Equal(t, os.FileMode(0o664), info.Mode().Perm())
}
