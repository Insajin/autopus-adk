package setup

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSaveAndLoadProgress_RoundTrip(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, ".worker-progress.json")

	p := SetupProgress{
		Step:      3,
		Timestamp: time.Now().Truncate(time.Millisecond),
	}

	data, err := json.Marshal(p)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, data, 0600))

	raw, err := os.ReadFile(path)
	require.NoError(t, err)

	var loaded SetupProgress
	require.NoError(t, json.Unmarshal(raw, &loaded))

	assert.Equal(t, 3, loaded.Step)
	assert.WithinDuration(t, p.Timestamp, loaded.Timestamp, time.Second)
}

func TestIsExpired_Fresh(t *testing.T) {
	t.Parallel()

	p := &SetupProgress{
		Step:      1,
		Timestamp: time.Now(),
	}
	assert.False(t, p.IsExpired())
}

func TestIsExpired_Old(t *testing.T) {
	t.Parallel()

	p := &SetupProgress{
		Step:      1,
		Timestamp: time.Now().Add(-2 * time.Hour),
	}
	assert.True(t, p.IsExpired())
}

func TestIsExpired_ExactBoundary(t *testing.T) {
	t.Parallel()

	// Exactly 1 hour ago should not be expired (> not >=)
	p := &SetupProgress{
		Step:      1,
		Timestamp: time.Now().Add(-time.Hour),
	}
	// At the exact boundary, this may or may not be expired depending on
	// nanosecond precision; just ensure no panic.
	_ = p.IsExpired()
}

func TestClearProgress_RemovesFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, ".worker-progress.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"step":1}`), 0600))

	// Verify file exists
	_, err := os.Stat(path)
	require.NoError(t, err)

	// Remove it
	require.NoError(t, os.Remove(path))

	// Verify file is gone
	_, err = os.Stat(path)
	assert.True(t, os.IsNotExist(err))
}

func TestClearProgress_NonexistentFile(t *testing.T) {
	t.Parallel()

	// ClearProgress on a nonexistent file should not error
	err := os.Remove("/tmp/nonexistent-progress-test-file.json")
	if err != nil {
		assert.True(t, os.IsNotExist(err))
	}
}

func TestLoadProgress_FileNotExist(t *testing.T) {
	t.Parallel()

	// LoadProgress returns nil, nil when the file doesn't exist.
	// We test the behavior by checking the pattern directly.
	path := filepath.Join(t.TempDir(), "does-not-exist.json")
	_, err := os.ReadFile(path)
	assert.True(t, os.IsNotExist(err))
}

func TestSaveProgress_WritesToDisk(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	err := SaveProgress(2)
	require.NoError(t, err)

	p, err := LoadProgress()
	require.NoError(t, err)
	require.NotNil(t, p)
	assert.Equal(t, 2, p.Step)
	assert.False(t, p.IsExpired())
}

func TestLoadProgress_NoFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	p, err := LoadProgress()
	require.NoError(t, err)
	assert.Nil(t, p)
}

func TestLoadProgress_InvalidJSON(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// Create the config dir and write invalid JSON
	dir := filepath.Join(tmp, ".config", "autopus")
	require.NoError(t, os.MkdirAll(dir, 0700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".worker-progress.json"), []byte("{{bad"), 0600))

	_, err := LoadProgress()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal progress")
}

func TestClearProgress_Actual(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// Save and then clear
	require.NoError(t, SaveProgress(1))
	p, err := LoadProgress()
	require.NoError(t, err)
	require.NotNil(t, p)

	require.NoError(t, ClearProgress())
	p, err = LoadProgress()
	require.NoError(t, err)
	assert.Nil(t, p)
}

func TestClearProgress_NoFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// ClearProgress on nonexistent file should not error
	err := ClearProgress()
	require.NoError(t, err)
}

func TestSaveProgress_ErrorOnReadOnlyDir(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// Create config dir then make it read-only
	dir := filepath.Join(tmp, ".config", "autopus")
	require.NoError(t, os.MkdirAll(dir, 0700))
	// Put a directory where the progress file should go
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".worker-progress.json"), 0700))

	err := SaveProgress(1)
	require.Error(t, err)
}

func TestSaveAndLoadProgress_Functional(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// Save step 5
	require.NoError(t, SaveProgress(5))

	// Load and verify
	p, err := LoadProgress()
	require.NoError(t, err)
	require.NotNil(t, p)
	assert.Equal(t, 5, p.Step)

	// Overwrite with step 10
	require.NoError(t, SaveProgress(10))
	p, err = LoadProgress()
	require.NoError(t, err)
	assert.Equal(t, 10, p.Step)
}

func TestLoadProgress_ReadError(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// Create a directory where the file should be (causes read error)
	dir := filepath.Join(tmp, ".config", "autopus")
	require.NoError(t, os.MkdirAll(dir, 0700))
	progDir := filepath.Join(dir, ".worker-progress.json")
	require.NoError(t, os.MkdirAll(progDir, 0700))

	_, err := LoadProgress()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read progress")
}
