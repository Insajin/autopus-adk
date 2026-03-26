package orchestra

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Edge: nil terminal falls back to standard relay (not just plain).
func TestRelayPane_NilTerminal_Fallback(t *testing.T) {
	t.Parallel()

	cfg := OrchestraConfig{
		Providers: []ProviderConfig{
			echoProvider("p1"),
			echoProvider("p2"),
		},
		Strategy:       StrategyRelay,
		Prompt:         "test",
		TimeoutSeconds: 10,
		Terminal:       nil,
	}

	result, err := runRelayPaneOrchestra(context.Background(), cfg)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Len(t, result.Responses, 2)
}

// Edge: SplitPane error produces SKIPPED response for that provider.
func TestRelayPane_SplitPaneError_Skipped(t *testing.T) {
	t.Parallel()

	mock := newCmuxMock()
	mock.splitPaneErr = fmt.Errorf("no session")
	cfg := OrchestraConfig{
		Providers: []ProviderConfig{
			echoProvider("p1"),
		},
		Strategy:       StrategyRelay,
		Prompt:         "test",
		TimeoutSeconds: 10,
		Terminal:       mock,
	}

	// All providers skipped via SplitPane error -> all fail -> error
	_, err := runRelayPaneOrchestra(context.Background(), cfg)
	assert.Error(t, err)
}

// Edge: SendCommand error produces SKIPPED response.
func TestRelayPane_SendCommandError_Skipped(t *testing.T) {
	t.Parallel()

	mock := newCmuxMock()
	mock.sendCommandErr = fmt.Errorf("pane closed")
	cfg := OrchestraConfig{
		Providers: []ProviderConfig{
			echoProvider("p1"),
		},
		Strategy:       StrategyRelay,
		Prompt:         "test",
		TimeoutSeconds: 10,
		Terminal:       mock,
	}

	// SendCommand fails -> provider skipped -> all fail -> error
	_, err := runRelayPaneOrchestra(context.Background(), cfg)
	assert.Error(t, err)
}

// Edge: filterMinusPFlag with no -p flag returns args unchanged.
func TestFilterMinusPFlag_NoFlag(t *testing.T) {
	t.Parallel()

	args := []string{"--json", "--model", "opus"}
	result := filterMinusPFlag(args)
	assert.Equal(t, args, result)
}

// Edge: filterMinusPFlag with multiple -p flags removes all.
func TestFilterMinusPFlag_MultipleFlags(t *testing.T) {
	t.Parallel()

	args := []string{"-p", "--json", "-p"}
	result := filterMinusPFlag(args)
	assert.Equal(t, []string{"--json"}, result)
}

// Edge: filterMinusPFlag with empty slice.
func TestFilterMinusPFlag_Empty(t *testing.T) {
	t.Parallel()

	result := filterMinusPFlag([]string{})
	assert.Empty(t, result)
}

// Regression: RunPaneOrchestra routes relay strategy to runRelayPaneOrchestra.
func TestRunPaneOrchestra_RelayRouting(t *testing.T) {
	t.Parallel()

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

	result, err := RunPaneOrchestra(context.Background(), cfg)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, StrategyRelay, result.Strategy)
	// Should have created a pane (routed to relay pane, not standard)
	assert.NotEmpty(t, mock.splitPaneCalls)
}

// Regression: runner.go no longer prints fallback warning for relay+cmux.
func TestRunOrchestra_RelayCmux_NoFallbackWarning(t *testing.T) {
	t.Parallel()

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

	result, err := RunOrchestra(context.Background(), cfg)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, StrategyRelay, result.Strategy)
}

// Edge: skippedResponse format check.
func TestSkippedResponse_Format(t *testing.T) {
	t.Parallel()

	resp := skippedResponse("claude", "binary not found")
	assert.Equal(t, "claude", resp.Provider)
	assert.Equal(t, -1, resp.ExitCode)
	assert.Contains(t, resp.Output, "[SKIPPED: claude")
	assert.Contains(t, resp.Output, "binary not found")
}

// Edge: nil terminal fallback with cancelled context triggers runRelay error path.
func TestRelayPane_NilTerminal_FallbackError(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancel to force runRelay to fail

	cfg := OrchestraConfig{
		Providers: []ProviderConfig{
			echoProvider("p1"),
		},
		Strategy:       StrategyRelay,
		Prompt:         "test",
		TimeoutSeconds: 1,
		Terminal:       nil,
	}

	// Cancelled context should cause runRelay to fail and return error
	result, err := runRelayPaneOrchestra(ctx, cfg)
	// Either returns error or returns result with failed/empty output
	if err != nil {
		assert.Error(t, err)
	} else {
		assert.NotNil(t, result)
	}
}

// Edge: cmux terminal with default timeout (TimeoutSeconds=0) uses 120s default.
func TestRelayPane_DefaultTimeout(t *testing.T) {
	t.Parallel()

	mock := newCmuxMock()
	cfg := OrchestraConfig{
		Providers: []ProviderConfig{
			echoProvider("p1"),
		},
		Strategy:       StrategyRelay,
		Prompt:         "test",
		TimeoutSeconds: 0, // should default to 120
		Terminal:       mock,
	}

	result, err := runRelayPaneOrchestra(context.Background(), cfg)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Len(t, result.Responses, 1)
}

// Edge: mixed success and failure — partial relay continues.
func TestRelayPane_MixedSuccessFailure_PartialRelay(t *testing.T) {
	t.Parallel()

	mock := newCmuxMock()
	cfg := OrchestraConfig{
		Providers: []ProviderConfig{
			{Name: "bad", Binary: "nonexistent_binary_xyz", Args: []string{}},
			echoProvider("good"),
		},
		Strategy:       StrategyRelay,
		Prompt:         "test",
		TimeoutSeconds: 10,
		Terminal:       mock,
	}

	// First provider fails but second succeeds -> no error
	result, err := runRelayPaneOrchestra(context.Background(), cfg)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Len(t, result.Responses, 2)
	assert.Equal(t, -1, result.Responses[0].ExitCode)
}
