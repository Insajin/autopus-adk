package orchestra

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecuteRound_Round2_NotReadySkipsPrompt(t *testing.T) {
	mock := newCmuxMock()
	mock.readScreenOutput = "still loading"
	cfg := OrchestraConfig{
		Providers: []ProviderConfig{
			{Name: "claude", Binary: "echo"},
		},
		Strategy:       StrategyDebate,
		Prompt:         "discuss testing",
		TimeoutSeconds: 1,
		Terminal:       mock,
		Interactive:    true,
		InitialDelay:   time.Millisecond,
	}
	panes := []paneInfo{{provider: cfg.Providers[0], paneID: "pane-1"}}
	prevResponses := []ProviderResponse{
		{Provider: "gemini", Output: "gemini's take"},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	responses := executeRound(ctx, cfg, panes, nil, 2, prevResponses)
	assert.Empty(t, mock.sendLongTextCalls, "round 2 prompt must not be sent when prompt is not ready")
	assert.True(t, panes[0].skipWait, "provider should be skipped after prompt-ready timeout")
	require.Len(t, responses, 1)
	assert.True(t, responses[0].TimedOut, "skipped provider should surface as timed out")
}
