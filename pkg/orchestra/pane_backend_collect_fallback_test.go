package orchestra

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCollectResponse_ReviewerTimedOutHarvestsTerminalFallback reproduces the
// answer-loss half of issue #59: codex/gemini run to the watchdog boundary
// without writing the response file but DO print their final answer to the
// terminal (the fallback promptFileInstruction authorizes). Before the fix the
// timed-out reviewer collection discarded that answer and reported empty output;
// now the terminal answer is harvested while the response stays TimedOut.
func TestCollectResponse_ReviewerTimedOutHarvestsTerminalFallback(t *testing.T) {
	t.Parallel()

	mock := newCmuxMock()
	mock.readScreenOutput = `{"verdict":"PASS","summary":"terminal fallback","findings":[]}`
	b := NewInteractivePaneBackend(OrchestraConfig{Terminal: mock})

	resp := b.collectResponse(context.Background(), ProviderRequest{
		Provider: "codex",
		Role:     "reviewer",
	}, paneInfo{
		paneID:       "pane-1",
		role:         "reviewer",
		responseFile: filepath.Join(t.TempDir(), "missing-response.md"),
	}, true)

	require.NotNil(t, resp)
	assert.Equal(t, "codex", resp.Provider)
	assert.True(t, resp.TimedOut, "the per-provider budget was exceeded, so the outcome stays timed out")
	assert.False(t, resp.EmptyOutput, "the terminal-fallback answer must be preserved")
	assert.Contains(t, resp.Output, "terminal fallback")
	assert.Equal(t, 1, mock.readScreenCalls, "the deadline harvest must read the screen exactly once")
}

// TestCollectResponse_ReviewerTimedOutEmptyScreenKeepsDiagnostic verifies that a
// timed-out reviewer with neither a response file nor any terminal output still
// surfaces the missing-response-file diagnostic so the failure stays attributable.
func TestCollectResponse_ReviewerTimedOutEmptyScreenKeepsDiagnostic(t *testing.T) {
	t.Parallel()

	mock := newCmuxMock()
	mock.readScreenOutput = ""
	b := NewInteractivePaneBackend(OrchestraConfig{Terminal: mock})

	resp := b.collectResponse(context.Background(), ProviderRequest{
		Provider: "gemini",
		Role:     "reviewer",
	}, paneInfo{
		paneID:       "pane-1",
		role:         "reviewer",
		responseFile: filepath.Join(t.TempDir(), "missing-response.md"),
	}, true)

	require.NotNil(t, resp)
	assert.True(t, resp.TimedOut)
	assert.True(t, resp.EmptyOutput)
	assert.Contains(t, resp.Error, "response file")
}
