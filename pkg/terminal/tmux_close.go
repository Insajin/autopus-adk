package terminal

import (
	"bytes"
	"context"
	"fmt"
	"strings"
)

// Close kills a global tmux pane ID or the named tmux session.
func (a *TmuxAdapter) Close(_ context.Context, name string) error {
	if strings.HasPrefix(name, "%") {
		if _, err := tmuxPaneTarget("", PaneID(name)); err != nil {
			return fmt.Errorf("tmux: %w", err)
		}
		return closeTmuxPane(name)
	}
	cmd := execCommand("tmux", "kill-session", "-t", name)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("tmux: kill session %q: %w", name, err)
	}
	return nil
}

func closeTmuxPane(target string) error {
	var stderr bytes.Buffer
	cmd := execCommand("tmux", "kill-pane", "-t", target)
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if tmuxPaneAlreadyAbsent(stderr.String(), target) {
			return nil
		}
		return closeCommandError(fmt.Sprintf("tmux: kill pane %q", target), err, stderr.String())
	}
	return nil
}

func tmuxPaneAlreadyAbsent(stderr, paneTarget string) bool {
	return strings.TrimSpace(stderr) == "can't find pane: "+paneTarget
}
