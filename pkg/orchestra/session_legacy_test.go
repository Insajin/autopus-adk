package orchestra

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func legacySessionTestPath(id string) string {
	return filepath.Join(os.TempDir(), "autopus-orch-session-"+id+".json")
}

func writeLegacySessionForTest(t *testing.T, session OrchestraSession) string {
	t.Helper()
	data, err := json.Marshal(session)
	require.NoError(t, err)
	path := legacySessionTestPath(session.ID)
	require.NoError(t, os.WriteFile(path, data, 0o600))
	return path
}

func TestLegacySession_LoadAndRemoveCompatibility(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	want := OrchestraSession{
		ID:        "legacy-compatible",
		Panes:     map[string]string{"claude": "pane-1"},
		CreatedAt: time.Now().UTC().Truncate(time.Second),
	}
	path := writeLegacySessionForTest(t, want)

	got, err := LoadSession(want.ID)
	require.NoError(t, err)
	assert.Equal(t, want.ID, got.ID)
	assert.Equal(t, want.Panes, got.Panes)

	require.NoError(t, RemoveSession(want.ID))
	_, err = os.Lstat(path)
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func TestLegacySession_SymlinkIsRejectedAndPreserved(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires elevated Windows privileges")
	}
	t.Setenv("TMPDIR", t.TempDir())
	target := filepath.Join(os.TempDir(), "outside.json")
	require.NoError(t, os.WriteFile(target, []byte(`{"id":"legacy-linked"}`), 0o600))
	link := legacySessionTestPath("legacy-linked")
	require.NoError(t, os.Symlink(target, link))

	_, loadErr := LoadSession("legacy-linked")
	assert.Error(t, loadErr)
	assert.Error(t, RemoveSession("legacy-linked"))

	info, err := os.Lstat(link)
	require.NoError(t, err)
	assert.NotZero(t, info.Mode()&os.ModeSymlink)
	data, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, `{"id":"legacy-linked"}`, string(data))
}

func TestLegacySession_ForeignPersistedIDIsPreserved(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	path := legacySessionTestPath("requested")
	writeLegacySessionForTest(t, OrchestraSession{ID: "requested"})
	replacement := filepath.Join(os.TempDir(), "foreign-replacement.json")
	require.NoError(t, os.WriteFile(replacement, []byte(`{"id":"foreign"}`), 0o600))
	require.NoError(t, os.Rename(replacement, path))

	_, loadErr := LoadSession("requested")
	assert.ErrorContains(t, loadErr, "does not match")
	assert.ErrorContains(t, RemoveSession("requested"), "does not match")

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, `{"id":"foreign"}`, string(data))
}

func TestLegacySession_DoesNotBypassUnsafeNewEntry(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires elevated Windows privileges")
	}
	t.Setenv("TMPDIR", t.TempDir())
	writeLegacySessionForTest(t, OrchestraSession{ID: "blocked-fallback"})
	require.NoError(t, os.Mkdir(sessionDirectoryPath(), 0o700))
	target := filepath.Join(os.TempDir(), "new-target.json")
	require.NoError(t, os.WriteFile(target, []byte(`{"id":"blocked-fallback"}`), 0o600))
	newEntry := isolatedSessionPath(t, "blocked-fallback")
	require.NoError(t, os.Symlink(target, newEntry))

	_, loadErr := LoadSession("blocked-fallback")
	assert.Error(t, loadErr)
	assert.Error(t, RemoveSession("blocked-fallback"))

	info, err := os.Lstat(newEntry)
	require.NoError(t, err)
	assert.NotZero(t, info.Mode()&os.ModeSymlink)
	_, err = os.Stat(legacySessionTestPath("blocked-fallback"))
	assert.NoError(t, err)
}
