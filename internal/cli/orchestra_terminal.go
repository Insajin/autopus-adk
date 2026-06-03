package cli

import (
	"os"

	"golang.org/x/term"

	"github.com/insajin/autopus-adk/pkg/terminal"
)

// detectStructuredTerminal returns the terminal that backend selection uses for
// structured orchestra paths (spec review, orchestra run). SelectBackend uses the
// returned terminal to choose the interactive pane backend (cmux/tmux) or the
// headless subprocess backend. Backend selection itself runs through the
// specReviewBackendFactory / orchestraRunBackendFactory seams, which default to
// orchestra.SelectBackend.
//
// Pane execution drives an interactive terminal multiplexer and spawns interactive
// provider CLIs that must reach a ready state and emit a completion sentinel. In a
// non-interactive context — piped stdio, CI, or a nested agent automation such as
// Claude Code — those panes never complete and every provider times out (0/N).
// CMUX_*/TMUX env vars are inherited into such nested processes, so the detected
// terminal name alone (cmux/tmux) is not sufficient. When the process is not
// attached to an interactive TTY, report a plain terminal so SelectBackend falls
// back to the subprocess backend, which works without a TTY.
func detectStructuredTerminal() terminal.Terminal {
	if !paneInteractiveContext(
		os.Getenv("CLAUDECODE"),
		os.Getenv("CI"),
		term.IsTerminal(int(os.Stdin.Fd())),
		term.IsTerminal(int(os.Stdout.Fd())),
	) {
		return &terminal.PlainAdapter{}
	}
	return terminal.DetectTerminal()
}

// paneInteractiveContext reports whether interactive terminal panes can be driven.
// It returns false in nested agent automation (CLAUDECODE), CI, or whenever stdio
// is not an interactive TTY — the contexts where spawned provider panes fail to
// complete and time out 0/N. Kept as a pure function so the decision is
// unit-testable without manipulating real file descriptors.
func paneInteractiveContext(claudeCode, ci string, stdinTTY, stdoutTTY bool) bool {
	if claudeCode != "" || ci != "" {
		return false
	}
	return stdinTTY && stdoutTTY
}
