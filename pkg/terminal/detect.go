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
// Priority: active tmux > active cmux > plain.
// @AX:ANCHOR [AUTO]: high fan-in terminal/backend selection entry point — used by 6 terminal handlers and 5 orchestra paths
// @AX:REASON: active-mux priority affects terminal commands plus orchestra launch, collection, cleanup, and injection
func DetectTerminal() Terminal {
	if os.Getenv("TMUX") != "" && isInstalled("tmux") {
		return &TmuxAdapter{}
	}
	if hasActiveCmuxContext() && isInstalled("cmux") {
		return &CmuxAdapter{}
	}
	return &PlainAdapter{}
}

// DetectInstalledTerminal returns the best installed adapter for explicit
// workspace management commands that do not require an inherited pane context.
// Priority: cmux > tmux > plain.
func DetectInstalledTerminal() Terminal {
	if isInstalled("cmux") {
		return &CmuxAdapter{}
	}
	if isInstalled("tmux") {
		return &TmuxAdapter{}
	}
	return &PlainAdapter{}
}

// hasActiveCmuxContext reports whether cmux exported an active runtime marker.
func hasActiveCmuxContext() bool {
	for _, key := range []string{
		"CMUX_SOCKET_PATH",
		"CMUX_WORKSPACE_ID",
		"CMUX_SURFACE_ID",
		"CMUX_PANE_ID",
	} {
		if os.Getenv(key) != "" {
			return true
		}
	}
	return false
}
