package omp

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/config"
)

func TestOMPContextBridgeLifecycle_ManifestUpdateAndCleanOwnOnlyExactFile(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	bridgePath := filepath.Join(root, filepath.FromSlash(ompContextBridgeTarget))
	userExtension := filepath.Join(root, ".omp", "extensions", "user-owned.ts")
	userSurface := filepath.Join(root, ".omp", "user-owned.json")
	const userExtensionBody = "export default function userExtension() {}\n"
	const userSurfaceBody = "{\"owner\":\"user\"}\n"

	optedIn := optedInOMPContextBridgeConfig()
	require.NoError(t, config.Save(root, optedIn))
	generated, err := NewWithRoot(root).Generate(ctx, optedIn)
	require.NoError(t, err)
	require.Contains(t, ompMappingTargets(generated.Files), ompContextBridgeTarget)
	assertFileBytesOMP(t, bridgePath, expectedOMPContextBridgeSource)
	bridgeInfo, err := os.Stat(bridgePath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o644), bridgeInfo.Mode().Perm())
	assertBridgeManifestEntry(t, root)

	require.NoError(t, os.WriteFile(userExtension, []byte(userExtensionBody), 0o640))
	require.NoError(t, os.WriteFile(userSurface, []byte(userSurfaceBody), 0o600))

	noOptIn := configForOMP()
	require.NoError(t, config.Save(root, noOptIn))
	updated, err := NewWithRoot(root).Update(ctx, noOptIn)
	require.NoError(t, err)
	assert.NotContains(t, ompMappingTargets(updated.Files), ompContextBridgeTarget)
	assert.NoFileExists(t, bridgePath)
	assertFileBytesOMP(t, userExtension, userExtensionBody)
	assertFileBytesOMP(t, userSurface, userSurfaceBody)
	manifest, err := adapter.LoadManifest(root, adapterName)
	require.NoError(t, err)
	require.NotNil(t, manifest)
	_, stillManaged := manifest.Files[ompContextBridgeTarget]
	assert.False(t, stillManaged)

	require.NoError(t, config.Save(root, optedIn))
	_, err = NewWithRoot(root).Update(ctx, optedIn)
	require.NoError(t, err)
	assertBridgeManifestEntry(t, root)
	require.NoError(t, NewWithRoot(root).Clean(ctx))
	assert.NoFileExists(t, bridgePath)
	assert.NoFileExists(t, filepath.Join(root, ".autopus", "omp-manifest.json"))
	assertFileBytesOMP(t, userExtension, userExtensionBody)
	assertFileBytesOMP(t, userSurface, userSurfaceBody)
}

func TestOMPContextBridgeLifecycle_RejectsExtensionDirectorySymlink(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".omp"), 0o755))
	require.NoError(t, os.Symlink(outside, filepath.Join(root, ".omp", "extensions")))

	_, err := NewWithRoot(root).Generate(context.Background(), optedInOMPContextBridgeConfig())
	require.Error(t, err)
	assert.NoFileExists(t, filepath.Join(outside, "autopus-context.ts"))
	entries, readErr := os.ReadDir(outside)
	require.NoError(t, readErr)
	assert.Empty(t, entries)
}

func TestOMPContextBridgeLifecycle_InvalidOptInFailsBeforeWrites(t *testing.T) {
	root := t.TempDir()
	cfg := configForOMP()
	cfg.OMPContextPolicy.Profile = "missing"

	_, err := NewWithRoot(root).Generate(context.Background(), cfg)
	require.ErrorContains(t, err, "omp_context_policy.profile_unknown")
	assert.NoDirExists(t, filepath.Join(root, ".agents"))
	assert.NoDirExists(t, filepath.Join(root, ".omp"))
	assert.NoFileExists(t, filepath.Join(root, ".autopus", "omp-manifest.json"))
}

func assertBridgeManifestEntry(t *testing.T, root string) {
	t.Helper()
	manifest, err := adapter.LoadManifest(root, adapterName)
	require.NoError(t, err)
	require.NotNil(t, manifest)
	assert.Equal(t, adapter.ManifestFile{
		Checksum: adapter.Checksum(expectedOMPContextBridgeSource),
		Policy:   adapter.OverwriteAlways,
	}, manifest.Files[ompContextBridgeTarget])
}
