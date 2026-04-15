package orchestra

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- runInteractiveDebate tests ---

// TestRunInteractiveDebate_DefaultRound verifies that DebateRounds=0
// defaults to 1 round (no rebuttal).
func TestRunInteractiveDebate_DefaultRound(t *testing.T) {
	t.Parallel()
	cfg := OrchestraConfig{
		Strategy: StrategyDebate, DebateRounds: 0,
		Prompt:         "default round",
		Providers:      []ProviderConfig{echoProvider("claude")},
		TimeoutSeconds: 10, Interactive: true, HookMode: true, SessionID: "test-debate-default", InitialDelay: time.Millisecond,
	}
	result, err := runInteractiveDebate(context.Background(), cfg)
	require.NoError(t, err)
	assert.NotNil(t, result)
}

// TestRunInteractiveDebate_RoundsFlag_Range verifies --rounds N validation.
func TestRunInteractiveDebate_RoundsFlag_Range(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		rounds    int
		expectErr bool
	}{
		{"valid 1", 1, false},
		{"valid 10", 10, false},
		{"zero defaults", 0, false},
		{"negative", -1, true},
		{"exceeds max", 11, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := OrchestraConfig{
				Strategy: StrategyDebate, DebateRounds: tt.rounds,
				Prompt:         "range test",
				Providers:      []ProviderConfig{echoProvider("claude")},
				TimeoutSeconds: 5, Interactive: true, HookMode: true, SessionID: "test-range", InitialDelay: time.Millisecond,
			}
			_, err := runInteractiveDebate(context.Background(), cfg)
			if tt.expectErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "rounds must be")
			}
		})
	}
}

// TestRunInteractiveDebate_ContextCancellation verifies context cancellation.
func TestRunInteractiveDebate_ContextCancellation(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cfg := OrchestraConfig{
		Strategy: StrategyDebate, DebateRounds: 3, Prompt: "cancel test",
		Providers:      []ProviderConfig{{Name: "claude", Binary: "echo"}},
		TimeoutSeconds: 10, Interactive: true, HookMode: true, SessionID: "test-cancel", InitialDelay: time.Millisecond,
	}
	_, err := runInteractiveDebate(ctx, cfg)
	assert.Error(t, err, "must return error on cancelled context")
}

// TestRunInteractiveDebate_NoTerminal_Fallback verifies nil terminal triggers
// non-interactive fallback path.
func TestRunInteractiveDebate_NoTerminal_Fallback(t *testing.T) {
	t.Parallel()
	cfg := OrchestraConfig{
		Strategy: StrategyDebate, DebateRounds: 1, Prompt: "no terminal test",
		Providers:      []ProviderConfig{echoProvider("claude")},
		TimeoutSeconds: 10, Terminal: nil,
	}
	result, err := runInteractiveDebate(context.Background(), cfg)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, StrategyDebate, result.Strategy)
}

// TestRunInteractiveDebate_MultiRound_NoTerminal verifies multi-round debate
// runs the non-interactive fallback path.
func TestRunInteractiveDebate_MultiRound_NoTerminal(t *testing.T) {
	t.Parallel()
	cfg := OrchestraConfig{
		Strategy: StrategyDebate, DebateRounds: 2, Prompt: "multi round",
		Providers: []ProviderConfig{
			echoProvider("claude"),
			echoProvider("gemini"),
		},
		TimeoutSeconds: 10, Terminal: nil,
	}
	result, err := runInteractiveDebate(context.Background(), cfg)
	require.NoError(t, err)
	assert.NotNil(t, result)
}

// TestRunInteractiveDebate_NonExistentBinary verifies fallback path returns an
// error when debate and fallback execution both fail.
func TestRunInteractiveDebate_NonExistentBinary(t *testing.T) {
	t.Parallel()
	cfg := OrchestraConfig{
		Strategy: StrategyDebate, DebateRounds: 1, Prompt: "bad binary test",
		Providers:      []ProviderConfig{{Name: "test", Binary: "nonexistent-binary-xyz"}},
		TimeoutSeconds: 5, Terminal: nil,
	}
	result, err := runInteractiveDebate(context.Background(), cfg)
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "fallback failed")
}

// --- REQ-2: Topic isolation ---

// TestExecuteRound_TopicIsolation verifies executeRound wraps prompts with topic isolation prefix.
func TestExecuteRound_TopicIsolation(t *testing.T) {
	t.Parallel()
	mock := newCmuxMock()
	mock.readScreenOutput = "❯\n"
	cfg := OrchestraConfig{
		Providers: []ProviderConfig{
			{Name: "claude", Binary: "echo"},
		},
		Strategy:       StrategyDebate,
		Prompt:         "discuss testing",
		TimeoutSeconds: 5,
		Terminal:       mock,
		Interactive:    true,
		InitialDelay:   time.Millisecond,
	}
	panes := []paneInfo{{provider: cfg.Providers[0], paneID: "pane-1"}}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Round 1 (no previous responses)
	_ = executeRound(ctx, cfg, panes, nil, 1, nil)

	// Verify the prompt sent contains the isolation instruction
	found := false
	for _, call := range mock.sendLongTextCalls {
		if strings.Contains(call.Text, "IMPORTANT: Discuss ONLY") {
			found = true
			break
		}
	}
	assert.True(t, found, "round 1 prompt must include topic isolation instruction")
}

// R8 tests are in interactive_debate_r8_test.go.

// Helper function tests (consensusReached, countNonEmpty, perRoundTimeout,
// buildDebateResult, mergeByStrategyWithRoundHistory) are in
// interactive_debate_helpers_test.go.

// TestRunInteractiveDebate_SingleProvider verifies debate with only one provider.
func TestRunInteractiveDebate_SingleProvider(t *testing.T) {
	t.Parallel()
	cfg := OrchestraConfig{
		Strategy: StrategyDebate, DebateRounds: 2, Prompt: "single provider debate",
		Providers:      []ProviderConfig{echoProvider("claude")},
		TimeoutSeconds: 10, Terminal: nil,
	}
	result, err := runInteractiveDebate(context.Background(), cfg)
	require.NoError(t, err)
	assert.NotNil(t, result)
}

// TestRunInteractiveDebate_WithJudge_NoTerminal verifies judge path in
// non-interactive fallback mode.
func TestRunInteractiveDebate_WithJudge_NoTerminal(t *testing.T) {
	t.Parallel()
	cfg := OrchestraConfig{
		Strategy: StrategyDebate, DebateRounds: 1, Prompt: "judge test",
		JudgeProvider: "claude",
		Providers: []ProviderConfig{
			echoProvider("claude"),
			echoProvider("gemini"),
		},
		TimeoutSeconds: 10, Terminal: nil,
	}
	result, err := runInteractiveDebate(context.Background(), cfg)
	require.NoError(t, err)
	assert.NotNil(t, result)
}

// TestExecuteRound_ArgsProviderSkipsSendRound1 verifies providers with
// InteractiveInput="args" skip SendLongText in round 1.
func TestExecuteRound_ArgsProviderSkipsSendRound1(t *testing.T) {
	t.Parallel()
	mock := newCmuxMock()
	mock.readScreenOutput = "❯\n"
	argsProvider := ProviderConfig{
		Name: "opencode", Binary: "opencode",
		InteractiveInput: "args",
	}
	cfg := OrchestraConfig{
		Providers:      []ProviderConfig{argsProvider},
		Strategy:       StrategyDebate,
		Prompt:         "test",
		TimeoutSeconds: 5,
		Terminal:       mock,
		Interactive:    true,
		InitialDelay:   time.Millisecond,
	}
	panes := []paneInfo{{provider: argsProvider, paneID: "pane-1"}}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_ = executeRound(ctx, cfg, panes, nil, 1, nil)
	// Args provider should NOT have received SendLongText in round 1
	assert.Empty(t, mock.sendLongTextCalls,
		"args provider must skip SendLongText in round 1")
}

// TestExecuteRound_Round2_RebuttalPrompt verifies round 2 sends rebuttal prompt
// built from previous round responses.
func TestExecuteRound_Round2_RebuttalPrompt(t *testing.T) {
	t.Parallel()
	mock := newCmuxMock()
	mock.readScreenOutput = "❯\n"
	cfg := OrchestraConfig{
		Providers: []ProviderConfig{
			{Name: "claude", Binary: "echo"},
		},
		Strategy:       StrategyDebate,
		Prompt:         "discuss testing",
		TimeoutSeconds: 5,
		Terminal:       mock,
		Interactive:    true,
		InitialDelay:   time.Millisecond,
	}
	panes := []paneInfo{{provider: cfg.Providers[0], paneID: "pane-1"}}
	prevResponses := []ProviderResponse{
		{Provider: "gemini", Output: "gemini's take"},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_ = executeRound(ctx, cfg, panes, nil, 2, prevResponses)
	// Round 2 should have sent a rebuttal prompt
	require.NotEmpty(t, mock.sendLongTextCalls)
	sent := mock.sendLongTextCalls[0].Text
	assert.Contains(t, sent, "IMPORTANT: Discuss ONLY",
		"round 2 rebuttal must include topic isolation")
}

// TestExecuteRound_SkipWaitProviders verifies skipWait providers are skipped.
func TestExecuteRound_SkipWaitProviders(t *testing.T) {
	t.Parallel()
	mock := newCmuxMock()
	mock.readScreenOutput = "❯\n"
	cfg := OrchestraConfig{
		Providers:      []ProviderConfig{{Name: "p1", Binary: "echo"}},
		Strategy:       StrategyDebate,
		Prompt:         "test",
		TimeoutSeconds: 5,
		Terminal:       mock,
		Interactive:    true,
		InitialDelay:   time.Millisecond,
	}
	panes := []paneInfo{{provider: cfg.Providers[0], paneID: "pane-1", skipWait: true}}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	responses := executeRound(ctx, cfg, panes, nil, 1, nil)
	// skipWait providers should not receive any prompts
	assert.Empty(t, mock.sendLongTextCalls)
	_ = responses
}
