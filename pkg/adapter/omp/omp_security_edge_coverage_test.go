package omp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadOMPModelOwnedFile_EnforcesRegularBoundedFile(t *testing.T) {
	root := t.TempDir()
	regular := filepath.Join(root, "config.yml")
	want := []byte("model: provider/model\n")
	require.NoError(t, os.WriteFile(regular, want, 0o600))

	got, err := readOMPModelOwnedFile(regular)
	require.NoError(t, err)
	assert.Equal(t, want, got)

	_, err = readOMPModelOwnedFile(filepath.Join(root, "missing.yml"))
	assert.ErrorContains(t, err, "open owned file")

	_, err = readOMPModelOwnedFile(root)
	assert.ErrorContains(t, err, "must be regular")

	oversized := filepath.Join(root, "oversized.yml")
	require.NoError(t, os.WriteFile(oversized, make([]byte, (4<<20)+1), 0o600))
	_, err = readOMPModelOwnedFile(oversized)
	assert.ErrorContains(t, err, "exceeds size limit")
}

func TestRestoreOMPCleanPreimage_CoversMissingDirectoryAndRollbackErrors(t *testing.T) {
	root := t.TempDir()
	workspace, err := openOMPRootedWorkspace(root)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, workspace.Close()) })

	require.NoError(t, restoreOMPCleanPreimage(workspace, ompCleanPreimage{
		path: "already-absent", missing: true,
	}))

	require.NoError(t, restoreOMPCleanPreimage(workspace, ompCleanPreimage{
		path: "restored/nested", directory: true, mode: 0o710,
	}))
	info, err := os.Stat(filepath.Join(root, "restored", "nested"))
	require.NoError(t, err)
	assert.True(t, info.IsDir())
	assert.Equal(t, os.FileMode(0o710), info.Mode().Perm())

	require.NoError(t, os.WriteFile(filepath.Join(root, "blocked"), []byte("file"), 0o600))
	err = rollbackOMPClean(workspace, []ompCleanPreimage{{
		path: "blocked/child", directory: true, mode: 0o700,
	}})
	assert.Error(t, err)
}

func TestOMPSecurityHelpers_FailClosedOnInvalidParentClosedRootAndTempRoot(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "file-parent"), []byte("file"), 0o600))
	workspace, err := openOMPRootedWorkspace(root)
	require.NoError(t, err)

	_, err = captureOMPCleanMissingParents(workspace, "file-parent/child/config.yml")
	assert.ErrorContains(t, err, "inspect clean backup parent")
	require.NoError(t, workspace.Close())
	assert.ErrorContains(t, validateOMPModelWorkspaceBinding(workspace), "workspace changed")

	t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "missing-temp-root"))
	path, cleanup, err := stageOMPModelActivationConfig([]byte("model: provider/model\n"))
	assert.Empty(t, path)
	assert.Nil(t, cleanup)
	assert.ErrorContains(t, err, "create activation staging root")
}

func TestMarkOMPReadinessOverlayCleanupFailure_DowngradesExactCapability(t *testing.T) {
	report := OMPReadinessReport{Capabilities: []OMPCapabilityResult{
		{ID: "identity.version", Supported: true, Reason: "version_verified"},
		{ID: "config.overlay_readback", Supported: true, Reason: "output_valid"},
	}}

	got := markOMPReadinessOverlayCleanupFailure(report)
	assert.True(t, got.Capabilities[0].Supported)
	assert.False(t, got.Capabilities[1].Supported)
	assert.Equal(t, "output_invalid", got.Capabilities[1].Reason)
}
