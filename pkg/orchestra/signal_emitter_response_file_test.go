package orchestra

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSignalEmitter_EmitsWhenResponseFileWritten(t *testing.T) {
	t.Parallel()

	mock := newEmitterMock()
	mock.readScreenOutput = "⣾ Generating...\nesc to cancel"
	responsePath := filepath.Join(t.TempDir(), "response.md")
	require.NoError(t, os.WriteFile(responsePath, []byte(responseBeginMarker+"\nPONG\n"+responseEndMarker+"\n"), 0o600))

	emitter := NewSignalEmitter(mock, mock)
	pi := paneInfo{
		paneID:       "pane-1",
		provider:     ProviderConfig{Name: "gemini"},
		responseFile: responsePath,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	emitter.Start(ctx, pi, DefaultCompletionPatterns(), "", 0)

	select {
	case <-mock.waitCh:
	case <-time.After(5 * time.Second):
		t.Fatal("emitter did not send signal after response file was written")
	}

	signals := mock.getSentSignals()
	require.Len(t, signals, 1)
	assert.Equal(t, "done-gemini", signals[0])
}
