package terminal

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
)

var canonicalCmuxWorkspaceUUID = regexp.MustCompile(
	`^[0-9A-Fa-f]{8}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{12}$`,
)

// WorkspaceContextProvider exposes optional workspace affinity without changing
// the Terminal interface implemented by tmux and plain adapters.
type WorkspaceContextProvider interface {
	WorkspaceRef() (string, error)
	WithWorkspaceRef(string) (Terminal, error)
}

var _ WorkspaceContextProvider = (*CmuxAdapter)(nil)

// NewCmuxAdapterWithWorkspace restores a cmux adapter for one persisted workspace.
func NewCmuxAdapterWithWorkspace(workspaceRef string) (*CmuxAdapter, error) {
	if err := validateCmuxWorkspaceRef(workspaceRef); err != nil {
		return nil, fmt.Errorf("cmux: restore workspace context: %w", err)
	}
	return &CmuxAdapter{workspaceRef: workspaceRef}, nil
}

// WorkspaceRef returns the validated stored workspace, or the inherited cmux
// workspace when no stored override exists.
func (a *CmuxAdapter) WorkspaceRef() (string, error) {
	if a.workspaceRef != "" {
		if err := validateCmuxWorkspaceRef(a.workspaceRef); err != nil {
			return "", fmt.Errorf("invalid stored workspace context: %w", err)
		}
		return a.workspaceRef, nil
	}
	workspaceRef := os.Getenv("CMUX_WORKSPACE_ID")
	if workspaceRef == "" {
		return "", fmt.Errorf("workspace context is unavailable: CMUX_WORKSPACE_ID is empty")
	}
	if err := validateCmuxWorkspaceRef(workspaceRef); err != nil {
		return "", fmt.Errorf("invalid CMUX_WORKSPACE_ID: %w", err)
	}
	return workspaceRef, nil
}

// WithWorkspaceRef returns a validated clone, leaving the original adapter's
// workspace affinity unchanged.
func (*CmuxAdapter) WithWorkspaceRef(workspaceRef string) (Terminal, error) {
	return NewCmuxAdapterWithWorkspace(workspaceRef)
}

func validateCmuxWorkspaceRef(workspaceRef string) error {
	if strings.HasPrefix(workspaceRef, "workspace:") && validCmuxRef.MatchString(workspaceRef) {
		return nil
	}
	if canonicalCmuxWorkspaceUUID.MatchString(workspaceRef) {
		return nil
	}
	return fmt.Errorf("invalid workspace ref %q: want workspace:N or canonical UUID", workspaceRef)
}

func (a *CmuxAdapter) workspaceCommandArgs(command string, trailing ...string) ([]string, error) {
	workspaceRef, err := a.WorkspaceRef()
	if err != nil {
		return nil, err
	}
	args := []string{command, "--workspace", workspaceRef}
	return append(args, trailing...), nil
}

func (a *CmuxAdapter) surfaceCommandArgs(
	command string,
	paneID PaneID,
	trailing ...string,
) ([]string, error) {
	workspaceRef, err := a.WorkspaceRef()
	if err != nil {
		return nil, err
	}
	args := []string{command, "--workspace", workspaceRef, "--surface", string(paneID)}
	return append(args, trailing...), nil
}

// Notify sends a notification message in the effective cmux workspace.
func (a *CmuxAdapter) Notify(_ context.Context, message string) error {
	args, err := a.workspaceCommandArgs("notify", "--title", message)
	if err != nil {
		return fmt.Errorf("cmux: notify: %w", err)
	}
	cmd := execCommand("cmux", args...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("cmux: notify: %w", err)
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
