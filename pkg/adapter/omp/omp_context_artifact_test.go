package omp

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWithOMPContextArtifact_CleansEveryTerminalPathAndPreservesUserConfig(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	userConfig := filepath.Join(base, "user-config.yml")
	require.NoError(t, os.WriteFile(userConfig, []byte("memory: user\n"), 0o640))
	before, err := os.ReadFile(userConfig)
	require.NoError(t, err)
	for _, terminalErr := range []error{nil, errors.New("abort"), errors.New("rollback")} {
		receipt, runErr := WithOMPContextArtifact(base, []string{base}, func(artifact *OMPContextArtifact) error {
			require.NoError(t, artifact.WriteOneShotOverlay([]byte("compaction: shadow\n")))
			rootInfo, statErr := os.Stat(artifact.root)
			require.NoError(t, statErr)
			require.LessOrEqual(t, rootInfo.Mode().Perm(), os.FileMode(0o700))
			overlayInfo, statErr := os.Stat(filepath.Join(artifact.root, "session-overlay.yml"))
			require.NoError(t, statErr)
			require.LessOrEqual(t, overlayInfo.Mode().Perm(), os.FileMode(0o600))
			return terminalErr
		})
		if terminalErr == nil {
			require.NoError(t, runErr)
		} else {
			require.ErrorIs(t, runErr, terminalErr)
		}
		require.Equal(t, 0, receipt.PostCleanupExistenceCount)
		require.Equal(t, "cleaned", receipt.CleanupStatus)
	}
	after, err := os.ReadFile(userConfig)
	require.NoError(t, err)
	require.Equal(t, before, after)
}

func TestCreateOMPContextArtifact_RejectsSymlinkAndEscape(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	target := t.TempDir()
	link := filepath.Join(base, "link")
	require.NoError(t, os.Symlink(target, link))
	_, err := CreateOMPContextArtifact(link, []string{base})
	require.ErrorContains(t, err, "artifact_base_symlink")
	_, err = CreateOMPContextArtifact(base, []string{target})
	require.ErrorContains(t, err, "artifact_base_outside_allowed_roots")
}

func TestCreateOMPContextArtifact_AncestorSymlinkEscapeCreatesNoExternalArtifact(t *testing.T) {
	t.Parallel()
	allowed := t.TempDir()
	outside := t.TempDir()
	externalBase := filepath.Join(outside, "subdir")
	require.NoError(t, os.Mkdir(externalBase, 0o700))
	require.NoError(t, os.Symlink(outside, filepath.Join(allowed, "link")))

	artifact, err := CreateOMPContextArtifact(filepath.Join(allowed, "link", "subdir"), []string{allowed})
	if artifact != nil {
		t.Cleanup(func() { artifact.Cleanup("abort") })
	}
	assert.Nil(t, artifact)
	assert.ErrorContains(t, err, "artifact_base_symlink")
	entries, readErr := os.ReadDir(externalBase)
	require.NoError(t, readErr)
	assert.Empty(t, entries, "rejected creation must not write outside the allowed root")
}

func TestCreateOMPContextArtifact_RejectsSymlinkedAllowedRootAndBaseComponents(t *testing.T) {
	t.Parallel()

	t.Run("symlinked allowed root", func(t *testing.T) {
		realRoot := t.TempDir()
		base := filepath.Join(realRoot, "nested")
		require.NoError(t, os.Mkdir(base, 0o700))
		allowedRoot := filepath.Join(t.TempDir(), "allowed")
		require.NoError(t, os.Symlink(realRoot, allowedRoot))

		artifact, err := CreateOMPContextArtifact(filepath.Join(allowedRoot, "nested"), []string{allowedRoot})
		if artifact != nil {
			t.Cleanup(func() { artifact.Cleanup("abort") })
		}
		assert.Nil(t, artifact)
		assert.ErrorContains(t, err, "artifact_allowed_root_symlink")
		entries, readErr := os.ReadDir(base)
		require.NoError(t, readErr)
		assert.Empty(t, entries)
	})

	t.Run("ancestor symlink within allowed root", func(t *testing.T) {
		allowed := t.TempDir()
		realParent := filepath.Join(allowed, "real")
		base := filepath.Join(realParent, "nested")
		require.NoError(t, os.MkdirAll(base, 0o700))
		require.NoError(t, os.Symlink(realParent, filepath.Join(allowed, "link")))

		artifact, err := CreateOMPContextArtifact(filepath.Join(allowed, "link", "nested"), []string{allowed})
		if artifact != nil {
			t.Cleanup(func() { artifact.Cleanup("abort") })
		}
		assert.Nil(t, artifact)
		assert.ErrorContains(t, err, "artifact_base_symlink")
		entries, readErr := os.ReadDir(base)
		require.NoError(t, readErr)
		assert.Empty(t, entries)
	})
}

func TestCreateOMPContextArtifact_AllowsNestedRealPath(t *testing.T) {
	t.Parallel()
	allowed := t.TempDir()
	base := filepath.Join(allowed, "real", "nested")
	require.NoError(t, os.MkdirAll(base, 0o700))

	artifact, err := CreateOMPContextArtifact(base, []string{allowed})
	require.NoError(t, err)
	require.NotNil(t, artifact)
	require.True(t, pathWithinOMPContextRoot(base, artifact.root))
	receipt := artifact.Cleanup("success")
	require.Equal(t, "cleaned", receipt.CleanupStatus)
}

func TestCreateOMPContextArtifact_UnwritableBaseFailsWithoutArtifact(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX directory permissions are required")
	}
	t.Parallel()
	base := t.TempDir()
	require.NoError(t, os.Chmod(base, 0o500))
	t.Cleanup(func() { require.NoError(t, os.Chmod(base, 0o700)) })

	artifact, err := CreateOMPContextArtifact(base, []string{base})
	require.Nil(t, artifact)
	require.Error(t, err)
	entries, readErr := os.ReadDir(base)
	require.NoError(t, readErr)
	require.Empty(t, entries)
}

func TestOMPContextArtifact_AncestorSwapDoesNotRedirectWriteOrCleanup(t *testing.T) {
	t.Parallel()
	allowed := t.TempDir()
	parent := filepath.Join(allowed, "parent")
	base := filepath.Join(parent, "base")
	require.NoError(t, os.MkdirAll(base, 0o700))
	artifact, err := CreateOMPContextArtifact(base, []string{allowed})
	require.NoError(t, err)
	artifactName := filepath.Base(artifact.root)

	movedParent := filepath.Join(allowed, "moved-parent")
	require.NoError(t, os.Rename(parent, movedParent))
	outside := t.TempDir()
	externalBase := filepath.Join(outside, "base")
	require.NoError(t, os.Mkdir(externalBase, 0o700))
	require.NoError(t, os.Symlink(outside, parent))
	externalArtifact := filepath.Join(externalBase, artifactName)
	require.NoError(t, os.Mkdir(externalArtifact, 0o700))
	sentinel := filepath.Join(externalArtifact, "sentinel")
	require.NoError(t, os.WriteFile(sentinel, []byte("external"), 0o600))

	require.NoError(t, artifact.WriteOneShotOverlay([]byte("compaction: anchored\n")))
	require.FileExists(t, filepath.Join(movedParent, "base", artifactName, "session-overlay.yml"))
	require.NoFileExists(t, filepath.Join(externalArtifact, "session-overlay.yml"))

	receipt := artifact.Cleanup("success")
	require.Equal(t, "cleaned", receipt.CleanupStatus)
	require.NoDirExists(t, filepath.Join(movedParent, "base", artifactName))
	contents, err := os.ReadFile(sentinel)
	require.NoError(t, err)
	require.Equal(t, []byte("external"), contents)
}

func TestOMPContextArtifact_ReplacedArtifactBindingFailsClosed(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	artifact, err := CreateOMPContextArtifact(base, []string{base})
	require.NoError(t, err)
	original := artifact.root
	require.NoError(t, os.Rename(original, original+"-moved"))
	external := t.TempDir()
	sentinel := filepath.Join(external, "sentinel")
	require.NoError(t, os.WriteFile(sentinel, []byte("external"), 0o600))
	require.NoError(t, os.Symlink(external, original))

	require.ErrorContains(t, artifact.WriteOneShotOverlay([]byte("escape")), "artifact_root_invalid")
	receipt := artifact.Cleanup("abort")
	require.Equal(t, "cleanup_unverified", receipt.CleanupStatus)
	require.Equal(t, 1, receipt.PostCleanupExistenceCount)
	contents, err := os.ReadFile(sentinel)
	require.NoError(t, err)
	require.Equal(t, []byte("external"), contents)
}

func TestOMPContextArtifact_AfterCleanupAndInvalidInputsFailClosed(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	artifact, err := CreateOMPContextArtifact(base, []string{base})
	require.NoError(t, err)
	require.NoError(t, artifact.WriteOneShotOverlay([]byte("one-shot")))
	require.Error(t, artifact.WriteOneShotOverlay([]byte("second-write")))
	receipt := artifact.Cleanup("fallback")
	require.Equal(t, "fallback", receipt.Reason)
	require.ErrorContains(t, artifact.WriteOneShotOverlay([]byte("x")), "artifact_unavailable")
	secondReceipt := artifact.Cleanup("canary")
	require.Equal(t, "cleaned", secondReceipt.CleanupStatus)
	require.Equal(t, "canary", secondReceipt.Reason)
	require.ErrorContains(t, (*OMPContextArtifact)(nil).WriteOneShotOverlay([]byte("x")), "artifact_unavailable")
	require.ErrorContains(t, (&OMPContextArtifact{}).WriteOneShotOverlay([]byte("x")), "artifact_root_invalid")

	nilReceipt := (*OMPContextArtifact)(nil).Cleanup("unknown")
	require.Equal(t, "cleanup_unverified", nilReceipt.CleanupStatus)
	require.Equal(t, "abort", nilReceipt.Reason)
	_, err = CreateOMPContextArtifact(filepath.Join(base, "missing"), []string{base})
	require.ErrorContains(t, err, "artifact_base_invalid")
	fileBase := filepath.Join(base, "file")
	require.NoError(t, os.WriteFile(fileBase, []byte("x"), 0o600))
	_, err = CreateOMPContextArtifact(fileBase, []string{base})
	require.ErrorContains(t, err, "artifact_base_outside_allowed_roots")
	receipt, err = WithOMPContextArtifact(filepath.Join(base, "missing"), []string{base}, func(*OMPContextArtifact) error {
		t.Fatal("run must not be called when creation fails")
		return nil
	})
	require.Error(t, err)
	require.Equal(t, "not_created", receipt.CleanupStatus)
	require.Equal(t, "create_failed", receipt.Reason)
}

func TestWithOMPContextArtifact_BindingTamperReportsCleanupFailure(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	receipt, err := WithOMPContextArtifact(base, []string{base}, func(artifact *OMPContextArtifact) error {
		require.NoError(t, os.Rename(artifact.root, artifact.root+"-moved"))
		return os.Symlink(t.TempDir(), artifact.root)
	})
	require.ErrorContains(t, err, "artifact_cleanup_unverified")
	require.Equal(t, "cleanup_unverified", receipt.CleanupStatus)
	require.Equal(t, 1, receipt.PostCleanupExistenceCount)
}

func TestOMPContextArtifact_InternalDirectoryHelpersFailClosed(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(base, "file"), []byte("x"), 0o600))
	require.NoError(t, os.Symlink(base, filepath.Join(base, "link")))
	root, err := os.OpenRoot(base)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, root.Close()) })

	_, err = openOMPContextRealSubdirectory(root, "missing")
	require.ErrorContains(t, err, "artifact_base_invalid")
	_, err = openOMPContextRealSubdirectory(root, "file")
	require.ErrorContains(t, err, "artifact_base_invalid")
	_, err = openOMPContextRealSubdirectory(root, "link")
	require.ErrorContains(t, err, "artifact_base_symlink")
	otherInfo, err := os.Lstat(t.TempDir())
	require.NoError(t, err)
	opened, err := openOMPContextArtifactBase(base, otherInfo, []string{base})
	require.Nil(t, opened)
	require.ErrorContains(t, err, "artifact_base_changed_or_outside_allowed_root")
	missingRoot := filepath.Join(t.TempDir(), "missing")
	opened, err = openOMPContextArtifactBase(filepath.Join(missingRoot, "nested"), otherInfo, []string{missingRoot})
	require.Nil(t, opened)
	require.ErrorContains(t, err, "artifact_allowed_root_invalid")

	closedRoot, err := os.OpenRoot(base)
	require.NoError(t, err)
	require.NoError(t, closedRoot.Close())
	_, _, _, err = createOMPContextArtifactDirectory(closedRoot)
	require.Error(t, err)
}
