package orchestra

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type failingSessionFile struct {
	info       fs.FileInfo
	failPhase  string
	closeCalls int
}

func (f *failingSessionFile) Stat() (fs.FileInfo, error) { return f.info, nil }
func (f *failingSessionFile) Chmod(os.FileMode) error    { return nil }
func (f *failingSessionFile) Write(data []byte) (int, error) {
	if f.failPhase == "write" {
		return 0, errors.New("injected write failure")
	}
	return len(data), nil
}
func (f *failingSessionFile) Sync() error {
	if f.failPhase == "sync" {
		return errors.New("injected sync failure")
	}
	return nil
}
func (f *failingSessionFile) Close() error {
	f.closeCalls++
	if f.failPhase == "close" {
		return errors.New("injected close failure")
	}
	return nil
}

func TestPersistCreatedSession_FailureRollsBackOwnedEntry(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	root, err := openSessionRoot(true)
	require.NoError(t, err)
	t.Cleanup(func() { _ = root.Close() })

	for _, phase := range []string{"write", "sync", "close"} {
		t.Run(phase, func(t *testing.T) {
			name := phase + ".json"
			path := filepath.Join(sessionDirectoryPath(), name)
			require.NoError(t, os.WriteFile(path, []byte("partial"), 0o600))
			info, statErr := os.Stat(path)
			require.NoError(t, statErr)
			file := &failingSessionFile{info: info, failPhase: phase}

			err := persistCreatedSession(root, name, file, []byte("payload"))

			require.ErrorContains(t, err, phase+" session")
			assert.Equal(t, 1, file.closeCalls)
			_, statErr = os.Stat(path)
			assert.ErrorIs(t, statErr, os.ErrNotExist)
		})
	}
}

func TestPersistCreatedSession_RollbackDoesNotDeleteReplacement(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("replacement identity behavior is platform-specific")
	}
	t.Setenv("TMPDIR", t.TempDir())
	root, err := openSessionRoot(true)
	require.NoError(t, err)
	t.Cleanup(func() { _ = root.Close() })

	name := "replacement.json"
	path := filepath.Join(sessionDirectoryPath(), name)
	require.NoError(t, os.WriteFile(path, []byte("owned"), 0o600))
	ownedInfo, err := os.Stat(path)
	require.NoError(t, err)
	replacement := filepath.Join(sessionDirectoryPath(), "replacement-source.json")
	require.NoError(t, os.WriteFile(replacement, []byte("replacement"), 0o600))
	require.NoError(t, os.Rename(replacement, path))

	err = persistCreatedSession(root, name, &failingSessionFile{
		info: ownedInfo, failPhase: "write",
	}, []byte("payload"))

	require.Error(t, err)
	data, readErr := os.ReadFile(path)
	require.NoError(t, readErr)
	assert.Equal(t, "replacement", string(data))
}
