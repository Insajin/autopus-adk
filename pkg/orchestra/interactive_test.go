package orchestra

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/insajin/autopus-adk/pkg/terminal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestInteractive_FullFlow verifies the complete interactive pane orchestration flow.
func TestInteractive_FullFlow_SplitPipelaunchWaitCollectMergeCleanup(t *testing.T) {
	mock := newCmuxMock()
	mock.readScreenOutput = "❯\n" // prompt pattern triggers immediate completion
	cfg := OrchestraConfig{
		Providers:      []ProviderConfig{echoProvider("p1"), echoProvider("p2")},
		Strategy:       StrategyConsensus,
		Prompt:         "write tests",
		TimeoutSeconds: 30,
		Terminal:       mock,
		Interactive:    true,
		InitialDelay:  time.Millisecond,
	}
	result, err := RunInteractivePaneOrchestra(context.Background(), cfg)
	require.NoError(t, err)
	assert.Len(t, result.Responses, 2)
	assert.NotEmpty(t, result.Merged)
	assert.Len(t, mock.splitPaneCalls, 2)
	assert.True(t, mock.pipePaneStartCalls >= 2, "must start pipe for each provider")
}

// TestInteractive_Flow_PipePaneStartCalledPerProvider verifies each provider gets pipe capture.
func TestInteractive_Flow_PipePaneStartCalledPerProvider(t *testing.T) {
	mock := newCmuxMock()
	mock.readScreenOutput = "❯\n"
	cfg := OrchestraConfig{
		Providers:      []ProviderConfig{echoProvider("p1"), echoProvider("p2"), echoProvider("p3")},
		Strategy:       StrategyConsensus,
		Prompt:         "test",
		TimeoutSeconds: 30,
		Terminal:       mock,
		Interactive:    true,
		InitialDelay:  time.Millisecond,
	}
	_, err := RunInteractivePaneOrchestra(context.Background(), cfg)
	require.NoError(t, err)
	assert.Equal(t, 3, mock.pipePaneStartCalls, "pipe-pane start must be called for each provider")
}

// TestInteractive_Flow_PipePaneStopCalledOnCleanup verifies pipe-pane stop on cleanup.
func TestInteractive_Flow_PipePaneStopCalledOnCleanup(t *testing.T) {
	mock := newCmuxMock()
	mock.readScreenOutput = "❯\n"
	cfg := OrchestraConfig{
		Providers:      []ProviderConfig{echoProvider("p1")},
		Strategy:       StrategyConsensus,
		Prompt:         "test",
		TimeoutSeconds: 30,
		Terminal:       mock,
		Interactive:    true,
		InitialDelay:  time.Millisecond,
	}
	_, _ = RunInteractivePaneOrchestra(context.Background(), cfg)
	assert.Equal(t, 1, mock.pipePaneStopCalls, "pipe-pane stop must be called during cleanup")
}

// TestInteractive_Flow_ResultsCollectedFromOutputFiles verifies output is populated from ReadScreen.
func TestInteractive_Flow_ResultsCollectedFromOutputFiles(t *testing.T) {
	mock := newCmuxMock()
	mock.readScreenOutput = ">\nsome output here"
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
	require.Len(t, result.Responses, 1)
	// ReadScreen returns ">\nsome output here", after cleaning prompt lines, output has content
	assert.NotEmpty(t, result.Responses[0].Output, "output should be populated from ReadScreen")
}

// TestInteractive_SentinelFallback_PlainTerminal verifies fallback to sentinel mode.
func TestInteractive_SentinelFallback_PlainTerminal(t *testing.T) {
	mock := newPlainMock()
	cfg := OrchestraConfig{
		Providers:      []ProviderConfig{echoProvider("p1")},
		Strategy:       StrategyConsensus,
		Prompt:         "test",
		TimeoutSeconds: 10,
		Terminal:       mock,
		Interactive:    true,
		InitialDelay:  time.Millisecond,
	}
	result, err := RunInteractivePaneOrchestra(context.Background(), cfg)
	require.NoError(t, err)
	assert.NotNil(t, result, "must fall back to sentinel mode, not error")
}

// TestInteractive_SentinelFallback_InteractiveModeFails verifies error fallback.
func TestInteractive_SentinelFallback_InteractiveModeFails(t *testing.T) {
	mock := newCmuxMock()
	mock.splitPaneErr = fmt.Errorf("interactive split failed")
	cfg := OrchestraConfig{
		Providers:      []ProviderConfig{echoProvider("p1")},
		Strategy:       StrategyConsensus,
		Prompt:         "test",
		TimeoutSeconds: 10,
		Terminal:       mock,
		Interactive:    true,
		InitialDelay:  time.Millisecond,
	}
	result, err := RunInteractivePaneOrchestra(context.Background(), cfg)
	require.NoError(t, err, "should fall back, not error")
	assert.NotNil(t, result)
}

// TestInteractive_SessionTimeout_ProducesPartialResult verifies TimedOut: true.
func TestInteractive_SessionTimeout_ProducesPartialResult(t *testing.T) {
	mock := newCmuxMock()
	mock.readScreenOutput = "" // never matches prompt -> forces timeout
	cfg := OrchestraConfig{
		Providers:      []ProviderConfig{echoProvider("slow")},
		Strategy:       StrategyConsensus,
		Prompt:         "test",
		TimeoutSeconds: 1, // very short timeout
		Terminal:       mock,
		Interactive:    true,
		InitialDelay:  time.Millisecond,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	result, err := RunInteractivePaneOrchestra(ctx, cfg)
	require.NoError(t, err)
	require.NotNil(t, result)
	// At least one response should be TimedOut
	found := false
	for _, r := range result.Responses {
		if r.TimedOut {
			found = true
			break
		}
	}
	assert.True(t, found, "timed out session must set TimedOut: true")
}

// TestInteractive_SessionTimeout_PartialOutputPreserved verifies partial output is kept.
func TestInteractive_SessionTimeout_PartialOutputPreserved(t *testing.T) {
	mock := newCmuxMock()
	mock.readScreenOutput = "partial output before timeout"
	cfg := OrchestraConfig{
		Providers:      []ProviderConfig{echoProvider("slow")},
		Strategy:       StrategyConsensus,
		Prompt:         "test",
		TimeoutSeconds: 1,
		Terminal:       mock,
		Interactive:    true,
		InitialDelay:  time.Millisecond,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	result, err := RunInteractivePaneOrchestra(ctx, cfg)
	require.NoError(t, err)
	require.NotNil(t, result)
	for _, r := range result.Responses {
		if r.TimedOut {
			assert.NotEmpty(t, r.Output, "partial output should be preserved on timeout")
		}
	}
}

// TestInteractive_ConfigField_InteractiveBool verifies OrchestraConfig Interactive field.
func TestInteractive_ConfigField_InteractiveBool(t *testing.T) {
	t.Parallel()
	cfg := OrchestraConfig{Interactive: true}
	assert.True(t, cfg.Interactive)
}

// TestBuildInteractiveLaunchCmd_PermissionBypass verifies Claude --dangerously-skip-permissions.
func TestBuildInteractiveLaunchCmd_PermissionBypass(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		provider ProviderConfig
		want     bool
	}{
		{
			"claude includes flag",
			ProviderConfig{Name: "claude", Binary: "claude", PaneArgs: []string{"--model", "opus"}},
			true,
		},
		{
			"opencode excludes flag",
			ProviderConfig{Name: "opencode", Binary: "opencode", PaneArgs: []string{"-m", "gpt-5.4"}},
			false,
		},
		{
			"gemini excludes flag",
			ProviderConfig{Name: "gemini", Binary: "gemini"},
			false,
		},
		{
			"claude with flag already in PaneArgs",
			ProviderConfig{Name: "claude", Binary: "claude", PaneArgs: []string{"--dangerously-skip-permissions"}},
			true, // should be present but NOT duplicated
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cmd := buildInteractiveLaunchCmd(tt.provider, "")
			if tt.want {
				assert.Contains(t, cmd, "--dangerously-skip-permissions")
				// Verify no duplication
				count := strings.Count(cmd, "--dangerously-skip-permissions")
				assert.Equal(t, 1, count, "flag should appear exactly once")
			} else {
				assert.NotContains(t, cmd, "--dangerously-skip-permissions")
			}
		})
	}
}

// TestWaitForCompletion_TwoPhase_ConsecutiveMatch verifies consecutive prompt matches.
func TestWaitForCompletion_TwoPhase_ConsecutiveMatch(t *testing.T) {
	t.Parallel()
	mock := newCmuxMock()
	mock.readScreenOutput = "❯\n" // always returns prompt — two consecutive matches
	patterns := DefaultCompletionPatterns()
	pi := paneInfo{provider: ProviderConfig{Name: "claude"}, paneID: "pane-1"}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result := waitForCompletion(ctx, mock, pi, patterns, "")
	assert.True(t, result, "two consecutive prompt matches should confirm completion")
}

// TestWaitForCompletion_TwoPhase_ContextCancel verifies context cancellation returns false.
func TestWaitForCompletion_TwoPhase_ContextCancel(t *testing.T) {
	t.Parallel()
	mock := newCmuxMock()
	mock.readScreenOutput = "" // no prompt match — never completes
	patterns := DefaultCompletionPatterns()
	pi := paneInfo{provider: ProviderConfig{Name: "claude"}, paneID: "pane-1"}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	result := waitForCompletion(ctx, mock, pi, patterns, "")
	assert.False(t, result, "context cancel should return false")
}

// TestWaitForCompletion_TwoPhase_ReadScreenError verifies ReadScreen error resets candidate.
func TestWaitForCompletion_TwoPhase_ReadScreenError(t *testing.T) {
	t.Parallel()
	mock := newCmuxMock()
	mock.readScreenErr = fmt.Errorf("read error")
	patterns := DefaultCompletionPatterns()
	pi := paneInfo{provider: ProviderConfig{Name: "claude"}, paneID: "pane-1"}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result := waitForCompletion(ctx, mock, pi, patterns, "")
	assert.False(t, result, "persistent ReadScreen errors should prevent completion")
}

// TestLaunchInteractiveSessions_UsesSendLongText verifies launch uses SendLongText
// for the command and a separate SendCommand("\n") for Enter.
func TestLaunchInteractiveSessions_UsesSendLongText(t *testing.T) {
	mock := newCmuxMock()
	panes := []paneInfo{
		{provider: echoProvider("p1"), paneID: "pane-1"},
	}
	cfg := OrchestraConfig{
		Providers: []ProviderConfig{echoProvider("p1")},
		Terminal:  mock,
		Prompt:    "test",
	}
	failed := launchInteractiveSessions(context.Background(), cfg, panes)
	assert.Empty(t, failed, "launch should succeed")
	require.NotEmpty(t, mock.sendLongTextCalls, "must use SendLongText for launch cmd")
	assert.Equal(t, terminal.PaneID("pane-1"), mock.sendLongTextCalls[0].PaneID)
	foundEnter := false
	for _, c := range mock.sendCommandCalls {
		if c.Cmd == "\n" { foundEnter = true }
	}
	assert.True(t, foundEnter, "must send Enter via SendCommand after SendLongText")
}

