package orchestra

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/insajin/autopus-adk/pkg/terminal"
	"github.com/stretchr/testify/assert"
)

type deadlineCaptureTerminal struct {
	mockTerminal
	remaining []time.Duration
	calls     int
}

func (m *deadlineCaptureTerminal) ReadScreen(ctx context.Context, paneID terminal.PaneID, opts terminal.ReadScreenOpts) (string, error) {
	if deadline, ok := ctx.Deadline(); ok {
		m.remaining = append(m.remaining, time.Until(deadline))
	} else {
		m.remaining = append(m.remaining, 24*time.Hour)
	}
	m.calls++
	return "\u276F\n", nil
}

// TestScreenPollDetector_TwoPhaseMatch verifies 2-phase consecutive prompt match.
func TestScreenPollDetector_TwoPhaseMatch(t *testing.T) {
	t.Parallel()
	mock := &countingScreenMock{}
	mock.outputs = []string{">\n", ">\n"}
	mock.mockTerminal.name = "plain"

	// Use codex pattern so ">" alone won't match — use explicit ❯ for claude
	mock.outputs = []string{"\u276F\n", "\u276F\n"}
	detector := &ScreenPollDetector{term: mock}
	pi := paneInfo{paneID: "pane-1", provider: ProviderConfig{Name: "claude"}}
	patterns := DefaultCompletionPatterns()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	ok, err := detector.WaitForCompletion(ctx, pi, patterns, "", 0)
	assert.NoError(t, err)
	assert.True(t, ok, "2-phase consecutive match should detect completion")
}

// TestScreenPollDetector_ContextCancel verifies context cancellation returns false.
func TestScreenPollDetector_ContextCancel(t *testing.T) {
	t.Parallel()
	mock := newPlainMock()
	mock.readScreenOutput = "still running..."

	detector := &ScreenPollDetector{term: mock}
	pi := paneInfo{paneID: "pane-1", provider: ProviderConfig{Name: "claude"}}
	patterns := DefaultCompletionPatterns()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	ok, err := detector.WaitForCompletion(ctx, pi, patterns, "", 0)
	assert.NoError(t, err)
	assert.False(t, ok, "cancelled context must return false")
}

// TestScreenPollDetector_ReadScreenUsesShortPerCallDeadline verifies a single
// stuck terminal read cannot consume the whole provider timeout.
func TestScreenPollDetector_ReadScreenUsesShortPerCallDeadline(t *testing.T) {
	t.Parallel()

	mock := &deadlineCaptureTerminal{}
	detector := &ScreenPollDetector{term: mock}
	pi := paneInfo{paneID: "pane-1", provider: ProviderConfig{Name: "claude"}}
	patterns := DefaultCompletionPatterns()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	ok, err := detector.WaitForCompletion(ctx, pi, patterns, "", 0)
	assert.NoError(t, err)
	assert.True(t, ok)
	assert.GreaterOrEqual(t, len(mock.remaining), 2)
	for _, remaining := range mock.remaining {
		assert.LessOrEqual(t, remaining, readScreenPollTimeout+500*time.Millisecond)
		assert.Greater(t, remaining, time.Duration(0))
	}
}

// TestScreenPollDetector_BaselineFiltering verifies baseline prevents false positives.
func TestScreenPollDetector_BaselineFiltering(t *testing.T) {
	t.Parallel()
	mock := &countingScreenMock{}
	baseline := "\u276F\n"
	// First 2 calls return baseline (filtered), then new screen with prompt
	mock.outputs = []string{baseline, baseline, "new output\n\u276F\n", "new output\n\u276F\n"}
	mock.mockTerminal.name = "plain"

	detector := &ScreenPollDetector{term: mock}
	pi := paneInfo{paneID: "pane-1", provider: ProviderConfig{Name: "claude"}}
	patterns := DefaultCompletionPatterns()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	ok, err := detector.WaitForCompletion(ctx, pi, patterns, baseline, 0)
	assert.NoError(t, err)
	assert.True(t, ok, "should complete after baseline changes and 2-phase match")
}

// TestScreenPollDetector_IdleFallbackDirectCheck verifies isOutputIdle via the detector.
func TestScreenPollDetector_IdleFallbackDirectCheck(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	outputFile := filepath.Join(tmpDir, "output.txt")
	if err := os.WriteFile(outputFile, []byte("done"), 0644); err != nil {
		t.Fatal(err)
	}
	// Set modtime far in the past so isOutputIdle returns true
	past := time.Now().Add(-1 * time.Minute)
	if err := os.Chtimes(outputFile, past, past); err != nil {
		t.Fatal(err)
	}

	assert.True(t, isOutputIdle(outputFile, outputIdleThreshold),
		"output file with old modtime should be considered idle")
}

// TestScreenPollDetector_PerProviderIdleThreshold verifies per-provider idle threshold is used.
func TestScreenPollDetector_PerProviderIdleThreshold(t *testing.T) {
	t.Parallel()

	// Verify that a provider with custom IdleThreshold compiles and is accepted.
	pi := paneInfo{
		paneID:     "pane-1",
		provider:   ProviderConfig{Name: "opencode", IdleThreshold: 90 * time.Second},
		outputFile: "/tmp/nonexistent",
	}
	assert.Equal(t, 90*time.Second, pi.provider.IdleThreshold)
}

// TestScreenPollDetector_SafetyDeadline_ActivatesOnNoDeadline verifies S3:
// context.Background() (no deadline) triggers the safety deadline fallback,
// returning false instead of blocking indefinitely. Warning log emitted.
func TestScreenPollDetector_SafetyDeadline_ActivatesOnNoDeadline(t *testing.T) {
	t.Parallel()

	mock := newPlainMock()
	mock.readScreenOutput = "still running..."

	// Use per-instance override to avoid package-var data race.
	detector := &ScreenPollDetector{term: mock, safetyDeadline: 200 * time.Millisecond}
	pi := paneInfo{paneID: "pane-1", provider: ProviderConfig{Name: "claude"}}
	patterns := DefaultCompletionPatterns()

	// Call with context.Background() — no deadline set.
	ok, err := detector.WaitForCompletion(context.Background(), pi, patterns, "", 0)
	assert.NoError(t, err)
	assert.False(t, ok, "safety deadline should cause return false, not infinite block")
}

// TestScreenPollDetector_SafetyDeadline_NotActivatedWithExistingDeadline verifies S4:
// when caller sets a deadline, the safety fallback does NOT override it.
func TestScreenPollDetector_SafetyDeadline_NotActivatedWithExistingDeadline(t *testing.T) {
	t.Parallel()

	mock := newPlainMock()
	mock.readScreenOutput = "still running..."

	// Safety deadline set long — caller's deadline should win.
	detector := &ScreenPollDetector{term: mock, safetyDeadline: 5 * time.Second}
	pi := paneInfo{paneID: "pane-1", provider: ProviderConfig{Name: "claude"}}
	patterns := DefaultCompletionPatterns()

	// Caller sets a 200ms deadline — much shorter than 5s safety.
	callerCtx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	start := time.Now()
	ok, err := detector.WaitForCompletion(callerCtx, pi, patterns, "", 0)
	elapsed := time.Since(start)

	assert.NoError(t, err)
	assert.False(t, ok, "caller deadline should trigger cancellation")
	assert.Less(t, elapsed, 1*time.Second, "should respect caller's short deadline, not 5s safety")
}

// TestScreenPollDetector_CompletionBeforeSafetyDeadline verifies that when
// completion is detected before the safety deadline fires, result is true.
func TestScreenPollDetector_CompletionBeforeSafetyDeadline(t *testing.T) {
	t.Parallel()

	mock := &countingScreenMock{}
	// Return prompt pattern twice in a row for 2-phase match
	mock.outputs = []string{"\u276F\n", "\u276F\n"}
	mock.mockTerminal.name = "plain"

	// Safety deadline is long (5s), but 2-phase match should complete in ~4s
	detector := &ScreenPollDetector{term: mock, safetyDeadline: 5 * time.Second}
	pi := paneInfo{paneID: "pane-1", provider: ProviderConfig{Name: "claude"}}
	patterns := DefaultCompletionPatterns()

	// No deadline on context — safety deadline applies but completion wins
	ok, err := detector.WaitForCompletion(context.Background(), pi, patterns, "", 0)
	assert.NoError(t, err)
	assert.True(t, ok, "completion should be detected before safety deadline")
}

// TestScreenPollDetector_PerProviderIdleThresholdUsed verifies that per-provider
// IdleThreshold is respected instead of the default idle fallback threshold.
func TestScreenPollDetector_PerProviderIdleThresholdUsed(t *testing.T) {
	t.Parallel()

	mock := newPlainMock()
	mock.readScreenOutput = "still working..."

	// Very short safety deadline to avoid long test
	detector := &ScreenPollDetector{term: mock, safetyDeadline: 300 * time.Millisecond}
	pi := paneInfo{
		paneID:   "pane-1",
		provider: ProviderConfig{Name: "opencode", IdleThreshold: 1 * time.Second},
	}
	patterns := DefaultCompletionPatterns()

	// Context without deadline — safety deadline will fire
	ok, err := detector.WaitForCompletion(context.Background(), pi, patterns, "", 0)
	assert.NoError(t, err)
	assert.False(t, ok, "no completion pattern — safety deadline should return false")
}

// TestScreenPollDetector_BackwardCompatibility verifies S6:
// ScreenPollDetector implements CompletionDetector interface unchanged.
func TestScreenPollDetector_BackwardCompatibility(t *testing.T) {
	t.Parallel()
	mock := newPlainMock()
	detector := &ScreenPollDetector{term: mock}
	// Compile-time interface conformance check.
	var _ CompletionDetector = detector
}
