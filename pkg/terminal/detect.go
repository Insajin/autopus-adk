// Package terminal provides terminal multiplexer detection.
package terminal

import "github.com/insajin/autopus-adk/pkg/detect"

// isInstalled is a mockable wrapper around detect.IsInstalled for testing.
// @AX:WARN [AUTO] global state mutation — isInstalled is a mutable package-level variable replaced by tests
// @AX:REASON: concurrent test execution may race on this variable; tests that replace it must restore the original value via defer
var isInstalled = detect.IsInstalled

// DetectTerminal returns the best available terminal adapter.
// Priority: cmux > tmux > plain.
// @AX:ANCHOR [AUTO] high fan-in entry point — called by 6 CLI command handlers in terminal_cmd.go
// @AX:REASON: changes to detection priority (cmux > tmux > plain) affect all terminal subcommands; coordinate with terminal_cmd.go callers before modifying
func DetectTerminal() Terminal {
	if isInstalled("cmux") {
		return &CmuxAdapter{}
	}
	if isInstalled("tmux") {
		return &TmuxAdapter{}
	}
	return &PlainAdapter{}
}
