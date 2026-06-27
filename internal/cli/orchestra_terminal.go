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
// non-interactive context — piped stdio, CI, or nested agent automation such as
// Claude Code/Codex — those panes never complete and every provider times out (0/N).
// CMUX_*/TMUX env vars are inherited into such nested processes, so the detected
// terminal name alone (cmux/tmux) is not sufficient.
//
// REQ-005/REQ-008 (nested-agent relaxation): when CLAUDECODE or CODEX is set and
// no CI env is present, pane execution is permitted if both the hook subsystem is
// available (isHookModeAvailable) AND a multiplexer (cmux or tmux) is installed.
// This allows agent runtimes to drive pane-based orchestra without requiring an
// interactive TTY. The floor is preserved: if either condition is false the result
// falls back to plain/subprocess just as before.
func detectStructuredTerminal() terminal.Terminal {
	hookAvail := isHookModeAvailable()
	detected := terminal.DetectTerminal()
	muxInstalled := detected.Name() != "plain"

	if !paneInteractiveContext(
		os.Getenv("CLAUDECODE"),
		os.Getenv("CODEX"),
		os.Getenv("CI"),
		term.IsTerminal(int(os.Stdin.Fd())),
		term.IsTerminal(int(os.Stdout.Fd())),
		hookAvail,
		muxInstalled,
	) {
		return &terminal.PlainAdapter{}
	}
	return detected
}

// paneInteractiveContext reports whether interactive terminal panes can be driven.
//
// Truth-table (REQ-005/REQ-008):
//
//	CI != ""                                      → false  (CI always forces subprocess floor)
//	CI == "" && (claudeCode != "" || codex != "") → hookAvailable && muxInstalled
//	CI == "" && no nested agent env               → stdinTTY && stdoutTTY  (normal interactive path)
//
// hookAvailable: isHookModeAvailable() (project-local OR user-global hook config).
// muxInstalled:  DetectTerminal().Name() != "plain" (cmux OR tmux present).
//
// Kept as a pure function so the decision is unit-testable without manipulating
// real file descriptors or environment variables.
func paneInteractiveContext(claudeCode, codex, ci string, stdinTTY, stdoutTTY bool, hookAvailable, muxInstalled bool) bool {
	// CI always forces the subprocess floor regardless of nested agent runtime.
	if ci != "" {
		return false
	}
	// Nested-agent relaxation: hook + mux must both be present.
	if claudeCode != "" || codex != "" {
		return hookAvailable && muxInstalled
	}
	// Normal interactive context: both stdio file descriptors must be TTYs.
	return stdinTTY && stdoutTTY
}
