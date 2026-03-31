package orchestra

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/insajin/autopus-adk/pkg/terminal"
)

// SignalEmitter runs background goroutines that poll provider panes for
// completion and emit cmux signals when detected. This bridges the gap
// between screen-based completion detection and the signal-based
// SignalDetector: the emitter polls, detects completion, and sends the
// signal that unblocks SignalDetector.WaitForCompletion instantly.
type SignalEmitter struct {
	term    terminal.Terminal
	signal  terminal.SignalCapable
	cancels map[string]context.CancelFunc
	mu      sync.Mutex
}

// emitterPollInterval is the screen polling interval for the emitter.
// Faster than ScreenPollDetector (2s) since the emitter's only job is
// early signal dispatch — CPU cost is negligible for 3-5 panes.
const emitterPollInterval = 1 * time.Second

// NewSignalEmitter creates a new emitter bound to the given terminal.
func NewSignalEmitter(term terminal.Terminal, signal terminal.SignalCapable) *SignalEmitter {
	return &SignalEmitter{
		term:    term,
		signal:  signal,
		cancels: make(map[string]context.CancelFunc),
	}
}

// Start launches a background goroutine that polls the pane for
// completion patterns and sends "done-{provider}" (or
// "done-{provider}-round{N}" for N > 1) when detected.
// Safe to call multiple times for different panes/rounds.
func (e *SignalEmitter) Start(ctx context.Context, pi paneInfo, patterns []CompletionPattern, baseline string, round int) {
	name := buildSignalName(pi.provider.Name, round)

	emitCtx, cancel := context.WithCancel(ctx)
	e.mu.Lock()
	// Cancel any previous emitter for the same signal (e.g., pane recreated).
	if prev, ok := e.cancels[name]; ok {
		prev()
	}
	e.cancels[name] = cancel
	e.mu.Unlock()

	go e.pollAndEmit(emitCtx, pi, patterns, baseline, name)
}

// Stop cancels the emitter goroutine for the given provider/round.
func (e *SignalEmitter) Stop(providerName string, round int) {
	name := buildSignalName(providerName, round)
	e.mu.Lock()
	defer e.mu.Unlock()
	if cancel, ok := e.cancels[name]; ok {
		cancel()
		delete(e.cancels, name)
	}
}

// StopAll cancels all running emitter goroutines.
func (e *SignalEmitter) StopAll() {
	e.mu.Lock()
	defer e.mu.Unlock()
	for name, cancel := range e.cancels {
		cancel()
		delete(e.cancels, name)
	}
}

// pollAndEmit is the background polling loop. It reads the pane screen
// at emitterPollInterval and sends the cmux signal on 2-phase match.
func (e *SignalEmitter) pollAndEmit(ctx context.Context, pi paneInfo, patterns []CompletionPattern, baseline, signalName string) {
	ticker := time.NewTicker(emitterPollInterval)
	defer ticker.Stop()

	candidateDetected := false

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			screen, err := e.term.ReadScreen(ctx, pi.paneID, terminal.ReadScreenOpts{})
			if err != nil {
				candidateDetected = false
				continue
			}
			// Skip if screen is still the pre-send baseline.
			if baseline != "" && screen == baseline {
				candidateDetected = false
				continue
			}
			if isPromptVisible(screen, patterns) {
				// Per-provider working check: defer completion if still working.
				if isProviderStillWorking(screen, pi.provider.WorkingPatterns) {
					candidateDetected = false
					continue
				}
				if candidateDetected {
					// Two consecutive matches — emit signal.
					if sendErr := e.signal.SendSignal(ctx, signalName); sendErr != nil {
						log.Printf("[SignalEmitter] send %s failed: %v", signalName, sendErr)
					}
					return
				}
				candidateDetected = true
			} else {
				candidateDetected = false
			}
		}
	}
}

// buildSignalName returns the cmux signal name for a provider/round.
func buildSignalName(providerName string, round int) string {
	safe := sanitizeProviderName(providerName)
	if round > 1 {
		return fmt.Sprintf("done-%s-round%d", safe, round)
	}
	return fmt.Sprintf("done-%s", safe)
}
