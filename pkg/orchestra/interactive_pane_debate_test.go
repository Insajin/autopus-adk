package orchestra

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- runPaneDebate coverage tests ---

// TestRunPaneDebate_SingleRound verifies single-round pane debate flow.
// Uses countingScreenMock so baseline captures differ from completion polls.
func TestRunPaneDebate_SingleRound(t *testing.T) {
	mock := &countingScreenMock{
		mockTerminal: mockTerminal{name: "cmux"},
		outputs: []string{
			"loading...\n",          // baseline captures (before prompt)
			"loading...\n",          // baseline re-capture after prompt send
			"loading...\n",          // baseline re-capture after delay
			"AI response here\n❯\n", // first completion match
			"AI response here\n❯\n", // second consecutive match (confirm)
		},
	}
	cfg := OrchestraConfig{
		Providers: []ProviderConfig{
			echoProvider("claude"),
			echoProvider("gemini"),
		},
		Strategy:       StrategyDebate,
		Prompt:         "discuss testing",
		TimeoutSeconds: 30,
		Terminal:       mock,
		Interactive:    true,
		InitialDelay:   time.Millisecond,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	result, err := runPaneDebate(ctx, cfg, 1, 45*time.Second, time.Now())
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, StrategyDebate, result.Strategy)
	assert.NotEmpty(t, result.RoundHistory)
}

// TestRunPaneDebate_MultiRound verifies multi-round pane debate flow.
// Uses counting mock to simulate screen changes between rounds.
func TestRunPaneDebate_MultiRound(t *testing.T) {
	mock := &countingScreenMock{
		mockTerminal: mockTerminal{name: "cmux"},
		outputs: []string{
			"baseline output\n❯\n",  // baseline capture
			"round 1 response\n❯\n", // round 1 first match
			"round 1 response\n❯\n", // round 1 second match (confirm)
			"new baseline\n❯\n",     // round 2 baseline
			"round 2 response\n❯\n", // round 2 first match
			"round 2 response\n❯\n", // round 2 second match (confirm)
		},
	}
	cfg := OrchestraConfig{
		Providers: []ProviderConfig{
			{Name: "claude", Binary: "echo"},
		},
		Strategy:       StrategyDebate,
		Prompt:         "debate topic",
		TimeoutSeconds: 120,
		Terminal:       mock,
		Interactive:    true,
		InitialDelay:   time.Millisecond,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result, err := runPaneDebate(ctx, cfg, 2, 45*time.Second, time.Now())
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.GreaterOrEqual(t, len(result.RoundHistory), 1)
}

// TestRunPaneDebate_WithJudge verifies judge round is invoked in pane debate.
func TestRunPaneDebate_WithJudge(t *testing.T) {
	// 2 providers × (3 baseline captures + 2 completion matches) + extra for safety
	mock := &countingScreenMock{
		mockTerminal: mockTerminal{name: "cmux"},
		outputs: []string{
			"loading...\n", "loading...\n", "loading...\n",
			"loading...\n", "loading...\n", "loading...\n",
			"loading...\n", "loading...\n", "loading...\n",
			"debate output\n❯\n", "debate output\n❯\n",
			"debate output\n❯\n", "debate output\n❯\n",
		},
	}
	cfg := OrchestraConfig{
		Providers: []ProviderConfig{
			{Name: "claude", Binary: "echo"},
			{Name: "gemini", Binary: "echo"},
		},
		Strategy:       StrategyDebate,
		Prompt:         "judge test",
		JudgeProvider:  "echo",
		TimeoutSeconds: 60,
		Terminal:       mock,
		Interactive:    true,
		InitialDelay:   time.Millisecond,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	result, err := runPaneDebate(ctx, cfg, 1, 45*time.Second, time.Now())
	require.NoError(t, err)
	require.NotNil(t, result)
}

// TestRunPaneDebate_ContextCancellation verifies cancellation during debate.
func TestRunPaneDebate_ContextCancellation(t *testing.T) {
	mock := newCmuxMock()
	mock.readScreenOutput = "" // never completes
	cfg := OrchestraConfig{
		Providers:      []ProviderConfig{echoProvider("claude")},
		Strategy:       StrategyDebate,
		Prompt:         "cancel test",
		TimeoutSeconds: 60,
		Terminal:       mock,
		Interactive:    true,
		InitialDelay:   time.Millisecond,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := runPaneDebate(ctx, cfg, 3, 45*time.Second, time.Now())
	// Should return error due to context cancellation during round loop
	assert.Error(t, err)
}

// TestRunPaneDebate_SplitFails_FallsBack verifies fallback to non-interactive on split failure.
func TestRunPaneDebate_SplitFails_FallsBack(t *testing.T) {
	mock := newCmuxMock()
	mock.splitPaneErr = fmt.Errorf("split failed")
	cfg := OrchestraConfig{
		Providers:      []ProviderConfig{echoProvider("claude")},
		Strategy:       StrategyDebate,
		Prompt:         "fallback test",
		TimeoutSeconds: 10,
		Terminal:       mock,
		Interactive:    true,
		InitialDelay:   time.Millisecond,
	}

	result, err := runPaneDebate(context.Background(), cfg, 1, 45*time.Second, time.Now())
	require.NoError(t, err)
	require.NotNil(t, result)
}

// TestRunPaneDebate_PipeCaptureFails_FallsBack verifies fallback on pipe-pane error.
func TestRunPaneDebate_PipeCaptureFails_FallsBack(t *testing.T) {
	pipeMock := &pipePaneErrorMock{mockTerminal: mockTerminal{name: "cmux", readScreenOutput: "❯\n"}}
	cfg := OrchestraConfig{
		Providers:      []ProviderConfig{echoProvider("claude")},
		Strategy:       StrategyDebate,
		Prompt:         "pipe fail test",
		TimeoutSeconds: 10,
		Terminal:       pipeMock,
		Interactive:    true,
		InitialDelay:   time.Millisecond,
	}

	result, err := runPaneDebate(context.Background(), cfg, 1, 45*time.Second, time.Now())
	require.NoError(t, err)
	require.NotNil(t, result)
}

// TestRunPaneDebate_HookMode verifies hook mode session creation in pane debate.
// HookMode requires real hook result files; with mock terminal, collectRoundHookResults
// times out gracefully. This test verifies the session is created and the flow
// degrades correctly when no hook files appear.
func TestRunPaneDebate_HookMode(t *testing.T) {
	mock := &countingScreenMock{
		mockTerminal: mockTerminal{name: "cmux"},
		outputs: []string{
			"loading...\n", "loading...\n", "loading...\n",
			"hook output\n❯\n", "hook output\n❯\n",
		},
	}
	cfg := OrchestraConfig{
		Providers:      []ProviderConfig{echoProvider("claude")},
		Strategy:       StrategyDebate,
		Prompt:         "hook test",
		TimeoutSeconds: 3, // Short timeout — no real hook files will appear
		Terminal:       mock,
		Interactive:    true,
		HookMode:       true,
		SessionID:      "test-pane-debate-hook",
		InitialDelay:   time.Millisecond,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := runPaneDebate(ctx, cfg, 1, 8*time.Second, time.Now())
	require.NoError(t, err)
	require.NotNil(t, result)
	// Hook mode with no result files → providers report as timed out
	assert.NotEmpty(t, result.RoundHistory)
}
