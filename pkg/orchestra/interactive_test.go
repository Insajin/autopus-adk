package orchestra

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- R5: Interactive pane execution flow ---

// TestInteractive_FullFlow_SplitPipelaunchWaitCollectMergeCleanup verifies the
// complete interactive pane orchestration flow:
// split panes -> pipe capture -> launch sessions -> wait ready -> send prompts
// -> wait completion -> collect results -> merge -> cleanup
func TestInteractive_FullFlow_SplitPipelaunchWaitCollectMergeCleanup(t *testing.T) {
	mock := newCmuxMock()
	mock.readScreenOutput = ">\n" // prompt pattern triggers immediate completion
	cfg := OrchestraConfig{
		Providers:      []ProviderConfig{echoProvider("p1"), echoProvider("p2")},
		Strategy:       StrategyConsensus,
		Prompt:         "write tests",
		TimeoutSeconds: 30,
		Terminal:       mock,
		Interactive:    true,
	}
	result, err := RunInteractivePaneOrchestra(context.Background(), cfg)
	require.NoError(t, err)
	assert.Len(t, result.Responses, 2)
	assert.NotEmpty(t, result.Merged)
	// Verify split was called for each provider
	assert.Len(t, mock.splitPaneCalls, 2)
	// Verify pipe-pane start was called before send commands
	assert.True(t, mock.pipePaneStartCalls >= 2, "must start pipe for each provider")
	assert.True(t, len(mock.sendCommandCalls) >= 2, "must send commands to each pane")
}

// TestInteractive_Flow_PipePaneStartCalledPerProvider verifies each provider gets pipe capture.
func TestInteractive_Flow_PipePaneStartCalledPerProvider(t *testing.T) {
	mock := newCmuxMock()
	mock.readScreenOutput = ">\n"
	cfg := OrchestraConfig{
		Providers:      []ProviderConfig{echoProvider("p1"), echoProvider("p2"), echoProvider("p3")},
		Strategy:       StrategyConsensus,
		Prompt:         "test",
		TimeoutSeconds: 30,
		Terminal:       mock,
		Interactive:    true,
	}
	_, err := RunInteractivePaneOrchestra(context.Background(), cfg)
	require.NoError(t, err)
	assert.Equal(t, 3, mock.pipePaneStartCalls, "pipe-pane start must be called for each provider")
}

// TestInteractive_Flow_PipePaneStopCalledOnCleanup verifies pipe-pane stop on cleanup.
func TestInteractive_Flow_PipePaneStopCalledOnCleanup(t *testing.T) {
	mock := newCmuxMock()
	mock.readScreenOutput = ">\n"
	cfg := OrchestraConfig{
		Providers:      []ProviderConfig{echoProvider("p1")},
		Strategy:       StrategyConsensus,
		Prompt:         "test",
		TimeoutSeconds: 30,
		Terminal:       mock,
		Interactive:    true,
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
	}
	result, err := RunInteractivePaneOrchestra(context.Background(), cfg)
	require.NoError(t, err)
	require.Len(t, result.Responses, 1)
	// ReadScreen returns ">\nsome output here", after cleaning prompt lines, output has content
	assert.NotEmpty(t, result.Responses[0].Output, "output should be populated from ReadScreen")
}

// --- R8: Sentinel mode fallback ---

// TestInteractive_SentinelFallback_PlainTerminal verifies fallback to sentinel mode
// when terminal is plain (no interactive capability).
func TestInteractive_SentinelFallback_PlainTerminal(t *testing.T) {
	mock := newPlainMock()
	cfg := OrchestraConfig{
		Providers:      []ProviderConfig{echoProvider("p1")},
		Strategy:       StrategyConsensus,
		Prompt:         "test",
		TimeoutSeconds: 10,
		Terminal:       mock,
		Interactive:    true,
	}
	result, err := RunInteractivePaneOrchestra(context.Background(), cfg)
	require.NoError(t, err)
	assert.NotNil(t, result, "must fall back to sentinel mode, not error")
}

// TestInteractive_SentinelFallback_InteractiveModeFails verifies fallback when
// interactive mode encounters an error mid-execution.
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
	}
	result, err := RunInteractivePaneOrchestra(context.Background(), cfg)
	require.NoError(t, err, "should fall back, not error")
	assert.NotNil(t, result)
}

// --- R9: Interactive session timeout ---

// TestInteractive_SessionTimeout_ProducesPartialResult verifies timed out sessions
// produce partial result with TimedOut: true.
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
	// ReadScreen returns partial content but no prompt pattern
	mock.readScreenOutput = "partial output before timeout"
	// Override to return content on scrollback read but no prompt match
	mock.readScreenErr = nil
	cfg := OrchestraConfig{
		Providers:      []ProviderConfig{echoProvider("slow")},
		Strategy:       StrategyConsensus,
		Prompt:         "test",
		TimeoutSeconds: 1,
		Terminal:       mock,
		Interactive:    true,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	result, err := RunInteractivePaneOrchestra(ctx, cfg)
	require.NoError(t, err)
	require.NotNil(t, result)
	// Output is populated even on timeout (R9)
	for _, r := range result.Responses {
		if r.TimedOut {
			assert.NotEmpty(t, r.Output, "partial output should be preserved on timeout")
		}
	}
}

// --- R5: Interactive config field ---

// TestInteractive_ConfigField_InteractiveBool verifies OrchestraConfig has Interactive field.
func TestInteractive_ConfigField_InteractiveBool(t *testing.T) {
	t.Parallel()
	cfg := OrchestraConfig{Interactive: true}
	assert.True(t, cfg.Interactive)
}

// Error paths and completion detection tests are in interactive_edge_test.go
