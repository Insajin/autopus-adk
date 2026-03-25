// Package terminal provides the cmux terminal adapter.
package terminal

import (
	"context"
	"fmt"
	"strings"
)

// CmuxAdapter implements Terminal using the cmux terminal multiplexer.
type CmuxAdapter struct {
	workspace string
}

// Name returns the adapter name.
func (a *CmuxAdapter) Name() string { return "cmux" }

// CreateWorkspace creates a cmux workspace with the given name.
func (a *CmuxAdapter) CreateWorkspace(_ context.Context, name string) error {
	if err := validateWorkspaceName(name); err != nil {
		return fmt.Errorf("cmux: %w", err)
	}
	a.workspace = name
	cmd := execCommand("cmux", "workspace", "create", name)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("cmux: create workspace %q: %w", name, err)
	}
	return nil
}

// SplitPane splits the current pane in the given direction.
func (a *CmuxAdapter) SplitPane(_ context.Context, dir Direction) (PaneID, error) {
	flag := "h"
	if dir == Vertical {
		flag = "v"
	}
	cmd := execCommand("cmux", "pane", "split", "--direction", flag)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("cmux: split pane: %w", err)
	}
	return PaneID(strings.TrimSpace(string(out))), nil
}

// SendCommand sends a command string to the specified pane.
func (a *CmuxAdapter) SendCommand(_ context.Context, paneID PaneID, command string) error {
	if err := validatePaneID(paneID); err != nil {
		return fmt.Errorf("cmux: %w", err)
	}
	cmd := execCommand("cmux", "send-keys", string(paneID), command)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("cmux: send command to pane %s: %w", paneID, err)
	}
	return nil
}

// Notify sends a notification message via cmux.
func (a *CmuxAdapter) Notify(_ context.Context, message string) error {
	cmd := execCommand("cmux", "notify", message)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("cmux: notify: %w", err)
	}
	return nil
}

// Close removes the cmux workspace with the given name.
func (a *CmuxAdapter) Close(_ context.Context, name string) error {
	if err := validateWorkspaceName(name); err != nil {
		return fmt.Errorf("cmux: %w", err)
	}
	cmd := execCommand("cmux", "workspace", "remove", name)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("cmux: remove workspace %q: %w", name, err)
	}
	return nil
}
