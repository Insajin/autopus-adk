package orchestra

import (
	"context"
	"log"
	"time"

	"github.com/insajin/autopus-adk/pkg/terminal"
)

// @AX:NOTE [AUTO] magic constants — tuned for typical AI model response times; adjust with care
const (
	// idleFallbackThreshold is how long 2-phase match must fail before trying idle fallback (R7).
	idleFallbackThreshold = 60 * time.Second
	// outputIdleThreshold is how long the output file must be unchanged to trigger idle completion (R7).
	outputIdleThreshold = 30 * time.Second
	// screenPollInterval keeps prompt detection responsive without waiting multi-second gaps.
	screenPollInterval = 500 * time.Millisecond
)

// defaultSafetyDeadline is the fallback deadline for WaitForCompletion when
// the caller does not set one. Package-level var so tests can override.
var defaultSafetyDeadline = 10 * time.Minute

// ScreenPollDetector uses 2-phase consecutive screen matching for completion detection.
// FALLBACK detector when signal-based detection is unavailable.
type ScreenPollDetector struct {
	term terminal.Terminal
	// safetyDeadline overrides defaultSafetyDeadline when non-zero. Used by tests.
	safetyDeadline time.Duration
}

// WaitForCompletion polls ReadScreen at short intervals using 2-phase consecutive match.
// Phase 1: First prompt pattern match detected.
// Phase 2: Second consecutive match confirms completion.
// Idle fallback: when the pipe-pane output file has been quiet for idleThresh
// (anchored to the file's own mtime, not wall-clock since the wait began) and no
// working indicator is visible, completion is assumed.
// The round parameter is accepted for interface conformance but unused by poll detection.
// @AX:NOTE [AUTO] blocking goroutine — safety deadline (10min) auto-applied when caller omits deadline (R3)
func (d *ScreenPollDetector) WaitForCompletion(ctx context.Context, pi paneInfo, patterns []CompletionPattern, baseline string, _ int) (bool, error) {
	// R3/R4: Enforce safety deadline when caller provides no deadline.
	if _, ok := ctx.Deadline(); !ok {
		deadline := defaultSafetyDeadline
		if d.safetyDeadline > 0 {
			deadline = d.safetyDeadline
		}
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, deadline)
		defer cancel()
		log.Printf("[WARN] WaitForCompletion called without deadline; using %v safety fallback", deadline)
	}

	ticker := time.NewTicker(screenPollInterval)
	defer ticker.Stop()

	candidateDetected := false

	// Use per-provider idle threshold if set; otherwise use package default.
	idleThresh := idleFallbackThreshold
	if pi.provider.IdleThreshold > 0 {
		idleThresh = pi.provider.IdleThreshold
	}

	for {
		select {
		case <-ctx.Done():
			return false, nil
		case <-ticker.C:
			if _, ok := readResponseFile(pi.responseFile); ok {
				return true, nil
			}
			screen, err := readScreenBounded(ctx, d.term, pi.paneID, terminal.ReadScreenOpts{})
			if err != nil {
				candidateDetected = false
				continue
			}
			// Auto-approve provider tool permission prompts (e.g., gemini "Action Required")
			if needsToolApproval(screen) {
				_ = d.term.SendCommand(ctx, pi.paneID, "1")
				_ = d.term.SendCommand(ctx, pi.paneID, "\n")
				candidateDetected = false
				continue
			}
			if requiresResponseFileCompletion(pi) {
				candidateDetected = false
				continue
			}
			// R2: Screen unchanged from baseline -- skip prompt matching to avoid
			// false positives from previous round's leftover prompt.
			// Still allow idle fallback to proceed (no continue).
			baselineMatch := baseline != "" && screen == baseline
			if baselineMatch {
				candidateDetected = false
			}
			if !baselineMatch && isPromptVisible(screen, patterns) {
				// Per-provider working check: if provider has working patterns
				// and any match, defer completion even though prompt is visible.
				if isProviderStillWorking(screen, pi.provider.WorkingPatterns) {
					candidateDetected = false
					continue
				}
				if candidateDetected {
					return true, nil // Two consecutive matches -- confirmed completion
				}
				candidateDetected = true // First match -- wait for confirmation
			} else if !baselineMatch {
				candidateDetected = false // Reset -- AI resumed output
			}
			// R7: Idle fallback -- when the 2-phase prompt match never succeeds,
			// declare completion once the output file itself has been quiet for
			// idleThresh and the provider shows no working indicator.
			if shouldIdleComplete(pi.outputFile, idleThresh, screen) {
				return true, nil
			}
		}
	}
}

// shouldIdleComplete reports whether the idle fallback should declare the
// provider finished: the pipe-pane output file must have been quiet for at least
// idleThresh AND the screen must show no active working indicator.
//
// The silence window is anchored to the output file's own modification time
// (via isOutputIdle), not to wall-clock time since the wait began. Anchoring to
// wall-clock let a long-running provider that briefly paused streaming be
// completed mid-response (truncating the capture). Anchoring to output mtime
// keeps that from happening while still guaranteeing the fallback fires once the
// provider truly stops writing — so a provider whose final prompt is
// unrecognized still finishes instead of hanging until the safety deadline.
func shouldIdleComplete(outputFile string, idleThresh time.Duration, screen string) bool {
	if outputFile == "" {
		return false
	}
	return isOutputIdle(outputFile, idleThresh) && !isProviderWorking(screen)
}
