package cli

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestModTime_ExistingFile returns the file's modification time.
func TestModTime_ExistingFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")
	require.NoError(t, os.WriteFile(path, []byte("hello"), 0o644))

	got := modTime(path)
	assert.False(t, got.IsZero(), "modTime should return a non-zero time for an existing file")
	assert.WithinDuration(t, time.Now(), got, 5*time.Second)
}

// TestModTime_MissingFile returns zero time.
func TestModTime_MissingFile(t *testing.T) {
	t.Parallel()

	got := modTime("/nonexistent/path/file.txt")
	assert.True(t, got.IsZero(), "modTime should return zero for a missing file")
}
