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
		Prompt: "default round", Providers: []ProviderConfig{{Name: "claude", Binary: "echo"}},
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
				Prompt: "range test", Providers: []ProviderConfig{{Name: "claude", Binary: "echo"}},
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
		Providers: []ProviderConfig{{Name: "claude", Binary: "echo"}},
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
		Providers: []ProviderConfig{{Name: "claude", Binary: "echo"}},
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
			{Name: "claude", Binary: "echo"},
			{Name: "gemini", Binary: "echo"},
		},
		TimeoutSeconds: 10, Terminal: nil,
	}
	result, err := runInteractiveDebate(context.Background(), cfg)
	require.NoError(t, err)
	assert.NotNil(t, result)
}

// TestRunInteractiveDebate_NonExistentBinary verifies fallback path when
// binary does not exist (runDebate fails, falls through to parallel/empty).
func TestRunInteractiveDebate_NonExistentBinary(t *testing.T) {
	t.Parallel()
	cfg := OrchestraConfig{
		Strategy: StrategyDebate, DebateRounds: 1, Prompt: "bad binary test",
		Providers: []ProviderConfig{{Name: "test", Binary: "nonexistent-binary-xyz"}},
		TimeoutSeconds: 5, Terminal: nil,
	}
	result, err := runInteractiveDebate(context.Background(), cfg)
	require.NoError(t, err)
	assert.NotNil(t, result)
}

// --- REQ-2: Topic isolation ---

// TestExecuteRound_TopicIsolation verifies executeRound wraps prompts with topic isolation prefix.
func TestExecuteRound_TopicIsolation(t *testing.T) {
	t.Parallel()
	mock := newCmuxMock()
	mock.readScreenOutput = ">\n"
	cfg := OrchestraConfig{
		Providers: []ProviderConfig{
			{Name: "claude", Binary: "echo"},
		},
		Strategy:       StrategyDebate,
		Prompt:         "discuss testing",
		TimeoutSeconds: 5,
		Terminal:       mock,
		Interactive:    true,
		InitialDelay:  time.Millisecond,
	}
	panes := []paneInfo{{provider: cfg.Providers[0], paneID: "pane-1"}}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Round 1 (no previous responses)
	_ = executeRound(ctx, cfg, panes, nil, 1, nil)

	// Verify the prompt sent contains the isolation instruction
	found := false
	for _, call := range mock.sendCommandCalls {
		if strings.Contains(call.Cmd, "IMPORTANT: Discuss ONLY") {
			found = true
			break
		}
	}
	assert.True(t, found, "round 1 prompt must include topic isolation instruction")
}

// Helper function tests (consensusReached, countNonEmpty, perRoundTimeout,
// buildDebateResult, mergeByStrategyWithRoundHistory) are in
// interactive_debate_helpers_test.go.

// TestRunInteractiveDebate_SingleProvider verifies debate with only one provider.
func TestRunInteractiveDebate_SingleProvider(t *testing.T) {
	t.Parallel()
	cfg := OrchestraConfig{
		Strategy: StrategyDebate, DebateRounds: 2, Prompt: "single provider debate",
		Providers:      []ProviderConfig{{Name: "claude", Binary: "echo"}},
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
		JudgeProvider: "echo",
		Providers: []ProviderConfig{
			{Name: "claude", Binary: "echo"},
			{Name: "gemini", Binary: "echo"},
		},
		TimeoutSeconds: 10, Terminal: nil,
	}
	result, err := runInteractiveDebate(context.Background(), cfg)
	require.NoError(t, err)
	assert.NotNil(t, result)
}
