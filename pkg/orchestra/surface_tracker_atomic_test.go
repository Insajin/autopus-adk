package orchestra

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type trackerWriteStub struct {
	info       fs.FileInfo
	writeCount int
	writeErr   error
	syncErr    error
	closeErr   error
	closed     bool
}

func (s *trackerWriteStub) Stat() (fs.FileInfo, error) { return s.info, nil }
func (s *trackerWriteStub) Chmod(os.FileMode) error    { return nil }
func (s *trackerWriteStub) Write(data []byte) (int, error) {
	if s.writeCount > 0 {
		return s.writeCount, s.writeErr
	}
	return len(data), s.writeErr
}
func (s *trackerWriteStub) Sync() error {
	return s.syncErr
}
func (s *trackerWriteStub) Close() error {
	s.closed = true
	return s.closeErr
}

func TestPersistTrackerReplacement_ShortWriteDoesNotCommit(t *testing.T) {
	dir := secureTrackerTestDir(t)
	temporary := filepath.Join(dir, "record.tmp")
	require.NoError(t, os.WriteFile(temporary, nil, 0o600))
	info, err := os.Stat(temporary)
	require.NoError(t, err)
	root, err := os.OpenRoot(dir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = root.Close() })
	commitCalled := false
	file := &trackerWriteStub{info: info, writeCount: 3}

	err = persistTrackerReplacement(root, "record.tmp", "record.surfaces", file, []byte("complete"),
		func(*os.Root, string, string) error {
			commitCalled = true
			return nil
		})

	require.ErrorIs(t, err, io.ErrShortWrite)
	assert.False(t, commitCalled)
	assert.True(t, file.closed)
}

func TestWriteTrackedSurfaces_RenameFailurePreservesOldHandle(t *testing.T) {
	dir := secureTrackerTestDir(t)
	path := filepath.Join(dir, "2147480110.surfaces")
	old := `{"surface_ref":"surface:1","terminal_kind":"cmux","workspace_ref":"workspace:13"}` + "\n"
	require.NoError(t, os.WriteFile(path, []byte(old), 0o600))
	injected := errors.New("rename failed")

	err := writeTrackedSurfacesWithCommit(path, trackedCmuxSurfaces("workspace:13", "surface:2"),
		func(*os.Root, string, string) error { return injected })

	require.ErrorIs(t, err, injected)
	data, readErr := os.ReadFile(path)
	require.NoError(t, readErr)
	assert.Equal(t, old, string(data))
}

func TestPersistTrackerReplacement_SyncFailureDoesNotCommit(t *testing.T) {
	dir := secureTrackerTestDir(t)
	root, err := os.OpenRoot(dir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = root.Close() })
	target := "2147480111.surfaces"
	temporary := target + ".tmp"
	old := []byte("surface:1\n")
	require.NoError(t, os.WriteFile(filepath.Join(dir, target), old, 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, temporary), nil, 0o600))
	info, err := os.Stat(filepath.Join(dir, temporary))
	require.NoError(t, err)
	injected := errors.New("sync failed")
	file := &trackerWriteStub{info: info, syncErr: injected}
	commitCalled := false

	err = persistTrackerReplacement(root, temporary, target, file, []byte("surface:2\n"),
		func(*os.Root, string, string) error {
			commitCalled = true
			return nil
		})

	require.ErrorIs(t, err, injected)
	assert.False(t, commitCalled)
	data, readErr := os.ReadFile(filepath.Join(dir, target))
	require.NoError(t, readErr)
	assert.Equal(t, old, data)
}

func TestTrackSurfaceRecord_ConcurrentRMWPreservesEveryHandle(t *testing.T) {
	original := surfaceTrackerBase
	surfaceTrackerBase = filepath.Join(secureTrackerTestDir(t), "surfaces")
	t.Cleanup(func() { surfaceTrackerBase = original })
	const count = 32
	var wg sync.WaitGroup
	wg.Add(count)
	for index := 0; index < count; index++ {
		index := index
		go func() {
			defer wg.Done()
			trackSurfaceRecord(trackedSurface{
				Ref: "surface:" + decimalString(index+1), TerminalKind: "cmux", WorkspaceRef: "workspace:13",
			})
		}()
	}

	wg.Wait()

	tracked := readTrackedSurfaces(surfaceTrackerFile(os.Getpid()))
	require.Len(t, tracked, count)
	seen := make(map[string]struct{}, count)
	for _, item := range tracked {
		seen[item.Ref] = struct{}{}
	}
	assert.Len(t, seen, count)
}

func decimalString(value int) string {
	const digits = "0123456789"
	if value == 0 {
		return "0"
	}
	var raw [20]byte
	index := len(raw)
	for value > 0 {
		index--
		raw[index] = digits[value%10]
		value /= 10
	}
	return string(raw[index:])
}
