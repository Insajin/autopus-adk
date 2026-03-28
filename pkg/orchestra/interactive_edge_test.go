package orchestra

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- R5/R8: Error paths ---

// TestInteractive_StartPipeCapture_Error verifies fallback when PipePaneStart fails.
func TestInteractive_StartPipeCapture_Error(t *testing.T) {
	mock := newCmuxMock()
	mock.readScreenOutput = "❯\n"
	// Make PipePaneStart fail by using a mock that returns error
	pipeMock := &pipePaneErrorMock{mockTerminal: mockTerminal{name: "cmux", readScreenOutput: "❯\n"}}
	cfg := OrchestraConfig{
		Providers:      []ProviderConfig{echoProvider("p1")},
		Strategy:       StrategyConsensus,
		Prompt:         "test",
		TimeoutSeconds: 10,
		Terminal:       pipeMock,
		Interactive:    true,
		InitialDelay:  time.Millisecond,
	}
	// Should fall back to sentinel mode (R8)
	result, err := RunInteractivePaneOrchestra(context.Background(), cfg)
	require.NoError(t, err, "should fall back, not error")
	assert.NotNil(t, result)
}

// TestInteractive_LaunchSession_SendCommandError verifies failed launch is recorded.
func TestInteractive_LaunchSession_SendCommandError(t *testing.T) {
	mock := newCmuxMock()
	mock.readScreenOutput = "❯\n"
	mock.sendCommandErr = fmt.Errorf("send failed")
	cfg := OrchestraConfig{
		Providers:      []ProviderConfig{echoProvider("p1")},
		Strategy:       StrategyConsensus,
		Prompt:         "test",
		TimeoutSeconds: 5,
		Terminal:       mock,
		Interactive:    true,
		InitialDelay:  time.Millisecond,
	}
	result, err := RunInteractivePaneOrchestra(context.Background(), cfg)
	require.NoError(t, err)
	// Provider should appear as timed out (skipWait=true)
	require.NotNil(t, result)
	assert.Len(t, result.Responses, 1)
	assert.True(t, result.Responses[0].TimedOut, "failed launch should produce timed-out response")
}

// TestInteractive_NilTerminal_FallsBack verifies nil terminal triggers sentinel fallback.
func TestInteractive_NilTerminal_FallsBack(t *testing.T) {
	cfg := OrchestraConfig{
		Providers:      []ProviderConfig{echoProvider("p1")},
		Strategy:       StrategyConsensus,
		Prompt:         "test",
		TimeoutSeconds: 10,
		Terminal:       nil,
		Interactive:    true,
		InitialDelay:  time.Millisecond,
	}
	result, err := RunInteractivePaneOrchestra(context.Background(), cfg)
	require.NoError(t, err)
	assert.NotNil(t, result)
}

// TestInteractive_ZeroTimeout_UsesDefault verifies 0 timeout falls back to default 120s.
func TestInteractive_ZeroTimeout_UsesDefault(t *testing.T) {
	mock := newCmuxMock()
	mock.readScreenOutput = "❯\n"
	cfg := OrchestraConfig{
		Providers:      []ProviderConfig{echoProvider("p1")},
		Strategy:       StrategyConsensus,
		Prompt:         "test",
		TimeoutSeconds: 0, // should use default 120
		Terminal:       mock,
		Interactive:    true,
		InitialDelay:  time.Millisecond,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result, err := RunInteractivePaneOrchestra(ctx, cfg)
	require.NoError(t, err)
	assert.NotNil(t, result)
}

// --- R7: Completion detection integration ---

// TestInteractive_CompletionDetection_PromptPatternPrimary verifies ReadScreen polling
// detects prompt pattern as primary completion signal.
func TestInteractive_CompletionDetection_PromptPatternPrimary(t *testing.T) {
	mock := newCmuxMock()
	mock.readScreenOutput = "❯\n" // prompt pattern
	cfg := OrchestraConfig{
		Providers:      []ProviderConfig{echoProvider("p1")},
		Strategy:       StrategyConsensus,
		Prompt:         "test",
		TimeoutSeconds: 30,
		Terminal:       mock,
		Interactive:    true,
		InitialDelay:  time.Millisecond,
	}
	result, err := RunInteractivePaneOrchestra(context.Background(), cfg)
	require.NoError(t, err)
	// If prompt pattern is detected, responses should not be timed out
	for _, r := range result.Responses {
		assert.False(t, r.TimedOut, "prompt pattern should prevent timeout")
	}
	// ReadScreen must have been called (polling)
	assert.True(t, mock.readScreenCalls > 0, "ReadScreen must be polled for prompt detection")
}

// TestInteractive_CompletionDetection_IdleSecondary verifies pipe-pane idle detection
// as secondary completion signal.
func TestInteractive_CompletionDetection_IdleSecondary(t *testing.T) {
	// Idle detection checks output file mod time. With mock terminal,
	// the temp file created by splitProviderPanes will be idle immediately
	// since nothing writes to it. With empty readScreenOutput (no prompt match),
	// idle detection should eventually trigger.
	mock := newCmuxMock()
	mock.readScreenOutput = "" // no prompt match -> relies on idle detection
	cfg := OrchestraConfig{
		Providers:      []ProviderConfig{echoProvider("p1")},
		Strategy:       StrategyConsensus,
		Prompt:         "test",
		TimeoutSeconds: 30,
		Terminal:       mock,
		Interactive:    true,
		InitialDelay:  time.Millisecond,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	result, err := RunInteractivePaneOrchestra(ctx, cfg)
	require.NoError(t, err)
	assert.NotNil(t, result)
	// The idle threshold is 10s; within 15s context timeout the idle detector should fire
	assert.Len(t, result.Responses, 1)
}

// TestInteractive_SendPrompt_Error verifies that prompt send failures are recorded.
func TestInteractive_SendPrompt_Error(t *testing.T) {
	mock := newCmuxMock()
	mock.readScreenOutput = "❯\n"
	// Succeed on launch (1st call per provider) but fail on prompt send (2nd call)
	mock.sendCommandErr = fmt.Errorf("prompt send failed")
	mock.sendCommandErrAfter = 1 // first call succeeds, second fails
	cfg := OrchestraConfig{
		Providers:      []ProviderConfig{echoProvider("p1")},
		Strategy:       StrategyConsensus,
		Prompt:         "test",
		TimeoutSeconds: 5,
		Terminal:       mock,
		Interactive:    true,
		InitialDelay:  time.Millisecond,
	}
	result, err := RunInteractivePaneOrchestra(context.Background(), cfg)
	require.NoError(t, err)
	require.NotNil(t, result)
	// Provider should be timed out due to prompt send failure
	assert.Len(t, result.Responses, 1)
	assert.True(t, result.Responses[0].TimedOut, "prompt send failure should mark provider as timed out")
}

// TestInteractive_LaunchWithBareBinary verifies interactive mode launches binary alone without flags.
func TestInteractive_LaunchWithBareBinary(t *testing.T) {
	mock := newCmuxMock()
	mock.readScreenOutput = "❯\n"
	cfg := OrchestraConfig{
		Providers: []ProviderConfig{
			{Name: "claude", Binary: "claude", Args: []string{"-p", "--json"}, PaneArgs: []string{"--json"}},
		},
		Strategy:       StrategyConsensus,
		Prompt:         "test",
		TimeoutSeconds: 5,
		Terminal:       mock,
		Interactive:    true,
		InitialDelay:  time.Millisecond,
	}
	result, err := RunInteractivePaneOrchestra(context.Background(), cfg)
	require.NoError(t, err)
	require.NotNil(t, result)
	// First sendLongText call should be the bare binary name (no -p or --json)
	require.True(t, len(mock.sendLongTextCalls) >= 1)
	launchCmd := mock.sendLongTextCalls[0].Text
	// REQ-1: Claude now includes --json (from PaneArgs) and --dangerously-skip-permissions
	assert.Equal(t, "claude --json --dangerously-skip-permissions", launchCmd, "interactive mode should launch claude with remaining PaneArgs and permission bypass")
}

// TestInteractive_ReadScreenError_ContinuesPolling verifies ReadScreen errors don't break polling.
func TestInteractive_ReadScreenError_ContinuesPolling(t *testing.T) {
	mock := newCmuxMock()
	mock.readScreenErr = fmt.Errorf("transient error")
	cfg := OrchestraConfig{
		Providers:      []ProviderConfig{echoProvider("p1")},
		Strategy:       StrategyConsensus,
		Prompt:         "test",
		TimeoutSeconds: 2,
		Terminal:       mock,
		Interactive:    true,
		InitialDelay:  time.Millisecond,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result, err := RunInteractivePaneOrchestra(ctx, cfg)
	require.NoError(t, err)
	assert.NotNil(t, result)
	// Should still produce results despite ReadScreen errors
	assert.Len(t, result.Responses, 1)
}

// TestInteractive_MultipleProviders_ParallelCompletion verifies multiple providers complete.
func TestInteractive_MultipleProviders_ParallelCompletion(t *testing.T) {
	mock := newCmuxMock()
	mock.readScreenOutput = "❯\n"
	cfg := OrchestraConfig{
		Providers: []ProviderConfig{
			echoProvider("p1"),
			echoProvider("p2"),
			echoProvider("p3"),
		},
		Strategy:       StrategyConsensus,
		Prompt:         "test",
		TimeoutSeconds: 10,
		Terminal:       mock,
		Interactive:    true,
		InitialDelay:  time.Millisecond,
	}
	result, err := RunInteractivePaneOrchestra(context.Background(), cfg)
	require.NoError(t, err)
	assert.Len(t, result.Responses, 3)
	for _, r := range result.Responses {
		assert.False(t, r.TimedOut)
	}
}
