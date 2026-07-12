package orchestra

import (
	"context"
	"log"

	"github.com/insajin/autopus-adk/pkg/terminal"
)

// focusFirstProviderPane brings the first provider pane forward after the
// complete fan-out is visible. Focus is a best-effort usability aid: terminals
// without focus support and transient focus failures must not abort the run.
func focusFirstProviderPane(ctx context.Context, term terminal.Terminal, panes []paneInfo) {
	if len(panes) == 0 || term == nil {
		return
	}
	focuser, ok := term.(terminal.PaneFocuser)
	if !ok {
		return
	}
	if err := focuser.FocusPane(ctx, panes[0].paneID); err != nil {
		log.Printf("[orchestra] focus first provider pane failed (non-fatal): %v", err)
	}
}
