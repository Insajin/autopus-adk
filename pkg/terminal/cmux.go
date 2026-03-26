// Package terminal provides the cmux terminal adapter.
package terminal

import (
	"context"
	"fmt"
	"strings"
)

// CmuxAdapter implements Terminal using the cmux terminal multiplexer.
type CmuxAdapter struct {
	workspaceRef string // e.g. "workspace:1" — stored from CreateWorkspace or env
}

// Name returns the adapter name.
func (a *CmuxAdapter) Name() string { return "cmux" }

// CreateWorkspace creates a new cmux workspace and renames it to the given name.
// It stores the workspace ref internally for use by Close.
func (a *CmuxAdapter) CreateWorkspace(_ context.Context, name string) error {
	if err := validateWorkspaceName(name); err != nil {
		return fmt.Errorf("cmux: %w", err)
	}
	cmd := execCommand("cmux", "new-workspace")
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("cmux: create workspace: %w", err)
	}
	a.workspaceRef = parseCmuxRef(string(out), "workspace")
	if a.workspaceRef == "" {
		return fmt.Errorf("cmux: create workspace: failed to parse workspace ref from output %q", string(out))
	}
	renameCmd := execCommand("cmux", "rename-workspace", "--workspace", a.workspaceRef, name)
	if err := renameCmd.Run(); err != nil {
		return fmt.Errorf("cmux: rename workspace %q: %w", name, err)
	}
	return nil
}

// SplitPane creates a new split pane in the given direction and returns its surface ref.
func (a *CmuxAdapter) SplitPane(_ context.Context, dir Direction) (PaneID, error) {
	direction := "right"
	if dir == Vertical {
		direction = "down"
	}
	cmd := execCommand("cmux", "new-split", direction)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("cmux: split pane: %w", err)
	}
	ref := parseCmuxRef(string(out), "surface")
	if ref == "" {
		return "", fmt.Errorf("cmux: split pane: failed to parse surface ref from output %q", string(out))
	}
	return PaneID(ref), nil
}

// SendCommand sends a command string to the specified pane via --surface flag.
func (a *CmuxAdapter) SendCommand(_ context.Context, paneID PaneID, command string) error {
	if err := validatePaneID(paneID); err != nil {
		return fmt.Errorf("cmux: %w", err)
	}
	cmd := execCommand("cmux", "send", "--surface", string(paneID), command)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("cmux: send command to pane %s: %w", paneID, err)
	}
	return nil
}

// Notify sends a notification message via cmux notify --title.
func (a *CmuxAdapter) Notify(_ context.Context, message string) error {
	cmd := execCommand("cmux", "notify", "--title", message)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("cmux: notify: %w", err)
	}
	return nil
}

// Close closes a surface or workspace by ref or stored workspace name.
// If name is a cmux ref (surface:N or pane:N), uses close-surface.
// If name is a workspace ref (workspace:N), uses close-workspace.
// Otherwise, uses the stored workspaceRef from CreateWorkspace.
func (a *CmuxAdapter) Close(_ context.Context, name string) error {
	if isCmuxRef(name) {
		if strings.HasPrefix(name, "surface:") || strings.HasPrefix(name, "pane:") {
			cmd := execCommand("cmux", "close-surface", "--surface", name)
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("cmux: close surface %s: %w", name, err)
			}
			return nil
		}
		cmd := execCommand("cmux", "close-workspace", "--workspace", name)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("cmux: close workspace %s: %w", name, err)
		}
		return nil
	}
	// Name-based: use stored workspace ref if available.
	ref := a.workspaceRef
	if ref == "" {
		return fmt.Errorf("cmux: close workspace %q: no workspace ref stored (call CreateWorkspace first)", name)
	}
	cmd := execCommand("cmux", "close-workspace", "--workspace", ref)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("cmux: close workspace %q: %w", name, err)
	}
	return nil
}

// ReadScreen reads pane content via cmux read-screen.
func (a *CmuxAdapter) ReadScreen(_ context.Context, paneID PaneID, opts ReadScreenOpts) (string, error) {
	if err := validatePaneID(paneID); err != nil {
		return "", fmt.Errorf("cmux: %w", err)
	}
	args := []string{"read-screen", "--surface", string(paneID)}
	if opts.Scrollback {
		args = append(args, "--scrollback")
	}
	if opts.Lines > 0 {
		args = append(args, "--lines", fmt.Sprintf("%d", opts.Lines))
	}
	cmd := execCommand("cmux", args...)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("cmux: read-screen pane %s: %w", paneID, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// PipePaneStart starts streaming pane output to a file via cmux pipe-pane.
func (a *CmuxAdapter) PipePaneStart(_ context.Context, paneID PaneID, outputFile string) error {
	if err := validatePaneID(paneID); err != nil {
		return fmt.Errorf("cmux: %w", err)
	}
	// SEC-007: shell-escape outputFile to prevent command injection via malicious paths
	cmd := execCommand("cmux", "pipe-pane", "--surface", string(paneID), "--command", "cat >> '"+strings.ReplaceAll(outputFile, "'", "'\\''")+"'")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("cmux: pipe-pane start pane %s: %w", paneID, err)
	}
	return nil
}

// PipePaneStop stops pipe-pane output streaming via empty command.
func (a *CmuxAdapter) PipePaneStop(_ context.Context, paneID PaneID) error {
	if err := validatePaneID(paneID); err != nil {
		return fmt.Errorf("cmux: %w", err)
	}
	cmd := execCommand("cmux", "pipe-pane", "--surface", string(paneID), "--command", "")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("cmux: pipe-pane stop pane %s: %w", paneID, err)
	}
	return nil
}

// parseCmuxRef extracts a typed ref (e.g., "surface:7") from cmux CLI output.
// Output format: "OK surface:7 workspace:1" or "OK workspace:5".
func parseCmuxRef(output, refType string) string {
	for field := range strings.FieldsSeq(strings.TrimSpace(output)) {
		if strings.HasPrefix(field, refType+":") {
			return field
		}
	}
	return ""
}

// isCmuxRef reports whether s is a cmux reference (type:number format).
func isCmuxRef(s string) bool {
	return validCmuxRef.MatchString(s)
}
