package orchestra

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func hookBasePath() string {
	return filepath.Join(os.TempDir(), "autopus")
}

func TestNewHookSession_RejectsEmptyAndUnsafeIDs(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	for _, id := range []string{"", ".", "..", "../session", "a/b", `a\b`, "has space"} {
		t.Run(id, func(t *testing.T) {
			session, err := NewHookSession(id)
			assert.Nil(t, session)
			assert.Error(t, err)
		})
	}
}

func TestNewHookSession_RejectsSymlinkedBase(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires elevated Windows privileges")
	}
	t.Setenv("TMPDIR", t.TempDir())
	external := filepath.Join(os.TempDir(), "external-base")
	require.NoError(t, os.Mkdir(external, 0o700))
	require.NoError(t, os.Symlink(external, hookBasePath()))

	session, err := NewHookSession("safe-session")

	assert.Nil(t, session)
	assert.Error(t, err)
	_, statErr := os.Stat(filepath.Join(external, "safe-session"))
	assert.ErrorIs(t, statErr, os.ErrNotExist)
}

func TestNewHookSession_RejectsSymlinkedSession(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires elevated Windows privileges")
	}
	t.Setenv("TMPDIR", t.TempDir())
	require.NoError(t, os.Mkdir(hookBasePath(), 0o700))
	external := filepath.Join(os.TempDir(), "external-session")
	require.NoError(t, os.Mkdir(external, 0o700))
	require.NoError(t, os.Symlink(external, filepath.Join(hookBasePath(), "linked-session")))

	session, err := NewHookSession("linked-session")

	assert.Nil(t, session)
	assert.Error(t, err)
	assert.DirExists(t, external)
}

func TestNewHookSession_RejectsInsecurePreclaimedDirectories(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix directory permissions are not available")
	}
	for _, target := range []string{"base", "session"} {
		t.Run(target, func(t *testing.T) {
			t.Setenv("TMPDIR", t.TempDir())
			base := hookBasePath()
			require.NoError(t, os.Mkdir(base, 0o700))
			if target == "session" {
				require.NoError(t, os.Mkdir(filepath.Join(base, "preclaimed"), 0o700))
				require.NoError(t, os.Chmod(filepath.Join(base, "preclaimed"), 0o755))
			} else {
				require.NoError(t, os.Chmod(base, 0o755))
			}

			session, err := NewHookSession("preclaimed")
			assert.Nil(t, session)
			assert.ErrorContains(t, err, "permissions")
		})
	}
}

func TestHookSession_NonOwnerCleanupPreservesSecureExistingDirectory(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	dir := filepath.Join(hookBasePath(), "shared-session")
	require.NoError(t, os.MkdirAll(dir, 0o700))
	sentinel := filepath.Join(dir, "sentinel")
	require.NoError(t, os.WriteFile(sentinel, []byte("shared"), 0o600))

	session, err := NewHookSession("shared-session")
	require.NoError(t, err)
	session.Cleanup()

	assert.DirExists(t, dir)
	assert.FileExists(t, sentinel)
}

func TestHookSession_OwnerCleanupPreservesRenamedAndReplacementDirectories(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("open-directory rename behavior is platform-specific")
	}
	t.Setenv("TMPDIR", t.TempDir())
	session, err := NewHookSession("owner-session")
	require.NoError(t, err)
	ownedSentinel := filepath.Join(session.Dir(), "owned-sentinel")
	require.NoError(t, os.WriteFile(ownedSentinel, []byte("owned"), 0o600))

	moved := filepath.Join(hookBasePath(), "moved-owner-session")
	require.NoError(t, os.Rename(session.Dir(), moved))
	require.NoError(t, os.Mkdir(session.Dir(), 0o700))
	replacementSentinel := filepath.Join(session.Dir(), "replacement-sentinel")
	require.NoError(t, os.WriteFile(replacementSentinel, []byte("replacement"), 0o600))

	session.Cleanup()

	assert.FileExists(t, filepath.Join(moved, "owned-sentinel"))
	assert.FileExists(t, replacementSentinel)
}

func TestHookSession_OwnerCleanupDoesNotFollowExternalSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires elevated Windows privileges")
	}
	t.Setenv("TMPDIR", t.TempDir())
	session, err := NewHookSession("symlink-content")
	require.NoError(t, err)
	external := filepath.Join(os.TempDir(), "external-target")
	require.NoError(t, os.WriteFile(external, []byte("keep"), 0o600))
	require.NoError(t, os.Symlink(external, filepath.Join(session.Dir(), "outside-link")))

	session.Cleanup()

	data, err := os.ReadFile(external)
	require.NoError(t, err)
	assert.Equal(t, "keep", string(data))
	assert.NoDirExists(t, session.Dir())
}

func TestHookSession_ArtifactMethodsRejectUnsafeProvider(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	session, err := NewHookSession("artifact-validation")
	require.NoError(t, err)
	defer session.Cleanup()
	const unsafeProvider = "../claude"
	tests := []struct {
		name string
		run  func() error
	}{
		{name: "wait done", run: func() error { return session.WaitForDone(time.Millisecond, unsafeProvider) }},
		{name: "wait done round", run: func() error { return session.WaitForDoneRound(time.Millisecond, unsafeProvider, 1) }},
		{name: "wait done round ctx", run: func() error {
			return session.WaitForDoneRoundCtx(context.Background(), time.Millisecond, unsafeProvider, 1)
		}},
		{name: "read result", run: func() error { _, err := session.ReadResult(unsafeProvider); return err }},
		{name: "read result round", run: func() error { _, err := session.ReadResultRound(unsafeProvider, 1); return err }},
		{name: "write input", run: func() error { return session.WriteInput(unsafeProvider, "prompt") }},
		{name: "write input round", run: func() error { return session.WriteInputRound(unsafeProvider, 1, "prompt") }},
		{name: "wait ready", run: func() error { return session.WaitForReady(time.Millisecond, unsafeProvider, 1) }},
		{name: "wait ready ctx", run: func() error {
			return session.WaitForReadyCtx(context.Background(), time.Millisecond, unsafeProvider, 1)
		}},
		{name: "write abort", run: func() error { return session.WriteAbortSignal(unsafeProvider, 1) }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.ErrorContains(t, test.run(), "unsafe")
		})
	}
}

func TestHookSession_OwnerAndReuserCleanupOrder(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	owner, err := NewHookSession("shared-owner")
	require.NoError(t, err)
	reuser, err := NewHookSession("shared-owner")
	require.NoError(t, err)
	require.NoError(t, reuser.WriteInput("claude", "prompt"))

	reuser.Cleanup()
	assert.DirExists(t, owner.Dir())
	assert.FileExists(t, filepath.Join(owner.Dir(), RoundSignalName("claude", 0, "input.json")))

	owner.Cleanup()
	assert.NoDirExists(t, owner.Dir())
}
