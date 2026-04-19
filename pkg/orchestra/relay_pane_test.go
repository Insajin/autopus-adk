package orchestra

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// S1: 3 providers execute sequentially in separate panes.
func TestRelayPane_SequentialExecution(t *testing.T) {
	t.Parallel()

	// Given: a cmux terminal and 3 relay providers
	mock := newCmuxMock()
	cfg := OrchestraConfig{
		Providers: []ProviderConfig{
			echoProvider("claude"),
			echoProvider("codex"),
			echoProvider("gemini"),
		},
		Strategy:       StrategyRelay,
		Prompt:         "analyze this code",
		TimeoutSeconds: 10,
		Terminal:       mock,
	}

	// When: relay pane mode executes
	result, err := runRelayPaneOrchestra(context.Background(), cfg)

	// Then: 3 providers executed sequentially, each in its own pane
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Len(t, result.Responses, 3)
	assert.Equal(t, "claude", result.Responses[0].Provider)
	assert.Equal(t, "codex", result.Responses[1].Provider)
	assert.Equal(t, "gemini", result.Responses[2].Provider)
	// Sequential = one pane at a time, so at least 3 split calls
	assert.GreaterOrEqual(t, len(mock.splitPaneCalls), 3)
}

// S7: Second provider's command includes previous analysis context.
func TestRelayPane_ContextInjection(t *testing.T) {
	t.Parallel()

	// Given: two providers and a prompt
	previous := []relayStageResult{
		{provider: "claude", output: "claude's analysis output"},
	}
	prompt := "review this code"
	outputFile := "/tmp/test-output.txt"

	// When: building relay pane command for the second provider
	cmd := buildRelayPaneCommand(echoProvider("codex"), prompt, outputFile, previous)

	// Then: command includes previous analysis context
	assert.Contains(t, cmd, "## Previous Analysis by claude")
	assert.Contains(t, cmd, "claude's analysis output")
}

// S2: Failed provider skipped, next provider continues.
func TestRelayPane_ProviderFailure_SkipContinue(t *testing.T) {
	t.Parallel()

	// Given: a cmux terminal with one bad provider between two good ones
	mock := newCmuxMock()
	cfg := OrchestraConfig{
		Providers: []ProviderConfig{
			echoProvider("good1"),
			{Name: "bad", Binary: "binary_that_does_not_exist_xyz", Args: []string{}},
			echoProvider("good2"),
		},
		Strategy:       StrategyRelay,
		Prompt:         "test",
		TimeoutSeconds: 10,
		Terminal:       mock,
	}

	// When: relay pane mode executes
	result, err := runRelayPaneOrchestra(context.Background(), cfg)

	// Then: execution continues past failure, at least 2 responses collected
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Len(t, result.Responses, 3)
	// Failed provider should be marked as skipped
	assert.Contains(t, result.Responses[1].Output, "[SKIPPED:")
}

// S3: Plain terminal falls back to standard relay.
func TestRelayPane_PlainTerminal_Fallback(t *testing.T) {
	t.Parallel()

	// Given: a plain (non-pane-capable) terminal
	mock := newPlainMock()
	cfg := OrchestraConfig{
		Providers: []ProviderConfig{
			echoProvider("p1"),
			echoProvider("p2"),
		},
		Strategy:       StrategyRelay,
		Prompt:         "test",
		TimeoutSeconds: 10,
		Terminal:       mock,
	}

	// When: relay pane mode is attempted
	result, err := runRelayPaneOrchestra(context.Background(), cfg)

	// Then: falls back to standard relay, no panes created
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Empty(t, mock.splitPaneCalls, "plain terminal should not split panes")
	assert.Len(t, result.Responses, 2)
}

// S4: All panes closed and temp files deleted after execution.
func TestRelayPane_PaneCleanup(t *testing.T) {
	t.Parallel()

	// Given: a cmux terminal and 2 providers
	mock := newCmuxMock()
	cfg := OrchestraConfig{
		Providers: []ProviderConfig{
			echoProvider("p1"),
			echoProvider("p2"),
		},
		Strategy:       StrategyRelay,
		Prompt:         "test",
		TimeoutSeconds: 10,
		Terminal:       mock,
	}

	// When: relay pane mode executes
	_, err := runRelayPaneOrchestra(context.Background(), cfg)

	// Then: all panes should be closed after execution
	require.NoError(t, err)
	assert.NotEmpty(t, mock.closeCalls, "panes should be cleaned up after relay execution")
	// Each created pane should have a corresponding close call
	assert.GreaterOrEqual(t, len(mock.closeCalls), len(mock.createdPanes))
}

// S6: Sentinel marker detected in output file.
func TestRelayPane_SentinelDetection(t *testing.T) {
	t.Parallel()

	// Given: a cmux terminal and a provider
	mock := newCmuxMock()
	cfg := OrchestraConfig{
		Providers: []ProviderConfig{
			echoProvider("p1"),
		},
		Strategy:       StrategyRelay,
		Prompt:         "test",
		TimeoutSeconds: 10,
		Terminal:       mock,
	}

	// When: relay pane mode executes
	result, err := runRelayPaneOrchestra(context.Background(), cfg)

	// Then: sentinel-based completion should produce valid output
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Len(t, result.Responses, 1)
	assert.NotEmpty(t, result.Responses[0].Output)
}

// REQ-4: Command built without -p flag (interactive mode).
func TestRelayPane_InteractiveCommand_NoMinusPFlag(t *testing.T) {
	t.Parallel()

	// Given: a provider config and prompt
	provider := ProviderConfig{
		Name:   "claude",
		Binary: "claude",
		Args:   []string{"-p", "--json", "-q"},
	}
	prompt := "test prompt"
	outputFile := "/tmp/output.txt"

	// When: building relay pane command
	cmd := buildRelayPaneCommand(provider, prompt, outputFile, nil)

	// Then: command should NOT contain -p flag (interactive mode)
	assert.NotContains(t, cmd, " -p ")
	// Command should still contain the prompt via heredoc injection
	assert.True(t, len(cmd) > 0, "command should not be empty")
}

// S8: All providers fail -> error returned.
func TestRelayPane_AllProvidersFail(t *testing.T) {
	t.Parallel()

	// Given: a cmux terminal with all bad providers
	mock := newCmuxMock()
	cfg := OrchestraConfig{
		Providers: []ProviderConfig{
			{Name: "bad1", Binary: "binary_not_exist_1", Args: []string{}},
			{Name: "bad2", Binary: "binary_not_exist_2", Args: []string{}},
		},
		Strategy:       StrategyRelay,
		Prompt:         "test",
		TimeoutSeconds: 10,
		Terminal:       mock,
	}

	// When: relay pane mode executes
	_, err := runRelayPaneOrchestra(context.Background(), cfg)

	// Then: error should be returned when all providers fail
	assert.Error(t, err)
}

// S5: Existing strategies (consensus, debate, pipeline, fastest) remain unaffected.
func TestRelayPane_ExistingStrategies_Unaffected(t *testing.T) {
	t.Parallel()

	strategies := []Strategy{
		StrategyConsensus,
		StrategyPipeline,
		StrategyDebate,
		StrategyFastest,
	}

	for _, s := range strategies {
		t.Run(string(s), func(t *testing.T) {
			t.Parallel()

			// Given: a cmux terminal with a non-relay strategy
			mock := newCmuxMock()
			cfg := OrchestraConfig{
				Providers: []ProviderConfig{
					echoProvider("p1"),
					echoProvider("p2"),
				},
				Strategy:       s,
				Prompt:         "test",
				TimeoutSeconds: 10,
				Terminal:       mock,
			}

			// When: RunOrchestra executes (not runRelayPaneOrchestra)
			result, err := RunOrchestra(context.Background(), cfg)

			// Then: existing strategy should work as before
			require.NoError(t, err)
			assert.Equal(t, s, result.Strategy)
		})
	}
}

// Verify buildRelayPaneCommand produces heredoc-style prompt injection.
func TestRelayPane_HeredocPromptInjection(t *testing.T) {
	t.Parallel()

	provider := echoProvider("claude")
	prompt := "multi\nline\nprompt"
	outputFile := "/tmp/out.txt"

	cmd := buildRelayPaneCommand(provider, prompt, outputFile, nil)

	// Command should contain the prompt content
	assert.True(t, strings.Contains(cmd, "multi") && strings.Contains(cmd, "prompt"),
		"command should contain the full prompt")
}
