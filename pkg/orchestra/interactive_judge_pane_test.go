package orchestra

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunJudgeRound_PaneCapableTerminalDoesNotExecuteSubprocess(t *testing.T) {
	provider, marker := newPaneBoundaryMarkerProvider(t, "claude", "")
	term := &paneCommitTerminal{
		screen: `{"recommendation":"keep the judge in a pane"}` + "\n❯\n",
	}
	cfg := OrchestraConfig{
		Providers:          []ProviderConfig{provider},
		JudgeProvider:      provider.Name,
		Terminal:           term,
		WorkingDir:         t.TempDir(),
		TimeoutSeconds:     3,
		InitialDelay:       time.Millisecond,
		CompletionDetector: &stubCompletionDetector{completed: true},
	}

	resp := runJudgeRound(context.Background(), cfg, nil, nil, []ProviderResponse{
		{Provider: "claude", Output: "candidate answer"},
	}, 1)

	require.NotNil(t, resp)
	assert.Equal(t, 1, term.splitCalls, "judge execution must provision a pane")
	assert.Equal(t, paneBackendName, resp.ExecutedBackend)
	assert.Contains(t, term.closed, string(committedPaneID), "judge pane must be cleaned up")
	_, statErr := os.Stat(marker)
	assert.ErrorIs(t, statErr, os.ErrNotExist, "pane judge must not execute the subprocess fixture")
}
