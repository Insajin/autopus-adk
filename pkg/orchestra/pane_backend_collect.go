package orchestra

import (
	"context"
	"time"

	"github.com/insajin/autopus-adk/pkg/terminal"
)

// finalReadTimeout bounds the scrollback read after completion/timeout so the
// backend never blocks indefinitely while harvesting the screen (REQ-011).
const finalReadTimeout = 5 * time.Second

// collectResponse prefers the file-backed response contract, then falls back to
// pane scrollback. timedOut marks deterministic timeout results when fallback
// screen collection is used.
func (b *InteractivePaneBackend) collectResponse(ctx context.Context, req ProviderRequest, pi paneInfo, timedOut bool) *ProviderResponse {
	if output, ok := readResponseFile(pi.responseFile); ok {
		return &ProviderResponse{
			Provider:        req.Provider,
			Output:          output,
			TimedOut:        false,
			EmptyOutput:     false,
			ExecutedBackend: paneBackendName,
		}
	}
	if requiresReviewerResponseFile(req, pi) && !timedOut {
		// While the reviewer pane is still running, a mid-render screen could be
		// mis-parsed as the final verdict, so before the deadline only the written
		// response file is trusted (anti-truncation; issue #59 — claude writes the
		// response file and completes early while codex/gemini do not, so they run
		// to the watchdog boundary).
		return &ProviderResponse{
			Provider:        req.Provider,
			TimedOut:        timedOut,
			EmptyOutput:     true,
			Error:           reviewerResponseFileMissingError(timedOut),
			ExecutedBackend: paneBackendName,
		}
	}

	// At the deadline a reviewer that printed its answer to the terminal — the
	// fallback that promptFileInstruction explicitly authorizes ("If you cannot
	// write the response file, print the final answer in the terminal as
	// fallback") — would otherwise be discarded. Harvest the screen so the
	// terminal-fallback answer is preserved instead of lost (issue #59). The
	// response stays TimedOut because the per-provider budget was still exceeded.
	//
	// Use a fresh, bounded context for the final read: the original ctx may be
	// cancelled after a completion timeout (mirrors interactive_collect.go).
	readCtx, cancel := context.WithTimeout(context.Background(), finalReadTimeout)
	defer cancel()
	screen, _ := b.cfg.Terminal.ReadScreen(readCtx, pi.paneID, terminal.ReadScreenOpts{
		Scrollback:      true,
		ScrollbackLines: scrollbackDepth(b.cfg.ScrollbackLines),
	})
	resp := b.buildResponseFromScreen(req.Provider, screen, timedOut)
	// When a timed-out reviewer pane also left an empty screen, restore the
	// missing-response-file diagnostic so the failure stays attributable.
	if resp.EmptyOutput && requiresReviewerResponseFile(req, pi) {
		resp.Error = reviewerResponseFileMissingError(timedOut)
	}
	return resp
}

// buildResponseFromScreen sanitizes a raw screen capture and constructs the
// ProviderResponse. Exposed as a seam so completion/timeout output shaping can
// be unit-tested without scripting the full pane poll loop (F-004).
func (b *InteractivePaneBackend) buildResponseFromScreen(provider, rawScreen string, timedOut bool) *ProviderResponse {
	sanitized := CleanScreenForCrossPollination(rawScreen)
	if sanitized == "" {
		// Fall back to the lighter sanitizer if the cross-pollination cleaner
		// stripped everything (e.g., prompt-only screens).
		sanitized = SanitizeScreenOutput(rawScreen)
	}
	return &ProviderResponse{
		Provider:        provider,
		Output:          sanitized,
		TimedOut:        timedOut,
		EmptyOutput:     sanitized == "",
		ExecutedBackend: paneBackendName,
	}
}
