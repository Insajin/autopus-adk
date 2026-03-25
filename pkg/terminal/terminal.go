// Package terminal provides a unified interface for interacting with terminal multiplexers.
package terminal

import "context"

// Direction represents the split direction for pane creation.
type Direction int

const (
	// Horizontal splits the pane horizontally (side by side).
	Horizontal Direction = iota
	// Vertical splits the pane vertically (top and bottom).
	Vertical
)

// PaneID is the identifier for a terminal pane.
type PaneID string

// Terminal is the unified interface for terminal multiplexer adapters.
// @AX:ANCHOR [AUTO] core public API contract — all adapters (cmux, tmux, plain) implement this interface
// @AX:REASON: any method signature change here breaks all three adapters and every CLI handler that calls them; treat as a stable boundary
type Terminal interface {
	// Name returns the terminal adapter name (e.g., "cmux", "tmux", "plain").
	Name() string
	// CreateWorkspace creates a named workspace/session.
	CreateWorkspace(ctx context.Context, name string) error
	// SplitPane splits the current pane in the given direction.
	SplitPane(ctx context.Context, direction Direction) (PaneID, error)
	// SendCommand sends a command string to the specified pane.
	SendCommand(ctx context.Context, paneID PaneID, cmd string) error
	// Notify displays a notification message in the terminal.
	Notify(ctx context.Context, message string) error
	// Close removes the workspace/session.
	Close(ctx context.Context, name string) error
}
