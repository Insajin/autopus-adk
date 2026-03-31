// Package terminal provides the tmux terminal adapter.
package terminal

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// TmuxAdapter implements Terminal using the tmux terminal multiplexer.
type TmuxAdapter struct {
	session string
}

// Name returns the adapter name.
func (a *TmuxAdapter) Name() string { return "tmux" }

// CreateWorkspace creates a new tmux session or window.
// When running inside an existing tmux session (TMUX env set), creates a new window
// instead to avoid nested session errors.
func (a *TmuxAdapter) CreateWorkspace(_ context.Context, name string) error {
	if err := validateWorkspaceName(name); err != nil {
		return fmt.Errorf("tmux: %w", err)
	}
	a.session = name
	cmd := buildTmuxCreateCmd(name)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("tmux: create workspace %q: %w", name, err)
	}
	return nil
}

// buildTmuxCreateCmd returns the appropriate tmux command for workspace creation.
// Uses new-window when already inside a tmux session to avoid nesting errors.
func buildTmuxCreateCmd(name string) *exec.Cmd {
	if os.Getenv("TMUX") != "" {
		return execCommand("tmux", "new-window", "-t", name)
	}
	return execCommand("tmux", "new-session", "-d", "-s", name)
}

// SplitPane splits the current pane horizontally or vertically.
func (a *TmuxAdapter) SplitPane(_ context.Context, dir Direction) (PaneID, error) {
	flag := "-h"
	if dir == Vertical {
		flag = "-v"
	}
	cmd := execCommand("tmux", "split-window", "-t", a.session, flag)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("tmux: split pane: %w", err)
	}
	return PaneID(strings.TrimSpace(string(out))), nil
}

// SendCommand sends a shell command to the specified pane.
func (a *TmuxAdapter) SendCommand(_ context.Context, paneID PaneID, command string) error {
	if err := validatePaneID(paneID); err != nil {
		return fmt.Errorf("tmux: %w", err)
	}
	target := a.session + ":" + string(paneID)
	cmd := execCommand("tmux", "send-keys", "-t", target, command, "Enter")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("tmux: send command to pane %s: %w", paneID, err)
	}
	return nil
}

// Notify displays a message in the tmux status bar.
func (a *TmuxAdapter) Notify(_ context.Context, message string) error {
	cmd := execCommand("tmux", "display-message", message)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("tmux: notify: %w", err)
	}
	return nil
}

// ReadScreen reads pane content via tmux capture-pane.
func (a *TmuxAdapter) ReadScreen(_ context.Context, paneID PaneID, opts ReadScreenOpts) (string, error) {
	if err := validatePaneID(paneID); err != nil {
		return "", fmt.Errorf("tmux: %w", err)
	}
	target := a.session + ":" + string(paneID)
	args := []string{"capture-pane", "-t", target, "-p"}
	if opts.ScrollbackLines > 0 {
		args = append(args, "-S", fmt.Sprintf("-%d", opts.ScrollbackLines))
	}
	cmd := execCommand("tmux", args...)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("tmux: capture-pane %s: %w", paneID, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// PipePaneStart starts streaming pane output to a file via tmux pipe-pane.
func (a *TmuxAdapter) PipePaneStart(_ context.Context, paneID PaneID, outputFile string) error {
	if err := validatePaneID(paneID); err != nil {
		return fmt.Errorf("tmux: %w", err)
	}
	target := a.session + ":" + string(paneID)
	// SEC-007: shell-escape outputFile to prevent command injection via malicious paths
	cmd := execCommand("tmux", "pipe-pane", "-t", target, "-O", "cat >> '"+strings.ReplaceAll(outputFile, "'", "'\\''")+"'")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("tmux: pipe-pane start %s: %w", paneID, err)
	}
	return nil
}

// PipePaneStop stops pipe-pane output streaming via no-arg pipe-pane.
func (a *TmuxAdapter) PipePaneStop(_ context.Context, paneID PaneID) error {
	if err := validatePaneID(paneID); err != nil {
		return fmt.Errorf("tmux: %w", err)
	}
	target := a.session + ":" + string(paneID)
	cmd := execCommand("tmux", "pipe-pane", "-t", target)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("tmux: pipe-pane stop %s: %w", paneID, err)
	}
	return nil
}

// SendLongText sends text to a pane, using load-buffer/paste-buffer for text >= 500 bytes
// to avoid tmux send-keys truncation. Does not send Enter — callers handle that.
func (a *TmuxAdapter) SendLongText(_ context.Context, paneID PaneID, text string) error {
	if err := validatePaneID(paneID); err != nil {
		return fmt.Errorf("tmux: %w", err)
	}
	target := a.session + ":" + string(paneID)

	// Short text: use send-keys without Enter
	if len(text) < 500 {
		cmd := execCommand("tmux", "send-keys", "-t", target, text)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("tmux: send-keys to pane %s: %w", paneID, err)
		}
		return nil
	}

	// Long text: write to temp file, load-buffer, paste-buffer, cleanup
	tmpFile, err := os.CreateTemp("", "autopus-tmux-buf-*")
	if err != nil {
		return fmt.Errorf("tmux: create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := tmpFile.WriteString(text); err != nil {
		tmpFile.Close()
		return fmt.Errorf("tmux: write temp file: %w", err)
	}
	tmpFile.Close()

	loadCmd := execCommand("tmux", "load-buffer", tmpPath)
	if err := loadCmd.Run(); err != nil {
		return fmt.Errorf("tmux: load-buffer: %w", err)
	}

	pasteCmd := execCommand("tmux", "paste-buffer", "-t", target)
	if err := pasteCmd.Run(); err != nil {
		return fmt.Errorf("tmux: paste-buffer to pane %s: %w", paneID, err)
	}

	return nil
}

// Close kills the named tmux session.
func (a *TmuxAdapter) Close(_ context.Context, name string) error {
	cmd := execCommand("tmux", "kill-session", "-t", name)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("tmux: kill session %q: %w", name, err)
	}
	return nil
}
