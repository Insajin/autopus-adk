package orchestra

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/insajin/autopus-adk/pkg/terminal"
)

// SurfaceManager monitors surface health in the background and provides
// proactive stale detection. Replaces the reactive validateSurface() approach.
// @AX:ANCHOR [AUTO] coordinator — owns background goroutine, health cache, and pane recovery; used by runPaneDebate and executeRound
type SurfaceManager struct {
	term     terminal.Terminal
	signal   terminal.SignalCapable // nil if terminal doesn't support signals
	interval time.Duration          // health check interval (default 5s)

	mu     sync.RWMutex
	health map[string]terminal.SurfaceStatus // paneID -> last known health
	cancel context.CancelFunc
}

// NewSurfaceManager creates a SurfaceManager. If the terminal supports
// SignalCapable, uses surface-health checks; otherwise falls back to no-op.
func NewSurfaceManager(term terminal.Terminal) *SurfaceManager {
	sm := &SurfaceManager{
		term:     term,
		interval: 5 * time.Second,
		health:   make(map[string]terminal.SurfaceStatus),
	}
	if sc, ok := term.(terminal.SignalCapable); ok {
		sm.signal = sc
	}
	return sm
}

// Start begins background health monitoring for the given panes.
// Call Stop() when the debate ends to clean up the goroutine.
func (sm *SurfaceManager) Start(ctx context.Context, panes []paneInfo) {
	if sm.signal == nil {
		return // No signal support -- skip monitoring
	}
	monCtx, cancel := context.WithCancel(ctx)
	sm.cancel = cancel

	go sm.monitorLoop(monCtx, panes)
}

// Stop stops the background monitoring goroutine.
func (sm *SurfaceManager) Stop() {
	if sm.cancel != nil {
		sm.cancel()
	}
}

// @AX:TODO @AX:CYCLE:1 [AUTO] P1 warm pool (R9) — pre-create spare panes for instant recovery instead of on-demand recreatePane
// IsHealthy returns true if the pane's last known health status is valid.
// Returns true by default (optimistic) if no health data is available yet.
func (sm *SurfaceManager) IsHealthy(paneID terminal.PaneID) bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	status, exists := sm.health[string(paneID)]
	if !exists {
		return true // Optimistic: no data yet means assume healthy
	}
	return status.Valid
}

// monitorLoop polls surface health at the configured interval.
func (sm *SurfaceManager) monitorLoop(ctx context.Context, panes []paneInfo) {
	ticker := time.NewTicker(sm.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			sm.checkAll(ctx, panes)
		}
	}
}

// checkAll checks health of all panes and updates the cache.
func (sm *SurfaceManager) checkAll(ctx context.Context, panes []paneInfo) {
	for _, pi := range panes {
		if pi.skipWait {
			continue
		}
		status, err := sm.signal.SurfaceHealth(ctx, pi.paneID)
		if err != nil {
			// On error, mark as unhealthy
			status = terminal.SurfaceStatus{Valid: false}
		}
		sm.mu.Lock()
		sm.health[string(pi.paneID)] = status
		sm.mu.Unlock()
	}
}

// ValidateAndRecover checks surface health and recreates stale panes.
// Returns updated paneInfo and whether recovery occurred.
// This replaces the inline surface validation logic in executeRound().
func (sm *SurfaceManager) ValidateAndRecover(ctx context.Context, cfg OrchestraConfig, pi paneInfo, round int) (paneInfo, bool, error) {
	healthy := sm.IsHealthy(pi.paneID)
	if healthy {
		// Double-check with live ReadScreen if signal says healthy
		if !validateSurface(ctx, cfg.Terminal, pi.paneID) {
			healthy = false
		}
	}
	if healthy {
		return pi, false, nil // No recovery needed
	}

	// Surface is stale -- recreate
	log.Printf("[SurfaceManager] %s surface stale, recreating", pi.provider.Name)
	newPI, err := recreatePane(ctx, cfg, pi, round)
	if err != nil {
		return pi, false, err
	}

	// Update health cache with new pane
	sm.mu.Lock()
	delete(sm.health, string(pi.paneID))
	sm.health[string(newPI.paneID)] = terminal.SurfaceStatus{Valid: true}
	sm.mu.Unlock()

	return newPI, true, nil
}

// captureBaselines reads the current screen content for all active panes.
// Used to establish baseline for false-positive completion detection prevention (R7).
func captureBaselines(ctx context.Context, term terminal.Terminal, panes []paneInfo) map[string]string {
	baselines := make(map[string]string)
	for _, pi := range panes {
		if pi.skipWait {
			continue
		}
		screen, _ := term.ReadScreen(ctx, pi.paneID, terminal.ReadScreenOpts{})
		baselines[pi.provider.Name] = screen
	}
	return baselines
}

// sendPromptWithRetry sends a prompt to a pane, retrying on the same pane first
// before falling back to pane recreation as a last resort.
// Returns updated paneInfo and whether recreation occurred.
func sendPromptWithRetry(ctx context.Context, cfg OrchestraConfig, pi paneInfo, prompt string, round int, baselines map[string]string) (paneInfo, bool, error) {
	// Initial attempt on existing pane
	if err := cfg.Terminal.SendLongText(ctx, pi.paneID, prompt); err == nil {
		return pi, false, nil
	}

	// Same-pane retries with exponential backoff (2s, 4s) before recreation
	samePaneBackoffs := []time.Duration{2 * time.Second, 4 * time.Second}
	for i, wait := range samePaneBackoffs {
		log.Printf("[Round %d] %s same-pane retry %d/%d, waiting %v...",
			round, pi.provider.Name, i+1, len(samePaneBackoffs), wait)
		time.Sleep(wait)
		if err := cfg.Terminal.SendLongText(ctx, pi.paneID, prompt); err == nil {
			return pi, false, nil
		}
	}

	// All same-pane retries exhausted — recreate pane as last resort
	log.Printf("[Round %d] %s all same-pane retries exhausted, recreating pane", round, pi.provider.Name)
	newPI, err := recreatePane(ctx, cfg, pi, round)
	if err != nil {
		return pi, false, fmt.Errorf("recreatePane failed: %w", err)
	}

	// Refresh baseline for the new pane
	if screen, rerr := cfg.Terminal.ReadScreen(ctx, newPI.paneID, terminal.ReadScreenOpts{}); rerr == nil {
		baselines[pi.provider.Name] = screen
	}

	// Final attempt on the newly created pane
	if retryErr := cfg.Terminal.SendLongText(ctx, newPI.paneID, prompt); retryErr != nil {
		return newPI, true, fmt.Errorf("SendLongText failed after recreation: %w", retryErr)
	}
	return newPI, true, nil
}
