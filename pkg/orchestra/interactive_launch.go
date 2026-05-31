package orchestra

import (
	"context"
	"os"
	"strings"

	"github.com/insajin/autopus-adk/pkg/terminal"
)

// buildInteractiveLaunchCmd constructs the launch command for interactive mode.
// Uses the binary name plus model/variant flags from PaneArgs, excluding print/pipe flags.
// When the provider supports launch-time input, prompt is a short instruction
// pointing at a Markdown prompt file, not the full prompt body.
// @AX:NOTE [AUTO] REQ-1 hardcoded provider check (p.Binary == "claude") — update when adding new providers needing permission bypass
func buildInteractiveLaunchCmd(p ProviderConfig, prompt string) string {
	return buildInteractiveLaunchCmdWithCWD(p, prompt, "")
}

func buildInteractiveLaunchCmdWithCWD(p ProviderConfig, prompt, workingDir string) string {
	cmd := shellQuoteCommandArg(p.Binary)
	for _, arg := range interactiveLaunchArgs(p) {
		// Skip non-interactive flags that conflict with TUI mode.
		// Only skip "run" when NOT using args-based input (args mode needs "run" for opencode).
		if arg == "--print" || arg == "-p" || arg == "--prompt" || arg == "--quiet" || arg == "-q" {
			continue
		}
		if arg == "" {
			continue
		}
		if arg == "run" && p.InteractiveInput != "args" {
			continue
		}
		cmd += " " + shellQuoteCommandArg(arg)
	}
	// REQ-1: Add permission bypass for interactive sessions that support it.
	if p.Binary == "claude" || usesAntigravityPromptInteractive(p) {
		if !strings.Contains(cmd, "--dangerously-skip-permissions") {
			cmd += " --dangerously-skip-permissions"
		}
	}
	// For providers that can take an initial prompt at launch, pass only the
	// short file-backed instruction here. The full prompt stays in the Markdown file.
	// Normalize newlines to spaces to prevent shell quote> continuation prompts
	// when the command is pasted via PTY (set-buffer/paste-buffer).
	if usesAntigravityPromptInteractive(p) && prompt != "" {
		normalized := strings.ReplaceAll(prompt, "\n", " ")
		cmd += " --prompt-interactive " + shellQuote(normalized)
	} else if p.InteractiveInput == "args" && prompt != "" {
		normalized := strings.ReplaceAll(prompt, "\n", " ")
		cmd += " " + shellQuote(normalized)
	}
	if workingDir != "" {
		return "cd " + shellQuote(workingDir) + " && " + cmd
	}
	return cmd
}

func interactiveLaunchArgs(p ProviderConfig) []string {
	if p.Name == "gemini" && p.Binary == "agy" && len(p.PaneArgs) == 0 && p.InteractiveInput != "args" {
		return p.PaneArgs
	}
	return paneArgs(p)
}

func promptDeliveredAtLaunch(p ProviderConfig) bool {
	return p.InteractiveInput == "args" || usesAntigravityPromptInteractive(p)
}

func usesAntigravityPromptInteractive(p ProviderConfig) bool {
	if p.Binary != "agy" && !strings.HasSuffix(p.Binary, "/agy") {
		return false
	}
	name := strings.TrimSpace(p.Name)
	return name == "gemini" || name == "antigravity" || name == "antigravity-cli"
}

func shellQuoteCommandArg(s string) string {
	if s == "" {
		return "''"
	}
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') ||
			r == '-' || r == '_' || r == '.' || r == '/' || r == ':' || r == '=' || r == '@' || r == '%' || r == '+' {
			continue
		}
		return shellQuote(s)
	}
	return s
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
		cleanupPromptFiles(pi.promptFiles)
		_ = os.Remove(pi.responseFile)
	}
}
