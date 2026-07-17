package selfupdate

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReplace_RejectsSourceEqualToTarget(t *testing.T) {
	t.Parallel()

	targetPath := filepath.Join(t.TempDir(), "auto")
	require.NoError(t, os.WriteFile(targetPath, []byte("installed"), 0711))

	err := NewReplacer().Replace(targetPath, targetPath)
	require.ErrorContains(t, err, "same file")
	assertInstalledBinary(t, targetPath, "installed", 0711)
	assertNoReplacementResidue(t, targetPath)
}

func TestReplace_RejectsHardlinkSourceAlias(t *testing.T) {
	t.Parallel()

	targetPath := filepath.Join(t.TempDir(), "auto")
	require.NoError(t, os.WriteFile(targetPath, []byte("installed"), 0711))
	sourcePath := filepath.Join(t.TempDir(), "auto-hardlink")
	require.NoError(t, os.Link(targetPath, sourcePath))

	err := NewReplacer().Replace(sourcePath, targetPath)
	require.ErrorContains(t, err, "same file")
	assertInstalledBinary(t, targetPath, "installed", 0711)
}

func TestReplace_RejectsSymlinkSource(t *testing.T) {
	t.Parallel()

	targetPath := filepath.Join(t.TempDir(), "auto")
	require.NoError(t, os.WriteFile(targetPath, []byte("installed"), 0711))
	realSourcePath := filepath.Join(t.TempDir(), "auto-new")
	require.NoError(t, os.WriteFile(realSourcePath, []byte("new"), 0644))
	sourcePath := filepath.Join(t.TempDir(), "auto-link")
	if err := os.Symlink(realSourcePath, sourcePath); err != nil {
		if runtime.GOOS == "windows" {
			t.Skipf("Windows runner cannot create symlinks: %v", err)
		}
		require.NoError(t, err)
	}

	err := NewReplacer().Replace(sourcePath, targetPath)
	require.ErrorContains(t, err, "regular file")
	assertInstalledBinary(t, targetPath, "installed", 0711)
	assertInstalledBinary(t, realSourcePath, "new", 0644)
}

func TestReplace_RejectsSymlinkTarget(t *testing.T) {
	t.Parallel()

	realTargetPath := filepath.Join(t.TempDir(), "auto-real")
	require.NoError(t, os.WriteFile(realTargetPath, []byte("installed"), 0711))
	targetPath := filepath.Join(t.TempDir(), "auto-link")
	if err := os.Symlink(realTargetPath, targetPath); err != nil {
		if runtime.GOOS == "windows" {
			t.Skipf("Windows runner cannot create symlinks: %v", err)
		}
		require.NoError(t, err)
	}
	sourcePath := filepath.Join(t.TempDir(), "auto-new")
	require.NoError(t, os.WriteFile(sourcePath, []byte("new"), 0644))

	err := NewReplacer().Replace(sourcePath, targetPath)
	require.ErrorContains(t, err, "regular file")
	info, statErr := os.Lstat(targetPath)
	require.NoError(t, statErr)
	require.True(t, info.Mode()&os.ModeSymlink != 0)
	assertInstalledBinary(t, realTargetPath, "installed", 0711)
}
