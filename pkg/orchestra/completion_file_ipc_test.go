package orchestra

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFileIPCDetector_DoneFileExists verifies completion when done file is present.
func TestFileIPCDetector_DoneFileExists(t *testing.T) {
	t.Parallel()
	session := newTestHookSession(t)

	detector := &FileIPCDetector{session: session}
	pi := paneInfo{provider: ProviderConfig{Name: "claude"}}

	// Create the done file before waiting.
	donePath := filepath.Join(session.Dir(), "claude-done")
	require.NoError(t, os.WriteFile(donePath, []byte{}, 0o600))

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	ok, err := detector.WaitForCompletion(ctx, pi, nil, "", 0)
	assert.NoError(t, err)
	assert.True(t, ok, "should detect completion when done file exists")
}

// TestFileIPCDetector_RoundDoneFileExists verifies round-scoped done file detection.
func TestFileIPCDetector_RoundDoneFileExists(t *testing.T) {
	t.Parallel()
	session := newTestHookSession(t)

	detector := &FileIPCDetector{session: session}
	pi := paneInfo{provider: ProviderConfig{Name: "gemini"}}

	// Create the round-scoped done file.
	doneName := RoundSignalName("gemini", 2, "done")
	donePath := filepath.Join(session.Dir(), doneName)
	require.NoError(t, os.WriteFile(donePath, []byte{}, 0o600))

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	ok, err := detector.WaitForCompletion(ctx, pi, nil, "", 2)
	assert.NoError(t, err)
	assert.True(t, ok, "should detect completion for round-scoped done file")
}

// TestFileIPCDetector_ContextCancellation verifies false return on context cancel.
func TestFileIPCDetector_ContextCancellation(t *testing.T) {
	t.Parallel()
	session := newTestHookSession(t)

	detector := &FileIPCDetector{session: session}
	pi := paneInfo{provider: ProviderConfig{Name: "codex"}}

	// Cancel immediately -- no done file exists.
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	ok, err := detector.WaitForCompletion(ctx, pi, nil, "", 0)
	assert.NoError(t, err)
	assert.False(t, ok, "should return false on context timeout")
}

func TestFileIPCDetector_ResponseFileMarkerExists(t *testing.T) {
	t.Parallel()
	session := newTestHookSession(t)
	responsePath := filepath.Join(t.TempDir(), "codex-response.md")
	writeMarkedResponse(t, responsePath, `{"verdict":"PASS","summary":"ok","findings":[]}`)

	detector := &FileIPCDetector{session: session}
	pi := paneInfo{
		provider:     ProviderConfig{Name: "codex"},
		role:         "reviewer",
		responseFile: responsePath,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	ok, err := detector.WaitForCompletion(ctx, pi, nil, "", 1)
	assert.NoError(t, err)
	assert.True(t, ok, "response-file marker should complete hook-mode reviewer without a done file")
}

func TestFileIPCDetector_ResponseFileMarkerArrivesBeforeDoneFile(t *testing.T) {
	t.Parallel()
	session := newTestHookSession(t)
	responsePath := filepath.Join(t.TempDir(), "codex-response.md")

	detector := &FileIPCDetector{session: session}
	pi := paneInfo{
		provider:     ProviderConfig{Name: "codex"},
		role:         "reviewer",
		responseFile: responsePath,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	content := markedResponse(`{"verdict":"PASS","summary":"ok","findings":[]}`)
	go func() {
		time.Sleep(120 * time.Millisecond)
		_ = os.WriteFile(responsePath, []byte(content), 0o600)
	}()

	start := time.Now()
	ok, err := detector.WaitForCompletion(ctx, pi, nil, "", 1)
	assert.NoError(t, err)
	assert.True(t, ok, "response-file marker should unblock FileIPC wait even when the hook done file never appears")
	assert.Less(t, time.Since(start), time.Second)
}

// TestNewCompletionDetectorWithConfig_FileIPC verifies factory returns FileIPCDetector
// when hookMode is true and terminal has no signal support.
func TestNewCompletionDetectorWithConfig_FileIPC(t *testing.T) {
	t.Parallel()
	mock := newPlainMock()
	session := newTestHookSession(t)

	detector := NewCompletionDetectorWithConfig(mock, true, session)
	_, ok := detector.(*FileIPCDetector)
	assert.True(t, ok, "should return FileIPCDetector for hookMode=true without signal support")
}

// TestNewCompletionDetectorWithConfig_SignalTakesPrecedence verifies SignalDetector
// is returned even when hookMode is true if the terminal supports signals.
func TestNewCompletionDetectorWithConfig_SignalTakesPrecedence(t *testing.T) {
	t.Parallel()
	mock := &signalMock{}
	mock.name = "cmux"
	session := newTestHookSession(t)

	detector := NewCompletionDetectorWithConfig(mock, true, session)
	_, ok := detector.(*SignalDetector)
	assert.True(t, ok, "SignalDetector should take precedence over FileIPCDetector")
}

// TestNewCompletionDetectorWithConfig_FallbackPoll verifies ScreenPollDetector
// is returned when hookMode is false and terminal has no signal support.
func TestNewCompletionDetectorWithConfig_FallbackPoll(t *testing.T) {
	t.Parallel()
	mock := newPlainMock()

	detector := NewCompletionDetectorWithConfig(mock, false, nil)
	_, ok := detector.(*ScreenPollDetector)
	assert.True(t, ok, "should return ScreenPollDetector when hookMode=false")
}

// TestFileIPCDetector_BoundedTimeout_S7 verifies S7: done-file never appears →
// WaitForCompletion returns completed=false within explicit tolerance (200ms–1s).
// This guards against infinite wait when the hook script never fires.
func TestFileIPCDetector_BoundedTimeout_S7(t *testing.T) {
	t.Parallel()
	session := newTestHookSession(t)

	detector := &FileIPCDetector{session: session}
	// Use a provider name with no done file in the session dir.
	pi := paneInfo{provider: ProviderConfig{Name: "codex-notarget"}}

	// 200ms deadline — no done file will be created.
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	start := time.Now()
	completed, err := detector.WaitForCompletion(ctx, pi, nil, "", 0)
	elapsed := time.Since(start)

	assert.NoError(t, err, "S7: WaitForCompletion must not return error on timeout")
	assert.False(t, completed, "S7: completed must be false when done-file never appears")
	// Explicit tolerance: must return between 200ms and 1s after context deadline.
	assert.GreaterOrEqual(t, elapsed, 200*time.Millisecond, "S7: must not return before deadline")
	assert.Less(t, elapsed, time.Second, "S7: must return within 1s of deadline (no infinite wait)")
}

// newTestHookSession creates a temporary HookSession for testing.
func newTestHookSession(t *testing.T) *HookSession {
	t.Helper()
	dir := t.TempDir()
	return &HookSession{
		sessionID:     "test-session",
		sessionDir:    dir,
		hookProviders: defaultHookProviders,
	}
}

func writeMarkedResponse(t *testing.T, path, body string) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte(markedResponse(body)), 0o600))
}

func markedResponse(body string) string {
	return responseBeginMarker + "\n" + body + "\n" + responseEndMarker + "\n"
}
