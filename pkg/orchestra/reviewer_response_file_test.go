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

func TestResponseFileCompletion_CodexBrainstormRequiresFinalMarker(t *testing.T) {
	t.Parallel()

	pi := paneInfo{
		provider:     ProviderConfig{Name: "codex", Binary: "codex"},
		responseFile: filepath.Join(t.TempDir(), "codex-brainstorm.md"),
	}

	assert.True(t, requiresResponseFileCompletion(pi),
		"a visible prompt must not complete Codex before its final response marker")
}

func TestScreenPollDetector_AntigravityReviewerUsesScreenCompletion(t *testing.T) {
	t.Parallel()

	mock := &countingScreenMock{}
	mock.outputs = []string{"> Type your message\n", "> Type your message\n"}

	detector := &ScreenPollDetector{term: mock}
	pi := paneInfo{
		paneID:       "pane-1",
		provider:     ProviderConfig{Name: "gemini", Binary: "agy"},
		role:         "reviewer",
		responseFile: filepath.Join(t.TempDir(), "missing-response.md"),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1600*time.Millisecond)
	defer cancel()

	ok, err := detector.WaitForCompletion(ctx, pi, DefaultCompletionPatterns(), "", 0)
	require.NoError(t, err)
	assert.True(t, ok, "agy reviewer must not wait forever for a response file it cannot reliably write")
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

func TestCollectResponse_AntigravityReviewerUsesScreenFallbackWithoutTimeout(t *testing.T) {
	t.Parallel()

	mock := newCmuxMock()
	mock.readScreenOutput = `{"verdict":"PASS","summary":"agy screen fallback","findings":[]}`
	b := NewInteractivePaneBackend(OrchestraConfig{Terminal: mock})

	resp := b.collectResponse(context.Background(), ProviderRequest{
		Provider: "gemini",
		Role:     "reviewer",
		Config:   ProviderConfig{Name: "gemini", Binary: "agy"},
	}, paneInfo{
		paneID:       "pane-1",
		provider:     ProviderConfig{Name: "gemini", Binary: "agy"},
		role:         "reviewer",
		responseFile: filepath.Join(t.TempDir(), "missing-response.md"),
	}, false)

	require.NotNil(t, resp)
	assert.Equal(t, "gemini", resp.Provider)
	assert.False(t, resp.TimedOut)
	assert.False(t, resp.EmptyOutput)
	assert.Contains(t, resp.Output, "agy screen fallback")
	assert.Equal(t, 1, mock.readScreenCalls)
}
