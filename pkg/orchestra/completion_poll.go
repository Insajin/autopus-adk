package orchestra

import (
	"context"
	"time"

	"github.com/insajin/autopus-adk/pkg/terminal"
)

// @AX:NOTE [AUTO] magic constants — tuned for typical AI model response times; adjust with care
const (
	// idleFallbackThreshold is how long 2-phase match must fail before trying idle fallback (R7).
	idleFallbackThreshold = 60 * time.Second
	// outputIdleThreshold is how long the output file must be unchanged to trigger idle completion (R7).
	outputIdleThreshold = 30 * time.Second
)

// ScreenPollDetector uses 2-phase consecutive screen matching for completion detection.
// FALLBACK detector when signal-based detection is unavailable.
type ScreenPollDetector struct {
	term terminal.Terminal
}

// WaitForCompletion polls ReadScreen at 2s intervals using 2-phase consecutive match.
// Phase 1: First prompt pattern match detected.
// Phase 2: Second consecutive match confirms completion.
// Idle fallback: After 60s without 2-phase match, checks pipe-pane output file idle.
// The round parameter is accepted for interface conformance but unused by poll detection.
// @AX:WARN [AUTO] blocking goroutine — runs until context cancellation; ensure caller sets deadline
func (d *ScreenPollDetector) WaitForCompletion(ctx context.Context, pi paneInfo, patterns []CompletionPattern, baseline string, _ int) (bool, error) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	candidateDetected := false
	// R7: Track when idle fallback becomes eligible
	idleFallbackStart := time.Now()

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
			screen, err := d.term.ReadScreen(ctx, pi.paneID, terminal.ReadScreenOpts{})
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
			// R7: Idle fallback -- if 2-phase match hasn't succeeded within threshold,
			// check if output file is idle (no modifications for outputIdleThreshold).
			// Skip idle check if provider is actively working (thinking/generating).
			if pi.outputFile != "" && time.Since(idleFallbackStart) >= idleThresh {
				if isOutputIdle(pi.outputFile, outputIdleThreshold) && !isProviderWorking(screen) {
					return true, nil
				}
			}
		}
	}
}
