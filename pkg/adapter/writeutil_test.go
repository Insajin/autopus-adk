package adapter

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteFileIfChanged_NewFile(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "new.json")
	err := WriteFileIfChanged(path, []byte(`{"key":"value"}`), 0644)
	require.NoError(t, err)
	data, _ := os.ReadFile(path)
	assert.Equal(t, `{"key":"value"}`, string(data))
}

func TestWriteFileIfChanged_SameContent_SkipsWrite(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "same.json")
	content := []byte(`{"key":"value"}`)
	require.NoError(t, os.WriteFile(path, content, 0644))

	// Record mtime before
	info1, _ := os.Stat(path)
	mtime1 := info1.ModTime()

	// Small delay to ensure mtime would differ if written
	time.Sleep(50 * time.Millisecond)

	err := WriteFileIfChanged(path, content, 0644)
	require.NoError(t, err)

	// Verify mtime unchanged
	info2, _ := os.Stat(path)
	assert.Equal(t, mtime1, info2.ModTime())
}

func TestWriteFileIfChanged_DifferentContent_Writes(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "diff.json")
	require.NoError(t, os.WriteFile(path, []byte("old"), 0644))

	err := WriteFileIfChanged(path, []byte("new"), 0644)
	require.NoError(t, err)
	data, _ := os.ReadFile(path)
	assert.Equal(t, "new", string(data))
}
