package orchestra

import (
	"bytes"
	"context"
	"errors"
	"log"
	"strings"
	"testing"
	"time"
)

// stubCompletionDetector returns fixed (completed, err) values for testing the
// shared completion error handler. It is intentionally not a *ScreenPollDetector
// so resolveCompletionDetector treats it as event-driven.
type stubCompletionDetector struct {
	completed bool
	err       error
}

func (s *stubCompletionDetector) WaitForCompletion(_ context.Context, _ paneInfo, _ []CompletionPattern, _ string, _ int) (bool, error) {
	return s.completed, s.err
}

// captureLog redirects the standard logger to a buffer for the duration of fn.
func captureLog(t *testing.T, fn func()) string {
	t.Helper()
	var buf bytes.Buffer
	prevOut := log.Writer()
	prevFlags := log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	defer func() {
		log.SetOutput(prevOut)
		log.SetFlags(prevFlags)
	}()
	fn()
	return buf.String()
}

// S1: cc21 detector error forces false and logs an I/O failure with provider name.
func TestWaitForCompletion_DetectorIOError_ForcesFalseAndLogs(t *testing.T) {
	cfg := OrchestraConfig{
		CompletionDetector: &stubCompletionDetector{completed: true, err: errors.New("io fail")},
	}
	pi := paneInfo{provider: ProviderConfig{Name: "claude"}}

	var got bool
	logged := captureLog(t, func() {
		// Bounded context: the polling fallback (ScreenPollDetector) must observe a
		// deadline shorter than its 500ms poll interval so it returns without ever
		// touching the nil terminal.
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
		defer cancel()
		got = waitForCompletion(ctx, cfg, pi, nil, "", nil, 1)
	})

	if got {
		t.Fatalf("expected waitForCompletion to force false on detector error, got true")
	}
	if !strings.Contains(logged, "claude") {
		t.Fatalf("expected log to contain provider name 'claude', got: %q", logged)
	}
	if !strings.Contains(logged, "io fail") {
		t.Fatalf("expected log to contain detector error 'io fail', got: %q", logged)
	}
	if !strings.Contains(logged, "I/O failure") {
		t.Fatalf("expected log to mark this as an I/O failure, got: %q", logged)
	}
}

// S1: a cancelled detector result is reported as cancellation (not I/O failure).
func TestWaitForCompletion_DetectorCancelled_DistinguishedFromIO(t *testing.T) {
	cfg := OrchestraConfig{
		CompletionDetector: &stubCompletionDetector{completed: false, err: context.Canceled},
	}
	pi := paneInfo{provider: ProviderConfig{Name: "gemini"}}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled input

	var got bool
	logged := captureLog(t, func() {
		got = waitForCompletion(ctx, cfg, pi, nil, "", nil, 1)
	})

	if got {
		t.Fatalf("expected false on cancelled detector, got true")
	}
	if !strings.Contains(logged, "gemini") {
		t.Fatalf("expected log to contain provider name 'gemini', got: %q", logged)
	}
	if !strings.Contains(logged, "cancelled") {
		t.Fatalf("expected log to mark cancellation, got: %q", logged)
	}
	if strings.Contains(logged, "I/O failure") {
		t.Fatalf("cancellation must not be logged as an I/O failure, got: %q", logged)
	}
}

// handleCompletionResult returns the original completed value untouched when there
// is no error (backward-compatible: success path is unchanged).
func TestHandleCompletionResult_NoError_PassThrough(t *testing.T) {
	if !handleCompletionResult(context.Background(), "claude", true, nil) {
		t.Fatalf("expected completed=true to pass through unchanged")
	}
	if handleCompletionResult(context.Background(), "claude", false, nil) {
		t.Fatalf("expected completed=false to pass through unchanged")
	}
}
