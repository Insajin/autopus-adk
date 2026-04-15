package orchestra

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/insajin/autopus-adk/pkg/terminal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPaneRunner_CmuxDetected_SplitsPanes verifies that when a cmux terminal
// is detected, N panes are created (one per provider) via horizontal split. (R1, R2)
func TestPaneRunner_CmuxDetected_SplitsPanes(t *testing.T) {
	t.Parallel()

	// Given: a cmux terminal and 3 providers
	mock := newCmuxMock()
	cfg := OrchestraConfig{
		Providers: []ProviderConfig{
			echoProvider("p1"),
			echoProvider("p2"),
			echoProvider("p3"),
		},
		Strategy:       StrategyConsensus,
		Prompt:         "test prompt",
		TimeoutSeconds: 10,
		Terminal:       mock, // Terminal field does not exist yet — compile error expected
	}

	// When: pane runner executes
	_, err := RunPaneOrchestra(context.Background(), cfg)

	// Then: 3 panes should be split horizontally
	require.NoError(t, err)
	assert.Len(t, mock.splitPaneCalls, 3)
	for _, dir := range mock.splitPaneCalls {
		assert.Equal(t, terminal.Horizontal, dir)
	}
}

// TestPaneRunner_PlainTerminal_FallsBack verifies that a plain (non-cmux)
// terminal falls back to existing non-interactive mode. (R1, R6)
func TestPaneRunner_PlainTerminal_FallsBack(t *testing.T) {
	t.Parallel()

	// Given: a plain terminal
	mock := newPlainMock()
	cfg := OrchestraConfig{
		Providers:      []ProviderConfig{echoProvider("p1")},
		Strategy:       StrategyConsensus,
		Prompt:         "test",
		TimeoutSeconds: 10,
		Terminal:       mock,
	}

	// When: pane runner executes
	result, err := RunPaneOrchestra(context.Background(), cfg)

	// Then: should fall back to existing mode, no panes created
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Empty(t, mock.splitPaneCalls, "plain terminal should not split panes")
}

// TestPaneRunner_SendsInteractiveCommand verifies that each pane receives
// a command using PaneArgs when set, or Args as-is when PaneArgs is nil. (R3)
func TestPaneRunner_SendsInteractiveCommand(t *testing.T) {
	t.Parallel()

	// Given: a cmux terminal and a provider with PaneArgs set (no -p/-q in pane mode)
	mock := newCmuxMock()
	cfg := OrchestraConfig{
		Providers: []ProviderConfig{
			{Name: "claude", Binary: "claude", Args: []string{"-p", "--json", "-q"}, PaneArgs: []string{"--json"}},
		},
		Strategy:       StrategyConsensus,
		Prompt:         "write tests",
		TimeoutSeconds: 10,
		Terminal:       mock,
	}

	// When: pane runner executes
	_, err := RunPaneOrchestra(context.Background(), cfg)

	// Then: command sent to pane should use PaneArgs (--json only, no -p or -q)
	require.NoError(t, err)
	require.Len(t, mock.sendCommandCalls, 1)
	sentCmd := mock.sendCommandCalls[0].Cmd
	assert.NotContains(t, sentCmd, " -p ")
	assert.NotContains(t, sentCmd, " -q ")
}

// TestPaneRunner_CollectsResults verifies that after all providers complete,
// results are collected from output files and merged. (R4, R7)
func TestPaneRunner_CollectsResults(t *testing.T) {
	t.Parallel()

	// Given: a cmux terminal and 2 providers
	mock := newCmuxMock()
	cfg := OrchestraConfig{
		Providers: []ProviderConfig{
			echoProvider("p1"),
			echoProvider("p2"),
		},
		Strategy:       StrategyConsensus,
		Prompt:         "test",
		TimeoutSeconds: 10,
		Terminal:       mock,
	}

	// When: pane runner executes
	result, err := RunPaneOrchestra(context.Background(), cfg)

	// Then: result should contain responses from both providers
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Len(t, result.Responses, 2)
	assert.NotEmpty(t, result.Merged)
}

// TestPaneRunner_CleansUpPanes verifies that all panes are closed and
// temporary files are deleted after execution completes. (R5)
func TestPaneRunner_CleansUpPanes(t *testing.T) {
	t.Parallel()

	// Given: a cmux terminal and 2 providers
	mock := newCmuxMock()
	cfg := OrchestraConfig{
		Providers: []ProviderConfig{
			echoProvider("p1"),
			echoProvider("p2"),
		},
		Strategy:       StrategyConsensus,
		Prompt:         "test",
		TimeoutSeconds: 10,
		Terminal:       mock,
	}

	// When: pane runner executes
	_, err := RunPaneOrchestra(context.Background(), cfg)

	// Then: Close should have been called to clean up
	require.NoError(t, err)
	assert.NotEmpty(t, mock.closeCalls, "panes should be cleaned up after execution")
}

// TestPaneRunner_SplitPaneFailure_FallsBack verifies that when SplitPane
// returns an error, the runner falls back to non-interactive mode. (R6)
func TestPaneRunner_SplitPaneFailure_FallsBack(t *testing.T) {
	t.Parallel()

	// Given: a cmux terminal that fails to split panes
	mock := newCmuxMock()
	mock.splitPaneErr = fmt.Errorf("pane split failed: session not found")
	cfg := OrchestraConfig{
		Providers: []ProviderConfig{
			echoProvider("p1"),
			echoProvider("p2"),
		},
		Strategy:       StrategyConsensus,
		Prompt:         "test",
		TimeoutSeconds: 10,
		Terminal:       mock,
	}

	// When: pane runner executes
	result, err := RunPaneOrchestra(context.Background(), cfg)

	// Then: should succeed via fallback, not error
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.Responses, 2, "fallback should still produce results")
}

// TestPaneRunner_Timeout_ForceClosesPane verifies that when a provider
// times out, the pane is force-closed and a FailedProvider is recorded. (R8)
func TestPaneRunner_Timeout_ForceClosesPane(t *testing.T) {
	t.Parallel()

	// Given: a cmux terminal and a slow provider with short timeout
	mock := newCmuxMock()
	mock.autoComplete = false
	cfg := OrchestraConfig{
		Providers:      []ProviderConfig{sleepProvider("slow")},
		Strategy:       StrategyConsensus,
		Prompt:         "test",
		TimeoutSeconds: 1,
		Terminal:       mock,
	}

	// When: pane runner executes with tight timeout
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	result, err := RunPaneOrchestra(ctx, cfg)

	// Then: pane should be force-closed and failure recorded
	if err == nil {
		require.NotNil(t, result)
		assert.NotEmpty(t, result.FailedProviders, "timed-out provider should be recorded")
	}
	assert.NotEmpty(t, mock.closeCalls, "timed-out pane should be force-closed")
}

// TestPaneRunner_PaneArgs verifies that paneArgs returns PaneArgs when set,
// and falls back to Args when PaneArgs is nil/empty. (R3)
func TestPaneRunner_PaneArgs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		provider ProviderConfig
		expected []string
	}{
		{
			name:     "uses PaneArgs when set",
			provider: ProviderConfig{Args: []string{"-p"}, PaneArgs: []string{"--json"}},
			expected: []string{"--json"},
		},
		{
			name:     "falls back to Args when PaneArgs nil",
			provider: ProviderConfig{Args: []string{"--model", "opus"}, PaneArgs: nil},
			expected: []string{"--model", "opus"},
		},
		{
			name:     "falls back to Args when PaneArgs empty",
			provider: ProviderConfig{Args: []string{"--model", "opus"}, PaneArgs: []string{}},
			expected: []string{"--model", "opus"},
		},
		{
			name:     "both nil returns nil",
			provider: ProviderConfig{},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := paneArgs(tt.provider)
			assert.Equal(t, tt.expected, got)
		})
	}
}

// TestRunOrchestra_NilTerminal_ExistingBehavior verifies that when Terminal
// is nil, RunOrchestra behaves exactly as before (regression guard).
func TestRunOrchestra_NilTerminal_ExistingBehavior(t *testing.T) {
	t.Parallel()

	// Given: config with nil Terminal (the default before this feature)
	cfg := OrchestraConfig{
		Providers: []ProviderConfig{
			echoProvider("p1"),
			echoProvider("p2"),
		},
		Strategy:       StrategyConsensus,
		Prompt:         "hello world",
		TimeoutSeconds: 10,
		Terminal:       nil,
	}

	// When: RunOrchestra executes
	result, err := RunOrchestra(context.Background(), cfg)

	// Then: should work exactly as before
	require.NoError(t, err)
	assert.Equal(t, StrategyConsensus, result.Strategy)
	assert.Len(t, result.Responses, 2)
}
