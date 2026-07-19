package orchestra

import (
	"context"
	"strings"
	"time"

	"github.com/insajin/autopus-adk/pkg/terminal"
)

// finalReadTimeout bounds the scrollback read after completion/timeout so the
// backend never blocks indefinitely while harvesting the screen (REQ-011).
const finalReadTimeout = 5 * time.Second

// collectResponse prefers the file-backed response contract, then a structured
// hook result, and finally pane scrollback. timedOut marks deterministic timeout
// results only when both structured collection paths are unavailable.
func (b *InteractivePaneBackend) collectResponse(ctx context.Context, req ProviderRequest, pi paneInfo, timedOut bool, hookSessions ...*HookSession) *ProviderResponse {
	if output, ok := readResponseFile(pi.responseFile); ok {
		return markUnavailableUsage(&ProviderResponse{
			Provider:        req.Provider,
			Output:          output,
			TimedOut:        false,
			EmptyOutput:     false,
			ExecutedBackend: paneBackendName,
		}, usageSourcePane, usageReasonPane)
	}
	if len(hookSessions) > 0 && hookSessions[0] != nil {
		if result, err := hookSessions[0].ReadResultRound(req.Provider, req.Round); err == nil && strings.TrimSpace(result.Output) != "" {
			response := HookResultToProviderResponse(*result, req.Provider, 0)
			response.TimedOut = false
			response.EmptyOutput = false
			response.ExecutedBackend = paneBackendName
			return &response
		}
	}
	// After an authoritative completion signal or at the deadline, a reviewer
	// that printed its answer to the terminal — the
	// fallback that promptFileInstruction explicitly authorizes ("If you cannot
	// write the response file, print the final answer in the terminal as
	// fallback") — would otherwise be discarded. Harvest the screen so the
	// terminal-fallback answer is preserved instead of lost (issue #59). This is
	// safe after completion because the pane is no longer mid-render. The response
	// stays TimedOut only when the per-provider budget was exceeded.
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
	return markUnavailableUsage(&ProviderResponse{
		Provider:        provider,
		Output:          sanitized,
		TimedOut:        timedOut,
		EmptyOutput:     sanitized == "",
		ExecutedBackend: paneBackendName,
	}, usageSourcePane, usageReasonPane)
}
