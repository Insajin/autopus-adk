package orchestra

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/insajin/autopus-adk/pkg/terminal"
)

// warmPane represents a pre-created spare pane ready for instant swap-in.
// @AX:ANCHOR [AUTO] warm pool entry — holds pane ID, output file, and creation time
type warmPane struct {
	paneID     terminal.PaneID
	outputFile string // temp file for idle fallback
	createdAt  time.Time
}

// WarmPool manages pre-created spare panes for instant surface recovery.
// Instead of calling recreatePane() on-demand (which is slow), the pool
// keeps spare panes ready for immediate swap-in.
// @AX:ANCHOR [AUTO] warm pool — pre-creates panes for O(1) surface recovery
type WarmPool struct {
	term     terminal.Terminal
	pool     []warmPane
	poolSize int // target pool size
	mu       sync.Mutex
}

// NewWarmPool creates a WarmPool targeting the given number of spare panes.
func NewWarmPool(term terminal.Terminal, size int) *WarmPool {
	if size <= 0 {
		size = 1
	}
	return &WarmPool{
		term:     term,
		pool:     make([]warmPane, 0, size),
		poolSize: size,
	}
}

// Init pre-creates spare panes up to the target pool size.
// Errors during pane creation are logged but not fatal — the pool
// starts partially filled and replenishes in the background.
func (wp *WarmPool) Init(ctx context.Context) {
	for i := range wp.poolSize {
		wp.mu.Lock()
		currentLen := len(wp.pool)
		wp.mu.Unlock()
		if currentLen >= wp.poolSize {
			break
		}
		w, err := wp.createWarmPane(ctx)
		if err != nil {
			log.Printf("[WarmPool] init: failed to create spare pane %d/%d: %v", i+1, wp.poolSize, err)
			continue
		}
		wp.mu.Lock()
		wp.pool = append(wp.pool, w)
		wp.mu.Unlock()
		log.Printf("[WarmPool] init: spare pane %d/%d created (%s)", i+1, wp.poolSize, w.paneID)
	}
}

// Acquire takes a spare pane from the pool. Returns nil if the pool is empty.
func (wp *WarmPool) Acquire() *warmPane {
	wp.mu.Lock()
	defer wp.mu.Unlock()
	if len(wp.pool) == 0 {
		return nil
	}
	// Pop from the end (LIFO — most recently created pane is freshest)
	w := wp.pool[len(wp.pool)-1]
	wp.pool = wp.pool[:len(wp.pool)-1]
	return &w
}

// Replenish creates a new spare pane to refill the pool in the background.
// Safe to call from a goroutine.
func (wp *WarmPool) Replenish(ctx context.Context) {
	wp.mu.Lock()
	if len(wp.pool) >= wp.poolSize {
		wp.mu.Unlock()
		return
	}
	wp.mu.Unlock()

	log.Printf("[SurfaceManager] replenishing warm pool")
	w, err := wp.createWarmPane(ctx)
	if err != nil {
		log.Printf("[WarmPool] replenish failed: %v", err)
		return
	}
	wp.mu.Lock()
	// Re-check after creation to avoid exceeding target
	if len(wp.pool) < wp.poolSize {
		wp.pool = append(wp.pool, w)
	} else {
		// Pool was filled by another goroutine; clean up
		wp.mu.Unlock()
		wp.cleanupPane(ctx, w)
		return
	}
	wp.mu.Unlock()
}

// Close cleans up all spare panes in the pool.
func (wp *WarmPool) Close(ctx context.Context) {
	wp.mu.Lock()
	panes := make([]warmPane, len(wp.pool))
	copy(panes, wp.pool)
	wp.pool = wp.pool[:0]
	wp.mu.Unlock()

	for _, w := range panes {
		wp.cleanupPane(ctx, w)
	}
	log.Printf("[WarmPool] closed, cleaned up %d spare pane(s)", len(panes))
}

// Size returns the current number of spare panes available.
func (wp *WarmPool) Size() int {
	wp.mu.Lock()
	defer wp.mu.Unlock()
	return len(wp.pool)
}

// createWarmPane creates a single spare pane with its output file.
func (wp *WarmPool) createWarmPane(ctx context.Context) (warmPane, error) {
	paneID, err := wp.term.SplitPane(ctx, terminal.Horizontal)
	if err != nil {
		return warmPane{}, fmt.Errorf("SplitPane: %w", err)
	}

	tmpFile, err := os.CreateTemp("", "autopus-warm-spare-")
	if err != nil {
		_ = wp.term.Close(ctx, string(paneID))
		return warmPane{}, fmt.Errorf("CreateTemp: %w", err)
	}
	tmpFile.Close()

	// Start pipe capture so the pane is ready for idle fallback detection.
	if pipeErr := wp.term.PipePaneStart(ctx, paneID, tmpFile.Name()); pipeErr != nil {
		log.Printf("[WarmPool] PipePaneStart failed for %s (non-fatal): %v", paneID, pipeErr)
		_ = os.Remove(tmpFile.Name())
		// Pane is still usable without pipe capture
		return warmPane{
			paneID:    paneID,
			createdAt: time.Now(),
		}, nil
	}

	return warmPane{
		paneID:     paneID,
		outputFile: tmpFile.Name(),
		createdAt:  time.Now(),
	}, nil
}

// cleanupPane closes a single warm pane and removes its output file.
func (wp *WarmPool) cleanupPane(ctx context.Context, w warmPane) {
	_ = wp.term.PipePaneStop(ctx, w.paneID)
	_ = wp.term.Close(ctx, string(w.paneID))
	if w.outputFile != "" {
		_ = os.Remove(w.outputFile)
	}
}
