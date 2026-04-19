package audit

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRotatingWriter_WriteAppends(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	w, err := NewRotatingWriter(path, 1024, time.Hour)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, w.Close())
	}()

	_, err = w.Write([]byte("line1\n"))
	require.NoError(t, err)
	_, err = w.Write([]byte("line2\n"))
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "line1\nline2\n", string(data))
}

func TestRotatingWriter_RotatesAtMaxSize(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	// maxSize of 20 bytes.
	w, err := NewRotatingWriter(path, 20, time.Hour)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, w.Close())
	}()

	// Write 15 bytes.
	_, err = w.Write([]byte("123456789012345"))
	require.NoError(t, err)

	// This write exceeds maxSize, triggering rotation.
	_, err = w.Write([]byte("AFTER_ROTATION"))
	require.NoError(t, err)

	// Original file should contain only the post-rotation data.
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "AFTER_ROTATION", string(data))

	// Rotated file .1 should contain the old data.
	rotated, err := os.ReadFile(path + ".1")
	require.NoError(t, err)
	assert.Equal(t, "123456789012345", string(rotated))
}

func TestRotatingWriter_RotatedFileNaming(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	w, err := NewRotatingWriter(path, 10, time.Hour)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, w.Close())
	}()

	// Trigger multiple rotations.
	for i := 0; i < 3; i++ {
		_, err = w.Write([]byte("0123456789X"))
		require.NoError(t, err)
	}

	// Check that rotated files exist with correct naming.
	assert.FileExists(t, path+".1")
	assert.FileExists(t, path+".2")
}

func TestRotatingWriter_MaxAgeCleanup(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	w, err := NewRotatingWriter(path, 1024, 1*time.Millisecond)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, w.Close())
	}()

	// Create fake rotated files with old timestamps.
	oldFile := path + ".1"
	require.NoError(t, os.WriteFile(oldFile, []byte("old"), 0o644))
	// Set mod time to the past.
	oldTime := time.Now().Add(-1 * time.Hour)
	require.NoError(t, os.Chtimes(oldFile, oldTime, oldTime))

	w.cleanup()

	assert.NoFileExists(t, oldFile, "old rotated file should be cleaned up")
}

func TestRotatingWriter_ConcurrentWrites(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	w, err := NewRotatingWriter(path, 10*1024, time.Hour)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, w.Close())
	}()

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, werr := w.Write([]byte("concurrent write\n"))
			assert.NoError(t, werr)
		}()
	}
	wg.Wait()
	// No race detector panic = pass.
}

func TestRotatingWriter_DefaultValues(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	// Pass zero values for maxSize and maxAge.
	w, err := NewRotatingWriter(path, 0, 0)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, w.Close())
	}()

	assert.Equal(t, int64(10*1024*1024), w.maxSize)
	assert.Equal(t, 7*24*time.Hour, w.maxAge)
}
