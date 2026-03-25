// Package terminal provides input validation for terminal adapter parameters.
package terminal

import (
	"fmt"
	"regexp"
)

// validName matches safe workspace/session names: alphanumeric, hyphens, underscores, dots.
var validName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]*$`)

// validateWorkspaceName checks that name is safe for use as a tmux session or cmux workspace name.
func validateWorkspaceName(name string) error {
	if name == "" {
		return fmt.Errorf("workspace name must not be empty")
	}
	if len(name) > 256 {
		return fmt.Errorf("workspace name too long: %d characters (max 256)", len(name))
	}
	if !validName.MatchString(name) {
		return fmt.Errorf("invalid workspace name %q: must be alphanumeric with hyphens, underscores, or dots", name)
	}
	return nil
}

// validatePaneID checks that paneID is safe for use as a tmux/cmux pane target.
func validatePaneID(id PaneID) error {
	if id == "" {
		return fmt.Errorf("pane ID must not be empty")
	}
	if !validName.MatchString(string(id)) {
		return fmt.Errorf("invalid pane ID %q: must be alphanumeric with hyphens, underscores, or dots", id)
	}
	return nil
}
