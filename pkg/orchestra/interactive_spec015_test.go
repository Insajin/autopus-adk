package orchestra

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- R2: Fresh context for final ReadScreen ---

// TestWaitAndCollectResults_FreshContextOnCancel verifies that when the parent context
// is cancelled, waitAndCollectResults still reads the screen using a fresh
// context.Background() with 5s timeout (interactive.go:241-243).
func TestWaitAndCollectResults_FreshContextOnCancel(t *testing.T) {
	t.Parallel()

	mock := newCmuxMock()
	// First few ReadScreen calls return empty (no prompt match) to force timeout,
	// then the final read (with fresh context) returns partial output.
	mock.readScreenOutput = "partial output from provider"

	patterns := DefaultCompletionPatterns()
	panes := []paneInfo{
		{provider: ProviderConfig{Name: "claude"}, paneID: "pane-1", outputFile: "/tmp/test-output"},
	}

	// Use a very short timeout so the parent context cancels quickly.
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	cfg := OrchestraConfig{
		Terminal:       mock,
		TimeoutSeconds: 1,
	}
	start := time.Now()
	responses := waitAndCollectResults(ctx, cfg, panes, patterns, start, nil)

	require.Len(t, responses, 1)
	// The response should have output even though the parent ctx was cancelled,
	// because the final ReadScreen uses context.Background().
	assert.NotEmpty(t, responses[0].Output, "must collect output via fresh context after parent cancel")
}

// TestWaitAndCollectResults_SkippedPaneTimedOut verifies that panes with skipWait=true
// produce a TimedOut=true response immediately.
func TestWaitAndCollectResults_SkippedPaneTimedOut(t *testing.T) {
	t.Parallel()

	mock := newCmuxMock()
	patterns := DefaultCompletionPatterns()
	panes := []paneInfo{
		{provider: ProviderConfig{Name: "failed-provider"}, paneID: "pane-1", skipWait: true},
	}

	start := time.Now()
	cfg := OrchestraConfig{Terminal: mock}
	responses := waitAndCollectResults(context.Background(), cfg, panes, patterns, start, nil)

	require.Len(t, responses, 1)
	assert.True(t, responses[0].TimedOut, "skipped pane must be marked as timed out")
	assert.Equal(t, "failed-provider", responses[0].Provider)
}

// --- R5: waitForSessionReady uses session-ready patterns ---

// TestWaitForSessionReady_ShellDollarNotReady verifies that waitForSessionReady
// does NOT return ready when the screen shows only a shell $ prompt.
func TestWaitForSessionReady_ShellDollarNotReady(t *testing.T) {
	t.Parallel()

	mock := newCmuxMock()
	mock.readScreenOutput = "$ " // only shell prompt, no CLI prompt

	panes := []paneInfo{
		{provider: ProviderConfig{Name: "claude"}, paneID: "pane-1"},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	waitForSessionReady(ctx, mock, panes)
	// If waitForSessionReady returned, it should have been via timeout, not match.
	// Verify by checking that it took close to the timeout duration.
	assert.True(t, ctx.Err() != nil || mock.readScreenCalls > 1,
		"shell $ prompt must NOT trigger session ready — should poll until timeout")
}

// TestPollUntilSessionReady_CLIPromptReturnsTrue verifies CLI prompt detection.
func TestPollUntilSessionReady_CLIPromptReturnsTrue(t *testing.T) {
	t.Parallel()

	mock := newCmuxMock()
	mock.readScreenOutput = "❯" // claude CLI prompt

	patterns := SessionReadyPatterns()
	result := pollUntilSessionReady(
		context.Background(), mock, "pane-1", patterns, 5*time.Second,
	)
	assert.True(t, result, "CLI prompt must trigger session ready")
}

// --- R6: Per-provider startup timeouts ---

// TestStartupTimeoutFor_ProviderSpecific verifies per-provider timeout values.
func TestStartupTimeoutFor_ProviderSpecific(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		provider ProviderConfig
		expected time.Duration
	}{
		{"claude 15s", ProviderConfig{Name: "claude"}, 15 * time.Second},
		{"gemini 10s", ProviderConfig{Name: "gemini"}, 10 * time.Second},
		{"opencode 15s", ProviderConfig{Name: "opencode"}, 15 * time.Second},
		{"unknown default 30s", ProviderConfig{Name: "unknown"}, 30 * time.Second},
		{"custom override", ProviderConfig{Name: "claude", StartupTimeout: 60 * time.Second}, 60 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := startupTimeoutFor(tt.provider)
			assert.Equal(t, tt.expected, got)
		})
	}
}

// TestStartupTimeoutFor_DefaultFallback verifies the 30s fallback for unknown providers.
func TestStartupTimeoutFor_DefaultFallback(t *testing.T) {
	t.Parallel()
	got := startupTimeoutFor(ProviderConfig{Name: "brand-new-provider"})
	assert.Equal(t, 30*time.Second, got, "unknown provider must get 30s default")
}

// --- R7: Round env skipped for TUI providers ---

// TestSendRoundEnv_SkippedForTUIProvider verifies that SendRoundEnvToPane is NOT called
// for TUI-based providers (InteractiveInput != "args"). The round env export command
// would otherwise be injected as chat text in the TUI input field.
func TestSendRoundEnv_SkippedForTUIProvider(t *testing.T) {
	t.Parallel()

	mock := newCmuxMock()
	// TUI provider: InteractiveInput is empty (not "args").
	tuiProvider := ProviderConfig{Name: "opencode", InteractiveInput: ""}
	panes := []paneInfo{
		{provider: tuiProvider, paneID: "pane-1"},
	}

	cfg := OrchestraConfig{
		Terminal:    mock,
		Prompt:      "discuss topic",
		InitialDelay: 1 * time.Millisecond,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	prevResponses := []ProviderResponse{
		{Provider: "claude", Output: "claude response"},
	}
	_ = executeRound(ctx, cfg, panes, nil, 2, prevResponses)

	// Verify that no "export AUTOPUS_ROUND" command was sent via SendCommand.
	for _, call := range mock.sendCommandCalls {
		assert.NotContains(t, call.Cmd, "AUTOPUS_ROUND",
			"TUI provider must not receive export AUTOPUS_ROUND via SendCommand")
	}
}

// TestSendRoundEnv_SentForArgsProvider verifies that SendRoundEnvToPane IS called
// for args-based providers (InteractiveInput == "args").
func TestSendRoundEnv_SentForArgsProvider(t *testing.T) {
	t.Parallel()

	mock := newCmuxMock()
	// Return a claude prompt so pollUntilPrompt exits immediately.
	mock.readScreenOutput = "❯"
	// args provider: InteractiveInput is "args". skipWait=false so the round-env path is reached.
	argsProvider := ProviderConfig{Name: "claude", InteractiveInput: "args"}
	panes := []paneInfo{
		{provider: argsProvider, paneID: "pane-2"},
	}

	cfg := OrchestraConfig{
		Terminal:     mock,
		Prompt:       "discuss topic",
		InitialDelay: 1 * time.Millisecond,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	prevResponses := []ProviderResponse{
		{Provider: "opencode", Output: "opencode response"},
	}
	_ = executeRound(ctx, cfg, panes, nil, 2, prevResponses)

	// Verify that "export AUTOPUS_ROUND=2" was sent via SendCommand for args-based provider.
	found := false
	for _, call := range mock.sendCommandCalls {
		if call.Cmd == "export AUTOPUS_ROUND=2" {
			found = true
			break
		}
	}
	assert.True(t, found, "args provider must receive export AUTOPUS_ROUND via SendCommand")
}
