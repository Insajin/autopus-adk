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

func TestRecreatePane_NotReadyPreservesOldPane(t *testing.T) {
	term := &mockTerminal{name: "cmux", readScreenOutput: "$ "}
	old := paneInfo{
		paneID:   "old-pane",
		provider: ProviderConfig{Name: "claude", Binary: "echo", StartupTimeout: 20 * time.Millisecond},
	}

	got, err := recreatePane(context.Background(), OrchestraConfig{Terminal: term}, old, 2)

	require.Error(t, err)
	assert.Equal(t, old.paneID, got.paneID)
	assert.NotContains(t, term.closeCalls, "old-pane",
		"the old pane must remain until the replacement session is ready")
	assert.Contains(t, term.closeCalls, "pane-1",
		"an unready replacement must be cleaned up")
}

func TestSurfaceManager_WarmReplacementNotReadyPreservesOldPane(t *testing.T) {
	term := &surfaceSignalMock{
		mockTerminal: mockTerminal{name: "cmux", readScreenOutput: "$ "},
		stalePanes:   map[terminal.PaneID]bool{"old-pane": true},
	}
	sm := NewSurfaceManager(term)
	sm.warmPool = &WarmPool{
		term:     term,
		poolSize: 1,
		pool:     []warmPane{{paneID: "warm-pane"}},
	}
	old := paneInfo{
		paneID:   "old-pane",
		provider: ProviderConfig{Name: "claude", Binary: "echo", StartupTimeout: 20 * time.Millisecond},
	}

	got, recovered, err := sm.ValidateAndRecover(
		context.Background(), OrchestraConfig{Terminal: term}, old, 2,
	)

	require.Error(t, err)
	assert.False(t, recovered)
	assert.Equal(t, old.paneID, got.paneID)
	assert.NotContains(t, term.closeCalls, "old-pane",
		"an unready warm replacement must not retire the old pane")
	assert.Contains(t, term.closeCalls, "warm-pane",
		"an unready warm replacement must be cleaned up")
}

type gatedSplitTerminal struct {
	mockTerminal
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

func newGatedSplitTerminal() *gatedSplitTerminal {
	return &gatedSplitTerminal{
		mockTerminal: mockTerminal{name: "cmux"},
		started:      make(chan struct{}),
		release:      make(chan struct{}),
	}
}

func (m *gatedSplitTerminal) SplitPane(ctx context.Context, _ terminal.Direction) (terminal.PaneID, error) {
	m.once.Do(func() { close(m.started) })
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case <-m.release:
		return "late-pane", nil
	}
}

func TestWarmPool_CloseRejectsLateInitPane(t *testing.T) {
	term := newGatedSplitTerminal()
	wp := NewWarmPool(term, 1)
	initDone := make(chan struct{})
	go func() {
		defer close(initDone)
		wp.Init(context.Background())
	}()
	<-term.started

	closeDone := make(chan struct{})
	go func() {
		defer close(closeDone)
		wp.Close(context.Background())
	}()
	close(term.release)

	select {
	case <-closeDone:
	case <-time.After(time.Second):
		t.Fatal("WarmPool.Close did not finish")
	}
	select {
	case <-initDone:
	case <-time.After(time.Second):
		t.Fatal("WarmPool.Init did not finish")
	}
	assert.Zero(t, wp.Size(), "no pane may be appended after Close")
	assert.Contains(t, term.closeCalls, "late-pane", "a pane created during Close must be cleaned up")
}

func TestSplitPaneSerialized_ContextCancellationWhileQueued(t *testing.T) {
	term := newGatedSplitTerminal()
	firstDone := make(chan error, 1)
	go func() {
		_, err := splitPaneSerialized(context.Background(), term, terminal.Horizontal)
		firstDone <- err
	}()
	<-term.started

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	secondDone := make(chan error, 1)
	go func() {
		_, err := splitPaneSerialized(ctx, term, terminal.Horizontal)
		secondDone <- err
	}()

	returnedWhileQueued := false
	select {
	case err := <-secondDone:
		returnedWhileQueued = true
		require.ErrorIs(t, err, context.DeadlineExceeded)
	case <-time.After(100 * time.Millisecond):
	}
	close(term.release)
	require.NoError(t, <-firstDone)
	if !returnedWhileQueued {
		err := <-secondDone
		require.ErrorIs(t, err, context.DeadlineExceeded)
		t.Fatal("queued split ignored context cancellation until the active split completed")
	}
}
