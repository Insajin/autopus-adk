package orchestra

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScreenPollDetector_ReviewerRequiresResponseFileDespitePrompt(t *testing.T) {
	t.Parallel()

	mock := &countingScreenMock{}
	mock.outputs = []string{"codex>\n", "codex>\n", "codex>\n"}

	detector := &ScreenPollDetector{term: mock}
	pi := paneInfo{
		paneID:       "pane-1",
		provider:     ProviderConfig{Name: "codex"},
		role:         "reviewer",
		responseFile: filepath.Join(t.TempDir(), "missing-response.md"),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1600*time.Millisecond)
	defer cancel()

	ok, err := detector.WaitForCompletion(ctx, pi, DefaultCompletionPatterns(), "", 0)
	require.NoError(t, err)
	assert.False(t, ok, "reviewer completion must wait for a written response file")
}

func TestSignalEmitter_ReviewerRequiresResponseFileBeforeSignal(t *testing.T) {
	mock := newEmitterMock()
	mock.readScreenOutput = "codex>\n"

	emitter := NewSignalEmitter(mock, mock)
	pi := paneInfo{
		paneID:       "pane-1",
		provider:     ProviderConfig{Name: "codex"},
		role:         "reviewer",
		responseFile: filepath.Join(t.TempDir(), "missing-response.md"),
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	emitter.Start(ctx, pi, DefaultCompletionPatterns(), "", 0)

	select {
	case <-mock.waitCh:
		t.Fatal("reviewer emitter signaled before the response file was written")
	case <-time.After(3200 * time.Millisecond):
	}
	assert.Empty(t, mock.getSentSignals())
}

func TestCollectResponse_ReviewerMissingResponseFileDoesNotUseScreenFallback(t *testing.T) {
	t.Parallel()

	mock := newCmuxMock()
	mock.readScreenOutput = `{"verdict":"PASS","summary":"screen fallback","findings":[]}`
	b := NewInteractivePaneBackend(OrchestraConfig{Terminal: mock})

	resp := b.collectResponse(context.Background(), ProviderRequest{
		Provider: "codex",
		Role:     "reviewer",
	}, paneInfo{
		paneID:       "pane-1",
		role:         "reviewer",
		responseFile: filepath.Join(t.TempDir(), "missing-response.md"),
	}, false)

	require.NotNil(t, resp)
	assert.Equal(t, "codex", resp.Provider)
	assert.Empty(t, resp.Output)
	assert.True(t, resp.EmptyOutput)
	assert.Contains(t, resp.Error, "response file")
	assert.Equal(t, "pane", resp.ExecutedBackend)
	assert.Zero(t, mock.readScreenCalls, "reviewer collection must not fall back to screen output")
}
