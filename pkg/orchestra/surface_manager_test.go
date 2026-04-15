package orchestra

import (
	"context"
	"testing"
	"time"

	"github.com/insajin/autopus-adk/pkg/terminal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// surfaceSignalMock implements both Terminal and SignalCapable for SurfaceManager testing.
type surfaceSignalMock struct {
	mockTerminal
	healthResults map[string]terminal.SurfaceStatus
	healthErr     error
	stalePanes    map[terminal.PaneID]bool
}

func (m *surfaceSignalMock) SurfaceHealth(_ context.Context, paneID terminal.PaneID) (terminal.SurfaceStatus, error) {
	if m.healthErr != nil {
		return terminal.SurfaceStatus{}, m.healthErr
	}
	if s, ok := m.healthResults[string(paneID)]; ok {
		return s, nil
	}
	return terminal.SurfaceStatus{Valid: true, SurfaceRef: string(paneID), InWindow: true}, nil
}

func (m *surfaceSignalMock) WaitForSignal(_ context.Context, _ string, _ time.Duration) error {
	return nil
}

func (m *surfaceSignalMock) SendSignal(_ context.Context, _ string) error {
	return nil
}

func (m *surfaceSignalMock) ReadScreen(ctx context.Context, paneID terminal.PaneID, opts terminal.ReadScreenOpts) (string, error) {
	if m.stalePanes != nil && m.stalePanes[paneID] {
		return "", assert.AnError
	}
	return m.mockTerminal.ReadScreen(ctx, paneID, opts)
}

// TestNewSurfaceManager_WithSignalCapable verifies signal is set for SignalCapable terminal.
func TestNewSurfaceManager_WithSignalCapable(t *testing.T) {
	t.Parallel()
	mock := &surfaceSignalMock{}
	mock.name = "cmux"
	sm := NewSurfaceManager(mock)
	assert.NotNil(t, sm)
	assert.NotNil(t, sm.signal, "signal should be set for SignalCapable terminal")
	assert.Equal(t, 5*time.Second, sm.interval)
}

// TestNewSurfaceManager_WithPlainTerminal verifies signal is nil for non-SignalCapable terminal.
func TestNewSurfaceManager_WithPlainTerminal(t *testing.T) {
	t.Parallel()
	mock := newPlainMock()
	sm := NewSurfaceManager(mock)
	assert.NotNil(t, sm)
	assert.Nil(t, sm.signal, "signal should be nil for plain terminal")
}

// TestSurfaceManager_IsHealthy_DefaultOptimistic verifies optimistic default.
func TestSurfaceManager_IsHealthy_DefaultOptimistic(t *testing.T) {
	t.Parallel()
	mock := &surfaceSignalMock{}
	mock.name = "cmux"
	sm := NewSurfaceManager(mock)
	// No health data yet -- should return true (optimistic)
	assert.True(t, sm.IsHealthy("unknown-pane"))
}

// TestSurfaceManager_IsHealthy_AfterUpdate verifies health cache reflects updates.
func TestSurfaceManager_IsHealthy_AfterUpdate(t *testing.T) {
	t.Parallel()
	mock := &surfaceSignalMock{}
	mock.name = "cmux"
	sm := NewSurfaceManager(mock)

	// Manually inject health data
	sm.mu.Lock()
	sm.health["pane-1"] = terminal.SurfaceStatus{Valid: true}
	sm.health["pane-2"] = terminal.SurfaceStatus{Valid: false}
	sm.mu.Unlock()

	assert.True(t, sm.IsHealthy("pane-1"))
	assert.False(t, sm.IsHealthy("pane-2"))
}

// TestSurfaceManager_CheckAll verifies checkAll updates health cache for active panes.
func TestSurfaceManager_CheckAll(t *testing.T) {
	t.Parallel()
	mock := &surfaceSignalMock{
		healthResults: map[string]terminal.SurfaceStatus{
			"pane-1": {Valid: true, SurfaceRef: "surface:1"},
			"pane-2": {Valid: false, SurfaceRef: "surface:2"},
		},
	}
	mock.name = "cmux"
	sm := NewSurfaceManager(mock)

	panes := []paneInfo{
		{paneID: "pane-1", provider: ProviderConfig{Name: "claude"}},
		{paneID: "pane-2", provider: ProviderConfig{Name: "gemini"}},
		{paneID: "pane-3", provider: ProviderConfig{Name: "codex"}, skipWait: true},
	}
	sm.checkAll(context.Background(), panes)

	assert.True(t, sm.IsHealthy("pane-1"))
	assert.False(t, sm.IsHealthy("pane-2"))
	// pane-3 is skipped -- should still be optimistic default
	assert.True(t, sm.IsHealthy("pane-3"))
}

// TestSurfaceManager_CheckAll_Error verifies checkAll marks pane unhealthy on error.
func TestSurfaceManager_CheckAll_Error(t *testing.T) {
	t.Parallel()
	mock := &surfaceSignalMock{}
	mock.name = "cmux"
	mock.healthErr = assert.AnError
	sm := NewSurfaceManager(mock)

	panes := []paneInfo{
		{paneID: "pane-1", provider: ProviderConfig{Name: "claude"}},
	}
	sm.checkAll(context.Background(), panes)

	assert.False(t, sm.IsHealthy("pane-1"), "should be unhealthy after health check error")
}

// TestSurfaceManager_StartStop verifies goroutine lifecycle without leaks.
func TestSurfaceManager_StartStop(t *testing.T) {
	t.Parallel()
	mock := &surfaceSignalMock{}
	mock.name = "cmux"
	sm := NewSurfaceManager(mock)

	panes := []paneInfo{
		{paneID: "pane-1", provider: ProviderConfig{Name: "claude"}},
	}
	// Override interval to be short for testing
	sm.interval = 50 * time.Millisecond

	ctx := context.Background()
	sm.Start(ctx, panes)

	// Wait a bit for at least one health check cycle
	time.Sleep(150 * time.Millisecond)

	sm.Stop()
	// After stop, the goroutine should exit. Wait a bit and verify no panic.
	time.Sleep(50 * time.Millisecond)
}

// TestSurfaceManager_Start_NoSignal verifies Start is no-op for plain terminal.
func TestSurfaceManager_Start_NoSignal(t *testing.T) {
	t.Parallel()
	mock := newPlainMock()
	sm := NewSurfaceManager(mock)

	panes := []paneInfo{
		{paneID: "pane-1", provider: ProviderConfig{Name: "claude"}},
	}
	sm.Start(context.Background(), panes)
	// No cancel function should be set
	assert.Nil(t, sm.cancel)
	sm.Stop() // Should be safe to call even without Start
}

// TestSurfaceManager_ValidateAndRecover_Healthy verifies no recovery for healthy pane.
func TestSurfaceManager_ValidateAndRecover_Healthy(t *testing.T) {
	t.Parallel()
	mock := &surfaceSignalMock{}
	mock.name = "cmux"
	mock.readScreenOutput = "some screen content" // validateSurface will succeed
	sm := NewSurfaceManager(mock)

	pi := paneInfo{paneID: "pane-1", provider: ProviderConfig{Name: "claude"}}
	cfg := OrchestraConfig{Terminal: mock}

	newPI, recovered, err := sm.ValidateAndRecover(context.Background(), cfg, pi, 1)
	require.NoError(t, err)
	assert.False(t, recovered)
	assert.Equal(t, pi.paneID, newPI.paneID)
}

// TestSurfaceManager_ValidateAndRecover_StaleReadScreen verifies recovery when
// IsHealthy returns true but ReadScreen fails (live double-check fails).
func TestSurfaceManager_ValidateAndRecover_StaleReadScreen(t *testing.T) {
	t.Parallel()
	mock := &surfaceSignalMock{}
	mock.name = "cmux"
	mock.stalePanes = map[terminal.PaneID]bool{"pane-1": true} // old pane fails -- new pane succeeds
	mock.nextPaneID = 10                                       // Ensure new pane gets a different ID
	sm := NewSurfaceManager(mock)

	pi := paneInfo{paneID: "pane-1", provider: ProviderConfig{Name: "claude", Binary: "echo"}}
	cfg := OrchestraConfig{Terminal: mock}

	// Recovery triggers because validateSurface fails (ReadScreen error).
	// recreatePane will succeed with the mock since SplitPane works.
	newPI, recovered, err := sm.ValidateAndRecover(context.Background(), cfg, pi, 1)
	require.NoError(t, err)
	assert.True(t, recovered, "recovery should occur when ReadScreen fails")
	assert.NotEqual(t, pi.paneID, newPI.paneID, "new pane should have different ID")
}

// TestSurfaceManager_ValidateAndRecover_CachedUnhealthy verifies recovery when
// cached health is false (no live double-check needed).
func TestSurfaceManager_ValidateAndRecover_CachedUnhealthy(t *testing.T) {
	t.Parallel()
	mock := &surfaceSignalMock{}
	mock.name = "cmux"
	mock.splitPaneErr = assert.AnError // Make recreation fail
	sm := NewSurfaceManager(mock)

	// Mark pane as unhealthy in cache
	sm.mu.Lock()
	sm.health["pane-1"] = terminal.SurfaceStatus{Valid: false}
	sm.mu.Unlock()

	pi := paneInfo{paneID: "pane-1", provider: ProviderConfig{Name: "claude", Binary: "echo"}}
	cfg := OrchestraConfig{Terminal: mock}

	_, _, err := sm.ValidateAndRecover(context.Background(), cfg, pi, 1)
	assert.Error(t, err, "should return error when recreation fails")
}
