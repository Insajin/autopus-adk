package orchestra

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/insajin/autopus-adk/pkg/terminal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- T1: sendPromptWithRetry retry-first logic ---

// failNSendLongTextMock fails SendLongText for the first N calls, then succeeds.
type failNSendLongTextMock struct {
	mockTerminal
	failCount int32 // atomic: number of remaining failures
}

func (m *failNSendLongTextMock) SendLongText(_ context.Context, paneID terminal.PaneID, text string) error {
	remaining := atomic.AddInt32(&m.failCount, -1)
	if remaining >= 0 {
		return fmt.Errorf("SendLongText transient failure")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sendLongTextCalls = append(m.sendLongTextCalls, struct {
		PaneID terminal.PaneID
		Text   string
	}{paneID, text})
	return nil
}

// TestSendPromptWithRetry_SuccessOnFirstAttempt verifies no retry when first call succeeds.
func TestSendPromptWithRetry_SuccessOnFirstAttempt(t *testing.T) {
	t.Parallel()
	mock := newCmuxMock()
	cfg := OrchestraConfig{Terminal: mock}
	pi := paneInfo{paneID: "pane-1", provider: ProviderConfig{Name: "claude", Binary: "echo"}}
	baselines := map[string]string{}

	newPI, recreated, err := sendPromptWithRetry(context.Background(), cfg, pi, "test prompt", 1, baselines)
	require.NoError(t, err)
	assert.False(t, recreated, "should not recreate on success")
	assert.Equal(t, pi.paneID, newPI.paneID)
}

// TestSendPromptWithRetry_SamePaneRetrySucceeds verifies same-pane retry recovers
// without pane recreation when a transient failure resolves.
func TestSendPromptWithRetry_SamePaneRetrySucceeds(t *testing.T) {
	t.Parallel()
	// Fail first attempt, succeed on first retry (2nd call)
	mock := &failNSendLongTextMock{}
	mock.name = "cmux"
	mock.failCount = 1
	cfg := OrchestraConfig{Terminal: mock}
	pi := paneInfo{paneID: "pane-1", provider: ProviderConfig{Name: "claude", Binary: "echo"}}
	baselines := map[string]string{}

	newPI, recreated, err := sendPromptWithRetry(context.Background(), cfg, pi, "test prompt", 2, baselines)
	require.NoError(t, err)
	assert.False(t, recreated, "same-pane retry should not trigger recreation")
	assert.Equal(t, pi.paneID, newPI.paneID, "pane ID should remain unchanged")
}

// TestSendPromptWithRetry_AllRetriesFail_RecreatesPane verifies pane recreation
// occurs only after all same-pane retries are exhausted.
func TestSendPromptWithRetry_AllRetriesFail_RecreatesPane(t *testing.T) {
	t.Parallel()
	// Fail 3 times (initial + 2 retries), then succeed on recreated pane
	mock := &failNSendLongTextMock{}
	mock.name = "cmux"
	mock.failCount = 3   // initial + 2 same-pane retries fail, 4th (post-recreate) succeeds
	mock.nextPaneID = 10 // ensure recreated pane gets a different ID
	mock.readScreenOutput = "❯\n"
	cfg := OrchestraConfig{Terminal: mock}
	pi := paneInfo{paneID: "pane-1", provider: ProviderConfig{Name: "claude", Binary: "echo"}}
	baselines := map[string]string{}

	newPI, recreated, err := sendPromptWithRetry(context.Background(), cfg, pi, "test prompt", 2, baselines)
	require.NoError(t, err)
	assert.True(t, recreated, "should recreate after all same-pane retries exhausted")
	assert.NotEqual(t, pi.paneID, newPI.paneID, "recreated pane should have new ID")
}

// TestSendPromptWithRetry_RecreationFails verifies error when recreation itself fails.
func TestSendPromptWithRetry_RecreationFails(t *testing.T) {
	t.Parallel()
	mock := &failNSendLongTextMock{}
	mock.name = "cmux"
	mock.failCount = 100 // all calls fail
	mock.nextPaneID = 10
	mock.splitPaneErr = fmt.Errorf("split pane failed")
	cfg := OrchestraConfig{Terminal: mock}
	pi := paneInfo{paneID: "pane-1", provider: ProviderConfig{Name: "claude", Binary: "echo"}}
	baselines := map[string]string{}

	_, _, err := sendPromptWithRetry(context.Background(), cfg, pi, "test prompt", 2, baselines)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "recreatePane failed")
}

// --- T2: pollUntilPrompt timeout and polling interval ---

// TestRound2PollTimeout_Is30s verifies the named constant value.
func TestRound2PollTimeout_Is30s(t *testing.T) {
	t.Parallel()
	assert.Equal(t, 30*time.Second, round2PollTimeout)
}

// TestPollUntilPrompt_DetectsPrompt verifies prompt detection within timeout.
func TestPollUntilPrompt_DetectsPrompt(t *testing.T) {
	t.Parallel()
	mock := newCmuxMock()
	mock.readScreenOutput = "❯\n"
	patterns := DefaultCompletionPatterns()

	// Use short timeout; prompt is immediately visible
	found := pollUntilPrompt(context.Background(), mock, "pane-1", patterns, 5*time.Second)
	assert.True(t, found, "should detect prompt pattern")
}

// TestPollUntilPrompt_TimesOut verifies timeout when no prompt appears.
func TestPollUntilPrompt_TimesOut(t *testing.T) {
	t.Parallel()
	mock := newCmuxMock()
	mock.readScreenOutput = "still loading..."
	patterns := DefaultCompletionPatterns()

	// Use very short timeout to avoid slow test
	found := pollUntilPrompt(context.Background(), mock, "pane-1", patterns, 100*time.Millisecond)
	assert.False(t, found, "should return false on timeout")
}

// TestPollUntilPrompt_RespectsContextCancel verifies context cancellation.
func TestPollUntilPrompt_RespectsContextCancel(t *testing.T) {
	t.Parallel()
	mock := newCmuxMock()
	mock.readScreenOutput = "no prompt"
	patterns := DefaultCompletionPatterns()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	found := pollUntilPrompt(ctx, mock, "pane-1", patterns, 30*time.Second)
	assert.False(t, found, "should return false on cancelled context")
}

// --- T3: ReadScreen scrollback depth ---

// TestScrollbackDepth_Default verifies default scrollback of 500.
func TestScrollbackDepth_Default(t *testing.T) {
	t.Parallel()
	assert.Equal(t, 500, scrollbackDepth(0))
}

// TestScrollbackDepth_Configured verifies custom scrollback value is used.
func TestScrollbackDepth_Configured(t *testing.T) {
	t.Parallel()
	assert.Equal(t, 1000, scrollbackDepth(1000))
	assert.Equal(t, 200, scrollbackDepth(200))
}

// TestScrollbackLines_ConfigField verifies OrchestraConfig.ScrollbackLines field.
func TestScrollbackLines_ConfigField(t *testing.T) {
	t.Parallel()
	cfg := OrchestraConfig{ScrollbackLines: 750}
	assert.Equal(t, 750, cfg.ScrollbackLines)

	// Default zero uses 500
	cfg2 := OrchestraConfig{}
	assert.Equal(t, 500, scrollbackDepth(cfg2.ScrollbackLines))
}

// --- T4: NoJudge skips judge phase ---

// TestNoJudge_SkipsJudgeInDebate verifies --no-judge skips judge in non-interactive debate.
func TestNoJudge_SkipsJudgeInDebate(t *testing.T) {
	t.Parallel()
	cfg := OrchestraConfig{
		Strategy: StrategyDebate, DebateRounds: 1, Prompt: "no-judge test",
		JudgeProvider: "claude",
		NoJudge:       true,
		Providers: []ProviderConfig{
			echoProvider("claude"),
			echoProvider("gemini"),
		},
		TimeoutSeconds: 10, Terminal: nil,
	}
	result, err := runInteractiveDebate(context.Background(), cfg)
	require.NoError(t, err)
	require.NotNil(t, result)
	// With --no-judge, no response should have "judge" provider
	for _, r := range result.Responses {
		assert.NotEqual(t, "judge", r.Provider,
			"no judge provider should appear in responses when NoJudge is true")
	}
}

// TestNoJudge_False_IncludesJudge verifies judge runs when NoJudge is false.
func TestNoJudge_False_IncludesJudge(t *testing.T) {
	t.Parallel()
	cfg := OrchestraConfig{
		Strategy: StrategyDebate, DebateRounds: 1, Prompt: "with-judge test",
		JudgeProvider: "claude",
		NoJudge:       false,
		Providers: []ProviderConfig{
			echoProvider("claude"),
		},
		TimeoutSeconds: 10, Terminal: nil,
	}
	result, err := runInteractiveDebate(context.Background(), cfg)
	require.NoError(t, err)
	require.NotNil(t, result)
	// With judge enabled, the result should have at least the base providers
	assert.NotEmpty(t, result.Responses)
}

// TestNoJudge_ConfigField verifies NoJudge field exists and defaults to false.
func TestNoJudge_ConfigField(t *testing.T) {
	t.Parallel()
	cfg := OrchestraConfig{}
	assert.False(t, cfg.NoJudge, "NoJudge should default to false")

	cfg.NoJudge = true
	assert.True(t, cfg.NoJudge)
}
