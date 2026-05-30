package cli

import (
	"github.com/insajin/autopus-adk/pkg/terminal"
)

// detectStructuredTerminal returns the detected terminal for structured orchestra
// paths (spec review, orchestra run) so SelectBackend can distinguish pane-capable
// (cmux/tmux) terminals from plain/CI terminals (REQ-006). Centralizing the
// terminal import here keeps spec_review_loop.go and orchestra_run.go from each
// importing the terminal package directly. Backend selection itself runs through
// the specReviewBackendFactory / orchestraRunBackendFactory seams, which default
// to orchestra.SelectBackend.
func detectStructuredTerminal() terminal.Terminal {
	return terminal.DetectTerminal()
}
