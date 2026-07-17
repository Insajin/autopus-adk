package selfupdate

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"
)

type stubStagedBinaryFile struct {
	syncErr  error
	closeErr error
}

func (f *stubStagedBinaryFile) Sync() error  { return f.syncErr }
func (f *stubStagedBinaryFile) Close() error { return f.closeErr }

func TestReplaceWithOps_CrossDeviceCopyKeepsTargetUntilCommit(t *testing.T) {
	t.Parallel()

	newBinaryPath, targetPath := writeReplacementFixture(t)
	ops := defaultReplaceOps()
	ops.rename = func(_, _ string) error {
		return &os.LinkError{Op: "rename", Err: syscall.EXDEV}
	}
	ops.copyFile = func(src, dst string) error {
		require.NotEqual(t, targetPath, dst)
		assertInstalledBinary(t, targetPath, "old", 0711)
		return copyFile(src, dst)
	}

	require.NoError(t, replaceWithOps(newBinaryPath, targetPath, ops))
	assertInstalledBinary(t, targetPath, "new", 0711)
	assertNoReplacementResidue(t, targetPath)
}

func TestReplaceWithOps_PartialCopyFailureLeavesTargetUntouched(t *testing.T) {
	t.Parallel()

	newBinaryPath, targetPath := writeReplacementFixture(t)
	sentinel := errors.New("injected copy failure")
	ops := defaultReplaceOps()
	ops.rename = func(_, _ string) error {
		return &os.LinkError{Op: "rename", Err: syscall.EXDEV}
	}
	ops.copyFile = func(_, dst string) error {
		require.NotEqual(t, targetPath, dst)
		require.NoError(t, os.WriteFile(dst, []byte("partial"), 0600))
		assertInstalledBinary(t, targetPath, "old", 0711)
		return sentinel
	}

	err := replaceWithOps(newBinaryPath, targetPath, ops)
	require.ErrorIs(t, err, sentinel)
	assertInstalledBinary(t, targetPath, "old", 0711)
	assertNoReplacementResidue(t, targetPath)
}

func TestReplaceWithOps_ChmodFailureLeavesTargetUntouched(t *testing.T) {
	t.Parallel()

	newBinaryPath, targetPath := writeReplacementFixture(t)
	sentinel := errors.New("injected chmod failure")
	ops := defaultReplaceOps()
	ops.chmod = func(path string, _ os.FileMode) error {
		require.NotEqual(t, targetPath, path)
		assertInstalledBinary(t, targetPath, "old", 0711)
		return sentinel
	}

	err := replaceWithOps(newBinaryPath, targetPath, ops)
	require.ErrorIs(t, err, sentinel)
	assertInstalledBinary(t, targetPath, "old", 0711)
	assertNoReplacementResidue(t, targetPath)
}

func TestReplaceWithOps_XattrFailureLeavesTargetUntouched(t *testing.T) {
	t.Parallel()

	newBinaryPath, targetPath := writeReplacementFixture(t)
	sentinel := errors.New("injected xattr failure")
	ops := defaultReplaceOps()
	ops.clearUpdateXattrs = func(path string) error {
		require.NotEqual(t, targetPath, path)
		assertInstalledBinary(t, targetPath, "old", 0711)
		return sentinel
	}

	err := replaceWithOps(newBinaryPath, targetPath, ops)
	require.ErrorIs(t, err, sentinel)
	assertInstalledBinary(t, targetPath, "old", 0711)
	assertNoReplacementResidue(t, targetPath)
}

func TestReplaceWithOps_CommitFailureLeavesTargetUntouched(t *testing.T) {
	t.Parallel()

	newBinaryPath, targetPath := writeReplacementFixture(t)
	sentinel := errors.New("injected commit failure")
	ops := defaultReplaceOps()
	ops.commit = func(stagePath, gotTargetPath string, _ os.FileInfo) error {
		require.NotEqual(t, targetPath, stagePath)
		require.Equal(t, targetPath, gotTargetPath)
		assertInstalledBinary(t, targetPath, "old", 0711)
		return sentinel
	}

	err := replaceWithOps(newBinaryPath, targetPath, ops)
	require.ErrorIs(t, err, sentinel)
	assertInstalledBinary(t, targetPath, "old", 0711)
	assertNoReplacementResidue(t, targetPath)
}

func TestReplaceWithOps_SyncFailureLeavesTargetUntouched(t *testing.T) {
	t.Parallel()

	newBinaryPath, targetPath := writeReplacementFixture(t)
	sentinel := errors.New("injected sync failure")
	ops := defaultReplaceOps()
	ops.openStage = func(string) (stagedBinaryFile, error) {
		return &stubStagedBinaryFile{syncErr: sentinel}, nil
	}

	err := replaceWithOps(newBinaryPath, targetPath, ops)
	require.ErrorIs(t, err, sentinel)
	assertInstalledBinary(t, targetPath, "old", 0711)
	assertNoReplacementResidue(t, targetPath)
}

func TestReplaceWithOps_NonCrossDeviceRenameErrorDoesNotCopy(t *testing.T) {
	t.Parallel()

	newBinaryPath, targetPath := writeReplacementFixture(t)
	sentinel := errors.New("injected rename failure")
	copyCalled := false
	ops := defaultReplaceOps()
	ops.rename = func(_, _ string) error { return sentinel }
	ops.copyFile = func(_, _ string) error {
		copyCalled = true
		return nil
	}

	err := replaceWithOps(newBinaryPath, targetPath, ops)
	require.ErrorIs(t, err, sentinel)
	require.False(t, copyCalled)
	assertInstalledBinary(t, targetPath, "old", 0711)
	assertNoReplacementResidue(t, targetPath)
}

func TestReplaceWithOps_TargetChangeBeforeCommitIsNotOverwritten(t *testing.T) {
	t.Parallel()

	newBinaryPath, targetPath := writeReplacementFixture(t)
	competitorPath := filepath.Join(filepath.Dir(targetPath), "competitor")
	require.NoError(t, os.WriteFile(competitorPath, []byte("concurrent"), 0700))
	ops := defaultReplaceOps()
	originalClear := ops.clearUpdateXattrs
	ops.clearUpdateXattrs = func(stagePath string) error {
		if err := originalClear(stagePath); err != nil {
			return err
		}
		return os.Rename(competitorPath, targetPath)
	}

	err := replaceWithOps(newBinaryPath, targetPath, ops)
	require.EqualError(t, err, "target binary changed during replacement")
	assertInstalledBinary(t, targetPath, "concurrent", 0700)
	assertNoReplacementResidue(t, targetPath)
}

func TestReplace_ZeroValueAndNilReceiverRemainUsable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		replacer *Replacer
	}{
		{name: "zero value", replacer: &Replacer{}},
		{name: "nil receiver", replacer: nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			newBinaryPath, targetPath := writeReplacementFixture(t)
			require.NoError(t, test.replacer.Replace(newBinaryPath, targetPath))
			assertInstalledBinary(t, targetPath, "new", 0711)
		})
	}
}

func writeReplacementFixture(t *testing.T) (string, string) {
	t.Helper()

	targetPath := filepath.Join(t.TempDir(), "auto")
	require.NoError(t, os.WriteFile(targetPath, []byte("old"), 0711))
	newBinaryPath := filepath.Join(t.TempDir(), "auto-new")
	require.NoError(t, os.WriteFile(newBinaryPath, []byte("new"), 0644))
	return newBinaryPath, targetPath
}

func assertInstalledBinary(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()

	assertInstalledContent(t, path, content)
	if runtime.GOOS == "windows" {
		return
	}
	info, err := os.Stat(path)
	require.NoError(t, err)
	require.Equal(t, mode, info.Mode().Perm())
}

func assertInstalledContent(t *testing.T, path, content string) {
	t.Helper()

	actual, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, content, string(actual))
}

func assertNoReplacementResidue(t *testing.T, targetPath string) {
	t.Helper()

	entries, err := os.ReadDir(filepath.Dir(targetPath))
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, filepath.Base(targetPath), entries[0].Name())
}
