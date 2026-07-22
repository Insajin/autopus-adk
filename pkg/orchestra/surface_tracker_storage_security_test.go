package orchestra

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReapOrphanSurfaces_InsecureLegacyDirectoryRejectsStructuredCapability(t *testing.T) {
	originalBase := surfaceTrackerBase
	originalLegacy := surfaceTrackerLegacyBase
	t.Cleanup(func() {
		surfaceTrackerBase = originalBase
		surfaceTrackerLegacyBase = originalLegacy
	})
	surfaceTrackerBase = filepath.Join(secureTrackerTestDir(t), "primary-missing")
	legacy := filepath.Join(secureTrackerTestDir(t), "legacy")
	require.NoError(t, os.Mkdir(legacy, 0o700))
	require.NoError(t, os.Chmod(legacy, 0o755))
	surfaceTrackerLegacyBase = legacy
	path := filepath.Join(legacy, "2147480101.surfaces")
	require.NoError(t, os.WriteFile(path, []byte(
		`{"surface_ref":"surface:7","terminal_kind":"cmux","workspace_ref":"workspace:13"}`+"\n",
	), 0o600))
	term := newTrackerContextTerminal("workspace:current")

	ReapOrphanSurfaces(term)

	assert.Empty(t, term.state.closeCalls)
	_, err := os.Lstat(path)
	assert.NoError(t, err, "untrusted input must remain untouched")
}

func TestReapOrphanSurfaces_SymlinkLegacyDirectoryFailsClosed(t *testing.T) {
	originalBase := surfaceTrackerBase
	originalLegacy := surfaceTrackerLegacyBase
	t.Cleanup(func() {
		surfaceTrackerBase = originalBase
		surfaceTrackerLegacyBase = originalLegacy
	})
	surfaceTrackerBase = filepath.Join(secureTrackerTestDir(t), "primary-missing")
	target := secureTrackerTestDir(t)
	writeRawTrackerForTest(t, target, 2147480102,
		`{"surface_ref":"surface:8","terminal_kind":"cmux","workspace_ref":"workspace:13"}`)
	legacy := filepath.Join(secureTrackerTestDir(t), "legacy-link")
	require.NoError(t, os.Symlink(target, legacy))
	surfaceTrackerLegacyBase = legacy
	term := newTrackerContextTerminal("workspace:current")

	ReapOrphanSurfaces(term)

	assert.Empty(t, term.state.closeCalls)
}

func TestReapOrphanSurfaces_RejectsSymlinkAndWrongModeFiles(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*testing.T, string, string)
	}{
		{
			name: "symlink",
			setup: func(t *testing.T, dir, path string) {
				t.Helper()
				target := filepath.Join(dir, "target")
				require.NoError(t, os.WriteFile(target, []byte(
					`{"surface_ref":"surface:9","terminal_kind":"cmux","workspace_ref":"workspace:13"}`+"\n",
				), 0o600))
				require.NoError(t, os.Symlink(target, path))
			},
		},
		{
			name: "wrong mode",
			setup: func(t *testing.T, _, path string) {
				t.Helper()
				require.NoError(t, os.WriteFile(path, []byte(
					`{"surface_ref":"surface:9","terminal_kind":"cmux","workspace_ref":"workspace:13"}`+"\n",
				), 0o644))
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			original := surfaceTrackerBase
			dir := secureTrackerTestDir(t)
			surfaceTrackerBase = dir
			t.Cleanup(func() { surfaceTrackerBase = original })
			path := surfaceTrackerFile(2147480103)
			test.setup(t, dir, path)
			term := newTrackerContextTerminal("workspace:current")

			ReapOrphanSurfaces(term)

			assert.Empty(t, term.state.closeCalls)
		})
	}
}

func TestReapOrphanSurfaces_OversizedTrackerFailsClosed(t *testing.T) {
	original := surfaceTrackerBase
	surfaceTrackerBase = secureTrackerTestDir(t)
	t.Cleanup(func() { surfaceTrackerBase = original })
	path := surfaceTrackerFile(2147480104)
	require.NoError(t, os.WriteFile(path, []byte(strings.Repeat("x", maxTrackerFileSize+1)), 0o600))
	term := newTrackerContextTerminal("workspace:current")

	ReapOrphanSurfaces(term)

	assert.Empty(t, term.state.closeCalls)
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, int64(maxTrackerFileSize+1), info.Size())
}

func TestReapOrphanSurfaces_TruncatedRecordIsRetained(t *testing.T) {
	original := surfaceTrackerBase
	surfaceTrackerBase = secureTrackerTestDir(t)
	t.Cleanup(func() { surfaceTrackerBase = original })
	path := surfaceTrackerFile(2147480105)
	truncated := `{"surface_ref":"surface:10","terminal_kind":"cmux"`
	valid := `{"surface_ref":"surface:11","terminal_kind":"cmux","workspace_ref":"workspace:13"}`
	require.NoError(t, os.WriteFile(path, []byte(truncated+"\n"+valid+"\n"), 0o600))
	term := newTrackerContextTerminal("workspace:current")

	ReapOrphanSurfaces(term)

	assert.Equal(t, []string{"surface:11"}, term.state.closeCalls)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, truncated+"\n", string(data))
}

func TestReapOrphanSurfaces_FIFODoesNotBlock(t *testing.T) {
	if !fifoSupported() {
		t.Skip("FIFO is not supported on this platform")
	}
	original := surfaceTrackerBase
	surfaceTrackerBase = secureTrackerTestDir(t)
	t.Cleanup(func() { surfaceTrackerBase = original })
	path := surfaceTrackerFile(2147480106)
	require.NoError(t, createTrackerFIFO(path))
	term := newTrackerContextTerminal("workspace:current")
	done := make(chan struct{})
	go func() {
		ReapOrphanSurfaces(term)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		unblockTrackerFIFO(path)
		<-done
		t.Fatal("reaper blocked while opening a FIFO tracker entry")
	}
	assert.Empty(t, term.state.closeCalls)
}

func writeRawTrackerForTest(t *testing.T, dir string, pid int, line string) string {
	t.Helper()
	path := filepath.Join(dir, strconv.Itoa(pid)+".surfaces")
	require.NoError(t, os.WriteFile(path, []byte(line+"\n"), 0o600))
	return path
}
