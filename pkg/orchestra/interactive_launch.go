package orchestra

import (
	"context"
	"os"
	"strings"

	"github.com/insajin/autopus-adk/pkg/terminal"
)

// buildInteractiveLaunchCmd constructs the launch command for interactive mode.
// Uses the binary name plus model/variant flags from PaneArgs, excluding print/pipe flags.
// For claude: "claude --model opus --effort high --dangerously-skip-permissions"
// For opencode: "opencode -m openai/gpt-5.4"
// For gemini: "gemini -m gemini-3.1-pro-preview"
// @AX:NOTE [AUTO] REQ-1 hardcoded provider check (p.Binary == "claude") — update when adding new providers needing permission bypass
func buildInteractiveLaunchCmd(p ProviderConfig) string {
	cmd := p.Binary
	for _, arg := range paneArgs(p) {
		// Skip non-interactive flags that conflict with TUI mode
		if arg == "--print" || arg == "-p" || arg == "--quiet" || arg == "-q" || arg == "run" {
			continue
		}
		cmd += " " + arg
	}
	// REQ-1: Add permission bypass for Claude interactive sessions
	if p.Binary == "claude" {
		if !strings.Contains(cmd, "--dangerously-skip-permissions") {
			cmd += " --dangerously-skip-permissions"
		}
	}
	return cmd
}

// cleanupInteractivePanes stops pipe capture and closes panes.
func cleanupInteractivePanes(term terminal.Terminal, panes []paneInfo) {
	ctx := context.Background()
	for _, pi := range panes {
		_ = term.PipePaneStop(ctx, pi.paneID)
		_ = term.Close(ctx, string(pi.paneID))
		_ = os.Remove(pi.outputFile)
	}
}
