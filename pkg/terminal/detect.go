// Package terminal provides terminal multiplexer detection.
package terminal

import (
	"os"

	"github.com/insajin/autopus-adk/pkg/detect"
)

// isInstalled is a mockable wrapper around detect.IsInstalled for testing.
// @AX:WARN [AUTO] global state mutation — isInstalled is a mutable package-level variable replaced by tests
// @AX:REASON: concurrent test execution may race on this variable; tests that replace it must restore the original value via defer
var isInstalled = detect.IsInstalled

// DetectTerminal returns the best available terminal adapter.
// Priority: active tmux > cmux > tmux > plain.
// @AX:ANCHOR [AUTO]: high fan-in terminal/backend selection entry point — used by 6 terminal handlers and 5 orchestra paths
// @AX:REASON: active-mux priority affects terminal commands plus orchestra launch, collection, cleanup, and injection
func DetectTerminal() Terminal {
	if os.Getenv("TMUX") != "" && isInstalled("tmux") {
		return &TmuxAdapter{}
	}
	if isInstalled("cmux") {
		return &CmuxAdapter{}
	}
	if isInstalled("tmux") {
		return &TmuxAdapter{}
	}
	return &PlainAdapter{}
}
