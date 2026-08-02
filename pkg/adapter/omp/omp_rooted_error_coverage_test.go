package omp

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOMPRootedWorkspace_RejectsInvalidRootsAndPaths(t *testing.T) {
	t.Parallel()

	parent := t.TempDir()
	fileRoot := filepath.Join(parent, "file")
	require.NoError(t, os.WriteFile(fileRoot, []byte("not a directory"), 0o600))
	symlinkRoot := filepath.Join(parent, "symlink")
	require.NoError(t, os.Symlink(t.TempDir(), symlinkRoot))

	for _, path := range []string{filepath.Join(parent, "missing"), fileRoot, symlinkRoot} {
		workspace, err := openOMPRootedWorkspace(path)
		assert.Nil(t, workspace)
		assert.ErrorContains(t, err, "real directory")
	}

	unsafe := []string{"", ".", "..", "../escape", "/absolute", `C:\escape`}
	for _, path := range unsafe {
		clean, err := cleanOMPRootedPath(path)
		assert.Empty(t, clean, "path=%q", path)
		assert.ErrorContains(t, err, "unsafe OMP workspace path", "path=%q", path)
	}
	clean, err := cleanOMPRootedPath("nested/../safe/file")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join("safe", "file"), clean)
}

func TestOMPRootedWorkspace_ObservableFileOperationsAndFailures(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	workspace, err := openOMPRootedWorkspace(root)
	require.NoError(t, err)
	t.Cleanup(func() { _ = workspace.Close() })

	absolute, err := workspace.absolute("nested/file.txt")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(root, "nested", "file.txt"), absolute)
	_, err = workspace.absolute("../escape")
	assert.Error(t, err)

	require.NoError(t, workspace.atomicWrite("nested/file.txt", []byte("payload"), 0o640))
	data, info, err := workspace.readFile("nested/file.txt", 7)
	require.NoError(t, err)
	assert.Equal(t, []byte("payload"), data)
	assert.Equal(t, os.FileMode(0o640), info.Mode().Perm())
	_, _, err = workspace.readFile("nested/file.txt", 6)
	assert.ErrorContains(t, err, "size or IO failure")

	entries, err := workspace.readDir("nested")
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "file.txt", entries[0].Name())
	_, err = workspace.readDir("missing")
	assert.Error(t, err)
	assert.Error(t, workspace.copyFile("missing", "copy", 0o600))
	require.NoError(t, workspace.removeEmptyParents("missing/deep"))
	require.NoError(t, workspace.atomicWrite("kept/file", []byte("keep"), 0o600))
	require.NoError(t, workspace.removeEmptyParents("kept"))
	assert.FileExists(t, filepath.Join(root, "kept", "file"))

	require.NoError(t, os.Mkdir(filepath.Join(root, "directory-target"), 0o700))
	err = workspace.atomicWrite("directory-target", []byte("blocked"), 0o600)
	assert.ErrorContains(t, err, "not a regular file")
	require.NoError(t, os.Symlink("nested/file.txt", filepath.Join(root, "link-target")))
	_, _, err = workspace.openRegular("link-target")
	assert.ErrorContains(t, err, "not a regular file")
	_, _, err = workspace.openRegular("directory-target")
	assert.ErrorContains(t, err, "not a regular file")

	require.NoError(t, workspace.remove("nested", true))
	_, statErr := os.Stat(filepath.Join(root, "nested"))
	assert.ErrorIs(t, statErr, os.ErrNotExist)
}

func TestOMPRootedWorkspace_CloseAndBoundHandleErrors(t *testing.T) {
	t.Parallel()

	var nilWorkspace *ompRootedWorkspace
	assert.NoError(t, nilWorkspace.Close())

	var returnErr error = errors.New("operation failed")
	joinOMPRootedCloseError(&returnErr, errors.New("close failed"))
	assert.ErrorContains(t, returnErr, "operation failed")
	assert.ErrorContains(t, returnErr, "close rooted OMP handle: close failed")
	joinOMPRootedCloseError(&returnErr, nil)

	workspace, err := openOMPRootedWorkspace(t.TempDir())
	require.NoError(t, err)
	require.NoError(t, workspace.Close())
	assert.NoError(t, workspace.Close())
	_, err = workspace.openDir(".", false, 0)
	assert.ErrorContains(t, err, "open bound OMP workspace")
	_, _, err = workspace.openParent("file", false)
	assert.Error(t, err)
}

func TestOpenOMPRootedChild_RejectsMissingFilesAndSymlinks(t *testing.T) {
	t.Parallel()

	rootPath := t.TempDir()
	parent, err := os.OpenRoot(rootPath)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, parent.Close()) })

	child, err := openOMPRootedChild(parent, "created", true, 0o750)
	require.NoError(t, err)
	opened, err := child.Stat(".")
	require.NoError(t, err)
	assert.True(t, opened.IsDir())
	require.NoError(t, child.Close())

	for _, tc := range []struct {
		name string
		make func()
		want string
	}{
		{name: "missing", make: func() {}, want: "inspect OMP path component"},
		{name: "regular", make: func() {
			require.NoError(t, os.WriteFile(filepath.Join(rootPath, "regular"), []byte("x"), 0o600))
		}, want: "not a real directory"},
		{name: "symlink", make: func() {
			require.NoError(t, os.Symlink("created", filepath.Join(rootPath, "symlink")))
		}, want: "not a real directory"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.make()
			got, openErr := openOMPRootedChild(parent, tc.name, false, 0)
			assert.Nil(t, got)
			assert.ErrorContains(t, openErr, tc.want)
		})
	}

	assert.False(t, sameOMPRootedDirectory(nil, opened))
	assert.False(t, sameOMPRootedDirectory(parent, nil))
	assert.True(t, sameOMPRootedDirectory(parent, mustOMPRootedInfo(t, rootPath)))
}

func TestApplyOMPRootedTransaction_RollsBackNewAndExistingWrites(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeOMPDoctorOwnedFile(t, root, "existing.txt", []byte("original"), 0o640)
	writeOMPDoctorOwnedFile(t, root, "blocker", []byte("not a directory"), 0o600)
	workspace, err := openOMPRootedWorkspace(root)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, workspace.Close()) })

	journal, err := applyOMPTransactionAt(workspace, adapterName, adapter.TransactionPlan{
		Writes: []adapter.TransactionWrite{
			{Path: "new/deep/file.txt", Content: []byte("new")},
			{Path: "existing.txt", Content: []byte("managed"), Perm: 0o600},
			{Path: "blocker/late.txt", Content: []byte("must fail"), Perm: 0o600},
		},
	})
	assert.Nil(t, journal)
	require.Error(t, err)
	assert.Equal(t, []byte("original"), mustReadOMPRootedCoverageFile(t, filepath.Join(root, "existing.txt")))
	info, statErr := os.Stat(filepath.Join(root, "existing.txt"))
	require.NoError(t, statErr)
	assert.Equal(t, os.FileMode(0o640), info.Mode().Perm())
	_, statErr = os.Stat(filepath.Join(root, "new"))
	assert.ErrorIs(t, statErr, os.ErrNotExist)
}

func TestOMPRootedTransaction_FailsClosedOnUnsafeAndInvalidOperations(t *testing.T) {
	t.Parallel()

	unsafePlans := []adapter.TransactionPlan{
		{Writes: []adapter.TransactionWrite{{Path: "../escape"}}},
		{Removes: []adapter.TransactionRemove{{Path: "/absolute"}}},
		{Manifest: adapter.NewManifest("../../escape")},
	}
	for _, plan := range unsafePlans {
		assert.Error(t, validateOMPRootedTransactionPlan(plan))
	}

	root := t.TempDir()
	workspace, err := openOMPRootedWorkspace(root)
	require.NoError(t, err)
	t.Cleanup(func() { _ = workspace.Close() })
	tx, err := newOMPRootedTransaction(workspace, adapterName)
	require.NoError(t, err)

	tx.journal.Entries = []adapter.TransactionJournalEntry{{Path: "no-backup", Operation: "write"}}
	err = tx.rollback()
	assert.ErrorContains(t, err, "rollback backup missing")

	writeOMPDoctorOwnedFile(t, root, "non-empty/file", []byte("keep"), 0o600)
	tx.snapshots["non-empty"] = true
	err = tx.removePath(adapter.TransactionRemove{Path: "non-empty"})
	assert.ErrorContains(t, err, "transaction remove")
	assert.FileExists(t, filepath.Join(root, "non-empty", "file"))

	require.NoError(t, os.Mkdir(filepath.Join(root, "directory"), 0o700))
	tx.snapshots["directory"] = true
	err = tx.writeFile(adapter.TransactionWrite{Path: "directory", Content: []byte("blocked")})
	assert.ErrorContains(t, err, "transaction target is directory")

	require.NoError(t, workspace.Close())
	_, err = newOMPRootedTransaction(workspace, adapterName)
	assert.ErrorContains(t, err, "transaction dir")
}

func TestLoadOMPRootedManifest_ReturnsMissingAndParseErrors(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	workspace, err := openOMPRootedWorkspace(root)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, workspace.Close()) })

	manifest, err := loadOMPManifestAt(workspace, adapterName)
	require.NoError(t, err)
	assert.Nil(t, manifest)
	writeOMPDoctorOwnedFile(t, root, ".autopus/omp-manifest.json", []byte("{invalid"), 0o600)
	manifest, err = loadOMPManifestAt(workspace, adapterName)
	assert.Nil(t, manifest)
	assert.ErrorContains(t, err, "매니페스트 파싱 실패")
}

func mustOMPRootedInfo(t *testing.T, path string) os.FileInfo {
	t.Helper()
	info, err := os.Stat(path)
	require.NoError(t, err)
	return info
}

func mustReadOMPRootedCoverageFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return data
}
