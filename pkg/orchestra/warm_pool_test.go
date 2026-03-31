package orchestra

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWarmPool_Init_CreatesSpares verifies Init creates the target number of spare panes.
func TestWarmPool_Init_CreatesSpares(t *testing.T) {
	t.Parallel()
	mock := newCmuxMock()
	wp := NewWarmPool(mock, 2)

	wp.Init(context.Background())

	assert.Equal(t, 2, wp.Size(), "pool should have 2 spare panes after init")
	assert.Equal(t, 2, len(mock.createdPanes), "should have created 2 panes via SplitPane")
}

// TestWarmPool_Init_HandlesError verifies Init continues when pane creation fails.
func TestWarmPool_Init_HandlesError(t *testing.T) {
	t.Parallel()
	mock := newCmuxMock()
	mock.splitPaneErr = assert.AnError
	wp := NewWarmPool(mock, 2)

	wp.Init(context.Background())

	assert.Equal(t, 0, wp.Size(), "pool should be empty when SplitPane fails")
}

// TestWarmPool_Acquire_ReturnsPaneWhenAvailable verifies Acquire pops a pane.
func TestWarmPool_Acquire_ReturnsPaneWhenAvailable(t *testing.T) {
	t.Parallel()
	mock := newCmuxMock()
	wp := NewWarmPool(mock, 1)
	wp.Init(context.Background())
	require.Equal(t, 1, wp.Size())

	w := wp.Acquire()
	require.NotNil(t, w, "should return a warm pane")
	assert.NotEmpty(t, w.paneID)
	assert.Equal(t, 0, wp.Size(), "pool should be empty after acquire")
}

// TestWarmPool_Acquire_ReturnsNilWhenEmpty verifies Acquire returns nil on empty pool.
func TestWarmPool_Acquire_ReturnsNilWhenEmpty(t *testing.T) {
	t.Parallel()
	mock := newCmuxMock()
	wp := NewWarmPool(mock, 1)
	// No Init — pool is empty

	w := wp.Acquire()
	assert.Nil(t, w, "should return nil when pool is empty")
}

// TestWarmPool_Replenish_RefillsPool verifies Replenish adds a pane after acquire.
func TestWarmPool_Replenish_RefillsPool(t *testing.T) {
	t.Parallel()
	mock := newCmuxMock()
	wp := NewWarmPool(mock, 1)
	wp.Init(context.Background())

	// Drain the pool
	w := wp.Acquire()
	require.NotNil(t, w)
	assert.Equal(t, 0, wp.Size())

	// Replenish
	wp.Replenish(context.Background())
	assert.Equal(t, 1, wp.Size(), "pool should have 1 pane after replenish")
}

// TestWarmPool_Replenish_SkipsWhenFull verifies Replenish is no-op when pool is full.
func TestWarmPool_Replenish_SkipsWhenFull(t *testing.T) {
	t.Parallel()
	mock := newCmuxMock()
	wp := NewWarmPool(mock, 1)
	wp.Init(context.Background())

	initialPanes := len(mock.createdPanes)
	wp.Replenish(context.Background())
	assert.Equal(t, initialPanes, len(mock.createdPanes), "should not create pane when pool is full")
}

// TestWarmPool_Close_CleansUpAll verifies Close removes all spare panes.
func TestWarmPool_Close_CleansUpAll(t *testing.T) {
	t.Parallel()
	mock := newCmuxMock()
	wp := NewWarmPool(mock, 2)
	wp.Init(context.Background())
	require.Equal(t, 2, wp.Size())

	wp.Close(context.Background())

	assert.Equal(t, 0, wp.Size(), "pool should be empty after close")
	assert.Equal(t, 2, len(mock.closeCalls), "should have closed 2 panes")
}

// TestWarmPool_DefaultSize verifies pool size defaults to 1 when given 0 or negative.
func TestWarmPool_DefaultSize(t *testing.T) {
	t.Parallel()
	mock := newCmuxMock()
	wp := NewWarmPool(mock, 0)
	assert.Equal(t, 1, wp.poolSize)

	wp2 := NewWarmPool(mock, -5)
	assert.Equal(t, 1, wp2.poolSize)
}

// TestSurfaceManager_ValidateAndRecover_UsesWarmPool verifies warm pool is used
// before falling back to recreatePane.
func TestSurfaceManager_ValidateAndRecover_UsesWarmPool(t *testing.T) {
	t.Parallel()
	mock := &surfaceSignalMock{}
	mock.name = "cmux"
	mock.readScreenErr = assert.AnError // Force stale detection
	sm := NewSurfaceManager(mock)

	// Manually set up warm pool with a pre-created pane
	sm.warmPool = NewWarmPool(mock, 1)
	sm.warmPool.Init(context.Background())
	require.Equal(t, 1, sm.warmPool.Size())

	pi := paneInfo{paneID: "stale-pane", provider: ProviderConfig{Name: "claude", Binary: "echo"}}
	cfg := OrchestraConfig{Terminal: mock}

	newPI, recovered, err := sm.ValidateAndRecover(context.Background(), cfg, pi, 1)
	require.NoError(t, err)
	assert.True(t, recovered, "should have recovered")
	assert.NotEqual(t, "stale-pane", newPI.paneID, "should use warm pane, not old one")

	// Warm pool should be drained (replenish runs in background)
	// Give a moment for async replenish goroutine
	time.Sleep(50 * time.Millisecond)
}

// TestSurfaceManager_ValidateAndRecover_FallsBackToRecreate verifies recreatePane
// is used when warm pool is empty.
func TestSurfaceManager_ValidateAndRecover_FallsBackToRecreate(t *testing.T) {
	t.Parallel()
	mock := &surfaceSignalMock{}
	mock.name = "cmux"
	mock.readScreenErr = assert.AnError // Force stale detection
	mock.nextPaneID = 50
	sm := NewSurfaceManager(mock)

	// Empty warm pool
	sm.warmPool = NewWarmPool(mock, 1)
	// Don't init — pool stays empty

	pi := paneInfo{paneID: "stale-pane", provider: ProviderConfig{Name: "claude", Binary: "echo"}}
	cfg := OrchestraConfig{Terminal: mock}

	newPI, recovered, err := sm.ValidateAndRecover(context.Background(), cfg, pi, 1)
	require.NoError(t, err)
	assert.True(t, recovered, "should have recovered via recreatePane fallback")
	assert.NotEqual(t, "stale-pane", newPI.paneID)
}

// TestSurfaceManager_Stop_CleansUpWarmPool verifies Stop cleans up the warm pool.
func TestSurfaceManager_Stop_CleansUpWarmPool(t *testing.T) {
	t.Parallel()
	mock := &surfaceSignalMock{}
	mock.name = "cmux"
	sm := NewSurfaceManager(mock)

	panes := []paneInfo{
		{paneID: "pane-1", provider: ProviderConfig{Name: "claude"}},
	}
	sm.interval = 50 * time.Millisecond
	sm.Start(context.Background(), panes)

	// Wait for warm pool init
	time.Sleep(200 * time.Millisecond)

	sm.Stop()
	// After stop, warm pool should be cleaned up
	assert.Equal(t, 0, sm.warmPool.Size(), "warm pool should be empty after stop")
}
