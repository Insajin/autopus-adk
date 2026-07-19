package orchestra

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHookSessionResetAttempt_RemovesOnlyTargetProviderRound(t *testing.T) {
	t.Parallel()

	session := testHookSessionAt(t.TempDir())
	targets := []string{
		RoundSignalName("codex", 2, "done"),
		RoundSignalName("codex", 2, "result.json"),
		RoundSignalName("codex", 2, "ready"),
		RoundSignalName("codex", 2, "input.json"),
		RoundSignalName("codex", 2, "abort"),
	}
	preserved := []string{
		RoundSignalName("claude", 2, "done"),
		RoundSignalName("claude", 2, "result.json"),
		RoundSignalName("codex", 1, "done"),
		RoundSignalName("codex", 1, "result.json"),
		RoundSignalName("codex", 3, "done"),
		RoundSignalName("codex", 3, "result.json"),
	}
	writeAttemptFiles(t, session, append(targets, preserved...))

	require.NoError(t, session.ResetAttempt("codex", 2))
	assertAttemptFilesMissing(t, session, targets)
	assertAttemptFilesPresent(t, session, preserved)

	// Reset is intentionally idempotent so every retry may call it before launch.
	require.NoError(t, session.ResetAttempt("codex", 2))
}

func TestHookSessionResetAttempt_UnscopedAttemptPreservesSiblingAndRounds(t *testing.T) {
	t.Parallel()

	session := testHookSessionAt(t.TempDir())
	targets := []string{
		"codex-done",
		"codex-result.json",
		RoundSignalName("codex", 0, "ready"),
		RoundSignalName("codex", 0, "input.json"),
		RoundSignalName("codex", 0, "abort"),
	}
	preserved := []string{
		"claude-done",
		"claude-result.json",
		RoundSignalName("codex", 1, "done"),
		RoundSignalName("codex", 1, "result.json"),
	}
	writeAttemptFiles(t, session, append(targets, preserved...))

	require.NoError(t, session.ResetAttempt("codex", 0))
	assertAttemptFilesMissing(t, session, targets)
	assertAttemptFilesPresent(t, session, preserved)
}

func TestHookSessionResetAttempt_ReturnsRemovalError(t *testing.T) {
	t.Parallel()

	session := testHookSessionAt(t.TempDir())
	blockedPath := filepath.Join(session.Dir(), "codex-done")
	require.NoError(t, os.MkdirAll(filepath.Join(blockedPath, "child"), 0o700))

	err := session.ResetAttempt("codex", 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "codex-done")
}

func TestHookSessionResetAttempt_RejectsCollidingProviderName(t *testing.T) {
	t.Parallel()

	session := testHookSessionAt(t.TempDir())
	preserved := []string{"codex-done", "codex-result.json"}
	writeAttemptFiles(t, session, preserved)

	err := session.ResetAttempt("co/dex", 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "provider name")
	assertAttemptFilesPresent(t, session, preserved)
}

func writeAttemptFiles(t *testing.T, session *HookSession, names []string) {
	t.Helper()
	for _, name := range names {
		require.NoError(t, os.WriteFile(filepath.Join(session.Dir(), name), []byte("stale"), 0o600))
	}
}

func assertAttemptFilesMissing(t *testing.T, session *HookSession, names []string) {
	t.Helper()
	for _, name := range names {
		_, err := os.Stat(filepath.Join(session.Dir(), name))
		assert.ErrorIs(t, err, os.ErrNotExist, "%s must be reset", name)
	}
}

func assertAttemptFilesPresent(t *testing.T, session *HookSession, names []string) {
	t.Helper()
	for _, name := range names {
		_, err := os.Stat(filepath.Join(session.Dir(), name))
		assert.NoError(t, err, "%s belongs to a sibling provider or different round", name)
	}
}
