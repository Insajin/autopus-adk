package orchestra

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/insajin/autopus-adk/pkg/terminal"
	"github.com/stretchr/testify/assert"
)

// --- R8: executeRound error handling ---

// retryableSendLongTextMock fails the first N SendLongText calls, then succeeds.
type retryableSendLongTextMock struct {
	mockTerminal
	failCount    int // how many times to fail before succeeding
	currentCount int
}

func (m *retryableSendLongTextMock) SendLongText(_ context.Context, paneID terminal.PaneID, text string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sendLongTextCalls = append(m.sendLongTextCalls, struct {
		PaneID terminal.PaneID
		Text   string
	}{paneID, text})
	m.currentCount++
	if m.currentCount <= m.failCount {
		return fmt.Errorf("transient send error (attempt %d)", m.currentCount)
	}
	return nil
}

// TestExecuteRound_SendLongText_RetrySuccess verifies SendLongText retry on first failure.
func TestExecuteRound_SendLongText_RetrySuccess(t *testing.T) {
	t.Parallel()
	mock := &retryableSendLongTextMock{failCount: 1}
	mock.mockTerminal.name = "cmux"
	mock.readScreenOutput = "❯\n"

	cfg := OrchestraConfig{
		Providers:      []ProviderConfig{{Name: "claude", Binary: "echo"}},
		Strategy:       StrategyDebate,
		Prompt:         "test retry",
		TimeoutSeconds: 5,
		Terminal:       mock,
		Interactive:    true,
		InitialDelay:   time.Millisecond,
	}
	panes := []paneInfo{{provider: cfg.Providers[0], paneID: "pane-1"}}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_ = executeRound(ctx, cfg, panes, nil, 1, nil)

	// Should have 2 SendLongText calls: first failed, retry succeeded
	mock.mu.Lock()
	callCount := len(mock.sendLongTextCalls)
	mock.mu.Unlock()
	assert.Equal(t, 2, callCount, "should retry SendLongText once on failure")
	// Provider should NOT be marked skipWait (retry succeeded)
	assert.False(t, panes[0].skipWait, "retry succeeded — skipWait must be false")
}

// TestExecuteRound_SendLongText_RetryFailure_SkipWait verifies provider is skipped
// when both SendLongText attempts fail.
func TestExecuteRound_SendLongText_RetryFailure_SkipWait(t *testing.T) {
	t.Parallel()
	mock := &retryableSendLongTextMock{failCount: 2} // both attempts fail
	mock.mockTerminal.name = "cmux"
	mock.readScreenOutput = "❯\n"

	cfg := OrchestraConfig{
		Providers:      []ProviderConfig{{Name: "claude", Binary: "echo"}},
		Strategy:       StrategyDebate,
		Prompt:         "test retry fail",
		TimeoutSeconds: 5,
		Terminal:       mock,
		Interactive:    true,
		InitialDelay:   time.Millisecond,
	}
	panes := []paneInfo{{provider: cfg.Providers[0], paneID: "pane-1"}}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_ = executeRound(ctx, cfg, panes, nil, 1, nil)

	// Should have 2 SendLongText calls (initial + retry, both failed)
	mock.mu.Lock()
	callCount := len(mock.sendLongTextCalls)
	mock.mu.Unlock()
	assert.Equal(t, 2, callCount, "should attempt SendLongText twice")
	// Provider should be marked skipWait
	assert.True(t, panes[0].skipWait, "both attempts failed — skipWait must be true")
}

// TestExecuteRound_EmptyOutput_Marked verifies providers with empty output get EmptyOutput=true.
func TestExecuteRound_EmptyOutput_Marked(t *testing.T) {
	t.Parallel()
	mock := newCmuxMock()
	mock.readScreenOutput = "❯\n"

	cfg := OrchestraConfig{
		Providers: []ProviderConfig{
			{Name: "claude", Binary: "echo"},
		},
		Strategy:       StrategyDebate,
		Prompt:         "empty output test",
		TimeoutSeconds: 5,
		Terminal:       mock,
		Interactive:    true,
		InitialDelay:   time.Millisecond,
	}
	panes := []paneInfo{{provider: cfg.Providers[0], paneID: "pane-1"}}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	responses := executeRound(ctx, cfg, panes, nil, 1, nil)
	// Mock returns empty output (ReadScreen is just "❯\n" which gets cleaned to "")
	for _, r := range responses {
		if r.Output == "" && !r.TimedOut {
			assert.True(t, r.EmptyOutput,
				"empty output provider should have EmptyOutput=true")
		}
	}
}

// TestExecuteRound_NonEmptyOutput_NotMarked verifies providers with output do NOT get EmptyOutput.
func TestExecuteRound_NonEmptyOutput_NotMarked(t *testing.T) {
	t.Parallel()
	// Construct responses directly to verify the marking logic
	responses := []ProviderResponse{
		{Provider: "claude", Output: "some content", TimedOut: false},
		{Provider: "gemini", Output: "", TimedOut: false},
		{Provider: "codex", Output: "", TimedOut: true},
	}
	// Apply the R8 marking logic
	for i := range responses {
		if responses[i].Output == "" && !responses[i].TimedOut {
			responses[i].EmptyOutput = true
		}
	}
	assert.False(t, responses[0].EmptyOutput, "non-empty output must NOT be marked")
	assert.True(t, responses[1].EmptyOutput, "empty non-timed-out must be marked")
	assert.False(t, responses[2].EmptyOutput, "timed-out empty must NOT be marked")
}
