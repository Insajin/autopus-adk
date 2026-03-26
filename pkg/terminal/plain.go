// Package terminal provides the plain (no-op) terminal adapter.
package terminal

import (
	"context"
	"log"
)

// PlainAdapter implements Terminal as a no-op fallback when no multiplexer is available.
type PlainAdapter struct{}

// Name returns the adapter name.
func (a *PlainAdapter) Name() string { return "plain" }

// CreateWorkspace is a no-op that logs a warning.
func (a *PlainAdapter) CreateWorkspace(_ context.Context, _ string) error {
	log.Println("visual pipeline unavailable: no terminal multiplexer detected")
	return nil
}

// SplitPane is a no-op.
func (a *PlainAdapter) SplitPane(_ context.Context, _ Direction) (PaneID, error) { return "", nil }

// SendCommand is a no-op.
func (a *PlainAdapter) SendCommand(_ context.Context, _ PaneID, _ string) error { return nil }

// Notify is a no-op.
func (a *PlainAdapter) Notify(_ context.Context, _ string) error { return nil }

// ReadScreen is a no-op that returns an empty string.
func (a *PlainAdapter) ReadScreen(_ context.Context, _ PaneID, _ ReadScreenOpts) (string, error) {
	return "", nil
}

// PipePaneStart is a no-op.
func (a *PlainAdapter) PipePaneStart(_ context.Context, _ PaneID, _ string) error { return nil }

// PipePaneStop is a no-op.
func (a *PlainAdapter) PipePaneStop(_ context.Context, _ PaneID) error { return nil }

// Close is a no-op.
func (a *PlainAdapter) Close(_ context.Context, _ string) error { return nil }
