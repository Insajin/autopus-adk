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
	if requiresReviewerResponseFile(req, pi) {
		return &ProviderResponse{
			Provider:        req.Provider,
			TimedOut:        timedOut,
			EmptyOutput:     true,
			Error:           reviewerResponseFileMissingError(timedOut),
			ExecutedBackend: paneBackendName,
		}
	}

	// Use a fresh, bounded context for the final read: the original ctx may be
	// cancelled after a completion timeout (mirrors interactive_collect.go).
	readCtx, cancel := context.WithTimeout(context.Background(), finalReadTimeout)
	defer cancel()
	screen, _ := b.cfg.Terminal.ReadScreen(readCtx, pi.paneID, terminal.ReadScreenOpts{
		Scrollback:      true,
		ScrollbackLines: scrollbackDepth(b.cfg.ScrollbackLines),
	})
	return b.buildResponseFromScreen(req.Provider, screen, timedOut)
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
