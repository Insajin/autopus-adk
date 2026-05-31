package orchestra

import (
	"context"
	"time"

	"github.com/insajin/autopus-adk/pkg/terminal"
)

const readScreenPollTimeout = 2 * time.Second

func readScreenBounded(ctx context.Context, term terminal.Terminal, paneID terminal.PaneID, opts terminal.ReadScreenOpts) (string, error) {
	readCtx, cancel := context.WithTimeout(ctx, readScreenPollTimeout)
	defer cancel()
	return term.ReadScreen(readCtx, paneID, opts)
}
