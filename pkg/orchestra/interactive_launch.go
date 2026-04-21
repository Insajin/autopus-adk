package orchestra

import (
	"context"
	"os"
	"strings"

	"github.com/insajin/autopus-adk/pkg/terminal"
)

// buildInteractiveLaunchCmd constructs the launch command for interactive mode.
// Uses the binary name plus model/variant flags from PaneArgs, excluding print/pipe flags.
// When InteractiveInput == "args" and prompt is non-empty, the prompt is appended as the
// last CLI argument (non-interactive run mode, e.g., opencode run -m model "prompt").
// @AX:NOTE [AUTO] REQ-1 hardcoded provider check (p.Binary == "claude") — update when adding new providers needing permission bypass
func buildInteractiveLaunchCmd(p ProviderConfig, prompt string) string {
	return buildInteractiveLaunchCmdWithCWD(p, prompt, "")
}

func buildInteractiveLaunchCmdWithCWD(p ProviderConfig, prompt, workingDir string) string {
	cmd := p.Binary
	for _, arg := range paneArgs(p) {
		// Skip non-interactive flags that conflict with TUI mode.
		// Only skip "run" when NOT using args-based input (args mode needs "run" for opencode).
		if arg == "--print" || arg == "-p" || arg == "--prompt" || arg == "--quiet" || arg == "-q" {
			continue
		}
		if arg == "run" && p.InteractiveInput != "args" {
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
	// For args-based providers, append the prompt as the final CLI argument.
	// Normalize newlines to spaces to prevent shell quote> continuation prompts
	// when the command is pasted via PTY (set-buffer/paste-buffer).
	if p.InteractiveInput == "args" && prompt != "" {
		normalized := strings.ReplaceAll(prompt, "\n", " ")
		cmd += " " + shellQuote(normalized)
	}
	if workingDir != "" {
		return "cd " + shellQuote(workingDir) + " && " + cmd
	}
	return cmd
}

// shellQuote wraps a string in single quotes, escaping any embedded single quotes.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
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
