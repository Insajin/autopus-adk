package orchestra

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/insajin/autopus-adk/pkg/terminal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// emitterSignalMock tracks SendSignal calls and provides controllable ReadScreen output.
type emitterSignalMock struct {
	mockTerminal
	sentSignals []string
	sentMu      sync.Mutex
	waitName    string
	waitCh      chan struct{} // closed when signal is received
}

func newEmitterMock() *emitterSignalMock {
	m := &emitterSignalMock{
		waitCh: make(chan struct{}),
	}
	m.name = "cmux"
	return m
}

func (m *emitterSignalMock) SurfaceHealth(_ context.Context, _ terminal.PaneID) (terminal.SurfaceStatus, error) {
	return terminal.SurfaceStatus{Valid: true}, nil
}

func (m *emitterSignalMock) WaitForSignal(ctx context.Context, name string, _ time.Duration) error {
	m.waitName = name
	select {
	case <-m.waitCh:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (m *emitterSignalMock) SendSignal(_ context.Context, name string) error {
	m.sentMu.Lock()
	m.sentSignals = append(m.sentSignals, name)
	m.sentMu.Unlock()
	// Unblock WaitForSignal.
	select {
	case <-m.waitCh:
	default:
		close(m.waitCh)
	}
	return nil
}

func (m *emitterSignalMock) getSentSignals() []string {
	m.sentMu.Lock()
	defer m.sentMu.Unlock()
	out := make([]string, len(m.sentSignals))
	copy(out, m.sentSignals)
	return out
}

// TestSignalEmitter_EmitsOnCompletion verifies that the emitter sends a signal
// when two consecutive prompt matches are detected.
func TestSignalEmitter_EmitsOnCompletion(t *testing.T) {
	t.Parallel()

	mock := newEmitterMock()
	// Simulate: first ReadScreen shows output, then two consecutive prompts.
	mock.readScreenOutput = "❯ \n"

	emitter := NewSignalEmitter(mock, mock)
	pi := paneInfo{paneID: "pane-1", provider: ProviderConfig{Name: "claude"}}
	patterns := DefaultCompletionPatterns()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	emitter.Start(ctx, pi, patterns, "", 0)

	// Wait for signal to be sent.
	select {
	case <-mock.waitCh:
		// Signal sent.
	case <-time.After(5 * time.Second):
		t.Fatal("emitter did not send signal within timeout")
	}

	signals := mock.getSentSignals()
	require.Len(t, signals, 1)
	assert.Equal(t, "done-claude", signals[0])
}

// TestSignalEmitter_RoundScopedSignalName verifies round > 1 uses round-scoped name.
func TestSignalEmitter_RoundScopedSignalName(t *testing.T) {
	t.Parallel()

	mock := newEmitterMock()
	mock.readScreenOutput = "❯ \n"

	emitter := NewSignalEmitter(mock, mock)
	pi := paneInfo{paneID: "pane-1", provider: ProviderConfig{Name: "gemini"}}
	patterns := DefaultCompletionPatterns()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	emitter.Start(ctx, pi, patterns, "", 3)

	select {
	case <-mock.waitCh:
	case <-time.After(5 * time.Second):
		t.Fatal("emitter did not send signal within timeout")
	}

	signals := mock.getSentSignals()
	require.Len(t, signals, 1)
	assert.Equal(t, "done-gemini-round3", signals[0])
}

// TestSignalEmitter_SkipsBaseline verifies that emitter ignores baseline-matching screens.
func TestSignalEmitter_SkipsBaseline(t *testing.T) {
	t.Parallel()

	baseline := "old output\n❯ \n"
	outputs := []string{baseline, baseline, "thinking...\n", "new output\n❯ \n", "new output\n❯ \n"}
	callIdx := 0
	mu := sync.Mutex{}

	mock := newEmitterMock()
	// Override ReadScreen via the sequential output pattern.
	origReadScreen := mock.readScreenOutput
	_ = origReadScreen

	// Use a custom mock that returns sequential outputs.
	seqMock := &emitterSeqMock{
		emitterSignalMock: mock,
		outputs:           outputs,
		mu:                &mu,
		idx:               &callIdx,
	}

	emitter := NewSignalEmitter(seqMock, seqMock)
	pi := paneInfo{paneID: "pane-1", provider: ProviderConfig{Name: "codex"}}
	patterns := DefaultCompletionPatterns()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	emitter.Start(ctx, pi, patterns, baseline, 0)

	select {
	case <-seqMock.waitCh:
	case <-time.After(10 * time.Second):
		t.Fatal("emitter did not send signal within timeout")
	}

	signals := seqMock.getSentSignals()
	require.Len(t, signals, 1)
	assert.Equal(t, "done-codex", signals[0])

	// Verify that the first two baseline reads were skipped (no premature signal).
	mu.Lock()
	assert.GreaterOrEqual(t, callIdx, 4, "should have read at least 4 screens (2 baseline + 1 working + 2 prompt)")
	mu.Unlock()
}

// TestSignalEmitter_StopCancels verifies that Stop cancels the emitter goroutine.
func TestSignalEmitter_StopCancels(t *testing.T) {
	t.Parallel()

	mock := newEmitterMock()
	mock.readScreenOutput = "thinking...\n" // Never shows prompt

	emitter := NewSignalEmitter(mock, mock)
	pi := paneInfo{paneID: "pane-1", provider: ProviderConfig{Name: "claude"}}
	patterns := DefaultCompletionPatterns()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	emitter.Start(ctx, pi, patterns, "", 0)
	time.Sleep(100 * time.Millisecond) // Let goroutine start

	emitter.Stop("claude", 0)
	time.Sleep(200 * time.Millisecond) // Let goroutine exit

	signals := mock.getSentSignals()
	assert.Empty(t, signals, "stopped emitter should not send signal")
}

// TestSignalEmitter_StopAll verifies StopAll cancels all emitters.
func TestSignalEmitter_StopAll(t *testing.T) {
	t.Parallel()

	mock := newEmitterMock()
	mock.readScreenOutput = "thinking...\n"

	emitter := NewSignalEmitter(mock, mock)
	patterns := DefaultCompletionPatterns()

	ctx := context.Background()
	emitter.Start(ctx, paneInfo{paneID: "p1", provider: ProviderConfig{Name: "a"}}, patterns, "", 0)
	emitter.Start(ctx, paneInfo{paneID: "p2", provider: ProviderConfig{Name: "b"}}, patterns, "", 0)

	emitter.StopAll()

	emitter.mu.Lock()
	assert.Empty(t, emitter.cancels, "StopAll should clear all cancel funcs")
	emitter.mu.Unlock()
}

// TestSignalDetector_WithEmitter_Integration verifies end-to-end: SignalDetector
// creates an emitter, emitter polls, sends signal, detector receives instantly.
func TestSignalDetector_WithEmitter_Integration(t *testing.T) {
	t.Parallel()

	mock := newEmitterMock()
	mock.readScreenOutput = "❯ \n"

	detector := &SignalDetector{term: mock, signal: mock}
	pi := paneInfo{paneID: "pane-1", provider: ProviderConfig{Name: "claude"}}
	patterns := DefaultCompletionPatterns()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	start := time.Now()
	ok, err := detector.WaitForCompletion(ctx, pi, patterns, "", 0)
	elapsed := time.Since(start)

	assert.NoError(t, err)
	assert.True(t, ok, "should detect completion via emitter signal")
	// Emitter polls at 1s interval, needs 2 matches → ~2-3s. Should be well under 15s fallback.
	assert.Less(t, elapsed, 10*time.Second, "should complete via signal, not poll fallback")
}

// TestBuildSignalName verifies signal name construction.
func TestBuildSignalName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		provider string
		round    int
		want     string
	}{
		{"claude", 0, "done-claude"},
		{"claude", 1, "done-claude"},
		{"gemini", 2, "done-gemini-round2"},
		{"codex", 5, "done-codex-round5"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, buildSignalName(tt.provider, tt.round))
	}
}

// emitterSeqMock returns sequential ReadScreen outputs.
type emitterSeqMock struct {
	*emitterSignalMock
	outputs []string
	mu      *sync.Mutex
	idx     *int
}

func (m *emitterSeqMock) ReadScreen(_ context.Context, _ terminal.PaneID, _ terminal.ReadScreenOpts) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	i := *m.idx
	*m.idx = i + 1
	if i < len(m.outputs) {
		return m.outputs[i], nil
	}
	return m.outputs[len(m.outputs)-1], nil
}
