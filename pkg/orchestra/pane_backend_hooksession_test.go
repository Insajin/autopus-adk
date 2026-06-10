// Package orchestra tests that InteractivePaneBackend wires a real HookSession
// and selects FileIPCDetector when HookMode is enabled (SPEC-ORCH-022 REQ-006/007,
// oracle S6).
package orchestra

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestResolveHookSession_HookModeEnabled verifies that resolveHookSession returns
// a non-nil *HookSession when HookMode=true and SessionID is set.
func TestResolveHookSession_HookModeEnabled(t *testing.T) {
	t.Parallel()

	cfg := OrchestraConfig{
		HookMode:  true,
		SessionID: "test-pane-backend-hook-enabled",
	}

	sess := resolveHookSession(cfg)
	require.NotNil(t, sess, "resolveHookSession must return non-nil when HookMode=true and SessionID is set")
	defer sess.Cleanup()
}

// TestResolveHookSession_HookModeDisabled verifies that resolveHookSession returns
// nil when HookMode=false, regardless of SessionID.
func TestResolveHookSession_HookModeDisabled(t *testing.T) {
	t.Parallel()

	cfg := OrchestraConfig{
		HookMode:  false,
		SessionID: "test-pane-backend-hook-disabled",
	}

	sess := resolveHookSession(cfg)
	assert.Nil(t, sess, "resolveHookSession must return nil when HookMode=false")
}

// TestResolveHookSession_EmptySessionID verifies graceful degrade when
// HookMode=true but SessionID is empty.
func TestResolveHookSession_EmptySessionID(t *testing.T) {
	t.Parallel()

	cfg := OrchestraConfig{
		HookMode:  true,
		SessionID: "",
	}

	sess := resolveHookSession(cfg)
	assert.Nil(t, sess, "resolveHookSession must return nil when SessionID is empty")
}

// TestNewCompletionDetectorWithConfig_FileIPCSelectedWhenSessionNonNil verifies
// that NewCompletionDetectorWithConfig selects *FileIPCDetector when
// hookMode=true and session is non-nil (no signal-capable terminal).
// This is the oracle for the T6 completion-detector wiring path (S6).
func TestNewCompletionDetectorWithConfig_FileIPCSelectedWhenSessionNonNil(t *testing.T) {
	t.Parallel()

	// Create a real HookSession so the detector factory can use it.
	sess, err := NewHookSession("test-pane-backend-detector-fileipc")
	require.NoError(t, err)
	defer sess.Cleanup()

	// Use the shared mockTerminal which does NOT implement SignalCapable,
	// so the factory falls through to the FileIPCDetector branch.
	mock := &mockTerminal{name: "plain"}
	detector := NewCompletionDetectorWithConfig(mock, true, sess)
	require.NotNil(t, detector)

	_, ok := detector.(*FileIPCDetector)
	assert.True(t, ok, "detector must be *FileIPCDetector when hookMode=true and session is non-nil")
}

// TestNewCompletionDetectorWithConfig_ScreenPollWhenHookModeOff verifies that
// ScreenPollDetector is selected when HookMode=false (nil session).
func TestNewCompletionDetectorWithConfig_ScreenPollWhenHookModeOff(t *testing.T) {
	t.Parallel()

	mock := &mockTerminal{name: "plain"}
	detector := NewCompletionDetectorWithConfig(mock, false, nil)
	require.NotNil(t, detector)

	_, ok := detector.(*ScreenPollDetector)
	assert.True(t, ok, "detector must be *ScreenPollDetector when hookMode=false")
}
