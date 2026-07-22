package orchestra

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewHookSession_CreatesSessionDir verifies that NewHookSession creates
// a session directory under /tmp/autopus with 0o700 permissions.
func TestNewHookSession_CreatesSessionDir(t *testing.T) {
	t.Parallel()

	sess, err := NewHookSession("test-session-create")
	require.NoError(t, err)
	defer sess.Cleanup()

	info, err := os.Stat(sess.Dir())
	require.NoError(t, err, "session directory must exist")
	assert.True(t, info.IsDir(), "session path must be a directory")
	assert.Equal(t, os.FileMode(0o700), info.Mode().Perm(),
		"session directory must have 0700 permissions")
}

// TestHookSession_WaitForDone_DetectsDoneFile verifies that WaitForDone
// detects a done file within 500ms of it being written.
func TestHookSession_WaitForDone_DetectsDoneFile(t *testing.T) {
	t.Parallel()

	sess, err := NewHookSession("test-session-done")
	require.NoError(t, err)
	defer sess.Cleanup()

	// Write done file after a short delay.
	go func() {
		time.Sleep(100 * time.Millisecond)
		err := os.WriteFile(filepath.Join(sess.Dir(), "done"), []byte("1"), 0o644)
		require.NoError(t, err)
	}()

	err = sess.WaitForDone(2 * time.Second)
	assert.NoError(t, err, "WaitForDone must detect the done file")
}

// TestHookSession_WaitForDone_Timeout verifies that WaitForDone returns
// an error when the done file is not created within the timeout period.
func TestHookSession_WaitForDone_Timeout(t *testing.T) {
	t.Parallel()

	sess, err := NewHookSession("test-session-timeout")
	require.NoError(t, err)
	defer sess.Cleanup()

	err = sess.WaitForDone(200 * time.Millisecond)
	assert.Error(t, err, "WaitForDone must return error on timeout")
}

// TestHookSession_ReadResult_ValidJSON verifies that ReadResult correctly
// parses a valid result.json file into a HookResult.
func TestHookSession_ReadResult_ValidJSON(t *testing.T) {
	t.Parallel()

	sess, err := NewHookSession("test-session-read-valid")
	require.NoError(t, err)
	defer sess.Cleanup()

	result := map[string]interface{}{
		"output":    "test response content",
		"exit_code": 0,
	}
	data, err := json.Marshal(result)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(sess.Dir(), "result.json"), data, 0o644)
	require.NoError(t, err)

	hookResult, err := sess.ReadResult()

	require.NoError(t, err)
	assert.Equal(t, "test response content", hookResult.Output)
	assert.Equal(t, 0, hookResult.ExitCode)
}

// TestHookSession_ReadResult_InvalidJSON verifies that ReadResult returns
// an error when result.json contains malformed JSON.
func TestHookSession_ReadResult_InvalidJSON(t *testing.T) {
	t.Parallel()

	sess, err := NewHookSession("test-session-read-invalid")
	require.NoError(t, err)
	defer sess.Cleanup()

	err = os.WriteFile(
		filepath.Join(sess.Dir(), "result.json"),
		[]byte("{invalid json}"),
		0o644,
	)
	require.NoError(t, err)

	_, err = sess.ReadResult()
	assert.Error(t, err, "ReadResult must return error for invalid JSON")
}

// TestHookSession_ReadResult_MissingFile verifies that ReadResult returns
// an error when result.json does not exist.
func TestHookSession_ReadResult_MissingFile(t *testing.T) {
	t.Parallel()

	sess, err := NewHookSession("test-session-read-missing")
	require.NoError(t, err)
	defer sess.Cleanup()

	_, err = sess.ReadResult()
	assert.Error(t, err, "ReadResult must return error when result.json missing")
	assert.Contains(t, err.Error(), "read result file")
}

// TestHookSession_Cleanup_RemovesDir verifies that Cleanup removes the
// entire session directory.
func TestHookSession_Cleanup_RemovesDir(t *testing.T) {
	t.Parallel()

	sess, err := NewHookSession("test-session-cleanup")
	require.NoError(t, err)

	dir := sess.Dir()
	_, err = os.Stat(dir)
	require.NoError(t, err, "session directory must exist before cleanup")

	sess.Cleanup()

	_, err = os.Stat(dir)
	assert.True(t, os.IsNotExist(err), "session directory must be removed after cleanup")
}

// TestHookSession_HasHook verifies that HasHook correctly detects
// whether a provider has a hook configured.
func TestHookSession_HasHook(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		provider string
		expect   bool
	}{
		{"claude has hook", "claude", true},
		{"gemini has hook", "gemini", true},
		{"codex has hook", "codex", true},
		{"unknown provider has no hook", "unknown-provider", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			sess, err := NewHookSession("test-has-hook-" + tt.provider)
			require.NoError(t, err)
			defer sess.Cleanup()

			got := sess.HasHook(tt.provider)
			assert.Equal(t, tt.expect, got)
		})
	}
}

// TestHookSession_SetHookProviders verifies that SetHookProviders overrides
// the default hook provider map.
func TestHookSession_SetHookProviders(t *testing.T) {
	t.Parallel()

	sess, err := NewHookSession("test-set-hook-providers")
	require.NoError(t, err)
	defer sess.Cleanup()

	// Default: claude has hook.
	assert.True(t, sess.HasHook("claude"))

	// Override: only custom-provider has hook.
	sess.SetHookProviders(map[string]bool{"custom-provider": true})

	assert.False(t, sess.HasHook("claude"), "claude must not have hook after override")
	assert.True(t, sess.HasHook("custom-provider"), "custom-provider must have hook after override")
}

// TestHookSession_SessionID verifies that SessionID returns the ID passed
// at creation time.
func TestHookSession_SessionID(t *testing.T) {
	t.Parallel()

	sess, err := NewHookSession("test-session-id-check")
	require.NoError(t, err)
	defer sess.Cleanup()

	assert.Equal(t, "test-session-id-check", sess.SessionID())
}

// TestHookSession_SpecialCharSessionID verifies that unsafe session IDs fail closed.
func TestHookSession_SpecialCharSessionID(t *testing.T) {
	t.Parallel()

	sess, err := NewHookSession("test/../../../etc/passwd")
	assert.Nil(t, sess)
	assert.Error(t, err)
}

func TestHookSession_UppercaseSessionIDFailsBeforeFilesystemMutation(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())

	sess, err := NewHookSession("Alias-ABC")

	assert.Nil(t, sess)
	assert.ErrorContains(t, err, "lowercase")
	assert.NoDirExists(t, filepath.Join(os.TempDir(), "autopus"))
}

// TestHookSession_ConcurrentWaitForDone verifies that multiple goroutines
// can call WaitForDone simultaneously without interference.
func TestHookSession_ConcurrentWaitForDone(t *testing.T) {
	t.Parallel()

	sess, err := NewHookSession("test-concurrent-wait")
	require.NoError(t, err)
	defer sess.Cleanup()

	go func() {
		time.Sleep(100 * time.Millisecond)
		_ = os.WriteFile(filepath.Join(sess.Dir(), "done"), []byte("1"), 0o644)
	}()

	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() {
			errs <- sess.WaitForDone(2 * time.Second)
		}()
	}

	for i := 0; i < 2; i++ {
		assert.NoError(t, <-errs, "concurrent WaitForDone must succeed")
	}
}
