package terminal

import (
	"bytes"
	"context"
	"fmt"
	"strings"
)

// Close closes a surface or workspace by ref or stored workspace name.
// If name is a cmux ref (surface:N or pane:N), it uses close-surface.
// If name is a workspace ref (workspace:N), it uses close-workspace.
// Otherwise, it uses the stored workspaceRef from CreateWorkspace.
func (a *CmuxAdapter) Close(_ context.Context, name string) error {
	if isCmuxRef(name) {
		if strings.HasPrefix(name, "surface:") || strings.HasPrefix(name, "pane:") {
			return a.closeSurface(name)
		}
		cmd := execCommand("cmux", "close-workspace", "--workspace", name)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("cmux: close workspace %s: %w", name, err)
		}
		return nil
	}

	ref := a.workspaceRef
	if ref == "" {
		return fmt.Errorf("cmux: close workspace %q: no workspace ref stored (call CreateWorkspace first)", name)
	}
	if err := validateCmuxWorkspaceRef(ref); err != nil {
		return fmt.Errorf("cmux: close workspace %q: %w", name, err)
	}
	cmd := execCommand("cmux", "close-workspace", "--workspace", ref)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("cmux: close workspace %q: %w", name, err)
	}
	return nil
}

func (a *CmuxAdapter) closeSurface(name string) error {
	args, err := a.surfaceCommandArgs("close-surface", PaneID(name))
	if err != nil {
		return fmt.Errorf("cmux: close surface %s: %w", name, err)
	}

	var stderr bytes.Buffer
	cmd := execCommand("cmux", args...)
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if cmuxSurfaceAlreadyAbsent(stderr.String(), name) {
			return nil
		}
		return closeCommandError(fmt.Sprintf("cmux: close surface %s", name), err, stderr.String())
	}
	return nil
}

func cmuxSurfaceAlreadyAbsent(stderr, surfaceRef string) bool {
	message := strings.TrimSpace(stderr)
	return message == "Error: not_found: Surface not found" ||
		message == "Error: Surface ref not found: "+surfaceRef
}
