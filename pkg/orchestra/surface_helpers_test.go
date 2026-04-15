package orchestra

import (
	"context"
	"testing"

	"github.com/insajin/autopus-adk/pkg/terminal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sendLongTextCountMock fails on first N calls, then succeeds on subsequent calls.
// Used to test sendPromptWithRetry retry-after-recreation path.
type sendLongTextCountMock struct {
	mockTerminal
	callCount int
	failUntil int // fail the first N calls
}

func (m *sendLongTextCountMock) SendLongText(_ context.Context, paneID terminal.PaneID, text string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callCount++
	m.sendLongTextCalls = append(m.sendLongTextCalls, struct {
		PaneID terminal.PaneID
		Text   string
	}{paneID, text})
	if m.callCount <= m.failUntil {
		return assert.AnError
	}
	return nil
}

// TestCaptureBaselines verifies baseline capture for active panes.
func TestCaptureBaselines(t *testing.T) {
	t.Parallel()
	mock := newCmuxMock()
	mock.readScreenOutput = "prompt content\n"

	panes := []paneInfo{
		{paneID: "pane-1", provider: ProviderConfig{Name: "claude"}},
		{paneID: "pane-2", provider: ProviderConfig{Name: "gemini"}, skipWait: true},
		{paneID: "pane-3", provider: ProviderConfig{Name: "codex"}},
	}

	baselines := captureBaselines(context.Background(), mock, panes)
	assert.Equal(t, "prompt content\n", baselines["claude"])
	assert.Empty(t, baselines["gemini"], "skipWait pane should not have baseline")
	assert.Equal(t, "prompt content\n", baselines["codex"])
}

// TestCaptureBaselines_ReadScreenError verifies empty baseline on ReadScreen error.
func TestCaptureBaselines_ReadScreenError(t *testing.T) {
	t.Parallel()
	mock := newCmuxMock()
	mock.readScreenErr = assert.AnError

	panes := []paneInfo{
		{paneID: "pane-1", provider: ProviderConfig{Name: "claude"}},
	}

	baselines := captureBaselines(context.Background(), mock, panes)
	// On error, ReadScreen returns "" -- should still be in map
	assert.Equal(t, "", baselines["claude"])
}

// TestCaptureBaselines_EmptyPanes verifies empty map for empty panes list.
func TestCaptureBaselines_EmptyPanes(t *testing.T) {
	t.Parallel()
	mock := newCmuxMock()
	baselines := captureBaselines(context.Background(), mock, nil)
	assert.Empty(t, baselines)
}

// TestSendPromptWithRetry_Success verifies first-try success.
func TestSendPromptWithRetry_Success(t *testing.T) {
	t.Parallel()
	mock := newCmuxMock()
	cfg := OrchestraConfig{Terminal: mock}
	pi := paneInfo{paneID: "pane-1", provider: ProviderConfig{Name: "claude"}}
	baselines := map[string]string{}

	newPI, recreated, err := sendPromptWithRetry(
		context.Background(), cfg, pi, "hello world", 1, baselines,
	)
	require.NoError(t, err)
	assert.False(t, recreated)
	assert.Equal(t, pi.paneID, newPI.paneID)
}

// TestSendPromptWithRetry_FailsAllAttempts verifies error after all retries fail.
func TestSendPromptWithRetry_FailsAllAttempts(t *testing.T) {
	t.Parallel()
	mock := &sendLongTextErrorMock{mockTerminal: mockTerminal{name: "cmux"}}
	cfg := OrchestraConfig{Terminal: mock}
	pi := paneInfo{paneID: "pane-1", provider: ProviderConfig{Name: "claude", Binary: "echo"}}
	baselines := map[string]string{}

	// SendLongText always fails, and recreatePane fails too because it calls SendLongText.
	_, _, err := sendPromptWithRetry(
		context.Background(), cfg, pi, "hello world", 1, baselines,
	)
	assert.Error(t, err)
}

// TestSendPromptWithRetry_RecreateAndRetrySuccess verifies recreation + retry success.
// After SPEC-ORCH-018 R1, same-pane retries occur first (2x with backoff),
// so recreation only happens after all same-pane retries are exhausted.
func TestSendPromptWithRetry_RecreateAndRetrySuccess(t *testing.T) {
	t.Parallel()
	// Fail first 3 calls (initial + 2 same-pane retries),
	// succeed on 4th call (after recreation).
	mock := &sendLongTextCountMock{
		mockTerminal: mockTerminal{name: "cmux", readScreenOutput: "❯\n"},
		failUntil:    3,
	}
	cfg := OrchestraConfig{Terminal: mock}
	pi := paneInfo{paneID: "pane-1", provider: ProviderConfig{Name: "claude", Binary: "echo"}}
	baselines := map[string]string{"claude": "old baseline"}

	newPI, recreated, err := sendPromptWithRetry(
		context.Background(), cfg, pi, "hello world", 1, baselines,
	)
	require.NoError(t, err)
	assert.True(t, recreated, "pane should be recreated after same-pane retries exhausted")
	assert.NotEmpty(t, newPI.paneID)
	// Baseline should be refreshed for the provider
	assert.NotEqual(t, "old baseline", baselines["claude"])
}
